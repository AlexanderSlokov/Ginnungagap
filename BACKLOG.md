# Backlog

This backlog divides the items of [ROADMAP.md](ROADMAP.md) into tasks.
When a task is complete, remove it from this file and record the result in `CHANGELOG.md`.

## Phase 1: Package proxy (MVP), `0.1.0`: PyPI

The ADRs for this phase are accepted: ADR-0003 to ADR-0008 in `docs/explanation/adr/`.

### Build and deployment

#### B1: Go module, configuration, and Docker Compose stack

- **Files:** `go.mod`, `go.sum`, `cmd/ginnungagap/main.go`, `internal/config/`, `compose.yaml`, `Dockerfile`, `.env.example`.
- **Dependencies:** None.
- **Goal:** `docker compose up` starts PostgreSQL, SeaweedFS (S3 endpoint), Valkey, and Ginnungagap.
  Ginnungagap reads its configuration from environment variables.
  `GET /healthz` returns `200` only when PostgreSQL, S3, and Valkey are reachable.

### Storage

#### B2: Blob storage on S3

- **Files:** `internal/blob/`.
- **Dependencies:** B1.
- **Goal:** The service can put, get (as a stream), check, and delete a blob by its SHA-256 key on any S3-compatible endpoint.
  A test runs against the SeaweedFS container.

#### B3: Metadata migrations

- **Files:** `internal/store/`, `internal/store/migrations/`.
- **Dependencies:** B1.
- **Goal:** At start, the service applies the migrations for the tables of ADR-0003.
  A second start makes no change.

### PyPI proxy

#### B4: PyPI Simple API proxy

- **Files:** `internal/pypi/`.
- **Dependencies:** B2, B3.
- **Goal:**
  - The proxy serves the Simple API (PEP 503 HTML and PEP 691 JSON) for proxy and group repositories.
  - It reads JSON and HTML from upstreams. It reads `upload-time` and `data-upload-time` (ADR-0008).
  - It rewrites file links to point to the proxy.
  - On the first download, it gets the file from the upstream, checks the SHA-256, and keeps the file in S3.
  - A group repository uses the first upstream that has the package name (ADR-0008).

#### B5: Cooldown, blocklist, and allowlist

- **Files:** `internal/policy/`, `internal/pypi/`.
- **Dependencies:** B4.
- **Goal:** The index does not show a file that is newer than the cooldown (default 14 days) or that the blocklist blocks.
  A cooldown override or an allowlist rule shows the file.
  A direct download of a hidden file returns `403`.

### Jobs

#### B6: Job queue and worker

- **Files:** `internal/jobs/`, `cmd/ginnungagap/main.go`.
- **Dependencies:** B1.
- **Goal:** The service runs asynq on Valkey (ADR-0004).
  It runs periodic jobs.
  A test runs a job against the Valkey container.

#### B7: Garbage collection

- **Files:** `internal/gc/`, `internal/jobs/`.
- **Dependencies:** B3, B6.
- **Goal:** The mark job and the sweep job follow ADR-0005.
  Tests show that a pinned file and a file that another row refers to are not removed.

### Access control

#### B8: Accounts, roles, and robot tokens

- **Files:** `internal/auth/`, `internal/store/migrations/`.
- **Dependencies:** B3.
- **Goal:** Each request checks the role of ADR-0006 on the repository.
  `pip` and `uv` can authenticate with HTTP basic authentication.
  The system keeps only a hash of each robot token. It shows the token one time only.

### Web UI

#### B9: Management UI

- **Files:** `internal/web/`.
- **Dependencies:** B4, B5, B7, B8.
- **Goal:** With templ, htmx, and the Nord palette (ADR-0007), an operator can:
  see the cached packages, the versions, the storage size, and the pull history;
  pin, purge, and block a version; add a cooldown override;
  see the files that GC marked.

### Tests

#### B10: End-to-end test

- **Files:** `test/e2e/`.
- **Dependencies:** B4, B5, B8.
- **Goal:** In the Compose stack, `pip install` and `uv sync` get packages through the proxy, with PyPI and the PyTorch index as upstreams.
  A second install gets all files from the cache.
