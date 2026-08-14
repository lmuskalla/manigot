# Brief: jdi-status sidecar cleanup

status: done
type: chore
id: sweep
date: 2026-08-14
author: Leander Muskalla

## What

Make `mg delete` and `mg done` (and orphaned-worktree removal) clean up the
job's `.manigot/jdi-status/<job>/` sidecar — the status file and run.log that
`mg jdi` leaves behind — instead of leaving them in the project's
`.manigot/jdi-status/` forever after the job itself is gone.

## Why

`mg delete` left `.manigot/jdi-status/<job>/` (status + run.log) behind
forever — the stale sidecar dirs in this repo's own `.manigot/jdi-status/`
were the proof. `mg done` had no deliberate keep-vs-remove decision either.
Same family as the orphaned-worktree cleanup: the tool not cleaning up after
itself quietly erodes trust in the lifecycle.

## Out of scope

- Anything else in the `mg jdi` loop or the TUI.
- Moving the existing stale sidecar dirs in this repo's own
  `.manigot/jdi-status/` — those were removed by hand as part of this job's
  verification, not by the code change.

## Notes

- `mg done`'s keep-vs-remove decision is **remove**: the archive keeps the
  job's docs, `mg jdi` never runs against an archived job, so the sidecar
  would otherwise be dead weight forever.
- Best-effort: a sidecar-removal failure warns, never aborts the already-
  succeeded delete/finish.
