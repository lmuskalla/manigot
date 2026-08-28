# Implementation: add jq to tailing

id: break
status: open
developer: @developer
date: 2026-08-28

<!-- Produced by @developer after implementation. -->

## Summary

Implemented the analyst's decision: the TUI's `l` key (`launch.Tail`) now
pipes the `tail -f` of a job's `session.log` through
`jq -R -r 'fromjson? // .'` when `jq` is available on the host, pretty-
printing the JSONL raw stream both agent CLIs write while passing mg-jdi's
`=== ... ===` section headers and blank lines through unchanged. When `jq`
isn't on the host, `Tail` silently falls back to the plain `tail -f` it used
before — no regression for hosts without `jq`. All five task files from
`tasks.md` were touched; build, vet, and the targeted test suites pass.

## Changes

TASK-1: Added a `JqLookPath`/`JqAvailable` seam to the launch package,
mirroring the existing `TigLookPath`/`TigAvailable` pattern exactly (an
exported `exec.LookPath`-backed var plus a lightweight availability check),
purely additive.
     files: src/internal/launch/launch.go, src/internal/launch/launch_test.go

TASK-2: Added a parallel `jqTailShellCommand` builder alongside the existing
`tailShellCommand` (kept unchanged, per the analysis's "separate function per
launch path" convention) and made `Tail` pick between them on
`JqAvailable()`. Neither builder is wrapped in `holdOnFailure` — Ctrl+C
(exit 130) must still close the pane cleanly on both the plain and piped
forms.
     files: src/internal/launch/launch.go, src/internal/launch/launch_test.go

TASK-3: Added `stubJqLookPath` to `internal/ui/tail_test.go` (mirroring
`stubTigLookPath` in `tig_test.go`) and updated
`TestTailKeyLaunchesTailInTmuxPane` to stub jq as available so it asserts the
exact jq-piped command deterministically instead of depending on whether the
test machine has jq. Added `TestTailKeyFallsBackToPlainTailWhenJqMissing` for
the other branch.
     files: src/internal/ui/tail_test.go

TASK-4: Updated the canonical `docs/AGENTS.md` and
`project-template/docs/AGENTS.md` descriptions of the `l` tail key to mention
the jq pipe and its fallback to plain `tail -f`. Confirmed (grep) that no
`agents/*.md` file mentions tailing, so none needed a change.
     files: docs/AGENTS.md, project-template/docs/AGENTS.md

TASK-5: Verified `go build ./...` and `go vet ./...` (both clean), and
`go test ./internal/launch/... ./internal/ui/... -run 'Jq|Tail'` (all pass).
A full `go test ./...` was also run: every failure it reports is pre-existing
and environmental — this session's git shim refuses `git init`
("`git 'init' is not allowed in agent sessions`"), which many unrelated test
suites (internal/git, internal/job, internal/session, cmd/mg, plus the
non-tail tests in internal/ui) use to build throwaway fixture repos; none of
the failures touch tail/jq code. The manual smoke test from the task
description (`printf '=== ... ===\n{"type":"message","content":"hi"}\n\n' |
jq -R -r 'fromjson? // .'`) could not be run directly: `jq` is not installed
on this host and installing packages requires asking first, which is out of
scope for a non-interactive session — see Known issues below.
     files: none (verification only)

## Known issues / follow-ups

- The manual jq-pipeline smoke test called for in `tasks.md` TASK-5 was not
  run against a real `jq` binary — this host has none installed, and
  installing one needs to be asked about first. The filter's exact
  construction (`jqTailShellCommand`) is covered by unit tests instead
  (`TestJqTailShellCommandFormat` and friends), and the filter itself
  (`jq -R -r 'fromjson? // .'`) is the one specified by the analyst's
  decision in `tasks.md`. Worth a real-jq smoke test on a host that has it
  before merging, if that matters to the reviewer.
- Nothing else found in scope but left undone.
