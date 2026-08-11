# Verdict: Add new global setting

id: r5x2a7
status: reviewed
reviewer: @reviewer (unattended run)
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PARTIAL
notes: Scope-decision content is sound and well-reasoned (storage field,
tmux-override semantics, flag-convention fallback, settings-form field
placement all directly grounded in the brief and `Editor`'s existing
precedent). However it has no commit of its own — `tasks.md`'s ~200-line
scope-decision write-up was committed together with TASK-2's code change in
`faec77a` ("[r5x2a7] TASK-2: add Terminal field to config.Settings"), which
also touches `tui/internal/config/config.go`. `implementation.md` says "the
analyst's scope-decision write-up in tasks.md was already in place before
this run started", but `git log` shows it landed in the same commit as
TASK-2, not before it / separately. Not a functional problem, but it
violates "each task has its own commit" and misattributes TASK-2's commit as
containing TASK-1's analyst output. Should be split into its own `[r5x2a7]
TASK-1: ...` commit (or the message/implementation.md note corrected) before
merge.

TASK-2: PASS
notes: `tui/internal/config/config.go` — `Terminal string
\`json:"terminal,omitempty"\`` added with a doc comment mirroring `Editor`'s;
round-trips via existing `Save`/`Load` JSON marshaling; covered by
`config_test.go`'s new round-trip and default-empty tests. Matches TASK-2 as
specified.

TASK-3: PASS
notes: `tui/internal/launch/launch.go` — `terminal` parameter threaded
through `Agent`/`Quick`/`AgentQuick`/`launchDetached`/`buildCmd`;
`buildOverrideCmd` added implementing the override (first token as binary via
`exec.LookPath`, remaining tokens as leading args, known-name-vs-`-e`
convention lookup via the extracted `terminalCandidates` table); unset
(`terminal == ""`) path verified unchanged — all pre-existing spawn-path
tests pass unmodified aside from the added trailing `""` arg. `launchDetached`
correctly bypasses the tmux-detection branch when `terminal != ""`, matching
TASK-1 point 2. Edge case (whitespace-only override) is defensively guarded
in `buildOverrideCmd`. `go build`/`go vet`/`go test ./...` all pass.

TASK-4: PASS
notes: All three call sites in `tui/internal/ui/app.go`
(`launch.Agent`/`Quick`/`AgentQuick`) updated to pass `a.settings.Terminal`.
Confirmed no other production call site of these three functions was missed.

TASK-5: PASS
notes: `tui/internal/ui/settings.go` — `Terminal` field added as the 5th
field (`stFocusTerminal = 4`, `stFieldCount` 4→5) after Profile, per TASK-1
point 4; existing focus constants unchanged; wired into
`newSettingsView`/`update`/`setFocus`/`settingsValue`/`render`/`hint()`; tab
and shift+tab wrap correctly (verified via `go test` and by reading the
modulo-based cycling logic). The `setFocus` refactor (unconditional blur of
all four inputs before focusing the target) is a reasonable, low-risk
simplification directly motivated by adding the new field, not unrelated
scope creep.
Minor: `gofmt -l tui/internal/ui/settings.go` reports the file as not
gofmt-clean — the `settingsView` struct's field alignment (tab-stops) is off
by one space per field due to the new `terminal` field being added without
re-running `gofmt`. Cosmetic only (build/vet/tests all pass), but should be
fixed with `gofmt -w` before merge for repo hygiene.

TASK-6: PASS
notes: New/updated tests in `config_test.go`, `launch_test.go` (override
bypasses tmux at both `buildCmd` and `launchDetached` levels, known-name
convention reuse incl. case-insensitivity, unknown-name `-e` fallback,
leading-args passthrough, missing-binary error), and `settings_test.go`
(5-field tab/shift-tab cycle incl. new wrap targets, seeding, editing, value
trimming, render). Full suite passes: `go build ./...`, `go vet ./...`, `go
test ./...` all green.

TASK-7: PASS
notes: `docs/AGENTS.md`'s `config/tui-settings.json` bullet and `README.md`'s
"Supported platforms" section both updated accurately, matching actual
behavior (override wins unconditionally, including over tmux). `backlog.md`'s
"In-TUI agent terminal" entry correctly left untouched — it describes a
different, unrelated embedded-PTY feature.

## Scope check

No changes outside `tasks.md`'s file list. `launch.Jdi` (headless) correctly
left untouched, matching the analyst's stated out-of-scope note. No
container/`scripts/*.sh` changes, as scoped. No unrelated refactors found in
the diff.

## Security

None — no new external input trust boundary beyond what `Editor` already
has (an arbitrary shell command string a user types into their own local,
personal settings file, then invoked in the shell they'd run manually
anyway). The `-e`/missing-binary error paths fail closed (clear error, no
silent fallback).

## Overall

NEEDS WORK — two small, easy fixes, no design or functional problems:

1. Split TASK-1's `tasks.md` write-up out of TASK-2's commit (`faec77a`)
   into its own `[r5x2a7] TASK-1: ...` commit, so every task has its own
   commit per repo convention — and correct `implementation.md`'s claim that
   TASK-1 "was already in place before this run started" to match what
   actually happened.
2. Run `gofmt -w tui/internal/ui/settings.go` to fix the struct-field
   alignment `gofmt -l` currently flags.

Everything else — requirement coverage, correctness, edge cases (unset
override, whitespace-only override, unknown terminal name, missing binary,
tmux interaction), test coverage, and documentation — is solid and matches
`tasks.md`/`brief.md`. Re-review after the two fixes above should be a quick
pass.
