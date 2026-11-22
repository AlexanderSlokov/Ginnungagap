package pypi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexanderSlokov/Ginnungagap/internal/auth"
	"github.com/AlexanderSlokov/Ginnungagap/internal/store"
	"github.com/AlexanderSlokov/Ginnungagap/internal/testenv"
)

// fakeUpstream serves Simple API pages and files. Each test run uses unique content,
// so blobs from earlier runs in the shared bucket do not change the result.
type fakeUpstream struct {
	mu        sync.Mutex
	pages     map[string]string // path -> JSON page
	files     map[string][]byte // path -> content
	downloads atomic.Int64
	srv       *httptest.Server
}

func newUpstream(t *testing.T) *fakeUpstream {
	u := &fakeUpstream{pages: map[string]string{}, files: map[string][]byte{}}
	u.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		defer u.mu.Unlock()
		if page, ok := u.pages[r.URL.Path]; ok {
			w.Header().Set("Content-Type", typeJSON)
			io.WriteString(w, page)
			return
		}
		if data, ok := u.files[r.URL.Path]; ok {
			u.downloads.Add(1)
			w.Write(data)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(u.srv.Close)
	return u
}

type upFile struct {
	name    string
	content []byte
	hash    string // empty: the hash of content
	upload  time.Time
}

// setProject publishes a project page with files under /{index}/.
func (u *fakeUpstream) setProject(index, project string, files ...upFile) {
	u.mu.Lock()
	defer u.mu.Unlock()
	var entries []string
	for _, f := range files {
		path := "/" + index + "/files/" + f.name
		u.files[path] = f.content
		h := f.hash
		if h == "" {
			s := sha256.Sum256(f.content)
			h = hex.EncodeToString(s[:])
		}
		entries = append(entries, fmt.Sprintf(`{"filename":%q,"url":%q,"hashes":{"sha256":%q},"upload-time":%q}`,
			f.name, u.srv.URL+path, h, f.upload.Format(time.RFC3339)))
	}
	u.pages["/"+index+"/simple/"+project+"/"] = `{"meta":{"api-version":"1.1"},"files":[` + strings.Join(entries, ",") + `]}`
}

type env struct {
	t     *testing.T
	db    *pgxpool.Pool
	proxy *httptest.Server
	up    *fakeUpstream
	auth  *auth.Auth
	run   string
}

func setup(t *testing.T) *env {
	db := testenv.DB(t)
	b := testenv.Blob(t)
	ctx := context.Background()
	a, err := auth.New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := auth.HashPassword("secret")
	for _, q := range []string{
		`INSERT INTO account (name, password_hash, is_admin) VALUES ('root', '` + hash + `', true), ('nobody', '` + hash + `', false)`,
	} {
		if _, err := db.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	up := newUpstream(t)
	// The test upstream is on 127.0.0.1, so use a client without the address guard of NewClient.
	p := &Proxy{DB: db, Blob: b, Auth: a, Client: http.DefaultClient}
	mux := http.NewServeMux()
	p.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &env{t: t, db: db, proxy: srv, up: up, auth: a, run: fmt.Sprint(time.Now().UnixNano())}
}

func (e *env) exec(q string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Exec(context.Background(), q, args...); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) proxyRepo(name, index string) int64 {
	var id int64
	ctx := context.Background()
	if err := e.db.QueryRow(ctx, `INSERT INTO repository (name, format, type) VALUES ($1, 'pypi', 'proxy') RETURNING id`, name).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	e.exec(`INSERT INTO upstream (repository_id, url) VALUES ($1, $2)`, id, e.up.srv.URL+"/"+index+"/simple")
	return id
}

// get sends a request as user root (or as user, password) and returns the status and body.
func (e *env) get(path string, creds ...string) (int, string) {
	e.t.Helper()
	req, _ := http.NewRequest("GET", e.proxy.URL+path, nil)
	req.Header.Set("Accept", typeJSON)
	if creds == nil {
		creds = []string{"root", "secret"}
	}
	if creds[0] != "" {
		req.SetBasicAuth(creds[0], creds[1])
	}
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

type indexDoc struct {
	Files []struct {
		Filename string            `json:"filename"`
		URL      string            `json:"url"`
		Hashes   map[string]string `json:"hashes"`
	} `json:"files"`
}

func (e *env) index(path string) map[string]string {
	e.t.Helper()
	code, body := e.get(path)
	if code != http.StatusOK {
		e.t.Fatalf("GET %s = %d %s", path, code, body)
	}
	var doc indexDoc
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		e.t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range doc.Files {
		out[f.Filename] = f.URL
	}
	return out
}

// fileURL returns the proxy URL of a stored file, also for a file that the index hides.
func (e *env) fileURL(repo, filename string) string {
	var id int64
	if err := e.db.QueryRow(context.Background(), `SELECT id FROM file WHERE filename = $1`, filename).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return fmt.Sprintf("/pypi/%s/files/%d/%s", repo, id, filename)
}

func (e *env) cached(filename string) bool {
	var c bool
	e.db.QueryRow(context.Background(), `SELECT cached FROM file WHERE filename = $1`, filename).Scan(&c)
	return c
}

func (e *env) expire(project string) {
	e.exec(`UPDATE project SET refreshed_at = now() - interval '1 hour' WHERE name = $1`, project)
}

func TestProxy(t *testing.T) {
	e := setup(t)
	old := time.Now().Add(-60 * 24 * time.Hour)
	recent := time.Now().Add(-1 * time.Hour)
	repoID := e.proxyRepo("up", "a")
	good := []byte("good content " + e.run)
	e.up.setProject("a", "demo",
		upFile{name: "demo-1.0.tar.gz", content: good, upload: old},
		upFile{name: "demo-2.0.tar.gz", content: []byte("new " + e.run), upload: recent},
		upFile{name: "demo-3.0.tar.gz", content: []byte("tampered " + e.run), hash: strings.Repeat("ab", 32), upload: old},
	)

	t.Run("redirects to the normalized name", func(t *testing.T) {
		code, _ := e.get("/pypi/up/simple/Demo/")
		if code != http.StatusMovedPermanently {
			t.Fatalf("got %d", code)
		}
	})

	idx := e.index("/pypi/up/simple/demo/")
	t.Run("cooldown hides the new version", func(t *testing.T) {
		if _, ok := idx["demo-1.0.tar.gz"]; !ok {
			t.Error("demo-1.0 is missing")
		}
		if _, ok := idx["demo-2.0.tar.gz"]; ok {
			t.Error("demo-2.0 is visible during the cooldown")
		}
		if code, _ := e.get(e.fileURL("up", "demo-2.0.tar.gz")); code != http.StatusForbidden {
			t.Errorf("direct download during cooldown = %d", code)
		}
	})

	t.Run("caches on the first download", func(t *testing.T) {
		code, body := e.get(idx["demo-1.0.tar.gz"])
		if code != http.StatusOK || body != string(good) {
			t.Fatalf("download = %d %q", code, body)
		}
		if !e.cached("demo-1.0.tar.gz") {
			t.Fatal("file is not cached")
		}
		before := e.up.downloads.Load()
		if code, body := e.get(idx["demo-1.0.tar.gz"]); code != http.StatusOK || body != string(good) {
			t.Fatalf("second download = %d", code)
		}
		if e.up.downloads.Load() != before {
			t.Error("second download went to the upstream")
		}
		var pulls int
		e.db.QueryRow(context.Background(), `SELECT count(*) FROM pull_log`).Scan(&pulls)
		if pulls != 2 {
			t.Errorf("pull_log has %d rows, want 2", pulls)
		}
	})

	t.Run("does not cache a file with a wrong hash", func(t *testing.T) {
		e.get(idx["demo-3.0.tar.gz"])
		if e.cached("demo-3.0.tar.gz") {
			t.Error("tampered file is cached")
		}
	})

	t.Run("cooldown override shows the version", func(t *testing.T) {
		e.exec(`INSERT INTO policy (repository_id, project_name, version, kind) VALUES ($1, 'demo', '2.0', 'cooldown_override')`, repoID)
		if _, ok := e.index("/pypi/up/simple/demo/")["demo-2.0.tar.gz"]; !ok {
			t.Error("demo-2.0 is hidden after the override")
		}
	})

	t.Run("block hides the version", func(t *testing.T) {
		e.exec(`INSERT INTO policy (repository_id, project_name, version, kind) VALUES ($1, 'demo', '1.0', 'block')`, repoID)
		if _, ok := e.index("/pypi/up/simple/demo/")["demo-1.0.tar.gz"]; ok {
			t.Error("blocked file is visible")
		}
		if code, _ := e.get(idx["demo-1.0.tar.gz"]); code != http.StatusForbidden {
			t.Errorf("download of a blocked file = %d", code)
		}
		e.exec(`DELETE FROM policy WHERE kind = 'block'`)
	})

	t.Run("hides a file that the upstream removed", func(t *testing.T) {
		e.up.setProject("a", "demo", upFile{name: "demo-2.0.tar.gz", content: []byte("new " + e.run), upload: recent})
		e.expire("demo")
		if _, ok := e.index("/pypi/up/simple/demo/")["demo-1.0.tar.gz"]; ok {
			t.Error("removed file is visible")
		}
		if code, _ := e.get(idx["demo-1.0.tar.gz"]); code != http.StatusForbidden {
			t.Errorf("download of a removed file = %d", code)
		}
	})

	t.Run("access control", func(t *testing.T) {
		if code, _ := e.get("/pypi/up/simple/demo/", "", ""); code != http.StatusUnauthorized {
			t.Errorf("anonymous = %d", code)
		}
		if code, _ := e.get("/pypi/up/simple/demo/", "root", "wrong"); code != http.StatusUnauthorized {
			t.Errorf("wrong password = %d", code)
		}
		if code, _ := e.get("/pypi/up/simple/demo/", "nobody", "secret"); code != http.StatusForbidden {
			t.Errorf("account without a role = %d", code)
		}
		token, err := e.auth.CreateRobot(context.Background(), "ci", repoID, store.RoleGuest, time.Hour, 0)
		if err != nil {
			t.Fatal(err)
		}
		if code, _ := e.get("/pypi/up/simple/demo/", auth.RobotPrefix+"ci", token); code != http.StatusOK {
			t.Errorf("robot = %d", code)
		}
		if code, _ := e.get("/pypi/up/simple/demo/", auth.RobotPrefix+"ci", token+"x"); code != http.StatusUnauthorized {
			t.Errorf("robot with a wrong token = %d", code)
		}
		e.exec(`UPDATE robot SET expires_at = now() - interval '1 second'`)
		if code, _ := e.get("/pypi/up/simple/demo/", auth.RobotPrefix+"ci", token); code != http.StatusUnauthorized {
			t.Errorf("expired robot = %d", code)
		}
	})
}

func TestGroupPreventsDependencyConfusion(t *testing.T) {
	e := setup(t)
	old := time.Now().Add(-60 * 24 * time.Hour)
	first := e.proxyRepo("first", "a")
	second := e.proxyRepo("second", "b")
	var group int64
	e.db.QueryRow(context.Background(), `INSERT INTO repository (name, format, type) VALUES ('grp', 'pypi', 'group') RETURNING id`).Scan(&group)
	e.exec(`INSERT INTO repository_member (group_id, member_id, priority) VALUES ($1, $2, 1), ($1, $3, 2)`, group, first, second)

	e.up.setProject("a", "common", upFile{name: "common-1.0.tar.gz", content: []byte("a common " + e.run), upload: old})
	e.up.setProject("b", "common", upFile{name: "common-9.0.tar.gz", content: []byte("b common " + e.run), upload: old})
	e.up.setProject("b", "internal", upFile{name: "internal-1.0.tar.gz", content: []byte("b internal " + e.run), upload: old})

	if idx := e.index("/pypi/grp/simple/common/"); len(idx) != 1 || idx["common-1.0.tar.gz"] == "" {
		t.Errorf("common = %v, want only the first member", idx)
	}
	idx := e.index("/pypi/grp/simple/internal/")
	if idx["internal-1.0.tar.gz"] == "" {
		t.Fatalf("internal = %v", idx)
	}
	if code, _ := e.get(idx["internal-1.0.tar.gz"]); code != http.StatusOK {
		t.Errorf("download through the group = %d", code)
	}

	// An attacker publishes "internal" on the first member, which has priority.
	e.up.setProject("a", "internal", upFile{name: "internal-99.0.tar.gz", content: []byte("evil " + e.run), upload: old})
	e.expire("internal")
	idx = e.index("/pypi/grp/simple/internal/")
	if _, ok := idx["internal-99.0.tar.gz"]; ok || idx["internal-1.0.tar.gz"] == "" {
		t.Errorf("internal after the attack = %v, want the recorded route", idx)
	}

	// A file of another repository is not available through this group.
	e.exec(`INSERT INTO repository (name, format, type) VALUES ('other', 'pypi', 'proxy')`)
	if code, _ := e.get(strings.Replace(idx["internal-1.0.tar.gz"], "/grp/", "/other/", 1)); code != http.StatusNotFound {
		t.Errorf("download through another repository = %d", code)
	}
}
