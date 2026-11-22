// Package blob keeps artifact files in an S3-compatible bucket.
// The key of a blob is the SHA-256 of its content, in lowercase hex.
package blob

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/minio/minio-go/v7"
)

// ErrDigest means that the content does not have the expected SHA-256.
var ErrDigest = errors.New("blob: sha256 mismatch")

var validKey = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Store struct {
	s3     *minio.Client
	bucket string
}

func New(s3 *minio.Client, bucket string) *Store {
	return &Store{s3: s3, bucket: bucket}
}

func object(sha string) (string, error) {
	if !validKey.MatchString(sha) {
		return "", fmt.Errorf("blob: invalid key %q", sha)
	}
	return "sha256/" + sha, nil
}

// Put reads r and keeps it as the blob sha. It returns ErrDigest if the content
// does not match sha. The blob becomes visible only after the check passes.
//
// shortcut: upload to a temporary key, check the digest, then copy to the final key.
// This works because S3 CopyObject is atomic for the destination: a reader sees the
// old blob or the new blob, never a partial or unchecked one.
// Limit: a single CopyObject supports objects up to 5 GiB.
// Upgrade: use a multipart copy (ComposeObject) when an artifact is larger than 5 GiB.
func (s *Store) Put(ctx context.Context, sha string, r io.Reader, size int64) error {
	key, err := object(sha)
	if err != nil {
		return err
	}
	var nonce [16]byte
	rand.Read(nonce[:])
	tmp := "tmp/" + hex.EncodeToString(nonce[:])

	h := sha256.New()
	if _, err := s.s3.PutObject(ctx, s.bucket, tmp, io.TeeReader(r, h), size, minio.PutObjectOptions{}); err != nil {
		return err
	}
	defer s.s3.RemoveObject(context.WithoutCancel(ctx), s.bucket, tmp, minio.RemoveObjectOptions{})

	if hex.EncodeToString(h.Sum(nil)) != sha {
		return ErrDigest
	}
	_, err = s.s3.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: s.bucket, Object: key},
		minio.CopySrcOptions{Bucket: s.bucket, Object: tmp})
	return err
}

// Get returns a stream of the blob. The caller must close it.
func (s *Store) Get(ctx context.Context, sha string) (io.ReadCloser, error) {
	key, err := object(sha)
	if err != nil {
		return nil, err
	}
	return s.s3.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
}

// Exists reports whether the blob sha is in the bucket.
func (s *Store) Exists(ctx context.Context, sha string) (bool, error) {
	key, err := object(sha)
	if err != nil {
		return false, err
	}
	_, err = s.s3.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
		return false, nil
	}
	return err == nil, err
}

// Delete removes the blob sha. It is not an error if the blob does not exist.
func (s *Store) Delete(ctx context.Context, sha string) error {
	key, err := object(sha)
	if err != nil {
		return err
	}
	return s.s3.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
