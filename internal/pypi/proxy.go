// Package pypi is a pull-through proxy for the PyPI Simple API (PEP 503, PEP 691).
package pypi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexanderSlokov/Ginnungagap/internal/auth"
	"github.com/AlexanderSlokov/Ginnungagap/internal/blob"
	"github.com/AlexanderSlokov/Ginnungagap/internal/policy"
	"github.com/AlexanderSlokov/Ginnungagap/internal/store"
)

// indexTTL is the time that the proxy uses a project page before it reads the upstream again.
const indexTTL = 5 * time.Minute

const maxMetadataSize = 16 << 20

type Proxy struct {
	DB     *pgxpool.Pool
	Blob   *blob.Store
	Auth   *auth.Auth
	Client *http.Client
}

func (p *Proxy) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /pypi/{repo}/simple/{$}", p.root)
	mux.HandleFunc("GET /pypi/{repo}/simple/{project}/{$}", p.project)
	mux.HandleFunc("GET /pypi/{repo}/simple/{project}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /pypi/{repo}/files/{id}/{filename}", p.download)
}

// File is one row of the file table, with its project.
type File struct {
	ID             int64
	Filename       string
	Version        string
	SHA256         string
	Size           *int64
	UpstreamURL    string
	UploadTime     *time.Time
	FirstSeen      time.Time
	RequiresPython *string
	Yanked         *string
	MetadataSHA256 *string
	RemovedAt      *time.Time
	Cached         bool
	LastPull       *time.Time
	Pinned         bool
	MarkedAt       *time.Time
	ProjectRepoID  int64
	Project        string
}

// The upload time can come from another row with the same SHA-256 (ADR-0008, section 2.2):
// a mirror (for example, the PyTorch index) often gives no upload time for a file that PyPI has.
const fileColumns = `f.id, f.filename, f.version, f.sha256, f.size, f.upstream_url,
	coalesce(f.upload_time, (SELECT min(o.upload_time) FROM file o WHERE o.sha256 = f.sha256)), f.first_seen,
	f.requires_python, f.yanked, f.metadata_sha256, f.removed_upstream_at, f.cached, f.last_pull, f.pinned,
	f.gc_marked_at, p.repository_id, p.name
	FROM file f JOIN project p ON p.id = f.project_id`

func scanFiles(rows pgx.Rows) ([]File, error) {
	return pgx.CollectRows(rows, pgx.RowToStructByPos[File])
}

func (f File) policyFile() policy.File {
	return policy.File{Version: f.Version, UploadTime: f.UploadTime, FirstSeen: f.FirstSeen, RemovedUpstream: f.RemovedAt != nil}
}

// LoadFiles returns the files of a project, ordered by file name.
func LoadFiles(ctx context.Context, db *pgxpool.Pool, projectID int64) ([]File, error) {
	rows, _ := db.Query(ctx, `SELECT `+fileColumns+` WHERE f.project_id = $1 ORDER BY f.filename`, projectID)
	return scanFiles(rows)
}

func loadFile(ctx context.Context, db *pgxpool.Pool, id int64) (File, error) {
	rows, _ := db.Query(ctx, `SELECT `+fileColumns+` WHERE f.id = $1`, id)
	files, err := scanFiles(rows)
	if err != nil {
		return File{}, err
	}
	if len(files) == 0 {
		return File{}, store.ErrNotFound
	}
	return files[0], nil
}

// Statuses applies the policies of repo and of the repository that supplies the files.
func Statuses(ctx context.Context, db *pgxpool.Pool, repo store.Repository, project string, files []File, now time.Time) ([]policy.Status, error) {
	ids := []int64{repo.ID}
	for _, f := range files {
		if !slices.Contains(ids, f.ProjectRepoID) {
			ids = append(ids, f.ProjectRepoID)
		}
	}
	rules, err := policy.Load(ctx, db, project, ids...)
	if err != nil {
		return nil, err
	}
	cooldown := time.Duration(repo.CooldownDays) * 24 * time.Hour
	out := make([]policy.Status, len(files))
	for i, f := range files {
		out[i] = policy.Evaluate(rules, f.policyFile(), cooldown, now)
	}
	return out, nil
}

