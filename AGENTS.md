# Agent guidance

## Keep project documentation in sync

- Keep `AGENTS.md` and `README.md` up to date together as the project changes.
- When a change affects user-facing behavior, setup, configuration, testing, or
  development workflow, update the relevant README sections and the corresponding
  guidance here in the same change.
- When either document's instructions or documented behavior changes, review the
  other document for consistency and update it if needed. Do not leave conflicting
  or stale guidance between these files.
- Keep the README's event behavior description aligned with the app, including
  start/end date defaults, inclusive multi-day ranges, and validation rules.
- The README points contributors to this file for the full documentation-maintenance
  rules; preserve that link if reorganizing the README.

## Pull request versioning

- Every pull request must identify its Semantic Versioning impact: **major**,
  **minor**, or **patch**.
- If a PR does not change the released product and needs no version bump, say so
  explicitly and explain why in the PR description rather than omitting versioning.
