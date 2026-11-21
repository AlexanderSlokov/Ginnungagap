# Ginnungagap architecture

This document describes the architecture of Ginnungagap.
It also records the open design questions and the research for each question.
[ADR-0001](adr/0001-rebuild-as-package-proxy.md) gives the base decisions.

## 1. Components

| Component     | Function                                                        |
|---------------|-----------------------------------------------------------------|
| Proxy         | Receives requests from package clients (`pip`, `uv`, `npm`, `cargo`). Sends the cached artifact, or gets it from the upstream registry. |
| Policy engine | Applies the cooldown, the blocklist, and the allowlist. Later, it applies the advisory check. |
| Job queue     | Runs background jobs, for example warmup and garbage collection. |
| Blob storage  | Keeps the artifact files. It is an S3-compatible endpoint. The default is SeaweedFS. |
| Metadata store| Keeps the package index, the pull history, and the policy data. It is PostgreSQL. |
| Web UI        | Shows the cache contents. Lets an operator manage packages and policies. |

## 2. Open design questions

### 2.1 Cache warmup (ADR-0002)

**Status:** Unknown.

The Founder's requirement:
the cache must keep each artifact that it proxies,
and it must get the packages that users need before they request them.
The method to know "the packages that users need" is not defined.

**Research:**

- JFrog Artifactory: a remote repository is a proxy, not a mirror.
  It does not get artifacts before a client requests them.
  Pull replication can fill the cache before requests,
  but only when the source is another Artifactory instance.
- Sonatype Nexus: a proxy repository gets an artifact only when a client requests it.
  The recorder did not find a warmup function in Nexus.

**Possible mechanisms (not decided):**

1. The user sends a lockfile (`uv.lock`, `requirements.txt`, `package-lock.json`, `Cargo.lock`).
   The system puts one job for each artifact into the queue.
2. The system refreshes the packages that users pulled frequently in the past.
3. The system gets new versions of pinned packages after the cooldown ends.

**Next step:** Do more research. Find how other proxies do warmup.

### 2.2 Metadata store (ADR-0003)

**Status:** PostgreSQL is decided. The schema is open.

**Research:**

- Nexus Repository supports an embedded H2 database and an external PostgreSQL database.
- Sonatype recommends PostgreSQL for production.
- Sonatype does not support H2 for container-based deployments.
- The Nexus PostgreSQL setup needs the `pg_trgm` (trigram) extension.

**Next step:** Read the Nexus data model before we design the schema.

### 2.3 Job queue (ADR-0004)

**Status:** Open.

The queue runs background jobs.

**Recommendation from the recorder:**
use a queue that keeps its jobs in PostgreSQL, for example River (`riverqueue`).
This option adds no new service to the Docker Compose stack.

### 2.4 Garbage collection (ADR-0005)

**Status:** Unknown.

**Research:**

- Nexus uses two steps.
  First, a cleanup policy marks components as deleted in the database (soft delete).
  Then, the "Admin - Compact blob store" task removes the files from the blob store.
- Sonatype published an advisory about a cleanup bug that deleted binaries by mistake.
  The design must prevent this type of error.
- Artifactory removes cached artifacts that nobody used for a set period.

### 2.5 Access control (ADR-0006)

**Status:** Use Harbor as a reference.

**Research (Harbor):**

- Project roles: Limited Guest, Guest, Developer, Maintainer, Project Admin.
- Robot accounts: for CI. They cannot log in to the UI.
  Their tokens expire. Harbor shows the secret one time only.
- User accounts can come from OIDC or LDAP. Directory groups can map to project roles.

### 2.6 Frontend (ADR-0007)

**Status:** Open.

**Requirement:** The UI must be clean and easy to use for daily work.

**Recommendation from the recorder:**
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
- [JFrog: Remote repositories](https://docs.jfrog.com/artifactory/docs/remote-repositories)
- [JFrog: Pull replication](https://jfrog.com/help/r/jfrog-artifactory-documentation/pull-replication)
- [Harbor: User permissions by role](https://goharbor.io/docs/edge/administration/managing-users/user-permissions-by-role)
- [Harbor: Robot accounts](https://goharbor.io/docs/edge/administration/robot-accounts/)
- [pnpm: Mitigating supply chain attacks](https://pnpm.io/supply-chain-security)
- [Andrew Nesbitt: Package managers need to cool down](https://nesbitt.io/2026/03/04/package-managers-need-to-cool-down)
