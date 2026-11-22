# ADR-0005: Garbage collection

| Field    | Value                          |
|----------|--------------------------------|
| Status   | Accepted                       |
| Date     | 2026-11-22                     |
| Decider  | Founder (`@AleksandrSlokov`)   |
| Recorder | Claude (BA and secretary role) |

## 1. Context

[ADR-0001](0001-rebuild-as-package-proxy.md) selects a two-step method: mark, then sweep.
Sonatype published an advisory about a cleanup bug that deleted binaries by mistake.
The rules must prevent this type of error.

## 2. Decision

### 2.1 Mark

- The operator sets a storage limit.
- When the total size is more than the limit, the mark job marks files.
  It marks the file with the oldest last pull time first.
  It stops when the total size of the unmarked files is less than the limit.
- The mark job does not mark a pinned file.
- When a client pulls a marked file, the system removes the mark.

### 2.2 Sweep

- The sweep job removes a blob only if all these conditions are true:
  1. The mark is older than the grace period.
  2. No unmarked `file` row refers to the blob.
- Then the sweep job sets the `file` row to "not cached". It keeps the row.

> **Amendment (2026-11-22, B7):** The first text said "removes the marked `file` rows".
> The row keeps the first-seen time (for the cooldown, ADR-0008) and the pull history.
> If the sweep removes the row, the cooldown of a file without an upload time starts again,
> and the pull history of the file is lost.
- The grace period is 24 hours.

### 2.3 Schedule

- The mark job and the sweep job are periodic jobs in the queue ([ADR-0004](0004-job-queue.md)).
- They run one time each hour.

## 3. Consequences

- The total size can be more than the limit for up to 24 hours.
- A mistake in the mark job does not remove data immediately.
  An operator can see the marked files in the UI during the grace period.
