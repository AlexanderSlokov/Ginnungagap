# ADR-0001: Rebuild Ginnungagap as a package proxy and cache

| Field    | Value                          |
|----------|--------------------------------|
| Status   | Accepted                       |
| Date     | 2026-11-21 (proposed), 2026-11-22 (accepted) |
| Decider  | Founder (`@AleksandrSlokov`)   |
| Recorder | Claude (BA and secretary role) |

## 1. Context

### 1.1 The problem

The Founder builds Docker images for AI compute.
These builds download many packages from PyPI, npm, and crates.io.
The builds take a long time.
When one build step fails, the next build downloads all packages again.

The Founder made a pull-through cache with NGINX.
This cache stores packages, but it has no management functions.
An operator cannot see which packages are in the cache.
An operator cannot remove or block one package version.
If the cache gets a package that contains malware, nobody knows.

The "by devs, for devs" ecosystems (PyPI, npm, crates.io) have weak supply chain security.
Supply chain attacks on these ecosystems occur frequently.

### 1.2 The old project

The old Ginnungagap project was a malware investigation kit.
It ran npm packages in an isolated container and monitored them with Falco.
The Founder stops this project.
The new project does not keep code or design from the old project.

### 1.3 Market

Sonatype Nexus Repository and JFrog Artifactory solve this problem.
Their supply chain security functions are paid features
(Nexus Firewall quarantine, JFrog Curation).
The Nexus Community Edition is free, but it has usage limits.

## 2. Decision drivers

- Builds must not download the same package from the internet two times.
- An operator must see and control the cached packages.
- The system must decrease the risk of malware from upstream registries.
- The system must be free and open source.
- The system must be simple to operate.
- The Founder wants a small and simple stack.

## 3. Decision

### 3.1 Purpose

We remove all contents of the old project.
We build a new project with the same name: Ginnungagap.

Ginnungagap is "a Sonatype Nexus, but free".
It is a pull-through proxy and cache for software packages.
For packages, the proxy gets an artifact only when a client requests it (on demand), as Nexus does.
For container images, an operator can also select a repository and tags to pull in advance, as Harbor does.
In the future, it becomes a central logistics gateway for the software supply chain.
This gateway supplies all the artifacts that a team needs to build a product.

### 3.2 Release plan

| Version | Scope                                                    |
|---------|----------------------------------------------------------|
| `0.1.0` | PyPI proxy. This includes extra indexes, such as PyTorch and CUDA wheel indexes. |
| `0.2.0` | npm proxy.                                               |
| `0.3.0` | Cargo (crates.io) proxy.                                 |
| `2.0.0` | Container image registry proxy (Docker / OCI). This includes pull in advance of selected tags. |
| `3.0.0` | AI model registry proxy.                                 |

The scope of `1.0.0` is not defined.
We define it after the MVP is complete.

### 3.3 Technical baseline

| Area            | Decision                                                                 |
|-----------------|--------------------------------------------------------------------------|
| Backend         | Go.                                                                      |
| Frontend        | Go server-side rendering. If this is not possible, a web UI that is embedded in the Go binary. |
| Blob storage    | Any S3-compatible endpoint. The default Docker Compose stack uses SeaweedFS. |
| Metadata store  | PostgreSQL.                                                              |
| Job queue       | Valkey or Redis, as a separate service. The database keeps data. The queue keeps jobs. |
| Garbage collection | Two steps, as Nexus does. First, mark the items in the database. Then, remove the files from blob storage. |
| Deployment      | Docker Compose.                                                          |
| Access control  | Role-based access control. A combination of the Nexus and Harbor models. Refer to 3.6. |
| License         | AGPL-3.0 (no change; the repository already uses it).                    |

### 3.4 Supply chain protection

We add these protections in this order:

1. **Cooldown.** The proxy does not serve a version until it is older than a set age.
2. **Blocklist and allowlist.** An operator can block or allow a package or a version manually.
3. **Advisory check.** The proxy checks versions against OSV.dev, which includes known malicious packages.

Items 1 and 2 are in the first release.
Item 3 is in a later release.

### 3.5 Management functions

The UI and the API must let an operator do these tasks:

- See the cached packages and versions.
- See the storage size.
- See who pulled a package, and when.
- Remove or purge a package or a version.
- Block a version.

The UI must be clean and easy to use for daily work.
The design direction is industrial, neo-brutalism, with the clean look of e-ink.
A follow-up ADR selects the color palette.

### 3.6 Access control

We use a combination of the Nexus model and the Harbor model.

- **Resource model (from Nexus).** The unit of access is the repository.
  A repository has a format (PyPI, npm, Cargo, and later Docker) and a type (proxy or group).
  One design supports all formats.
- **Permission model (from Harbor).** Each member gets one fixed role on a repository or on a repository group.
- **Robot accounts (from Harbor).** CI uses robot accounts.
  A robot account cannot log in to the UI.
  Its token expires.
  The system shows the secret one time only.
- **Directory groups (from Harbor).** The system can map OIDC groups to roles.

We do not use these Nexus functions in the first releases:

- Free-form privilege strings.
- Content selectors.

Reason: the Harbor permission model is simple to configure.
The Project unit of Harbor is a container image concept.
PyPI and crates.io have no namespace that matches a Harbor project.

## 4. Options that we considered

### 4.1 Keep the NGINX pull-through cache

- Good: It works now, and it is simple.
- Bad: It has no management functions.
- Bad: It has no supply chain protection.
- Rejected.

### 4.2 Use Sonatype Nexus Community Edition or JFrog Artifactory

- Good: These are mature products.
- Bad: The supply chain protection functions are paid features.
- Bad: The free edition of Nexus has usage limits.
- Rejected.

### 4.3 Continue the old Ginnungagap (malware investigation kit)

- Good: Some code exists.
- Bad: It does not solve the build speed problem.
- Bad: It finds malware after the package runs. A proxy can stop the package before it gets to the build.
- Rejected.

### 4.4 Build a new proxy and cache (selected)

- Good: It solves the build speed problem and the management problem.
- Good: It puts supply chain protection at one central point.
- Bad: It is a large amount of new work.
- Selected.

## 5. Consequences

### 5.1 Good consequences

- Builds get packages from a local cache. Failed builds do not download packages again.
- One central point controls which packages go into the builds.
- The default stack is self-contained in Docker Compose.

### 5.2 Bad consequences and risks

- The proxy becomes a single point of failure for all builds.
- The cooldown can stop urgent security fixes. The design must include an override.
- Each package ecosystem has a different protocol. Each ecosystem needs its own adapter.
- PostgreSQL and the queue service add two more services to operate.

### 5.3 Required actions

1. Remove all old project files, except `LICENSE` and `docs/images/bombastic_side_eyes_cat.png`.
2. Write a new `README.md`.
3. After the Founder accepts this ADR, update `ROADMAP.md`.

## 6. Follow-up decisions

These items need a separate ADR or design document.
[The architecture document](../architecture.md) records the open questions and the research.

| ID       | Topic                                   | Status  |
|----------|-----------------------------------------|---------|
| ADR-0002 | Pull in advance of container image tags | For `2.0.0`. Use Harbor as a reference. |
| ADR-0003 | Metadata schema in PostgreSQL           | Research Nexus first. |
| ADR-0004 | Job queue: Valkey or Redis, and the Go client library | Open. |
| ADR-0005 | Garbage collection: mark rules and sweep schedule | Open. The two-step method is decided. |
| ADR-0006 | Access control: role list and permissions for each role | The model is decided (refer to 3.6). The role list is open. |
| ADR-0007 | Frontend technology, visual design, and color palette | Open. |
