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

## Quick start

1. Copy the example configuration, then change the passwords in `.env`:

   ```sh
   cp .env.example .env
   ```

2. Start the stack:

   ```sh
   docker compose up -d --build
   ```

3. Open <http://localhost:8080> and log in as `admin` with `GINNUNGAGAP_ADMIN_PASSWORD`.

4. Install a package through the proxy:

   ```sh
   pip install --index-url http://admin:PASSWORD@localhost:8080/pypi/python/simple/ six
   ```

On the first start, Ginnungagap creates three repositories:

| Repository | Type  | Upstream                                  |
|------------|-------|-------------------------------------------|
| `pypi`     | proxy | `https://pypi.org/simple`                 |
| `pytorch`  | proxy | `https://download.pytorch.org/whl/cu128`  |
| `python`   | group | `pypi`, then `pytorch`. `torch`, `torchvision`, and `torchaudio` route to `pytorch`. |

> New versions are hidden for 14 days (the cooldown).
> To get a newer version now, add a "cooldown override" or an "allow" rule on the package page.

## Tests

```sh
go test ./...            # unit tests only
test/integration.sh      # all Go tests with PostgreSQL, SeaweedFS, and Valkey
test/e2e.sh              # pip and uv through the full stack (needs internet access)
```

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
