# ADR-0006: Access control roles

| Field    | Value                          |
|----------|--------------------------------|
| Status   | Accepted                       |
| Date     | 2026-11-22                     |
| Decider  | Founder (`@AleksandrSlokov`)   |
| Recorder | Claude (BA and secretary role) |

## 1. Context

[ADR-0001, section 3.6](0001-rebuild-as-package-proxy.md#36-access-control) selects the model:
the unit of access is the repository, and each member gets one fixed role.

## 2. Decision

| Role       | Permissions on the repository                                          |
|------------|------------------------------------------------------------------------|
| Guest      | Pull packages. See the cached packages.                                |
| Developer  | All Guest permissions. See the pull history. Pin a file.               |
| Maintainer | All Developer permissions. Edit the blocklist, the allowlist, and the cooldown overrides. Purge a file. |
| Admin      | All Maintainer permissions. Edit the repository, its upstreams, and its members. Create robot accounts. |

- A system administrator has the Admin role on all repositories.
- A robot account can have only the Guest role or the Developer role.
- In `0.1.0`, users are local accounts. OIDC is not scheduled yet.
- Package clients (`pip`, `uv`) send the user name and the token with HTTP basic authentication.
