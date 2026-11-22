// Package gc removes cached files in two steps: mark, then sweep (ADR-0005).
package gc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexanderSlokov/Ginnungagap/internal/blob"
)

const Grace = 24 * time.Hour

type GC struct {
	DB    *pgxpool.Pool
	Blob  *blob.Store
	Limit int64 // bytes; 0 means no limit
	Grace time.Duration
}

// Mark marks the cached files with the oldest last pull, until the unmarked files
// are smaller than the limit. It does not mark a pinned file.
func (g *GC) Mark(ctx context.Context) (int64, error) {
	if g.Limit <= 0 {
		return 0, nil
	}
	tag, err := g.DB.Exec(ctx, `
		WITH total AS (
			SELECT coalesce(sum(size), 0) - $1 AS excess FROM file WHERE cached AND gc_marked_at IS NULL
		), ordered AS (
			SELECT id, size, sum(size) OVER (ORDER BY coalesce(last_pull, first_seen), id) AS running
			FROM file WHERE cached AND NOT pinned AND gc_marked_at IS NULL
		)
		UPDATE file SET gc_marked_at = now()
		FROM ordered, total
		WHERE file.id = ordered.id AND total.excess > 0 AND ordered.running - ordered.size < total.excess`, g.Limit)
	if err != nil {
		return 0, err
	}
	if n := tag.RowsAffected(); n > 0 {
		slog.Info("gc marked files", "count", n)
	}
	return tag.RowsAffected(), nil
}

// Sweep uncaches the files that are marked for longer than the grace period.
func (g *GC) Sweep(ctx context.Context) (int, error) {
	rows, _ := g.DB.Query(ctx, `SELECT id FROM file WHERE cached AND gc_marked_at < now() - make_interval(secs => $1)`, g.Grace.Seconds())
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, err
	}
	swept := 0
	for _, id := range ids {
		ok, err := uncache(ctx, g.DB, g.Blob, id, g.Grace)
		if err != nil {
			return swept, err
		}
		if ok {
			swept++
		}
	}
	if swept > 0 {
		slog.Info("gc swept files", "count", swept)
	}
	return swept, nil
}

// Purge uncaches one file now. An operator uses it from the UI.
func Purge(ctx context.Context, db *pgxpool.Pool, b *blob.Store, fileID int64) error {
	_, err := uncache(ctx, db, b, fileID, 0)
	return err
}

// uncache sets cached = false on the file row and keeps the row: the row keeps the
// first-seen time for the cooldown and the pull history. If grace > 0, it does this
// only if the file is still marked for longer than grace (a pull removes the mark).
// It removes the blob only if no other cached row has the same SHA-256.
func uncache(ctx context.Context, db *pgxpool.Pool, b *blob.Store, id int64, grace time.Duration) (bool, error) {
	var sha string
	var shared bool
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		q := `UPDATE file SET cached = false, gc_marked_at = NULL WHERE id = $1 AND cached RETURNING sha256`
		args := []any{id}
		if grace > 0 {
			q = `UPDATE file SET cached = false, gc_marked_at = NULL
				WHERE id = $1 AND cached AND gc_marked_at < now() - make_interval(secs => $2) RETURNING sha256`
			args = append(args, grace.Seconds())
		}
		if err := tx.QueryRow(ctx, q, args...).Scan(&sha); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM file WHERE sha256 = $1 AND cached)`, sha).Scan(&shared)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if shared {
		return true, nil
	}
	// A download can cache the same blob again at this moment. The proxy then sees a
	// missing blob and gets the file from the upstream again (pypi.Proxy.serveFile).
	return true, b.Delete(ctx, sha)
}
