# Brief: Add stage column to TUI overview

status: done
type: feature
id: 3iqg8j
branch: feature/3iqg8j_add-stage-column-to-tui-overview
date: 2026-08-13
author: Leander Muskalla

## What

Add a stage column to the TUI's job overview list (`mg tui`). The list currently shows id / status / type / date / title, with no sense of where each job is in the workflow. This honors the intent of the previously scaffolded `6ro7eg_add-stage-to-overview`.

`job.Stage()` already computes the current stage — this is a rendering change, not a data one.

## Why

For someone juggling several parallel jobs (which the worktree model explicitly enables), "which of my five jobs is stuck in review" at a glance is the single most useful piece of overview information. The `[running @agent]` / `[finished]` / `[needs human]` jdi badges exist and the detail view has the stage timeline, but the list itself doesn't convey workflow position.

## Out of scope

- Anything about the `mg jdi` state machine itself (separate jobs `63quv2`, `ru97hg`).
- Job lifecycle or worktree changes (separate jobs `nepbxu`, `ui5f6q`).

## Notes

- `job.Stage()` already exists — reuse it.
- Previous scoping: `6ro7eg_add-stage-to-overview` was scaffolded and never built; `sd62w9_add-jdi-in-overview` was effectively delivered by the status badges.

