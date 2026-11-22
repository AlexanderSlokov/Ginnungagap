# Ginnungagap architecture

This document describes the architecture of Ginnungagap.
It also records the open design questions and the research for each question.
[ADR-0001](adr/0001-rebuild-as-package-proxy.md) gives the base decisions.

## 1. Components

| Component     | Function                                                        |
|---------------|-----------------------------------------------------------------|
| Proxy         | Receives requests from package clients (`pip`, `uv`, `npm`, `cargo`). Sends the cached artifact, or gets it from the upstream registry. |
| Policy engine | Applies the cooldown, the blocklist, and the allowlist. Later, it applies the advisory check. |
| Job queue     | Keeps the background jobs, for example garbage collection. It is Valkey or Redis, as a separate service. |
| Worker        | Takes jobs from the queue and runs them.                        |
| Blob storage  | Keeps the artifact files. It is an S3-compatible endpoint. The default is SeaweedFS. |
| Metadata store| Keeps the package index, the pull history, and the policy data. It is PostgreSQL. |
| Web UI        | Shows the cache contents. Lets an operator manage packages and policies. |

The database and the queue have different functions.
Do not keep jobs in PostgreSQL.
Do not keep metadata in the queue.

## 2. Design questions

### 2.1 Cache fill (ADR-0002)

**Status:** Decided.

- **Packages (`0.1.0` to `0.3.0`):** The proxy gets an artifact only when a client requests it.
  This is the same as Nexus and Artifactory.
  There is no warmup function for packages.
- **Container images (`2.0.0`):** An operator selects a repository and the tags.
  The system pulls these tags before a client requests them.
  This is the same as a Harbor pull-based replication rule.

**Research:**

- JFrog Artifactory: a remote repository is a proxy, not a mirror.
  It does not get artifacts before a client requests them.
  Pull replication can fill the cache before requests,
  but only when the source is another Artifactory instance.
- Sonatype Nexus: a proxy repository gets an artifact only when a client requests it.
- Harbor: a proxy cache project gets an image on the first pull.
  A replication rule can copy selected tags in advance.
  The rule can filter tags by a list, a range, or a wildcard pattern.
  Harbor sets a 7-day retention policy on a new proxy cache project.

### 2.2 Metadata store (ADR-0003)

**Status:** PostgreSQL is decided. The schema is open.

**Research:**

- Nexus Repository supports an embedded H2 database and an external PostgreSQL database.
- Sonatype recommends PostgreSQL for production.
- Sonatype does not support H2 for container-based deployments.
- The Nexus PostgreSQL setup needs the `pg_trgm` (trigram) extension.

**Next step:** Read the Nexus data model before we design the schema.

### 2.3 Job queue (ADR-0004)

**Status:** Valkey or Redis is decided. The final selection and the Go client library are open.

**Research:**

- Valkey is a fork of Redis 7.2.4 under the Linux Foundation. Its license is BSD 3-Clause.
- Redis 8 has three license options: RSALv2, SSPLv1, and AGPLv3.
- Both licenses are compatible with the AGPL-3.0 license of Ginnungagap.

**Recommendation from the recorder:** Valkey.
The BSD license gives the fewest limits to users who deploy the stack.

### 2.4 Garbage collection (ADR-0005)

**Status:** The two-step method is decided. The mark rules and the schedule are open.

The method:

1. **Mark.** A cleanup policy marks components as deleted in the database (soft delete).
2. **Sweep.** A separate job removes the files from blob storage.

**Research:**

- Nexus uses this method.
  The "Admin - Compact blob store" task does step 2.
- Sonatype published an advisory about a cleanup bug that deleted binaries by mistake.
  The design must prevent this type of error.
- Artifactory removes cached artifacts that nobody used for a set period.

### 2.5 Access control (ADR-0006)

