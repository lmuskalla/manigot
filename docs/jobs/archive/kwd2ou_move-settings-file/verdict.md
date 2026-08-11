# Verdict: move settings file

id: kwd2ou
status: complete
reviewer: reviewer (deepseek-v4-flash)
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Branch `feature/kwd2ou_move-settings-file` matches the brief. Verified against `git diff main...HEAD` (3 renames + reference updates, no stray files) and a fresh `go build ./...` / `go test ./...` (all green) and `bash -n` on every touched script.

TASK-1: PASS
     notes: `docs/manigot.json` → `.manigot/manigot.json` via `git mv` (100% rename). `tui/internal/project/settings.go` Path() joins `.manigot`, Save() MkdirAll's `.manigot/` instead of `docs/`; `settings_test.go` round-trip asserts the new path; `tui/internal/ui/settings.go` (form hint + comments), `tui/internal/ui/app.go` (comments), and `scripts/new-job.sh` (SETTINGS_FILE + usage comments) all repointed. Repo-wide grep confirms no reader of the old path remains outside archive/ and this job's own docs. The base-branch degrade-to-"main" risk is covered: a missing file was already treated as defaults, and the repo's own `.manigot/manigot.json` is `{}` (no key), so behavior is unchanged.

TASK-2: PASS
     notes: `scripts/init.sh` now copies AGENTS.md/CLAUDE.md into `docs/` and seeds `project-template/.manigot/manigot.json` (git mv'd) into the target's `.manigot/`; template existence checks and echo output updated. Verified live in a fresh temp dir: produces `docs/{AGENTS.md,CLAUDE.md,jobs/}` + `.manigot/manigot.json` with `{"baseBranch":"main"}`. Guard still keys on `docs/` presence (the initialized marker), preserving the "already initialized" skip. Doc header updated to match.

TASK-3: PASS
     notes: Sidecar moved to `.manigot/jdi-status/`. `tui/internal/job/jdistatus.go`: new exported `ManigotRelDir = ".manigot"`, `JDISidecarDirName = "jdi-status"`, `JDIStatusDir` joins under `ManigotRelDir`. `tui/cmd/jdi/output.go` rebuilds `sidecarExcludePattern` from the same constants (cannot drift from the write path). `output_test.go` assertions, `discover.go`/`discover_test.go` comments + test layout, and root `.gitignore` all updated. `TestEnsureSidecarIgnoredActuallyWorksWithGit` (the real `git add -A` regression) passes against the new pattern; `git check-ignore` confirms `.manigot/manigot.json` tracked while `.manigot/jdi-status/` is ignored. No sidecar data existed, so no migration needed (confirmed: no `.jdi-status` dir anywhere).

TASK-4: PASS
     notes: `docs/quotes.json` → `assets/quotes.json` via `git mv` (100% rename). `scripts/run.sh` QUOTES_FILE + comment point at `assets/quotes.json`. All three `docs/NAMING.md` references updated. Grep confirms no other tracked code/docs reference quotes.json (entrypoint.sh only consumes the already-picked quote via env var; README has no reference).

TASK-5: PASS
     notes: `docs/AGENTS.md` updated for every moved path (`.manigot/manigot.json` in the new-job.sh/config/settings/`mg job` bullets, `.manigot/jdi-status/` in the tui/cmd/jdi bullet, init.sh bullet rewritten for the split copy source), plus a new `.manigot/` Architecture bullet and the "never touch files outside docs/" hard rule now carves out `.manigot/` as the one deliberate host-side-tooling exception. AGENTS.md never referenced quotes.json, so nothing to update for that move — NAMING.md was the only docs-file reference. `agents/*.md` and `project-template/docs/AGENTS.md` contain no path references, so the keep-in-sync rule is unaffected.

TASK-6: PASS
     notes: Investigation recorded in implementation.md: the `docs/` package artifacts are untracked local state (`@opencode-ai/plugin` 1.18.16 installed in `docs/` so it lands in the `docs/` → `/workspace/.opencode` container bind mount — verified live on this host — for OpenCode plugin imports), ignored by the untracked `docs/.gitignore`; no tracked consumer. Keep recommendation is sound; files were not committed or deleted, as instructed. Brief's "why do we have a package.json…?" question is answered.

TASK-7: PASS
     notes: `go build ./...` + `go test ./...` in `tui/` all pass (re-verified during review); `bash -n` clean on `new-job.sh`, `init.sh`, `run.sh`; working tree clean with exactly the intended change set (3 renames, no strays); `mg init` template copy path re-verified in a fresh directory; `.manigot/manigot.json` tracked vs `.manigot/jdi-status/` ignored verified.

Commit discipline: one `[kwd2ou] TASK-N:` commit per task (TASK-1..TASK-6) plus the tasks.md commit and a final `[kwd2ou] implementation: add summary` commit. TASK-7 is verification-only (no code changes of its own); its result is documented in the implementation summary commit.

## Security

No security findings. Changes move untracked/tracked data files between directories within the same project root; the `.git/info/exclude` safeguard for the sidecar is preserved (rebuilt from shared constants) and its regression test still passes. No secrets, no new exposure: `.manigot/manigot.json` holds only a public ref name, `.manigot/jdi-status/` is gitignored, `docs/jobs/archive/` left untouched as required.

## Overall

APPROVED

No blockers. Nothing needs to change before merging.

Informational (not a blocker, already noted in implementation.md's Known issues): downstream projects that had a custom `baseBranch` in their old `docs/manigot.json` will lose it on pulling this change (the file is moved, not migrated) and must re-save settings in the TUI — the repo's own file was `{}`, so no local impact here. Same for the untracked `docs/` package artifacts: intentionally kept.
