// Package testenv gives integration tests a new database and a blob store.
//
// Set TEST_DATABASE_URL (a PostgreSQL server; the user must be able to create databases)
// and TEST_S3_ENDPOINT, TEST_S3_ACCESS_KEY, TEST_S3_SECRET_KEY, TEST_S3_BUCKET.
// test/integration.sh sets them from the Docker Compose stack.
package testenv

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/AlexanderSlokov/Ginnungagap/internal/blob"
	"github.com/AlexanderSlokov/Ginnungagap/internal/store"
)

// DB creates a new, migrated database and removes it after the test.
func DB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	var b [6]byte
	rand.Read(b[:])
	name := "test_" + hex.EncodeToString(b[:])
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(base)
	u.Path = "/" + name
	db, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		admin.Close()
	})
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db
}

// Blob returns the test blob store.
func Blob(t *testing.T) *blob.Store {
	t.Helper()
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_S3_ENDPOINT is not set")
	}
	c, err := minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(os.Getenv("TEST_S3_ACCESS_KEY"), os.Getenv("TEST_S3_SECRET_KEY"), ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	return blob.New(c, os.Getenv("TEST_S3_BUCKET"))
}