// SourceProject returns the project row that holds the files of name in repo, from the
// database only. For a group, it follows the route of ADR-0008.
func SourceProject(ctx context.Context, db *pgxpool.Pool, repo store.Repository, name string) (int64, error) {
	q := `SELECT id FROM project WHERE repository_id = $1 AND name = $2`
	if repo.Type == "group" {
		q = `SELECT s.id FROM project g JOIN project s ON s.repository_id = g.upstream_id AND s.name = g.name
			WHERE g.repository_id = $1 AND g.name = $2`
	}
	var id int64
	err := db.QueryRow(ctx, q, repo.ID, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, store.ErrNotFound
	}
	return id, err
}

func (p *Proxy) repo(w http.ResponseWriter, r *http.Request, min store.Role) (store.Repository, *auth.Principal, bool) {
	repo, err := store.GetRepository(r.Context(), p.DB, r.PathValue("repo"))
	if err != nil || repo.Format != "pypi" {
		http.NotFound(w, r)
		return repo, nil, false
	}
	who, _, ok := p.Auth.Require(w, r, repo.ID, min)
	return repo, who, ok
}

// refresh reads the upstream project page of a proxy repository into the database,
// if the stored page is older than indexTTL. It returns the project ID.
func (p *Proxy) refresh(ctx context.Context, repo store.Repository, name string) (int64, error) {
	var id int64
	var refreshed *time.Time
	err := p.DB.QueryRow(ctx, `SELECT id, refreshed_at FROM project WHERE repository_id = $1 AND name = $2`, repo.ID, name).Scan(&id, &refreshed)
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	if exists && refreshed != nil && time.Since(*refreshed) < indexTTL {
		return id, nil
	}

	files, err := FetchProject(ctx, p.Client, repo.UpstreamURL, name)
	if errors.Is(err, errNotFound) {
		if exists {
			// The upstream removed the whole project. Do not serve its files any more.
			if _, err := p.DB.Exec(ctx, `UPDATE file SET removed_upstream_at = now()
				WHERE project_id = $1 AND removed_upstream_at IS NULL`, id); err != nil {
				return 0, err
			}
		}
		return 0, store.ErrNotFound
	}
	if err != nil {
		if exists {
			slog.Warn("upstream is not available, using the stored index", "repo", repo.Name, "project", name, "err", err)
			return id, nil
		}
		return 0, err
	}

	err = pgx.BeginFunc(ctx, p.DB, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO project (repository_id, name, refreshed_at) VALUES ($1, $2, now())
			ON CONFLICT (repository_id, name) DO UPDATE SET refreshed_at = now() RETURNING id`, repo.ID, name).Scan(&id); err != nil {
			return err
		}
		b := &pgx.Batch{}
		names := make([]string, 0, len(files))
		for _, f := range files {
			names = append(names, f.Filename)
			var meta *string
			if f.MetadataSHA256 != "" {
				meta = &f.MetadataSHA256
			}
			// A file name is immutable on PyPI. Never change the hash of a known file.
			b.Queue(`INSERT INTO file (project_id, filename, version, sha256, size, upstream_url, upload_time,
					requires_python, yanked, metadata_sha256)
				VALUES ($1, $2, $3, $4, $5, $6, $7, nullif($8, ''), $9, $10)
				ON CONFLICT (project_id, filename) DO UPDATE SET
					size = coalesce(file.size, EXCLUDED.size),
					upstream_url = CASE WHEN file.sha256 = EXCLUDED.sha256 THEN EXCLUDED.upstream_url ELSE file.upstream_url END,
					upload_time = coalesce(file.upload_time, EXCLUDED.upload_time),
					requires_python = EXCLUDED.requires_python,
					yanked = EXCLUDED.yanked,
					metadata_sha256 = coalesce(file.metadata_sha256, EXCLUDED.metadata_sha256),
					removed_upstream_at = NULL
				RETURNING sha256`,
				id, f.Filename, Version(f.Filename), f.SHA256, f.Size, f.URL, f.UploadTime, f.RequiresPython, f.Yanked, meta,
			).QueryRow(func(row pgx.Row) error {
				var stored string
				if err := row.Scan(&stored); err != nil {
					return err
				}
				if stored != f.SHA256 {
					slog.Error("upstream changed the hash of a known file, keeping the first hash",
						"repo", repo.Name, "file", f.Filename, "stored", stored, "upstream", f.SHA256)
				}
				return nil
			})
		}
		if err := tx.SendBatch(ctx, b).Close(); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE file SET removed_upstream_at = now()
			WHERE project_id = $1 AND removed_upstream_at IS NULL AND filename <> ALL($2)`, id, names)
		return err
	})
	return id, err
}

