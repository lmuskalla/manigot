# Implementation: move settings file

id: kwd2ou
status: in-progress
developer: developer (deepseek-v4-flash)
date: 2026-08-11

<!-- Produced by @developer after implementation. -->

## Summary

Moved the project-settings file, the mg-jdi sidecar, and quotes.json out of
`docs/` into their proper homes, repointing every reader/writer. Docs are for
documents again.

- Project settings: `docs/manigot.json` → `.manigot/manigot.json` (committed —
  it holds only a public ref name).
- mg-jdi run state: `docs/jobs/.jdi-status/` → `.manigot/jdi-status/`
  (gitignored — ephemeral).
- Quotes: `docs/quotes.json` → `assets/quotes.json`.
- `mg init` + project template now seed `.manigot/manigot.json`.
- `docs/AGENTS.md` (project context) updated for every moved path, plus a new
  `.manigot/` Architecture bullet and a wording caveat on the "never touch
  files outside docs/" hard rule.
- TASK-6 investigation (why docs/ has package.json/lock/node_modules)
  recorded below; files left untracked as instructed.

## Changes

TASK-1: Moved `docs/manigot.json` → `.manigot/manigot.json` (`git mv`, 100%
rename). `tui/internal/project/settings.go`: `Path()` now joins `.manigot` +
`manigot.json`, `Save()` creates `.manigot/` instead of `docs/`, doc comments
updated. `settings_test.go`: round-trip test asserts the new path.
`tui/internal/ui/settings.go`: form hint + comments say `.manigot/manigot.json`.
`tui/internal/ui/app.go`: field/refresh comments updated. `scripts/new-job.sh`:
`SETTINGS_FILE` now `.manigot/manigot.json`, usage comments updated. If any
reader had been missed the base branch would silently degrade to "main" —
verified none remain (grep).

TASK-2: `scripts/init.sh` now copies AGENTS.md/CLAUDE.md into docs/ and seeds
the settings from `project-template/.manigot/manigot.json` (moved with `git
mv`) into the target's `.manigot/`; template file-existence checks and echo
output updated. Verified live: `mg init` in a fresh dir creates
`docs/{AGENTS.md,CLAUDE.md,jobs/}` + `.manigot/manigot.json` with
`{"baseBranch":"main"}`. `docs/` remains the "initialized" marker; the guard
still keys on its presence.

