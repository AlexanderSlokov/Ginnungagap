# Changelog

All important changes to this project are recorded in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `scripts/update-htmx.sh` updates the embedded htmx file from the npm registry and checks its integrity hash. `internal/web/vendor.txt` records the version and the SHA-256.
- Go service skeleton and a Docker Compose stack with PostgreSQL, SeaweedFS (S3), and Valkey. Copy `.env.example` to `.env`, then run `docker compose up`. `GET /healthz` returns `200` only when all three services are reachable.
- Blob storage on any S3-compatible endpoint. The key of a blob is its SHA-256. A blob with the wrong content is never visible.
- PostgreSQL schema for repositories, packages, files, accounts, robot accounts, roles, pull history, and policies. The service applies new migrations at start.
- PyPI pull-through proxy at `/pypi/{repository}/simple/`. It reads JSON and HTML upstream indexes, keeps each file in S3 on the first download, checks the SHA-256, and serves PEP 658 metadata. A file that the upstream removes is not served any more.
- Group repositories. A package name uses the first member that has it, and the route stays (ADR-0008). This prevents dependency confusion.
- Default repositories on the first start: `pypi`, `pytorch` (CUDA 12.8 index), and the group `python`. `torch`, `torchvision`, and `torchaudio` route to `pytorch`.
- Cooldown (default 14 days), blocklist, allowlist, and cooldown overrides. A hidden file returns `403`. A mirrored file without an upload time uses the upload time of the same SHA-256 from another upstream.
- Accounts, robot accounts (`robot+NAME`, token shown one time), and the roles guest, developer, maintainer, and admin on each repository.
- Garbage collection every hour on Valkey with asynq: mark when the cache is larger than `GC_STORAGE_LIMIT`, sweep after 24 hours. Pinned files stay.
- Web UI (templ, htmx, Nord palette): repositories, packages, files with their status, pull history, policies, routes, members, robot accounts, accounts, and storage.
- `test/integration.sh` runs the Go tests against PostgreSQL, SeaweedFS, and Valkey. `test/e2e.sh` installs packages with pip and uv through the stack.
- ADR-0003 to ADR-0008: metadata schema, job queue, garbage collection, roles, web UI, and PyPI upstream resolution with a 14-day default cooldown.

### Changed

- The web UI uses htmx 2.0.11 (was 2.0.4).
- Ginnungagap is now a pull-through proxy and cache for software packages ([ADR-0001](docs/explanation/adr/0001-rebuild-as-package-proxy.md)). The old malware investigation kit is stopped.
- The README describes the new project.

### Removed

- All code and configuration of the old malware investigation kit: the `isolator` sandbox, the Falco rules, the Go CLI, the honeypot script, and the Docker files. The new project does not use them.
