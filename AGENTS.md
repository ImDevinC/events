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

- Every pull request must identify its impact in the description: **major**,
  **minor**, or **patch** when it requires a version bump, or explicitly state
  that no version bump is required and explain why (for example, documentation-only
  changes).
- Every pull request must include the matching GitHub label: `major`, `minor`, or
  `patch` for versioned changes, or `chore` when no version bump is required. Ensure
  the label matches the impact stated in the PR description.
