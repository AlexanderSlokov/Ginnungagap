// Package web is the management UI (ADR-0007): server-side rendering with templ and htmx.
package web

//go:generate go tool templ generate

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexanderSlokov/Ginnungagap/internal/auth"
	"github.com/AlexanderSlokov/Ginnungagap/internal/blob"
	"github.com/AlexanderSlokov/Ginnungagap/internal/gc"
	"github.com/AlexanderSlokov/Ginnungagap/internal/jobs"
	"github.com/AlexanderSlokov/Ginnungagap/internal/policy"
	"github.com/AlexanderSlokov/Ginnungagap/internal/pypi"
	"github.com/AlexanderSlokov/Ginnungagap/internal/store"
)

//go:embed static
var static embed.FS

type UI struct {
	DB           *pgxpool.Pool
	Blob         *blob.Store
	Auth         *auth.Auth
	Jobs         *jobs.Client
	StorageLimit int64
}

func (u *UI) Register(mux *http.ServeMux) {
	mux.Handle("GET /static/", http.FileServerFS(static))
	mux.HandleFunc("GET /login", u.loginForm)
	mux.HandleFunc("POST /login", u.login)
	mux.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {
		auth.Logout(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /{$}", u.dashboard)
	mux.HandleFunc("GET /repos/new", u.admin(u.newRepoForm))
	mux.HandleFunc("POST /repos", u.admin(u.createRepo))
	mux.HandleFunc("GET /repos/{repo}", u.repoPage)
	mux.HandleFunc("POST /repos/{repo}/settings", u.repoAction(store.RoleAdmin, u.saveSettings))
	mux.HandleFunc("POST /repos/{repo}/policies", u.repoAction(store.RoleMaintainer, u.addPolicy))
	mux.HandleFunc("POST /repos/{repo}/policies/{id}/delete", u.repoAction(store.RoleMaintainer, u.deletePolicy))
	mux.HandleFunc("POST /repos/{repo}/routes", u.repoAction(store.RoleMaintainer, u.setRoute))
	mux.HandleFunc("POST /repos/{repo}/members", u.repoAction(store.RoleAdmin, u.setMember))
	mux.HandleFunc("POST /repos/{repo}/members/{account}/delete", u.repoAction(store.RoleAdmin, u.deleteMember))
	mux.HandleFunc("POST /repos/{repo}/robots", u.repoAction(store.RoleAdmin, u.createRobot))
	mux.HandleFunc("POST /repos/{repo}/robots/{id}/delete", u.repoAction(store.RoleAdmin, u.deleteRobot))
	mux.HandleFunc("GET /repos/{repo}/projects/{project}", u.projectPage)
	mux.HandleFunc("POST /repos/{repo}/files/{id}/pin", u.repoAction(store.RoleDeveloper, u.pin))
	mux.HandleFunc("POST /repos/{repo}/files/{id}/purge", u.repoAction(store.RoleMaintainer, u.purge))
	mux.HandleFunc("GET /accounts", u.admin(u.accountsPage))
	mux.HandleFunc("POST /accounts", u.admin(u.createAccount))
	mux.HandleFunc("GET /gc", u.admin(u.gcPage))
	mux.HandleFunc("POST /gc/run", u.admin(u.runGC))
}

// ---- helpers ----

func (u *UI) user(w http.ResponseWriter, r *http.Request) (*auth.Principal, bool) {
	p, ok := u.Auth.Identify(r)
	if !ok || p.IsRobot() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil, false
	}
	return p, true
}

func (u *UI) fail(w http.ResponseWriter, r *http.Request, code int, msg string) {
	p, _ := u.Auth.Identify(r)
	w.WriteHeader(code)
	errorPage(p, code, msg).Render(r.Context(), w)
}

func (u *UI) internal(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("ui error", "path", r.URL.Path, "err", err)
	u.fail(w, r, http.StatusInternalServerError, "Internal error. Refer to the service log.")
}

func (u *UI) admin(h func(http.ResponseWriter, *http.Request, *auth.Principal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := u.user(w, r)
		if !ok {
			return
		}
		if !p.Admin {
			u.fail(w, r, http.StatusForbidden, "Only a system administrator can do this.")
			return
		}
		h(w, r, p)
	}
}

type repoCtx struct {
	Repo store.Repository
	Who  *auth.Principal
	Role store.Role
}

func (u *UI) loadRepo(w http.ResponseWriter, r *http.Request, min store.Role) (repoCtx, bool) {
	p, ok := u.user(w, r)
	if !ok {
		return repoCtx{}, false
	}
	repo, err := store.GetRepository(r.Context(), u.DB, r.PathValue("repo"))
	if errors.Is(err, store.ErrNotFound) {
		u.fail(w, r, http.StatusNotFound, "Repository not found.")
		return repoCtx{}, false
	}
	if err != nil {
		u.internal(w, r, err)
		return repoCtx{}, false
	}
	role, err := u.Auth.Role(r.Context(), p, repo.ID)
	if err != nil {
		u.internal(w, r, err)
		return repoCtx{}, false
	}
	if role < min {
		u.fail(w, r, http.StatusForbidden, fmt.Sprintf("You need the %s role on %s.", min, repo.Name))
		return repoCtx{}, false
	}
	return repoCtx{repo, p, role}, true
}

func (u *UI) repoAction(min store.Role, h func(http.ResponseWriter, *http.Request, repoCtx)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rc, ok := u.loadRepo(w, r, min)
		if ok {
			h(w, r, rc)
		}
	}
}

