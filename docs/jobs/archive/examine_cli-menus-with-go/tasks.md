# Tasks: cli menus with go

id: examine
status: open
analyst: analyst
date: 2026-08-13

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Add a reusable single-select Bubble Tea picker model to internal/ui — a generic component that renders a title plus a scrollable, cursor-highlighted list of pre-rendered rows (each carrying an id and a search key), supports up/down/k/j/home/end navigation, enter to submit, esc/q to cancel, and resize handling, tested directly through Update/View like agentspicker_test.go does.
     files: internal/ui/picker.go (new), internal/ui/picker_test.go (new)
     depends: none
     risk: medium — new component with cursor/scroll/resize edge cases that must not disturb the existing TUI views; reuse of the shared styles (selectedStyle/dimStyle/titleStyle) keeps it consistent.

TASK-2: Add type-to-filter to the picker so the user can narrow the list by typing against a row's search key (job id/title for jobs, agent name/description for agents), with esc clearing the filter before falling through to cancel and the cursor clamped to the filtered list.
     files: internal/ui/picker.go, internal/ui/picker_test.go
     depends: TASK-1
     risk: medium — the navigation-vs-input mode interplay and cursor clamping after filtering are the classic Bubble Tea footguns, but the scope is one component.

TASK-3: Replace `mg jobs`' numbered `cli.Select` prompt with the picker on a TTY: build job rows from the same ID/status/type/date/title + jdi-badge info as today's plain listing, run the picker with the alt screen, on submit print "→ Starting a session in <id>..." and re-exec `mg --job <id> <passthrough>`, on cancel exit 0; keep the orphaned-worktree surfacing + removal offer (before the picker) and keep the non-TTY listing + refusal output byte-identical.
     files: cmd/mg/jobs.go, cmd/mg/jobs_test.go
     depends: TASK-1
     risk: medium — the picker must be injected through a seam (following the confirm-func injection runJobs already uses for orphan removal) so the wiring tests don't start a real Bubble Tea program; the TTY-path tests (e.g. TestJobsSelectWritesChosenAndLaunches) need rework, the non-TTY tests must stay green, and the re-exec launch flow must survive the terminal-mode transition.

TASK-4: Replace `mg agents`' numbered `cli.Select` prompt with the picker on a TTY, mirroring TASK-3: rows show name, description and the "(project)"/"(project override)" source tag, submit re-execs `mg --agent <name> <passthrough>`, cancel exits 0, and the non-TTY listing + refusal output stays byte-identical.
     files: cmd/mg/agents.go, cmd/mg/agents_test.go
     depends: TASK-1 (shares the seam pattern from TASK-3)
     risk: low-medium — same pattern as TASK-3 with fewer moving parts (no orphan flow), so the main risk is keeping the existing non-TTY tests green and matching the TASK-3 seam exactly.

TASK-5: Sync the documentation for the new selection UX: update docs/AGENTS.md, project-template/docs/AGENTS.md, README.md and the `mg --help` text in cmd/mg/main.go to describe the interactive picker in `mg jobs`/`mg agents`, and scan agents/*.md for any stale description of the old numbered selection.
     files: docs/AGENTS.md, project-template/docs/AGENTS.md, README.md, cmd/mg/main.go, agents/*.md (only if a stale mention is found)
     depends: TASK-3, TASK-4 (behavior must be final first)
     risk: low — documentation only, but the AGENTS.md sync rule (docs/AGENTS.md ↔ agents/*.md ↔ project-template/docs/AGENTS.md) is a hard rule.

TASK-6: Verify the result: `go build ./...`, `go vet ./...`, `go test ./...` all green, plus a manual smoke of both commands — TTY picker (navigation, filter, enter, esc/q cancel), non-TTY listing + refusal, the jobs orphan-removal offer, and the re-exec into a session.
     files: none (verification only)
     depends: TASK-3, TASK-4
     risk: low — verification task; the danger is only a picker that misbehaves on a real terminal, which the manual smoke covers.

## Out of scope / decisions

- `mg profiles`' interactive default-profile selection also uses `cli.Select`, but the brief names only `mg jobs` and `mg agents` — treat the profiles picker as out of scope unless explicitly requested; `cli.Select` stays for it (and for any other prompt) unchanged.
- Non-TTY behavior of both commands is preserved byte-for-byte (plain listing + the existing "needs an interactive terminal" refusal) — the picker is a TTY-only enhancement.
- Cancel (esc/q) from the new picker should exit 0 quietly rather than today's `mg jobs: quit` error path — a deliberate UX improvement; confirm when reviewing.
- Filtering (TASK-2) is recommended but could be dropped without breaking TASK-3/TASK-4 if scope tightens — it is self-contained in the picker component.
