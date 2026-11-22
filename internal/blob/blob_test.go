package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// The test needs a real S3 endpoint. Set TEST_S3_ENDPOINT, TEST_S3_ACCESS_KEY,
// TEST_S3_SECRET_KEY, and TEST_S3_BUCKET (an existing bucket).
func testStore(t *testing.T) *Store {
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
	return New(c, os.Getenv("TEST_S3_BUCKET"))
}

func TestRoundTrip(t *testing.T) {
	s, ctx := testStore(t), context.Background()
	data := []byte("hello ginnungagap")
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	if err := s.Put(ctx, sha, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.Exists(ctx, sha); err != nil || !ok {
		t.Fatalf("Exists = %v, %v", ok, err)
	}
	r, _, err := s.Get(ctx, sha)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	r.Close()
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if err := s.Delete(ctx, sha); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.Exists(ctx, sha); err != nil || ok {
		t.Fatalf("Exists after Delete = %v, %v", ok, err)
	}
	if _, _, err := s.Get(ctx, sha); !errors.Is(err, ErrNotExist) {
		t.Fatalf("Get after Delete = %v, want ErrNotExist", err)
	}
}

func TestPutRejectsWrongDigest(t *testing.T) {
	s, ctx := testStore(t), context.Background()
	sum := sha256.Sum256([]byte("expected"))
	sha := hex.EncodeToString(sum[:])

	err := s.Put(ctx, sha, bytes.NewReader([]byte("tampered")), 8)
	if !errors.Is(err, ErrDigest) {
		t.Fatalf("Put = %v, want ErrDigest", err)
	}
	if ok, _ := s.Exists(ctx, sha); ok {
		t.Fatal("a blob with the wrong content is visible")
	}
	for obj := range s.s3.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: "tmp/", Recursive: true}) {
		t.Errorf("temporary object is not removed: %s %v", obj.Key, obj.Err)
	}
}

func TestInvalidKey(t *testing.T) {
	s := New(nil, "unused")
	for _, k := range []string{"", "../etc/passwd", "ABCDEF", "sha256/" + string(make([]byte, 64))} {
		if _, _, err := s.Get(context.Background(), k); err == nil {
			t.Errorf("Get(%q) did not fail", k)
		}
	}
}
