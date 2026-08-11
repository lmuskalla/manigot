# Verdict: Add new global setting

id: r5x2a7
status: reviewed
reviewer: @reviewer (unattended run, second pass)
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

This is a second-pass review after the branch's first `verdict.md` (NEEDS
WORK: TASK-1 commit split + gofmt) was addressed. Re-verified everything from
scratch against `git diff main...HEAD`, not just the two flagged fixes.

TASK-1: PASS
notes: `docs/jobs/r5x2a7_add-new-global-setting/tasks.md`'s scope-decision
write-up now has its own commit (`f6397a2`, touches only `tasks.md`), separate
from TASK-2's `config.go` change (`4113b2d`). `implementation.md`'s TASK-1
note was corrected to describe the rebuild accurately. Confirmed the rebuild
was truly non-destructive: `git diff backup-r5x2a7-presplit HEAD --stat` shows
only the later gofmt fix and `implementation.md` edits — the split itself is
byte-for-byte tree-identical to the pre-split tip, as claimed.

TASK-2: PASS
notes: `tui/internal/config/config.go` — `Terminal string
\`json:"terminal,omitempty"\`` added with a doc comment mirroring `Editor`'s;
round-trips via existing `Save`/`Load`; covered by `config_test.go`'s new
round-trip and default-empty tests.

TASK-3: PASS
notes: `tui/internal/launch/launch.go` — `terminal` parameter threaded through
`Agent`/`Quick`/`AgentQuick`/`launchDetached`/`buildCmd`; `buildOverrideCmd`
implements the override (first token as binary via `exec.LookPath`, remaining
tokens as leading args, known-name-vs-`-e` convention via the extracted
`terminalCandidates` table, blank-override guard). Unset (`terminal == ""`)
path verified unchanged: all pre-existing spawn-path tests pass with only the
added trailing `""` arg. `launchDetached` bypasses the tmux-detection branch
when `terminal != ""`, matching TASK-1 point 2, and this is exercised at both
the `buildCmd` and `launchDetached` levels in `launch_test.go`. `go build`/
`go vet`/`go test ./...` all pass.

TASK-4: PASS
notes: All three call sites in `tui/internal/ui/app.go`
(`launch.Agent`/`Quick`/`AgentQuick`) updated to pass `a.settings.Terminal`.
No other production call site of these three functions exists.

TASK-5: PASS
notes: `tui/internal/ui/settings.go` — `Terminal` field added as the 5th field
(`stFocusTerminal = 4`, `stFieldCount` 4→5) after Profile, per TASK-1 point 4;
existing focus constants unchanged. Tab/shift+tab cycling uses generic
modulo arithmetic (`(v.focus + 1) % stFieldCount`), not a per-case wrap
table, so there's no separate wrap-target to get wrong. `setFocus`'s refactor
to unconditionally blur all four inputs before focusing the target is a
reasonable, low-risk simplification directly motivated by the new field, not
unrelated scope creep. `gofmt -l tui/internal/ui/... ` now reports nothing
(confirmed directly) — the struct-alignment issue flagged in the first-pass
verdict is fixed.

TASK-6: PASS
notes: New/updated tests in `config_test.go`, `launch_test.go` (override
bypasses tmux at both `buildCmd`/`launchDetached` levels, known-name
convention reuse incl. case-insensitivity, unknown-name `-e` fallback,
leading-args passthrough, missing-binary error, unset-terminal smoke check),
and `settings_test.go` (5-field tab/shift-tab cycle incl. new wrap targets,
seeding, editing with focus routed via 4 tabs, value trimming, render).
Verified independently: `go build ./...`, `go vet ./...`, `go test ./...` all
green from a clean run in this review.

TASK-7: PASS
notes: `docs/AGENTS.md`'s `config/tui-settings.json` bullet and `README.md`'s
"Supported platforms" section both updated accurately, matching actual
behavior (override wins unconditionally, including over tmux).
`docs/backlog.md`'s "In-TUI agent terminal" entry confirmed unchanged —
correctly describes a different, unrelated embedded-PTY feature.

## Scope check

`git diff main...HEAD --stat` shows only the files `tasks.md` lists per task,
plus the four job-workflow docs themselves (`brief.md`, `tasks.md`,
`implementation.md`, `verdict.md`) and their own commits. No
container/`scripts/*.sh` changes. `launch.Jdi` (headless) untouched, matching
the stated out-of-scope note. No unrelated refactors in the diff.

One minor, non-blocking observation: a local branch `backup-r5x2a7-presplit`
(created during the TASK-1/TASK-2 commit-split rebuild, per
`implementation.md`) still exists in this worktree. It's inert — not part of
`main...HEAD`, won't be touched by `mg done`'s squash-merge — but is stray
housekeeping that could be deleted (`git branch -D backup-r5x2a7-presplit`)
once this job is merged. Not a blocker.

## Security

None — no new external input trust boundary beyond what `Editor` already has
(an arbitrary shell command string a user types into their own local,
personal settings file, then invoked in the shell they'd run manually
anyway). The `-e`/missing-binary/blank-override error paths fail closed
(clear error, no silent fallback to auto-detect).

## Overall

APPROVED — both issues from the first-pass verdict (TASK-1's missing own
commit, `gofmt` alignment) are fixed and independently re-verified. Every
task in `tasks.md` is implemented as scoped, `go build`/`go vet`/`go test
./...` and `gofmt -l` are all clean, commit discipline is correct (one
commit per task, `implementation.md` and `verdict.md` each have their own
commits), and the diff contains no changes outside what was scoped. The one
non-blocking note above (stray `backup-r5x2a7-presplit` local branch) can be
cleaned up at leisure and does not need to hold up merge.
