# Ginnungagap

![Bombastic Side Eyes Cat](docs/images/bombastic_side_eyes_cat.png)

A Sonatype Nexus, but free.

Ginnungagap is a pull-through proxy and cache for software packages.
It puts one controlled point between your builds and the public package registries.

> **Status:** Ginnungagap is in development. No release is available yet.
> Refer to [ROADMAP.md](ROADMAP.md) for the plan.

## Why

Public package ecosystems such as PyPI, npm, and crates.io have weak supply chain security.
Attacks on these ecosystems occur frequently.

Builds also download the same packages many times.
When one build step fails, the next build downloads all packages again.
A plain cache (for example, NGINX) makes builds faster,
but it does not let you see or control what it keeps.

## What it does

- **Cache.** Ginnungagap gets a package from the upstream registry when a client requests it.
  Then it keeps the package and serves it to the next clients.
- **Cooldown.** Ginnungagap does not serve a new version until the version is older than a set age.
  Registries often remove malicious versions before this age.
- **Blocklist and allowlist.** An operator can block or allow a package or a version.
- **Management.** The web UI shows the cached packages, the storage size, and the pull history.
  An operator can remove or block a version.
- **Access control.** Each user gets a role on a repository. CI uses robot accounts.

## Supported ecosystems

| Version | Ecosystem                                              |
|---------|--------------------------------------------------------|
| `0.1.0` | PyPI, with extra indexes such as PyTorch and CUDA      |
| `0.2.0` | npm                                                    |
| `0.3.0` | Cargo (crates.io)                                      |
| `2.0.0` | Container images (Docker / OCI)                        |
| `3.0.0` | AI models                                              |

## Stack

- Backend and web UI: Go.
- Blob storage: any S3-compatible endpoint. The default stack uses SeaweedFS.
- Metadata: PostgreSQL.
- Job queue: Valkey.
- Deployment: Docker Compose.

## Documentation

- [ADR-0001: Rebuild Ginnungagap as a package proxy and cache](docs/explanation/adr/0001-rebuild-as-package-proxy.md)
- [Architecture](docs/explanation/architecture.md)

## License

[GNU Affero General Public License v3.0](LICENSE).