func back(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}

func repoPath(name string) string { return "/repos/" + url.PathEscape(name) }

func projectPath(repo, project string) string {
	return repoPath(repo) + "/projects/" + url.PathEscape(project)
}

// ---- login ----

func (u *UI) loginForm(w http.ResponseWriter, r *http.Request) {
	loginPage("").Render(r.Context(), w)
}

func (u *UI) login(w http.ResponseWriter, r *http.Request) {
	if !u.Auth.Login(w, r, r.FormValue("name"), r.FormValue("password")) {
		w.WriteHeader(http.StatusUnauthorized)
		loginPage("The name or the password is not correct.").Render(r.Context(), w)
		return
	}
	back(w, r, "/")
}

// ---- dashboard ----

type repoRow struct {
	store.Repository
	Role     store.Role
	Projects int64
	Cached   int64
	Bytes    int64
}

type dashboardView struct {
	Who   *auth.Principal
	Repos []repoRow
	Total int64
	Limit int64
}

func (u *UI) dashboard(w http.ResponseWriter, r *http.Request) {
	p, ok := u.user(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	repos, err := store.ListRepositories(ctx, u.DB)
	if err != nil {
		u.internal(w, r, err)
		return
	}
	stats := map[int64][3]int64{}
	rows, _ := u.DB.Query(ctx, `SELECT p.repository_id, count(DISTINCT p.id),
			count(f.id) FILTER (WHERE f.cached), coalesce(sum(f.size) FILTER (WHERE f.cached), 0)
		FROM project p LEFT JOIN file f ON f.project_id = p.id GROUP BY p.repository_id`)
	var id, a, b, c int64
	if _, err := pgx.ForEachRow(rows, []any{&id, &a, &b, &c}, func() error { stats[id] = [3]int64{a, b, c}; return nil }); err != nil {
		u.internal(w, r, err)
		return
	}
	v := dashboardView{Who: p, Limit: u.StorageLimit}
	for _, repo := range repos {
		role, err := u.Auth.Role(ctx, p, repo.ID)
		if err != nil {
			u.internal(w, r, err)
			return
		}
		if role == store.RoleNone {
			continue
		}
		s := stats[repo.ID]
		v.Repos = append(v.Repos, repoRow{repo, role, s[0], s[1], s[2]})
		v.Total += s[2]
	}
	dashboardPage(v).Render(ctx, w)
}

// ---- repositories ----

func (u *UI) newRepoForm(w http.ResponseWriter, r *http.Request, p *auth.Principal) {
	newRepoPage(p, "").Render(r.Context(), w)
}

func parseUpstream(s string) (string, error) {
	s = strings.TrimSpace(s)
	pu, err := url.Parse(s)
	if err != nil || (pu.Scheme != "https" && pu.Scheme != "http") || pu.Host == "" {
		return "", errors.New("the upstream must be an http or https URL")
	}
	return strings.TrimSuffix(s, "/"), nil
}

// memberIDs resolves a comma-separated list of proxy repository names, in priority order.
func (u *UI) memberIDs(ctx context.Context, list string) ([]int64, error) {
	var ids []int64
	for _, n := range strings.Split(list, ",") {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		m, err := store.GetRepository(ctx, u.DB, n)
		if err != nil {
			return nil, fmt.Errorf("repository %q not found", n)
		}
		if m.Type != "proxy" {
			return nil, fmt.Errorf("%q is not a proxy repository", n)
		}
		ids = append(ids, m.ID)
	}
	if len(ids) == 0 {
		return nil, errors.New("a group needs at least one member")
	}
	return ids, nil
}

func setMembers(ctx context.Context, tx pgx.Tx, group int64, ids []int64) error {
	if _, err := tx.Exec(ctx, `DELETE FROM repository_member WHERE group_id = $1`, group); err != nil {
		return err
	}
	for i, id := range ids {
		if _, err := tx.Exec(ctx, `INSERT INTO repository_member (group_id, member_id, priority) VALUES ($1, $2, $3)`, group, id, i+1); err != nil {
			return err
		}
	}
	return nil
}

func (u *UI) createRepo(w http.ResponseWriter, r *http.Request, p *auth.Principal) {
	ctx := r.Context()
	name, typ := strings.TrimSpace(r.FormValue("name")), r.FormValue("type")
	cooldown, err := strconv.Atoi(r.FormValue("cooldown_days"))
	formErr := func(msg string) {
		w.WriteHeader(http.StatusBadRequest)
		newRepoPage(p, msg).Render(ctx, w)
	}
	if err != nil || cooldown < 0 {
		formErr("The cooldown must be a number of days, 0 or more.")
		return
	}
	if !auth.ValidName.MatchString(name) || strings.ContainsAny(name, "._") {
		formErr("The name can contain only a-z, 0-9, and -.")
		return
	}
	var upstream string
	var members []int64
	switch typ {
	case "proxy":
		if upstream, err = parseUpstream(r.FormValue("upstream")); err != nil {
			formErr(err.Error())
			return
		}
	case "group":
		if members, err = u.memberIDs(ctx, r.FormValue("members")); err != nil {
			formErr(err.Error())
			return
		}
	default:
		formErr("Select a type.")
		return
	}
	err = pgx.BeginFunc(ctx, u.DB, func(tx pgx.Tx) error {
		var id int64
		if err := tx.QueryRow(ctx, `INSERT INTO repository (name, format, type, cooldown_days) VALUES ($1, 'pypi', $2, $3) RETURNING id`,
			name, typ, cooldown).Scan(&id); err != nil {
			return err
		}
		if typ == "proxy" {
			_, err := tx.Exec(ctx, `INSERT INTO upstream (repository_id, url) VALUES ($1, $2)`, id, upstream)
			return err
		}
		return setMembers(ctx, tx, id, members)
	})
	if err != nil {
		formErr("Cannot create the repository. The name can be in use already.")
		return
	}
	back(w, r, repoPath(name))
}

type packageRow struct {
	Name     string
	Source   string
	Files    int64
	Cached   int64
	Bytes    int64
	LastPull *time.Time
}

type memberRow struct {
	Name string
	Role store.Role
}

type robotRow struct {
	ID      int64
	Name    string
	Role    store.Role
	Expires time.Time
}

type repoView struct {
	repoCtx
	Members  []store.Repository
	Packages []packageRow
	Query    string
	Policies []policy.Rule
	Accounts []memberRow
	Robots   []robotRow
	Token    string // a new robot token, shown one time
	TokenFor string
	IndexURL string
}

func (u *UI) repoPage(w http.ResponseWriter, r *http.Request) {
	if rc, ok := u.loadRepo(w, r, store.RoleGuest); ok {
		u.renderRepo(w, r, rc, "", "")
	}
}

func (u *UI) renderRepo(w http.ResponseWriter, r *http.Request, rc repoCtx, token, tokenFor string) {
	ctx := r.Context()
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	v := repoView{repoCtx: rc, Query: r.FormValue("q"), Token: token, TokenFor: tokenFor,
		IndexURL: scheme + "://" + r.Host + "/pypi/" + rc.Repo.Name + "/simple/"}
	var err error
	if rc.Repo.Type == "group" {
		if v.Members, err = store.Members(ctx, u.DB, rc.Repo.ID); err != nil {
			u.internal(w, r, err)
			return
		}
	}
	rows, _ := u.DB.Query(ctx, `SELECT g.name, coalesce(m.name, ''), count(f.id), count(f.id) FILTER (WHERE f.cached),
			coalesce(sum(f.size) FILTER (WHERE f.cached), 0), max(f.last_pull)
		FROM project g
		LEFT JOIN repository m ON m.id = g.upstream_id
		LEFT JOIN project s ON s.repository_id = coalesce(g.upstream_id, g.repository_id) AND s.name = g.name
		LEFT JOIN file f ON f.project_id = s.id
		WHERE g.repository_id = $1 AND strpos(g.name, $2) > 0
		GROUP BY g.name, m.name ORDER BY g.name LIMIT 500`, rc.Repo.ID, pypi.Normalize(v.Query))
	if v.Packages, err = pgx.CollectRows(rows, pgx.RowToStructByPos[packageRow]); err != nil {
		u.internal(w, r, err)
		return
	}
	rows, _ = u.DB.Query(ctx, `SELECT id, project_name, coalesce(version, ''), kind, reason
		FROM policy WHERE repository_id = $1 ORDER BY project_name, version`, rc.Repo.ID)
	if v.Policies, err = pgx.CollectRows(rows, pgx.RowToStructByPos[policy.Rule]); err != nil {
		u.internal(w, r, err)
		return
	}
	if rc.Role >= store.RoleAdmin {
		rows, _ = u.DB.Query(ctx, `SELECT a.name, ar.role FROM account_role ar JOIN account a ON a.id = ar.account_id
			WHERE ar.repository_id = $1 ORDER BY a.name`, rc.Repo.ID)
		v.Accounts, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (memberRow, error) {
			var m memberRow
			var role string
			err := row.Scan(&m.Name, &role)
			m.Role = store.ParseRole(role)
			return m, err
		})
		if err != nil {
			u.internal(w, r, err)
			return
		}
		rows, _ = u.DB.Query(ctx, `SELECT r.id, r.name, rr.role, r.expires_at FROM robot r
			JOIN robot_role rr ON rr.robot_id = r.id WHERE rr.repository_id = $1 ORDER BY r.name`, rc.Repo.ID)
		v.Robots, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (robotRow, error) {
			var b robotRow
			var role string
			err := row.Scan(&b.ID, &b.Name, &role, &b.Expires)
			b.Role = store.ParseRole(role)
			return b, err
		})
		if err != nil {
			u.internal(w, r, err)
			return
		}
	}
	repoPage(v).Render(ctx, w)
}

