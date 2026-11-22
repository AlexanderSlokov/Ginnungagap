# ADR-0007: Web UI technology and color palette

| Field    | Value                          |
|----------|--------------------------------|
| Status   | Accepted                       |
| Date     | 2026-11-22                     |
| Decider  | Founder (`@AleksandrSlokov`)   |
| Recorder | Claude (BA and secretary role) |

## 1. Context

[ADR-0001](0001-rebuild-as-package-proxy.md) requires a clean UI for daily work.
The design direction is industrial, neo-brutalism, with the clean look of e-ink.

## 2. Decision

### 2.1 Technology

- Server-side rendering with `templ` (`github.com/a-h/templ`).
- `htmx` for partial page updates.
- The Go binary embeds all static files.

### 2.2 Color palette: Nord (trial)

We use the [Nord](https://www.nordtheme.com/) palette as a trial.
The default theme is light, because of the e-ink direction.

| Token          | Nord color        | Use                                  |
|----------------|-------------------|--------------------------------------|
| `--bg`         | nord6 `#ECEFF4`   | Page background                      |
| `--surface`    | nord5 `#E5E9F0`   | Panels and table rows                |
| `--line`       | nord0 `#2E3440`   | Borders (thick, as neo-brutalism)    |
| `--text`       | nord0 `#2E3440`   | Body text                            |
| `--text-muted` | nord3 `#4C566A`   | Secondary text                       |
| `--accent`     | nord13 `#EBCB8B`  | Fill of primary buttons and selected items |
| `--info`       | nord8 `#88C0D0`   | Fill of info badges                  |
| `--ok`         | nord14 `#A3BE8C`  | Fill of "cached" and "allowed" badges |
| `--warn`       | nord12 `#D08770`  | Fill of "cooldown" badges            |
| `--danger`     | nord11 `#BF616A`  | Borders and icons of "blocked" items |

### 2.3 Contrast rules (WCAG 2.2 AA)

| Pair                        | Ratio | Result |
|-----------------------------|-------|--------|
| nord0 text on nord6         | 10.84 | Pass   |
| nord3 text on nord6         | 6.40  | Pass   |
| nord0 text on nord13 fill   | 8.00  | Pass   |
| nord0 text on nord14 fill   | 6.13  | Pass   |
| nord0 text on nord8 fill    | 6.24  | Pass   |
| nord0 text on nord12 fill   | 4.39  | Pass   |
| nord11 border on nord6      | 3.55  | Pass (non-text) |

- Do not use Frost or Aurora colors for text on a light background. They fail AA.
- Use nord11 only for borders and icons. Put the text label in nord0.

## 3. Consequences

- The palette is a trial. A later ADR can replace it.
- A dark theme is not in `0.1.0`.
