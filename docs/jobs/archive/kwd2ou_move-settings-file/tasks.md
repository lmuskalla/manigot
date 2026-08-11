# Tasks: move settings file

id: kwd2ou
status: in-progress
analyst: analyst (deepseek-v4-flash)
date: 2026-08-11

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Move the project settings file `docs/manigot.json` → `.manigot/manigot.json` and repoint every reader/writer at the new path (TUI `project` package + UI strings + `scripts/new-job.sh`), creating `.manigot/` in `Save()`.
     files: docs/manigot.json (git mv → .manigot/manigot.json), tui/internal/project/settings.go, tui/internal/project/settings_test.go, tui/internal/ui/settings.go, tui/internal/ui/app.go, scripts/new-job.sh
     depends: none
     risk: medium — the base-branch setting silently degrades to "main" if any reader is missed, and the TUI's `Save()` must now create `.manigot/` instead of `docs/`.

TASK-2: Update `mg init` (`scripts/init.sh`) and the project template so fresh projects are seeded with `.manigot/manigot.json` instead of `docs/manigot.json` (docs/ itself still gets AGENTS.md + CLAUDE.md + empty jobs/).
     files: scripts/init.sh, project-template/docs/manigot.json (git mv → project-template/.manigot/manigot.json)
     depends: TASK-1
     risk: medium — a wrong template/settings path in init.sh silently breaks `mg init` for every new project, and init.sh is the only job-workflow command that runs in uninitialized projects.

TASK-3: Move the mg-jdi sidecar directory from `docs/jobs/.jdi-status/` to `.manigot/jdi-status/` and update the Go path construction, the `.git/info/exclude` pattern (must be rebuilt from the new constants, not `JobsRelDir`+old name), the root `.gitignore`, and the tests that assert the old strings.
     files: tui/internal/job/jdistatus.go (JDIStatusDir + JDISidecarDirName + new `.manigot` const + doc comments), tui/cmd/jdi/output.go (sidecarExcludePattern + comment), tui/cmd/jdi/output_test.go, tui/internal/job/discover.go (comments only), tui/internal/job/discover_test.go, .gitignore
     depends: none
     risk: medium — if the exclude pattern is wrong, mg-jdi's status/run.log sidecar can be swept into an agent's `git add -A`; several tests assert the exact `docs/jobs/.jdi-status/` string. No sidecar data exists in this repo today, so no local-data migration is needed.

TASK-4: Move `docs/quotes.json` → `assets/quotes.json` and update `scripts/run.sh` and `docs/NAMING.md` to reference the new path.
     files: docs/quotes.json (git mv → assets/quotes.json), scripts/run.sh (QUOTES_FILE + comment), docs/NAMING.md
     depends: none
     risk: low — one host-side path constant plus two doc references; a missing quotes file already degrades to "no quote this session" and entrypoint.sh only sees the already-selected quote via env var.

TASK-5: Update `docs/AGENTS.md` (the canonical project context) for every moved path — `.manigot/manigot.json`, `.manigot/jdi-status/`, `assets/quotes.json` — and add a `.manigot/` bullet plus a wording caveat to the "never touch files outside docs/" hard rule (host-side tooling now deliberately writes `.manigot/`).
     files: docs/AGENTS.md
     depends: TASK-1, TASK-2, TASK-3, TASK-4
     risk: low — documentation only, but it is the context agents read at session start and the repo's hard rule demands it stay in sync with agents/*.md and project-template (neither currently references these paths, so no sync burden).

TASK-6: Investigate and document why `docs/` contains `package.json`, `package-lock.json`, and `node_modules/` — all gitignored/untracked local artifacts (`@opencode-ai/plugin` SDK installed inside `docs/` so it lands in the `docs/` → `/workspace/.opencode` container mount, enabling plugin imports; nothing in the tracked repo consumes them). Record the finding and the keep-vs-delete recommendation in implementation.md; do not commit or delete the untracked files.
     files: docs/package.json, docs/package-lock.json, docs/node_modules/ (investigation only), docs/jobs/kwd2ou_move-settings-file/implementation.md (finding)
     depends: none
     risk: low — investigation/documentation only; no tracked files change.

TASK-7: Verify the full change set end to end: `go build ./...` + `go test ./...` in tui/, `bash -n` on every touched script, confirm `git status` shows exactly the intended moves (no stray untracked files staged), and sanity-check the init template copy path.
     files: all files from TASK-1..TASK-5
     depends: TASK-1, TASK-2, TASK-3, TASK-4, TASK-5, TASK-6
     risk: low — the moves cross Go code, bash scripts, and docs, so a final test pass before the verdict is worthwhile.

## Notes

- Do NOT edit anything under `docs/jobs/archive/` — those are historical job records and legitimately reference the old paths.
- Jobs themselves stay in `docs/jobs/`; `docs/` remains the "initialized project" marker for `find_project_root`/`FindProjectRoot` — only the settings file, the jdi sidecar, and quotes.json move.
- `.manigot/` does not exist yet in this repo and is not gitignored as a whole: `.manigot/manigot.json` is committed (it holds only a public ref name), while `.manigot/jdi-status/` is gitignored (ephemeral run state).
