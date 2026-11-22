# Backlog

This backlog divides the items of [ROADMAP.md](ROADMAP.md) into tasks.
When a task is complete, remove it from this file and record the result in `CHANGELOG.md`.

## Phase 0: Reset (`0.1.0`)

### Repository root

#### P0-1: Remove the old project files

- **Files:**
  `cmd/`, `isolator/`, `pcr_thermocycler/`, `spill_kit/`, `test_kit/`,
  `main.go`, `go.mod`, `go.sum`, `Dockerfile`, `isolator.Dockerfile`, `docker-compose.yml`, `.gitignore`.
- **Dependencies:** None.
- **Goal:** The repository contains no file of the old project, except `LICENSE` and `docs/images/bombastic_side_eyes_cat.png`.
  `.gitignore` keeps the Go rules and has no Falco rules.

### Documentation

#### P0-2: Write a new README

- **Files:** `README.md`.
- **Dependencies:** P0-1.
- **Goal:** `README.md` describes the new project, as ADR-0001 defines it.
  It has no text about the old project.
  It shows `docs/images/bombastic_side_eyes_cat.png`.

## Phase 1: Package proxy (MVP)

The tasks for Phase 1 are not written yet.
We write them on a new branch, after Phase 0 is merged to `main`.
