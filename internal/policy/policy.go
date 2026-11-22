// Package policy decides if the proxy shows a file (ADR-0001 section 3.4, ADR-0008).
package policy

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Rule struct {
	ID      int64
	Project string
	Version string // empty: all versions
	Kind    string // "block", "allow", or "cooldown_override"
	Reason  string
}

type File struct {
	Version         string
	UploadTime      *time.Time
	FirstSeen       time.Time
	RemovedUpstream bool
}

type Status string

const (
	Visible  Status = "visible"
	Cooldown Status = "cooldown"
	Blocked  Status = "blocked"
	Allowed  Status = "allowed"
	Override Status = "override"
	Removed  Status = "removed"
)

func (s Status) Visible() bool { return s == Visible || s == Allowed || s == Override }

// ReleaseTime is the upload time, or the first-seen time if the upstream gives no upload time.
func (f File) ReleaseTime() time.Time {
	if f.UploadTime != nil {
		return *f.UploadTime
	}
	return f.FirstSeen
}

// Evaluate applies the rules of one project to f.
// Order: removed upstream, block, allow, cooldown override, cooldown.
func Evaluate(rules []Rule, f File, cooldown time.Duration, now time.Time) Status {
	if f.RemovedUpstream {
		return Removed
	}
	match := func(kind string) bool {
		for _, r := range rules {
			if r.Kind == kind && (r.Version == "" || strings.EqualFold(r.Version, f.Version)) {
				return true
			}
		}
		return false
	}
	switch {
	case match("block"):
		return Blocked
	case match("allow"):
		return Allowed
	case match("cooldown_override"):
		return Override
	case now.Before(f.ReleaseTime().Add(cooldown)):
		return Cooldown
	}
	return Visible
}

// Load returns the rules for project in the given repositories.
func Load(ctx context.Context, db *pgxpool.Pool, project string, repoIDs ...int64) ([]Rule, error) {
	rows, _ := db.Query(ctx, `SELECT id, project_name, coalesce(version, ''), kind, reason
		FROM policy WHERE project_name = $1 AND repository_id = ANY($2)`, project, repoIDs)
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Rule])
}
