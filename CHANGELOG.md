# Changelog

All important changes to this project are recorded in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Go service skeleton and a Docker Compose stack with PostgreSQL, SeaweedFS (S3), and Valkey. Copy `.env.example` to `.env`, then run `docker compose up`. `GET /healthz` returns `200` only when all three services are reachable.
- Blob storage on any S3-compatible endpoint. The key of a blob is its SHA-256. A blob with the wrong content is never visible.
- PostgreSQL schema for repositories, packages, files, accounts, robot accounts, roles, pull history, and policies. The service applies new migrations at start.
- ADR-0003 to ADR-0008: metadata schema, job queue, garbage collection, roles, web UI, and PyPI upstream resolution with a 14-day default cooldown.

### Changed

- Ginnungagap is now a pull-through proxy and cache for software packages ([ADR-0001](docs/explanation/adr/0001-rebuild-as-package-proxy.md)). The old malware investigation kit is stopped.
- The README describes the new project.

### Removed

- All code and configuration of the old malware investigation kit: the `isolator` sandbox, the Falco rules, the Go CLI, the honeypot script, and the Docker files. The new project does not use them.