TASK-3: mg-jdi sidecar moved from `docs/jobs/.jdi-status/` to
`.manigot/jdi-status/`. `tui/internal/job/jdistatus.go`: new exported
`ManigotRelDir = ".manigot"` const, `JDISidecarDirName` is now `"jdi-status"`,
`JDIStatusDir` joins under `ManigotRelDir`. `tui/cmd/jdi/output.go`:
`sidecarExcludePattern` rebuilt from `job.ManigotRelDir` +
`job.JDISidecarDirName` (can't drift from where the sidecar is written).
`output_test.go`: all `docs/jobs/.jdi-status/` assertions + sidecar-path setup
updated; `discover.go`/`discover_test.go`: comments and the non-job-dir test
updated to the new location. Root `.gitignore`: `docs/jobs/.jdi-status/` →
`.manigot/jdi-status/`. No sidecar data existed locally, so no migration was
needed. All jdi/job tests pass, including the real `git add -A` regression
test against the new exclude pattern.

TASK-4: `docs/quotes.json` → `assets/quotes.json` (`git mv`, 100% rename).
`scripts/run.sh`: `QUOTES_FILE` and the flavor-quote comment now point at
`assets/quotes.json`. `docs/NAMING.md`: the three `docs/quotes.json`
references (flavor-text section + rap-sheet entry) updated. No other code or
docs reference quotes.json (entrypoint.sh only sees the already-selected
quote via `MANIGOT_QUOTE`).

TASK-5: `docs/AGENTS.md` updated: `.manigot/manigot.json` in the new-job.sh,
config, settings-file and `mg job` bullets; `.manigot/jdi-status/` in the
tui/cmd/jdi bullet; init.sh bullet rewritten for the split copy source; new
`.manigot/` Architecture bullet describing the directory; the "never touch
files outside docs/" hard rule now carves out `.manigot/` as the one
deliberate host-side-tooling exception. (AGENTS.md never referenced
quotes.json, so nothing to update for that move — NAMING.md was the only
docs-file reference.)

TASK-6: Investigation recorded in the "TASK-6" section below. The
`docs/` package artifacts are untracked local state, installed there so they
land in the `docs/` → `/workspace/.opencode` container mount (verified: that
mount is live on this host) for OpenCode plugin imports. Recommend keeping;
not committed or deleted.

TASK-7: `go build ./...` + `go test ./...` in `tui/` all pass; `bash -n` on
`scripts/new-job.sh`, `scripts/init.sh`, `scripts/run.sh` all pass; `git
status` shows exactly the intended change set (3 renames + reference
updates, no stray untracked files staged); `mg init` template copy path
verified in a fresh directory; `.manigot/manigot.json` confirmed tracked
while `.manigot/jdi-status/` is ignored.

## TASK-6: why does docs/ contain package.json, package-lock.json, node_modules?

**Finding.** All three are untracked local artifacts (confirmed via `git
check-ignore` / `git status`), ignored by the untracked `docs/.gitignore`
(which ignores `node_modules`, `package.json`, `package-lock.json`,
`bun.lock`, and itself). None of them exist in the repo's history.

- `docs/package.json` declares a single dependency: `@opencode-ai/plugin`
  `1.18.16` (the OpenCode plugin SDK, pulled in together with its
  `@opencode-ai/sdk` and transitive deps under `docs/node_modules/`).
- `docs/` is bind-mounted into the container at `/workspace/.opencode` for
  OpenCode sessions (`scripts/run.sh`: `DOCS_MOUNT_TARGET="/workspace/.opencode"`,
  `-v "$PROJECT_DOCS_DIR:$DOCS_MOUNT_TARGET:z"`). Verified live on this host:
  `/workspace/.opencode` is currently a bind mount of this repo's `docs/`.
- Installing the SDK inside `docs/` therefore places it at
  `/workspace/.opencode/node_modules/@opencode-ai/plugin` inside the
  container — exactly where OpenCode's plugin loader can import it — without
  any container-image change or root-level `package.json`.

**Consumers.** None in the tracked repo: no `opencode.json`/plugin config
exists anywhere in the repo or in the `.opencode/` mount, and nothing else
imports the package. The artifacts are purely a prepared, local
plugin-development dependency.

**Recommendation: keep.** Deleting them costs nothing to the tracked repo
(they are ignored) but silently breaks the one workflow they exist for —
writing/importing OpenCode plugins in a session. If the "docs are for
documents" objection persists, the clean fix (out of scope here) is a
root-level `package.json` + `node_modules` plus an opencode plugin config
that points at it, and dropping the `docs/` install entirely.

Per the task, these files were **not** committed or deleted.

## Known issues / follow-ups

- **`docs/` package artifacts kept, per TASK-6 recommendation.** `docs/package.json`,
  `docs/package-lock.json`, `docs/node_modules/` (and the untracked
  `docs/.gitignore` hiding them) remain as-is — untracked local state for the
  OpenCode plugin workflow. If the "docs are for documents" rule is to hold
  strictly, the follow-up is a root-level `package.json`/`node_modules` plus
  an opencode plugin config pointing at it (out of scope here).
- **`docs/jobs/archive/` untouched.** Historical job records still reference
  the old paths (`docs/manigot.json`, `docs/jobs/.jdi-status/`); per the task
  notes they were deliberately left alone.
- **README.md untouched** — it contains no direct references to any of the
  moved paths, so no sync was needed.
- **Legacy base-branch value**: the repo's own `.manigot/manigot.json` is
  `{}` (no `baseBranch` key), so `mg job` and the TUI keep defaulting to
  `main` — unchanged behavior.
