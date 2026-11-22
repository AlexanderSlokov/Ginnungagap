package gc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/AlexanderSlokov/Ginnungagap/internal/testenv"
)

func TestMarkAndSweep(t *testing.T) {
	db, b := testenv.DB(t), testenv.Blob(t)
	ctx := context.Background()
	run := fmt.Sprint(time.Now().UnixNano())

	put := func(content string) string {
		sum := sha256.Sum256([]byte(content))
		sha := hex.EncodeToString(sum[:])
		if err := b.Put(ctx, sha, bytes.NewReader([]byte(content)), int64(len(content))); err != nil {
			t.Fatal(err)
		}
		return sha
	}
	oldSha, pinSha, newSha, sharedSha := put("old"+run), put("pinned"+run), put("new"+run), put("shared"+run)

	var repo, project int64
	db.QueryRow(ctx, `INSERT INTO repository (name, format, type) VALUES ('r', 'pypi', 'proxy') RETURNING id`).Scan(&repo)
	db.QueryRow(ctx, `INSERT INTO project (repository_id, name) VALUES ($1, 'p') RETURNING id`, repo).Scan(&project)
	file := func(name, sha string, pullDaysAgo int, pinned bool) {
		if _, err := db.Exec(ctx, `INSERT INTO file (project_id, filename, version, sha256, size, upstream_url, cached, last_pull, pinned)
			VALUES ($1, $2, '1', $3, 100, 'x', true, now() - make_interval(days => $4), $5)`, project, name, sha, pullDaysAgo, pinned); err != nil {
			t.Fatal(err)
		}
	}
	file("old-1.tar.gz", oldSha, 30, false)
	file("pinned-1.tar.gz", pinSha, 40, true)
	file("new-1.tar.gz", newSha, 1, false)
	file("shared-a-1.tar.gz", sharedSha, 20, false)
	file("shared-b-1.tar.gz", sharedSha, 0, false)

	// Total 500 bytes, limit 300: GC must free 200 bytes. The pinned file is the oldest,
	// so GC marks the next two oldest: old (30 days) and shared-a (20 days).
	g := &GC{DB: db, Blob: b, Limit: 300, Grace: Grace}
	if n, err := g.Mark(ctx); err != nil || n != 2 {
		t.Fatalf("Mark = %d, %v; want 2", n, err)
	}
	marked := func(name string) bool {
		var m bool
		db.QueryRow(ctx, `SELECT gc_marked_at IS NOT NULL FROM file WHERE filename = $1`, name).Scan(&m)
		return m
	}
	if !marked("old-1.tar.gz") || !marked("shared-a-1.tar.gz") || marked("pinned-1.tar.gz") || marked("new-1.tar.gz") {
		t.Fatal("wrong files are marked")
	}
	if n, _ := g.Mark(ctx); n != 0 {
		t.Errorf("second Mark = %d, want 0", n)
	}

	// Inside the grace period, Sweep removes nothing.
	if n, err := g.Sweep(ctx); err != nil || n != 0 {
		t.Fatalf("Sweep in grace period = %d, %v", n, err)
	}
	db.Exec(ctx, `UPDATE file SET gc_marked_at = now() - interval '25 hours' WHERE gc_marked_at IS NOT NULL`)
	if n, err := g.Sweep(ctx); err != nil || n != 2 {
		t.Fatalf("Sweep = %d, %v; want 2", n, err)
	}
	exists := func(sha string) bool {
		ok, err := b.Exists(ctx, sha)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if exists(oldSha) {
		t.Error("blob of the old file is not removed")
	}
	if !exists(sharedSha) {
		t.Error("blob that another cached row uses is removed")
	}
	if !exists(pinSha) || !exists(newSha) {
		t.Error("blob of an unmarked file is removed")
	}
	var rows int
	db.QueryRow(ctx, `SELECT count(*) FROM file`).Scan(&rows)
	if rows != 5 {
		t.Errorf("file has %d rows, want 5 (sweep keeps the rows)", rows)
	}

	// Purge removes the last cached copy of the shared blob.
	var id int64
	db.QueryRow(ctx, `SELECT id FROM file WHERE filename = 'shared-b-1.tar.gz'`).Scan(&id)
	if err := Purge(ctx, db, b, id); err != nil {
		t.Fatal(err)
	}
	if exists(sharedSha) {
		t.Error("Purge did not remove the blob")
	}
	for _, sha := range []string{pinSha, newSha} {
		b.Delete(ctx, sha)
	}
}
