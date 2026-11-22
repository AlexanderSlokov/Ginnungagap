# Backlog

This backlog divides the items of [ROADMAP.md](ROADMAP.md) into tasks.
When a task is complete, remove it from this file and record the result in `CHANGELOG.md`.

## Phase 1: Package proxy (MVP)

The tasks for `0.2.0` (npm) are not written yet.

### Web UI

#### B11: Pagination of long lists

- **Files:** `internal/web/web.go`, `internal/web/views.templ`, `internal/web/views_templ.go`, `internal/web/static/style.css`.
- **Dependencies:** None.
- **Goal:**
  - These lists show one page at a time: the files on the package page, the packages on the repository page, and the marked files on the storage page.
  - The user can select 10, 20, or 50 rows for each page. The default is 20.
  - The query string keeps the page and the page size (`?page=2&size=50`), so a link opens the same page. The package filter (`q`) stays when the page changes.
  - The database query reads only the rows of the current page (`LIMIT` and `OFFSET`). It also counts all rows to show "page N of M".
  - An invalid page or page size falls back to page 1 or size 20.
  - Example of the problem: `requests` has 244 files and `charset-normalizer` has 2461 files on one page.
