# UAPI Documentation

This directory is the canonical documentation index for UAPI. New coding
sessions should start with [current/handoff.md](current/handoff.md), then read
the specific current document that matches the task.

## Structure

- `current/`: active product, frontend, backend, and architecture notes. Use
  these files for implementation decisions.
- `deployment/`: deployment and operations notes.
- `reference/`: stable background references. These can explain upstream tools
  or external behavior, but they are not product requirements by themselves.
- `archive/`: superseded historical documents. Do not use archived files as the
  source of truth for current work.

## Current Source Of Truth

- [current/handoff.md](current/handoff.md): first-read project state, commands,
  known gaps, and verification checklist.
- [current/frontend.md](current/frontend.md): frontend routes, UI boundaries, and
  backend API alignment.
- [current/platform-design.md](current/platform-design.md): current platform
  design, based on the original v3 design and updated for the UAPI product name.

## Maintenance Rules

- Keep `current/` aligned with implemented behavior before ending a major work
  session.
- Move obsolete planning notes into `archive/` with a clear superseded warning.
- Prefer adding cross-links from this index instead of scattering "start here"
  instructions across many files.
