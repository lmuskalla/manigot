# Verdict: opencode themes

id: prayer
status: open
reviewer: claude
date: 2026-08-27

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

This is a re-review of the branch after the developer's post-verdict
follow-up (commits `7785b68` and `9382662`) addressed the two blockers from
the prior `NEEDS WORK` verdict (`e98a9af`). Confirmed via `git log
main..HEAD`, full `git diff main...HEAD`, `go build ./...`, and targeted
`go test` runs of every theme-touching package.

TASK-1: PASS
notes: `internal/config/config.go`/`config_test.go` are correct —
`Settings.Theme`/`ThemeValue()` and `Load`/`Save` wiring to `OPENCODE_THEME`
mirror `Profile`'s persistence pattern, with round-trip, default-value, and
env-persistence tests, all passing (`TestThemeValueDefaultsToEmpty`,
`TestSaveThenLoadRoundTripsTheme`, `TestThemeDefaultsToEmptyAutoDetect`).
Commit-discipline gap from the prior verdict (TASK-1's diff bundled into
`9b43e97` "chore: commit pre-existing analyst tasks.md" rather than its own
`[prayer] TASK-1: ...` commit) remains in the immutable history — confirmed
still true (`git show 9b43e97 --stat` shows `internal/config/config.go`/
`config_test.go` alongside `tasks.md`). This is not fixable at this point:
this reviewer's own git access is the same restricted shim (`add`, `commit`,
`log`, `diff`, `show`, `status`, ...) with no `rebase`/`reset`/`commit
--amend`, confirming the developer's claim that splitting the commit
retroactively is genuinely not possible under this session's tooling. The
developer's `implementation.md` "Follow-up" section documents this clearly,
which is exactly the resolution the prior verdict itself offered as
acceptable ("a note explaining why it can't be split further at this
point"). Not a blocker.

TASK-2: PASS
notes: Core forwarding logic (`internal/session/session.go`'s
`OpenCodeTheme`/`CheckAuth`) and the researched `tui.json` pivot (verified
`scripts/entrypoint.sh` with `bash -n` — passes) are unchanged from the prior
review and still correct. The `mg host` scope leak is now fixed:
`internal/session/host.go`'s `hostEnv` excludes `OPENCODE_THEME` the same way
it excludes `OPENCODE_MODEL`, with a clear comment explaining why (container-
only knob, no host-mode equivalent) and a new test,
`TestBuildHostOpenCodeThemeNotForwarded`, which passes and correctly asserts
both that `OpenCodeTheme` is populated (sanity check) and that it's absent
from the built host invocation's env. All theme-related tests in
`internal/session` pass:
`TestResolveProfileThemeForwardedIndependentOfProfile` (all four OpenCode
profiles), `...ThemeUnsetOmitsKeyEnv`, `...ThemeNotForwardedForClaudePro`,
`...ThemeForwardedForLegacyOpenCode`, `TestBuildHostOpenCodeThemeNotForwarded`.
Remaining minor gap (carried over from the prior review, and not listed as a
blocker there either): `internal/session/docker_test.go` still has no
argv-level `-e OPENCODE_THEME=...` assertion alongside the existing
`OPENCODE_MODEL=...` ones — `session_test.go`'s tests only check
`info.KeyEnv`. Not a functional bug (the `argv = append(argv,
info.KeyEnv...)` passthrough is trivial and already covered), just an
unclosed loose end in the task's own file list. Non-blocking.

TASK-3: PASS
notes: Unchanged from the prior review — `cmd/mg/theme.go` mirrors `mg
profiles`' shape correctly (listing + TTY picker, `themeSet`/`themeList`,
unrecognized names accepted with a note rather than rejected), wired into
`main.go`'s dispatch (`case "theme":`) and `printHelp`. `theme_test.go`'s 10
tests all pass.

TASK-4: PASS
notes: Unchanged from the prior review — `internal/ui/settings.go`'s Theme
field is appended after Terminal without disturbing existing `stFocus*`
values, free-text (correctly not fixed-list), wired through
`resize`/`update`/`setFocus`/`settingsValue`/`render`/`hint`, and reaches
`config.Save` via `app.go`'s existing `settingsValue()` → `config.Save(s)`
call — confirmed end to end. `settings_test.go`'s tab-cycle and new
seed/edit/trim/render tests all pass.

TASK-5: PASS
notes: Unchanged from the prior review — `docs/AGENTS.md`,
`project-template/docs/AGENTS.md`, and `README.md` accurately describe what
was actually built (the `tui.json` split, not the originally-assumed
`opencode.json` key), including the `mg theme` command entries and the
`.env` key list. Re-checked against the current `entrypoint.sh` and
`config.go` — still accurate.

## Security

None run (no new attack surface: `mg theme` only writes a local `.env` key a
user already controls, and the container-side change is a config file
write gated the same way the existing model write is).

## Scope check

`git diff main...HEAD --stat` shows exactly the files tasks.md named across
TASK-1 through TASK-5 (config, session, host, cmd/mg/theme, ui/settings,
docs/README) plus the job's own four files — nothing unrelated was touched,
and `mg host`'s only change is the fix for the flagged scope leak itself, not
a broader host.go refactor. `go build ./...` succeeds. All theme-specific
tests pass; the pre-existing `git init`-blocked failures across the rest of
the suite (job lifecycle, TUI list/detail, mg-jdi fixtures) reproduce
identically on `main` and are unrelated to this job's changes (confirmed by
running the same suites — the failures are all in files this job never
touches).

## Overall

APPROVED

Both blockers from the prior `NEEDS WORK` verdict are resolved:
1. TASK-1's commit-discipline gap is documented as unfixable under this
   session's git-shim restrictions (verified true by this reviewer's own
   restricted git access) — the prior verdict's own suggested fallback
   resolution, so no further action is required.
2. The `OPENCODE_THEME` leak into `mg host` sessions is fixed with a test
   (`TestBuildHostOpenCodeThemeNotForwarded`) and an explanatory comment.

Everything else remains solid. Ready to merge via `mg done`.