// resolve returns the project that supplies name in repo. For a group, it uses
// the first member that has the name and records that route (ADR-0008).
func (p *Proxy) resolve(ctx context.Context, repo store.Repository, name string) (int64, error) {
	if repo.Type == "proxy" {
		return p.refresh(ctx, repo, name)
	}
	var routed *int64
	err := p.DB.QueryRow(ctx, `SELECT upstream_id FROM project WHERE repository_id = $1 AND name = $2`, repo.ID, name).Scan(&routed)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	members, err := store.Members(ctx, p.DB, repo.ID)
	if err != nil {
		return 0, err
	}
	for _, m := range members {
		if m.Type != "proxy" || (routed != nil && m.ID != *routed) {
			continue
		}
		id, err := p.refresh(ctx, m, name)
		if errors.Is(err, store.ErrNotFound) && routed == nil {
			continue
		}
		if err != nil {
			// Do not try the next member: it can be an attacker's package with the same name.
			return 0, err
		}
		if routed == nil {
			if _, err := p.DB.Exec(ctx, `INSERT INTO project (repository_id, name, upstream_id) VALUES ($1, $2, $3)
				ON CONFLICT (repository_id, name) DO NOTHING`, repo.ID, name, m.ID); err != nil {
				return 0, err
			}
		}
		return id, nil
	}
	return 0, store.ErrNotFound
}

func (p *Proxy) root(w http.ResponseWriter, r *http.Request) {
	repo, _, ok := p.repo(w, r, store.RoleGuest)
	if !ok {
		return
	}
	rows, _ := p.DB.Query(r.Context(), `SELECT name FROM project WHERE repository_id = $1 ORDER BY name`, repo.ID)
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		internalError(w, err)
		return
	}
	renderRoot(w, r, names)
}

func (p *Proxy) project(w http.ResponseWriter, r *http.Request) {
	repo, _, ok := p.repo(w, r, store.RoleGuest)
	if !ok {
		return
	}
	raw := r.PathValue("project")
	name := Normalize(raw)
	if raw != name {
		http.Redirect(w, r, "/pypi/"+url.PathEscape(repo.Name)+"/simple/"+url.PathEscape(name)+"/", http.StatusMovedPermanently)
		return
	}
	id, err := p.resolve(r.Context(), repo, name)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Warn("cannot read upstream", "repo", repo.Name, "project", name, "err", err)
		http.Error(w, "upstream is not available", http.StatusBadGateway)
		return
	}
	files, err := LoadFiles(r.Context(), p.DB, id)
	if err != nil {
		internalError(w, err)
		return
	}
	statuses, err := Statuses(r.Context(), p.DB, repo, name, files, time.Now())
	if err != nil {
		internalError(w, err)
		return
	}
	var visible []File
	for i, f := range files {
		if statuses[i].Visible() {
			visible = append(visible, f)
		}
	}
	renderProject(w, r, repo.Name, name, visible)
}

