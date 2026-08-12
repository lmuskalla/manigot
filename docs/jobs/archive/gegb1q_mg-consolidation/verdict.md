# Verdict: mg consolidation

id: gegb1q
status: open
reviewer: @reviewer
date: 2026-08-12

## Review

Re-check of the previous NEEDS WORK verdict (fresh-repo job-creation
regression in `job.CreateJob`, plus non-blocking `gofmt -l` findings). The
developer's follow-up is exactly the prescribed fix, verified independently:

- Commit `bc490b6` `[gegb1q] TASK-16: fix fresh-repo (zero commits) create
  regression + gofmt pass` and commit `2652cf1` `[gegb1q] implementation:
  record review fix for fresh-repo create regression` are the only new
  commits since the last verdict — no scope creep. The post-verdict diff is
  precisely: `internal/job/create.go` (the decision fix), `create_test.go`
  (new test + gofmt), four gofmt-only files (`cmd/mg/agents.go`,
  `internal/session/docker_test.go`, `internal/ui/app.go`,
  `internal/ui/settings.go`), and `implementation.md`.
- TASK-16 fix: `CreateJob` now bases the git/non-git decision on the
  no-branches signal — `git.LocalBranches(root)` (empty → non-git fallback),
  tolerating `git.ErrNotARepo`, propagating any other git error. This matches
  the verdict's prescription and new-job.sh's `git rev-parse --abbrev-ref
  HEAD` probe, which fails on an unborn HEAD. The `errors` import was already
  present. `git.for-each-ref refs/heads/` on an unborn-HEAD repo exits 0 with
  empty output, so the decision is sound.
- New test `TestCreateJobFreshRepoNoCommits` added and exercised (PASS): a
  `git init -b main` repo with no commits takes the non-git path, asserting
  `branch: (no git)` in the result, the exact warning line, the job dir under
  the project root's `docs/jobs/`, and the `branch: (no git)` field in the
  written brief.md.
- The verdict's exact repro was re-run by hand with the real `bin/mg`:
  `git init -b main` + `mg job "Fresh Repo Job"` → "Warning: not a git
  repository — skipping branch/worktree creation", `branch: (no git)`, exit
  0 — identical to the script.
- Normal git path unregressed: repo with a commit still creates the sibling
  worktree (`/tmp/.manigot-worktrees/<root>/<id>_<slug>`), the
  `fix/<id>_<slug>` branch, and the "Scaffold job" commit; full `mg done`
  roundtrip re-verified (verdict warning + "Continue anyway?", "Proceed?",
  archive move with `status: done`, squash merge onto main, worktree remove,
  branch `-D`, exit 0). Declined/EOF confirmations map to exit 0 without
  destroying anything (the script's `exit 0`), as before.
- `mg delete` on a fresh-repo-created job exits 1 with "job not found among
  local branches / Active job branches:" — verified byte-equivalent to
  delete-job.sh's behavior in the same state (the script's non-git path is
  gated on `git rev-parse --git-dir` failing, which a fresh `git init` repo
  passes, so the script also took the git path and errored). Pre-existing
  quirk of the original design, faithfully ported, not a regression.
- `gofmt -l` clean across the repo; `go build ./...`, `go vet ./...` and
  `go test ./...` all green (14 packages), including the new test.

- TASK-1: PASS — unchanged from prior review (module relocated and renamed,
  all imports updated, root build/test green).
- TASK-2: PASS — unchanged; dispatcher matches mg.sh (superseded by later
  phases).
- TASK-3: PASS — unchanged; Dockerfile prewarm COPY at root go.mod/go.sum.
- TASK-4: PASS — unchanged; `config.GetEnv`/`UpsertEnv` + `internal/cli`
  prompts with wording pinned by tests.
- TASK-5: PASS — unchanged; `mg profiles` matches profiles.sh 1:1.
- TASK-6: PASS — unchanged; `mg setup` wizard, `--check`, non-TTY refusal,
  `~/.claude.json` auto-extraction.
- TASK-7: PASS — unchanged; `mg agents`/`crew` menu + re-exec launch.
- TASK-8: PASS — unchanged; `mg init` template copy + @prompter hand-off.
- TASK-9: PASS — unchanged; profile precedence, legacy `--tool opencode`
  incl. `--print` rejection, auth checks, all pinned by tests.
- TASK-10: PASS — unchanged; root + `--job` resolution (exact→prefix,
  ambiguity, worktree lookup, HARD error on branch-without-worktree,
  no-branches flat scan).
- TASK-11: PASS — unchanged; docker argv/mount/env pinned by
  `docker_test.go`; `--print` split verified empirically.
- TASK-12: PASS — unchanged; bare `mg`/session flags run in-process.
- TASK-13: PASS — unchanged; jdi runner on the session `--print` path.
- TASK-14: PASS — unchanged; launch shell strings on `os.Executable()`.
- TASK-15: PASS — unchanged; lifecycle git ops tested; notARepo
  case-insensitive fix correct.
- TASK-16: PASS — the fresh-repo regression is fixed exactly as prescribed
  (see above): no-branches signal decision, zero-commits create test,
  verified by hand against the reviewer's repro; the git path and
  nested/mount-point/non-git cases all still pass.
- TASK-17: PASS — unchanged; full roundtrip re-verified by hand.
- TASK-18: PASS — unchanged; delete port matches delete-job.sh incl. the
  fresh-repo error path re-verified above.
- TASK-19: PASS — unchanged; subcommands + TUI in-process lifecycle calls.
- TASK-20: PASS — unchanged; `resolve` deleted, `internal/home` added.
- TASK-21: PASS — unchanged; tui/jdi folded into cmd/mg; only
  `scripts/entrypoint.sh` remains.
- TASK-22: PASS — unchanged; Makefile/README/docs/AGENTS.md synced.
- TASK-23: PASS — unchanged; `make check`; docker-dependent smokes recorded
  as a manual gap (no docker daemon/TTY here — correct handling).

## Security

No change since the prior review: no secrets committed, `.env` gitignored,
docker hardening and subscription-protection checks preserved verbatim, no
new network surface. The fix touches only the job-creation decision logic.
No findings.

## Overall

APPROVED

The single blocker from the previous verdict — TASK-16's fresh-repo (no
commits) create regression — is fixed exactly as prescribed, with the
prescribed test, verified both by the test suite and by hand against the
original repro. The non-blocking `gofmt -l` findings are all cleaned up
(`gofmt -l` now empty). Build, vet, and the full test suite are green; the
post-verdict diff is confined to the fix, its test, gofmt cleanups, and the
implementation record. No blockers remain.

One observation for the record (no change required): `CreateJob`'s new
no-branches decision means a pathological repo with commits but zero local
branches (detached HEAD, all refs deleted) now takes the non-git fallback
where the script's `rev-parse` probe would have taken the git path and
errored on the missing base branch. This aligns with the brief's own
no-branches definition and with the session launcher's `--job` flat-scan
fallback, and is not a realistic state; noted only for completeness.