**Status:** Decided in [ADR-0001, section 3.6](adr/0001-rebuild-as-package-proxy.md#36-access-control). The role list is open.

**Research (Nexus):**

- The unit of access is the **repository**. A repository has a format (PyPI, npm, Docker, and others) and a type (proxy, hosted, group).
- A privilege has the pattern `repository-view:{format}:{repository}:{action}`.
  The actions are browse, read, edit, add, and delete.
- A content selector limits a privilege to a path or a namespace in a repository.
- A role contains privileges and other roles.
- Disadvantage: the model is very flexible, but it is difficult to configure.
  Docker access for each team needs its own content selectors and privileges.

**Research (Harbor):**

- The unit of access is the **project**. A project contains container image repositories.
- Each project member has one fixed role: Limited Guest, Guest, Developer, Maintainer, or Project Admin.
- Robot accounts are for CI. They cannot log in to the UI.
  Their tokens expire. Harbor shows the secret one time only.
- User accounts can come from OIDC or LDAP. Directory groups can map to project roles.
- Disadvantage: the project is a container image concept.
  PyPI and crates.io have no namespace that matches a project.

**Decision:** Use a combination of the two models.

- **Resource model from Nexus.** The unit of access is the repository.
  This model supports all formats with one design.
  A Docker proxy is one more repository format.
- **Permission model from Harbor.** Each member gets one fixed role on a repository or a repository group.
  Use robot accounts for CI.
  Map OIDC groups to roles.
- Do not use free-form privilege strings or content selectors in the first releases.

### 2.6 Frontend (ADR-0007)

**Status:** The design direction is decided. The technology and the color palette are open.

**Requirements:**

- The UI must be clean and easy to use for daily work.
- Style: industrial and neo-brutalism.
- Aesthetic: the clean look of e-ink.
  Use high contrast, flat colors, and sharp edges.
  Do not use gradients, blur, or many animations.
- Mood reference: the color scheme and the interface of the game *Arknights: Endfield*.
  Use it only as a reference.
  Do not copy its logos, fonts, icons, or images.
- Text contrast must meet WCAG 2.2 level AA.

**Recommendation from the recorder (technology):**
use server-side rendering with `templ` and `htmx`.
All the UI code is in Go, and the result is one binary.

### 2.7 Cooldown

**Status:** Decided in ADR-0001. The details are open.

**Research:**

- pnpm has `minimumReleaseAge` since version 10.16 (value in minutes).
- uv has `exclude-newer`. Since version 0.9.17, it accepts a relative duration, for example `7 days`.
- One analysis found that 8 of 10 supply chain attacks were active for less than one week.
  A cooldown of 7 days can stop most of these attacks.
- Dependabot sets the cooldown to 0 for security updates.
  Ginnungagap must also have an override for urgent fixes.

## 3. References

- [Sonatype: Nexus Repository database options](https://help.sonatype.com/repomanager3/planning-your-implementation/database-options)
- [Sonatype: Install Nexus Repository with a PostgreSQL database](https://help.sonatype.com/en/install-nexus-repository-with-a-postgresql-database.html)
- [Sonatype: Cleanup policies](https://help.sonatype.com/repomanager3/nexus-repository-administration/repository-management/cleanup-policies)
- [Sonatype: Critical cleanup policy bug advisory](https://help.sonatype.com/repomanager3/product-information/critical-cleanup-policy-bug-advisory)
- [Sonatype: Privileges](https://help.sonatype.com/repomanager3/nexus-repository-administration/access-control/privileges)
- [Sonatype: Content selectors](https://help.sonatype.com/repomanager3/nexus-repository-administration/access-control/content-selectors)
- [Sonatype: Content selectors and Docker](https://help.sonatype.com/repomanager3/nexus-repository-administration/formats/docker-registry/content-selectors-and-docker)
- [JFrog: Remote repositories](https://docs.jfrog.com/artifactory/docs/remote-repositories)
- [JFrog: Pull replication](https://jfrog.com/help/r/jfrog-artifactory-documentation/pull-replication)
- [Harbor: User permissions by role](https://goharbor.io/docs/edge/administration/managing-users/user-permissions-by-role)
- [Harbor: Robot accounts](https://goharbor.io/docs/edge/administration/robot-accounts/)
- [Harbor: Configure proxy cache](https://goharbor.io/docs/main/administration/configure-proxy-cache/)
- [InfoQ: Redis adds the AGPL license](https://www.infoq.com/news/2025/05/redis-agpl-license)
- [pnpm: Mitigating supply chain attacks](https://pnpm.io/supply-chain-security)
- [Andrew Nesbitt: Package managers need to cool down](https://nesbitt.io/2026/03/04/package-managers-need-to-cool-down)