func (p *Proxy) download(w http.ResponseWriter, r *http.Request) {
	repo, who, ok := p.repo(w, r, store.RoleGuest)
	if !ok {
		return
	}
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := loadFile(ctx, p.DB, id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	if f.ProjectRepoID != repo.ID {
		members, err := store.Members(ctx, p.DB, repo.ID)
		if err != nil {
			internalError(w, err)
			return
		}
		if !slices.ContainsFunc(members, func(m store.Repository) bool { return m.ID == f.ProjectRepoID }) {
			http.NotFound(w, r)
			return
		}
	}
	name := r.PathValue("filename")
	metadata := f.MetadataSHA256 != nil && name == f.Filename+".metadata"
	if name != f.Filename && !metadata {
		http.NotFound(w, r)
		return
	}
	st, err := Statuses(ctx, p.DB, repo, f.Project, []File{f}, time.Now())
	if err != nil {
		internalError(w, err)
		return
	}
	if !st[0].Visible() {
		http.Error(w, fmt.Sprintf("ginnungagap: %s is not available (%s)", f.Filename, st[0]), http.StatusForbidden)
		return
	}
	if metadata {
		p.serveMetadata(w, r, f)
		return
	}
	p.serveFile(w, r, f, who)
}

func (p *Proxy) recordPull(ctx context.Context, fileID int64, who *auth.Principal) {
	_, err := p.DB.Exec(ctx, `WITH u AS (UPDATE file SET last_pull = now(), gc_marked_at = NULL WHERE id = $1)
		INSERT INTO pull_log (file_id, account_id, robot_id) VALUES ($1, nullif($2, 0), nullif($3, 0))`,
		fileID, who.AccountID, who.RobotID)
	if err != nil {
		slog.Warn("cannot record pull", "file", fileID, "err", err)
	}
}

func (p *Proxy) serveFile(w http.ResponseWriter, r *http.Request, f File, who *auth.Principal) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/octet-stream")
	if f.Cached {
		body, mod, err := p.Blob.Get(ctx, f.SHA256)
		if err == nil {
			defer body.Close()
			if r.Method == http.MethodGet {
				p.recordPull(ctx, f.ID, who)
			}
			http.ServeContent(w, r, f.Filename, mod, body)
			return
		}
		if !errors.Is(err, blob.ErrNotExist) {
			internalError(w, err)
			return
		}
		// The row says "cached", but the blob is missing (for example, after a GC race). Get it again.
		slog.Warn("cached blob is missing, fetching it again", "file", f.Filename)
	}
	if r.Method == http.MethodHead {
		return
	}

	// Continue the download when the client disconnects, so the next client gets the file from the cache.
	bg := context.WithoutCancel(ctx)
	req, err := http.NewRequestWithContext(bg, http.MethodGet, f.UpstreamURL, nil)
	if err != nil {
		internalError(w, err)
		return
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		slog.Warn("cannot fetch upstream file", "file", f.Filename, "err", err)
		http.Error(w, "upstream is not available", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("cannot fetch upstream file", "file", f.Filename, "status", resp.Status)
		http.Error(w, "upstream is not available", http.StatusBadGateway)
		return
	}
	slog.Info("fetch upstream file", "file", f.Filename, "size", resp.ContentLength)
	if resp.ContentLength >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}

	// Send the bytes to the client and to the blob store at the same time.
	// The client checks the SHA-256 from the index. The blob store checks it too (blob.Put).
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := p.Blob.Put(bg, f.SHA256, pr, resp.ContentLength)
		pr.CloseWithError(err)
		done <- err
	}()
	n, copyErr := io.Copy(&tolerantWriter{w: w}, io.TeeReader(resp.Body, pw))
	pw.CloseWithError(copyErr)
	switch err := <-done; {
	case err == nil:
		if _, err := p.DB.Exec(bg, `UPDATE file SET cached = true, size = $2, gc_marked_at = NULL WHERE id = $1`, f.ID, n); err != nil {
			slog.Error("cannot mark file as cached", "file", f.Filename, "err", err)
		}
		p.recordPull(bg, f.ID, who)
	case errors.Is(err, blob.ErrDigest):
		slog.Error("upstream file does not match the index hash, not cached", "file", f.Filename, "url", f.UpstreamURL)
	default:
		slog.Warn("cannot cache file", "file", f.Filename, "err", err)
	}
}

// tolerantWriter ignores write errors after the client disconnects,
// so that io.Copy continues to read the upstream body into the blob store.
type tolerantWriter struct {
	w      io.Writer
	failed bool
}

func (t *tolerantWriter) Write(b []byte) (int, error) {
	if !t.failed {
		if _, err := t.w.Write(b); err != nil {
			t.failed = true
		}
	}
	return len(b), nil
}

// serveMetadata serves the PEP 658 core metadata of f. The blob store keeps it by its own hash.
func (p *Proxy) serveMetadata(w http.ResponseWriter, r *http.Request, f File) {
	ctx := r.Context()
	sha := *f.MetadataSHA256
	w.Header().Set("Content-Type", "application/octet-stream")
	body, mod, err := p.Blob.Get(ctx, sha)
	if err == nil {
		defer body.Close()
		http.ServeContent(w, r, f.Filename+".metadata", mod, body)
		return
	}
	if !errors.Is(err, blob.ErrNotExist) {
		internalError(w, err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.UpstreamURL+".metadata", nil)
	if err != nil {
		internalError(w, err)
		return
	}
	resp, err := p.Client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if err == nil {
			resp.Body.Close()
		}
		http.Error(w, "upstream is not available", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataSize))
	if err != nil {
		http.Error(w, "upstream is not available", http.StatusBadGateway)
		return
	}
	if err := p.Blob.Put(ctx, sha, bytes.NewReader(data), int64(len(data))); err != nil {
		slog.Error("cannot keep metadata", "file", f.Filename, "err", err)
		http.Error(w, "upstream metadata is not valid", http.StatusBadGateway)
		return
	}
	w.Write(data)
}

func internalError(w http.ResponseWriter, err error) {
	slog.Error("internal error", "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
