package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

// Role is a fixed role on a repository (ADR-0006). A higher value has more permissions.
type Role int

const (
	RoleNone Role = iota
	RoleGuest
	RoleDeveloper
	RoleMaintainer
	RoleAdmin
)

var roleNames = []string{"none", "guest", "developer", "maintainer", "admin"}

func (r Role) String() string { return roleNames[r] }

func ParseRole(s string) Role {
	for i, n := range roleNames {
		if n == s {
			return Role(i)
		}
	}
	return RoleNone
}

type Repository struct {
	ID           int64
	Name         string
	Format       string
	Type         string // "proxy" or "group"
	CooldownDays int
	UpstreamURL  string // proxy only
}

const repoColumns = `r.id, r.name, r.format, r.type, r.cooldown_days, coalesce(u.url, '')
	FROM repository r LEFT JOIN upstream u ON u.repository_id = r.id`

func scanRepos(rows pgx.Rows) ([]Repository, error) {
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Repository, error) {
		var r Repository
		err := row.Scan(&r.ID, &r.Name, &r.Format, &r.Type, &r.CooldownDays, &r.UpstreamURL)
		return r, err
	})
}

func GetRepository(ctx context.Context, db *pgxpool.Pool, name string) (Repository, error) {
	rows, _ := db.Query(ctx, `SELECT `+repoColumns+` WHERE r.name = $1`, name)
	repos, err := scanRepos(rows)
	if err != nil {
		return Repository{}, err
	}
	if len(repos) == 0 {
		return Repository{}, ErrNotFound
	}
	return repos[0], nil
}

func ListRepositories(ctx context.Context, db *pgxpool.Pool) ([]Repository, error) {
	rows, _ := db.Query(ctx, `SELECT `+repoColumns+` ORDER BY r.name`)
	return scanRepos(rows)
}

// Members returns the members of a group repository in priority order.
func Members(ctx context.Context, db *pgxpool.Pool, groupID int64) ([]Repository, error) {
	rows, _ := db.Query(ctx, `SELECT `+repoColumns+`
		JOIN repository_member m ON m.member_id = r.id
		WHERE m.group_id = $1 ORDER BY m.priority`, groupID)
	return scanRepos(rows)
}

// Bootstrap creates the first admin account and the default repositories on an empty database.
func Bootstrap(ctx context.Context, db *pgxpool.Pool, adminName, adminPasswordHash string) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('ginnungagap.bootstrap'))`); err != nil {
			return err
		}
		if adminPasswordHash != "" {
			if _, err := tx.Exec(ctx, `INSERT INTO account (name, password_hash, is_admin)
				SELECT $1, $2, true WHERE NOT EXISTS (SELECT 1 FROM account)`, adminName, adminPasswordHash); err != nil {
				return err
			}
		}
		var empty bool
		if err := tx.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM repository)`).Scan(&empty); err != nil || !empty {
			return err
		}
		// Default repositories: "python" is a group of PyPI and the PyTorch CUDA 12.8 index.
		// PyPI has priority. The PyTorch packages are routed to the PyTorch index (ADR-0008).
		_, err := tx.Exec(ctx, `
			WITH pypi AS (INSERT INTO repository (name, format, type) VALUES ('pypi', 'pypi', 'proxy') RETURNING id),
			torch AS (INSERT INTO repository (name, format, type) VALUES ('pytorch', 'pypi', 'proxy') RETURNING id),
			grp AS (INSERT INTO repository (name, format, type) VALUES ('python', 'pypi', 'group') RETURNING id),
			up AS (INSERT INTO upstream (repository_id, url)
				SELECT id, 'https://pypi.org/simple' FROM pypi
				UNION ALL SELECT id, 'https://download.pytorch.org/whl/cu128' FROM torch),
			mem AS (INSERT INTO repository_member (group_id, member_id, priority)
				SELECT grp.id, pypi.id, 1 FROM grp, pypi
				UNION ALL SELECT grp.id, torch.id, 2 FROM grp, torch)
			INSERT INTO project (repository_id, name, upstream_id)
			SELECT grp.id, n, torch.id FROM grp, torch, unnest(ARRAY['torch', 'torchvision', 'torchaudio']) AS n`)
		return err
	})
}
