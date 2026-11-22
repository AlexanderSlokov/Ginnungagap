# ADR-0003: Metadata schema

| Field    | Value                          |
|----------|--------------------------------|
| Status   | Accepted                       |
| Date     | 2026-11-22                     |
| Decider  | Founder (`@AleksandrSlokov`)   |
| Recorder | Claude (BA and secretary role) |

## 1. Context

[ADR-0001](0001-rebuild-as-package-proxy.md) selects PostgreSQL as the metadata store.
The blob storage keeps the artifact files.
The metadata store keeps all other data.

## 2. Decision

The schema has these tables:

| Table        | Contents                                                                 |
|--------------|--------------------------------------------------------------------------|
| `repository` | One row for each repository: name, format (`pypi`), type (`proxy` or `group`), and the members of a group in priority order. |
| `upstream`   | The upstream index URL of a proxy repository.                            |
| `project`    | One row for each package name in a repository. It records the upstream that supplies this name (refer to [ADR-0008](0008-pypi-upstream-and-cooldown.md)). |
| `file`       | One row for each artifact file: file name, version, SHA-256, size, upstream URL, upload time, first-seen time, cached flag, last pull time, pin flag, and GC mark time. |
| `pull_log`   | One row for each download: file, user or robot account, time.            |
| `policy`     | Blocklist and allowlist rules, and cooldown overrides.                    |
| `account`    | Local user accounts and their role on each repository. (`user` is a reserved word in PostgreSQL.) |
| `robot`      | Robot accounts: name, token hash, expiry time, role on each repository.  |

The migration files in the code define the columns.
The SHA-256 of a file is its blob key in the blob storage.

## 3. Consequences

- The same file in two repositories uses one blob.
- GC must not remove a blob while another `file` row refers to it.
