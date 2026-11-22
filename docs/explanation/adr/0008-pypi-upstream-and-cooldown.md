# ADR-0008: PyPI upstream resolution and cooldown

| Field    | Value                          |
|----------|--------------------------------|
| Status   | Accepted                       |
| Date     | 2026-11-22                     |
| Decider  | Founder (`@AleksandrSlokov`)   |
| Recorder | Claude (BA and secretary role) |

## 1. Context

A group repository has more than one upstream, for example PyPI and the PyTorch index.
If two upstreams have the same package name, an attacker can publish a package with that name on PyPI.
This attack is dependency confusion.

The cooldown needs the upload time of each file.
PyPI gives `upload-time` in the JSON Simple API (PEP 691, PEP 700).
The PyPI HTML Simple API is frozen and does not give the upload time.
On 2026-11-22, `download.pytorch.org` gave HTML only, with a `data-upload-time` attribute.
Other indexes can give no upload time.

## 2. Decision

### 2.1 Upstream resolution

- The members of a group have a priority order.
- For each package name, the system uses only the first upstream that has the name.
- The system does not merge versions from two upstreams.
- The system records the selected upstream in the `project` table.
  An operator can change it.
- This is the same as the `first-index` strategy of uv.

### 2.2 Upload time

The system gets the upload time of a file in this order:

1. `upload-time` in the JSON Simple API.
2. `data-upload-time` in the HTML Simple API.
3. The earliest upload time of another file with the same SHA-256, from any upstream.
   The same SHA-256 means the same content.
   For example, the PyTorch index mirrors some PyPI files without an upload time.
4. The first-seen time: the time when the proxy sees the file for the first time.

> **Amendment (2026-11-22, B4):** Step 3 is new.
> The end-to-end test showed that the PyTorch index gives no upload time for mirrored PyPI files.

### 2.3 Cooldown

- The default cooldown is **14 days**.
- The proxy does not show a file in the index until the cooldown is complete.
- A Maintainer can add an override for one version (for an urgent fix).
- The allowlist also bypasses the cooldown.

## 3. Consequences

- New versions are not available for 14 days. Teams must use overrides for urgent fixes.
- For an upstream that gives no upload time, the cooldown starts when the proxy sees the file first.
  The real age of the file can be more than the cooldown.
