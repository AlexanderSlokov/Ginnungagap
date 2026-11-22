package store

import (
	"context"
	"io/fs"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The test needs a PostgreSQL database. Set TEST_DATABASE_URL.
func TestMigrateTwice(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`)

	for i := range 2 {
		if err := Migrate(ctx, db); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM schema_migration`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	names, _ := fs.Glob(migrations, "migrations/*.sql")
	if n != len(names) {
		t.Fatalf("schema_migration has %d rows, want %d", n, len(names))
	}
}