func (u *UI) saveSettings(w http.ResponseWriter, r *http.Request, rc repoCtx) {
	ctx := r.Context()
	cooldown, err := strconv.Atoi(r.FormValue("cooldown_days"))
	if err != nil || cooldown < 0 {
		u.fail(w, r, http.StatusBadRequest, "The cooldown must be a number of days, 0 or more.")
		return
	}
	var upstream string
	var members []int64
	if rc.Repo.Type == "proxy" {
		upstream, err = parseUpstream(r.FormValue("upstream"))
	} else {
		members, err = u.memberIDs(ctx, r.FormValue("members"))
	}
	if err != nil {
		u.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	err = pgx.BeginFunc(ctx, u.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE repository SET cooldown_days = $2 WHERE id = $1`, rc.Repo.ID, cooldown); err != nil {
			return err
		}
		if rc.Repo.Type == "proxy" {
			if _, err := tx.Exec(ctx, `UPDATE upstream SET url = $2 WHERE repository_id = $1`, rc.Repo.ID, upstream); err != nil {
				return err
			}
			// Read the new upstream on the next request.
			_, err := tx.Exec(ctx, `UPDATE project SET refreshed_at = NULL WHERE repository_id = $1`, rc.Repo.ID)
			return err
		}
		return setMembers(ctx, tx, rc.Repo.ID, members)
	})
	if err != nil {
		u.internal(w, r, err)
		return
	}
	back(w, r, repoPath(rc.Repo.Name))
}

func (u *UI) addPolicy(w http.ResponseWriter, r *http.Request, rc repoCtx) {
	project := pypi.Normalize(strings.TrimSpace(r.FormValue("project")))
	version := strings.TrimSpace(r.FormValue("version"))
	kind := r.FormValue("kind")
	if project == "" || !slices.Contains([]string{"block", "allow", "cooldown_override"}, kind) {
		u.fail(w, r, http.StatusBadRequest, "Give a package name and a rule type.")
		return
	}
	var v *string
	if version != "" {
		v = &version
	}
	if _, err := u.DB.Exec(r.Context(), `INSERT INTO policy (repository_id, project_name, version, kind, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)`, rc.Repo.ID, project, v, kind, strings.TrimSpace(r.FormValue("reason")), rc.Who.AccountID); err != nil {
		u.internal(w, r, err)
		return
	}
	if ret := r.FormValue("return"); ret == "project" {
		back(w, r, projectPath(rc.Repo.Name, project))
		return
	}
	back(w, r, repoPath(rc.Repo.Name))
}

func (u *UI) deletePolicy(w http.ResponseWriter, r *http.Request, rc repoCtx) {
	if _, err := u.DB.Exec(r.Context(), `DELETE FROM policy WHERE id = $1 AND repository_id = $2`, r.PathValue("id"), rc.Repo.ID); err != nil {
		u.internal(w, r, err)
		return
	}
	back(w, r, repoPath(rc.Repo.Name))
}

func (u *UI) setRoute(w http.ResponseWriter, r *http.Request, rc repoCtx) {
	ctx := r.Context()
	if rc.Repo.Type != "group" {
		u.fail(w, r, http.StatusBadRequest, "Only a group repository has routes.")
		return
	}
	project := pypi.Normalize(strings.TrimSpace(r.FormValue("project")))
	members, err := store.Members(ctx, u.DB, rc.Repo.ID)
	if err != nil {
		u.internal(w, r, err)
		return
	}
	i := slices.IndexFunc(members, func(m store.Repository) bool { return m.Name == r.FormValue("member") })
	if project == "" || i < 0 {
		u.fail(w, r, http.StatusBadRequest, "Give a package name and a member of this group.")
		return
	}
	if _, err := u.DB.Exec(ctx, `INSERT INTO project (repository_id, name, upstream_id) VALUES ($1, $2, $3)
		ON CONFLICT (repository_id, name) DO UPDATE SET upstream_id = EXCLUDED.upstream_id`, rc.Repo.ID, project, members[i].ID); err != nil {
		u.internal(w, r, err)
		return
	}
	back(w, r, repoPath(rc.Repo.Name))
}

func (u *UI) setMember(w http.ResponseWriter, r *http.Request, rc repoCtx) {
	role := store.ParseRole(r.FormValue("role"))
	if role == store.RoleNone {
		u.fail(w, r, http.StatusBadRequest, "Select a role.")
		return
	}
	tag, err := u.DB.Exec(r.Context(), `INSERT INTO account_role (account_id, repository_id, role)
		SELECT id, $2, $3 FROM account WHERE name = $1
		ON CONFLICT (account_id, repository_id) DO UPDATE SET role = EXCLUDED.role`,
		strings.TrimSpace(r.FormValue("account")), rc.Repo.ID, role.String())
	if err != nil {
		u.internal(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		u.fail(w, r, http.StatusBadRequest, "Account not found.")
		return
	}
	back(w, r, repoPath(rc.Repo.Name))
}

func (u *UI) deleteMember(w http.ResponseWriter, r *http.Request, rc repoCtx) {
	if _, err := u.DB.Exec(r.Context(), `DELETE FROM account_role WHERE repository_id = $1
		AND account_id = (SELECT id FROM account WHERE name = $2)`, rc.Repo.ID, r.PathValue("account")); err != nil {
		u.internal(w, r, err)
		return
	}
	back(w, r, repoPath(rc.Repo.Name))
}

func (u *UI) createRobot(w http.ResponseWriter, r *http.Request, rc repoCtx) {
	days, err := strconv.Atoi(r.FormValue("days"))
	if err != nil || days < 1 || days > 365 {
		u.fail(w, r, http.StatusBadRequest, "The validity must be 1 to 365 days.")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	token, err := u.Auth.CreateRobot(r.Context(), name, rc.Repo.ID, store.ParseRole(r.FormValue("role")),
		time.Duration(days)*24*time.Hour, rc.Who.AccountID)
	if err != nil {
		u.fail(w, r, http.StatusBadRequest, "Cannot create the robot account: "+err.Error()+". The name can be in use already.")
		return
	}
	u.renderRepo(w, r, rc, token, auth.RobotPrefix+name)
}

func (u *UI) deleteRobot(w http.ResponseWriter, r *http.Request, rc repoCtx) {
	if _, err := u.DB.Exec(r.Context(), `DELETE FROM robot WHERE id = $1
		AND id IN (SELECT robot_id FROM robot_role WHERE repository_id = $2)`, r.PathValue("id"), rc.Repo.ID); err != nil {
		u.internal(w, r, err)
		return
	}
	back(w, r, repoPath(rc.Repo.Name))
}

// ---- project ----

type fileRow struct {
	pypi.File
	Status      policy.Status
	AvailableAt time.Time
}

type pullRow struct {
	At   time.Time
	File string
	Who  string
}

type projectView struct {
	repoCtx
	Name  string
	Files []fileRow
	Pulls []pullRow
}

func (u *UI) projectPage(w http.ResponseWriter, r *http.Request) {
	rc, ok := u.loadRepo(w, r, store.RoleGuest)
	if !ok {
		return
	}
	ctx := r.Context()
	name := pypi.Normalize(r.PathValue("project"))
	v := projectView{repoCtx: rc, Name: name}
	id, err := pypi.SourceProject(ctx, u.DB, rc.Repo, name)
	if errors.Is(err, store.ErrNotFound) {
		projectPage(v).Render(ctx, w)
		return
	}
	if err != nil {
		u.internal(w, r, err)
		return
	}
	files, err := pypi.LoadFiles(ctx, u.DB, id)
	if err != nil {
		u.internal(w, r, err)
		return
	}
	statuses, err := pypi.Statuses(ctx, u.DB, rc.Repo, name, files, time.Now())
	if err != nil {
		u.internal(w, r, err)
		return
	}
	cooldown := time.Duration(rc.Repo.CooldownDays) * 24 * time.Hour
	for i, f := range files {
		release := f.FirstSeen
		if f.UploadTime != nil {
			release = *f.UploadTime
		}
		v.Files = append(v.Files, fileRow{f, statuses[i], release.Add(cooldown)})
	}
	// Newest release first. A file name does not sort by version ("2.9" > "2.32").
	slices.SortStableFunc(v.Files, func(a, b fileRow) int {
		if c := b.AvailableAt.Compare(a.AvailableAt); c != 0 {
			return c
		}
		return strings.Compare(b.Filename, a.Filename)
	})
	if rc.Role >= store.RoleDeveloper {
		rows, _ := u.DB.Query(ctx, `SELECT pl.pulled_at, f.filename, coalesce(a.name, 'robot+' || rb.name, '?')
			FROM pull_log pl JOIN file f ON f.id = pl.file_id
			LEFT JOIN account a ON a.id = pl.account_id LEFT JOIN robot rb ON rb.id = pl.robot_id
			WHERE f.project_id = $1 ORDER BY pl.pulled_at DESC LIMIT 50`, id)
		if v.Pulls, err = pgx.CollectRows(rows, pgx.RowToStructByPos[pullRow]); err != nil {
			u.internal(w, r, err)
			return
		}
	}
	projectPage(v).Render(ctx, w)
}

// repoFile loads a file and checks that it belongs to the repository or to a member of it.
func (u *UI) repoFile(w http.ResponseWriter, r *http.Request, rc repoCtx) (int64, string, bool) {
	ctx := r.Context()
	var id, repoID int64
	var project string
	err := u.DB.QueryRow(ctx, `SELECT f.id, p.repository_id, p.name FROM file f JOIN project p ON p.id = f.project_id
		WHERE f.id = $1`, r.PathValue("id")).Scan(&id, &repoID, &project)
	if err == nil && repoID != rc.Repo.ID {
		members, merr := store.Members(ctx, u.DB, rc.Repo.ID)
		if merr != nil || !slices.ContainsFunc(members, func(m store.Repository) bool { return m.ID == repoID }) {
			err = pgx.ErrNoRows
		}
	}
	if err != nil {
		u.fail(w, r, http.StatusNotFound, "File not found.")
		return 0, "", false
	}
	return id, project, true
}

func (u *UI) pin(w http.ResponseWriter, r *http.Request, rc repoCtx) {
	id, project, ok := u.repoFile(w, r, rc)
	if !ok {
		return
	}
	pinned := r.FormValue("pinned") == "true"
	if _, err := u.DB.Exec(r.Context(), `UPDATE file SET pinned = $2, gc_marked_at = CASE WHEN $2 THEN NULL ELSE gc_marked_at END
		WHERE id = $1`, id, pinned); err != nil {
		u.internal(w, r, err)
		return
	}
	back(w, r, projectPath(rc.Repo.Name, project))
}

func (u *UI) purge(w http.ResponseWriter, r *http.Request, rc repoCtx) {
	id, project, ok := u.repoFile(w, r, rc)
	if !ok {
		return
	}
	if err := gc.Purge(r.Context(), u.DB, u.Blob, id); err != nil {
		u.internal(w, r, err)
		return
	}
	back(w, r, projectPath(rc.Repo.Name, project))
}

// ---- accounts ----

type accountRow struct {
	Name    string
	Admin   bool
	Created time.Time
}

func (u *UI) accountsPage(w http.ResponseWriter, r *http.Request, p *auth.Principal) {
	u.renderAccounts(w, r, p, "")
}

func (u *UI) renderAccounts(w http.ResponseWriter, r *http.Request, p *auth.Principal, msg string) {
	rows, _ := u.DB.Query(r.Context(), `SELECT name, is_admin, created_at FROM account ORDER BY name`)
	accounts, err := pgx.CollectRows(rows, pgx.RowToStructByPos[accountRow])
	if err != nil {
		u.internal(w, r, err)
		return
	}
	accountsPage(p, accounts, msg).Render(r.Context(), w)
}

func (u *UI) createAccount(w http.ResponseWriter, r *http.Request, p *auth.Principal) {
	name, password := strings.TrimSpace(r.FormValue("name")), r.FormValue("password")
	if !auth.ValidName.MatchString(name) || strings.HasPrefix(name, "robot") {
		w.WriteHeader(http.StatusBadRequest)
		u.renderAccounts(w, r, p, "The name can contain only a-z, 0-9, '.', '_', and '-'. It cannot start with \"robot\".")
		return
	}
	if len(password) < 12 {
		w.WriteHeader(http.StatusBadRequest)
		u.renderAccounts(w, r, p, "The password must have 12 characters or more.")
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		u.internal(w, r, err)
		return
	}
	if _, err := u.DB.Exec(r.Context(), `INSERT INTO account (name, password_hash, is_admin) VALUES ($1, $2, $3)`,
		name, hash, r.FormValue("admin") == "on"); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		u.renderAccounts(w, r, p, "Cannot create the account. The name can be in use already.")
		return
	}
	back(w, r, "/accounts")
}

// ---- garbage collection ----

type markedRow struct {
	Repo     string
	Project  string
	Filename string
	Size     int64
	MarkedAt time.Time
}

type gcView struct {
	Who    *auth.Principal
	Limit  int64
	Total  int64
	Pinned int64
	Marked []markedRow
	Queued bool
}

func (u *UI) gcPage(w http.ResponseWriter, r *http.Request, p *auth.Principal) {
	ctx := r.Context()
	v := gcView{Who: p, Limit: u.StorageLimit, Queued: r.FormValue("queued") == "1"}
	if err := u.DB.QueryRow(ctx, `SELECT coalesce(sum(size) FILTER (WHERE cached), 0), coalesce(sum(size) FILTER (WHERE cached AND pinned), 0)
		FROM file`).Scan(&v.Total, &v.Pinned); err != nil {
		u.internal(w, r, err)
		return
	}
	rows, _ := u.DB.Query(ctx, `SELECT r.name, p.name, f.filename, coalesce(f.size, 0), f.gc_marked_at
		FROM file f JOIN project p ON p.id = f.project_id JOIN repository r ON r.id = p.repository_id
		WHERE f.cached AND f.gc_marked_at IS NOT NULL ORDER BY f.gc_marked_at LIMIT 500`)
	var err error
	if v.Marked, err = pgx.CollectRows(rows, pgx.RowToStructByPos[markedRow]); err != nil {
		u.internal(w, r, err)
		return
	}
	gcPage(v).Render(ctx, w)
}

func (u *UI) runGC(w http.ResponseWriter, r *http.Request, _ *auth.Principal) {
	if err := u.Jobs.RunGC(r.Context()); err != nil {
		u.internal(w, r, err)
		return
	}
	back(w, r, "/gc?queued=1")
}

// ---- formatting ----

func size(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func sizePtr(n *int64) string {
	if n == nil {
		return "?"
	}
	return size(*n)
}

func day(t time.Time) string { return t.UTC().Format("2006-01-02") }

func stamp(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 UTC") }

func ago(t *time.Time) string {
	if t == nil {
		return "never"
	}
	d := time.Since(*t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// templ removes line breaks in template text, so build the text of a <pre> block here.
func usageSnippet(indexURL string) string {
	return "pip install --index-url " + indexURL + " PACKAGE\n\n" +
		"# uv: pyproject.toml\n[[tool.uv.index]]\nname = \"ginnungagap\"\nurl = \"" + indexURL + "\"\ndefault = true"
}

func tokenSnippet(user, token string) string {
	return "user:  " + user + "\ntoken: " + token
}

func memberList(members []store.Repository) string {
	names := make([]string, len(members))
	for i, m := range members {
		names[i] = m.Name
	}
	return strings.Join(names, ", ")
}

func usage(total, limit int64) int {
	if limit <= 0 {
		return 0
	}
	return int(min(100, total*100/limit))
}
