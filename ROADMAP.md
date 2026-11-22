# Roadmap

This roadmap shows the planned capabilities of Ginnungagap, with a phase and a target version.
[ADR-0001](docs/explanation/adr/0001-rebuild-as-package-proxy.md) is the source of this roadmap.
Each item moves to `BACKLOG.md` before work starts.

## Phase 0: Reset

Target version: `0.1.0`

- Remove all files of the old project, except `LICENSE` and `docs/images/bombastic_side_eyes_cat.png`.
- Write a new `README.md`.

## Phase 1: Package proxy (MVP)

### `0.1.0`: PyPI

- PyPI pull-through proxy. The proxy gets an artifact only when a client requests it.
- Extra upstream indexes, for example the PyTorch and CUDA wheel indexes.
- Blob storage on any S3-compatible endpoint. The default stack uses SeaweedFS.
- Metadata store in PostgreSQL (ADR-0003).
- Job queue in Valkey, as a separate service (ADR-0004).
- Garbage collection in two steps: mark, then sweep (ADR-0005).
- Cooldown (default 14 days) and upstream resolution that prevents dependency confusion (ADR-0008). A Maintainer can override the cooldown for urgent fixes.
- Blocklist and allowlist for packages and versions.
- Access control: repositories, fixed roles, and robot accounts (ADR-0001, section 3.6; ADR-0006).
- Web UI for management tasks:
  see the cached packages and versions, see the storage size, see the pull history,
  remove or purge a version, and block a version (ADR-0007).
- Deployment with Docker Compose.

### `0.2.0`: npm

- npm pull-through proxy, with the same policies as PyPI.

### `0.3.0`: Cargo

- Cargo (crates.io) pull-through proxy, with the same policies as PyPI.

## Phase 2: Not scheduled

The target versions of these items are not set.

- Advisory check with OSV.dev, which includes known malicious packages.
- OIDC login and the map from OIDC groups to roles.
- The scope of `1.0.0`. We define it after the MVP is complete.

## Phase 3: Container images

### `2.0.0`: Docker / OCI registry proxy

- Pull-through proxy for container images.
- Pull in advance: an operator selects a repository and tags, and the system pulls them before a client requests them (ADR-0002).

## Phase 4: AI models

### `3.0.0`: AI model registry proxy

- Pull-through proxy for AI models.
