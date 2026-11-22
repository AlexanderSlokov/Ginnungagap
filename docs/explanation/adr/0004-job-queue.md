# ADR-0004: Job queue

| Field    | Value                          |
|----------|--------------------------------|
| Status   | Accepted                       |
| Date     | 2026-11-22                     |
| Decider  | Founder (`@AleksandrSlokov`)   |
| Recorder | Claude (BA and secretary role) |

## 1. Context

[ADR-0001](0001-rebuild-as-package-proxy.md) requires a job queue as a separate service.
The database keeps data. The queue keeps jobs.

## 2. Decision

- Queue service: **Valkey**.
- Go library: **asynq** (`github.com/hibiken/asynq`).
  It gives retries, scheduled jobs, and periodic jobs.

## 3. Options that we considered

| Option              | Result   | Reason                                                     |
|---------------------|----------|------------------------------------------------------------|
| Redis 8             | Rejected | The Founder selected Valkey. Valkey has the BSD 3-Clause license. |
| asynq               | Selected | Active project (release v0.26.0 in 2026-02). Retries and periodic jobs are included. |
| Own queue on Valkey Streams | Rejected | More code to write and test. |

## 4. Consequences and risks

- asynq uses the go-redis client. Valkey is compatible with the Redis protocol,
  but asynq does not state official support for Valkey (issue [#985](https://github.com/hibiken/asynq/issues/985)).
  The end-to-end test must run asynq against a Valkey container.
- If asynq fails with Valkey, write a new ADR.
