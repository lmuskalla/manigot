# Verdict: cli menus with go

id: examine
status: open
reviewer: reviewer
date: 2026-08-13

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed the full `git diff main...HEAD` (14 files) against tasks.md, plus
each task commit individually. Shell access in this session is git-only, so
`go build/vet/test` could not be re-run here; the build/test claim was
verified statically (imports, symbol usage, fixture ordering all checked and
consistent).

TASK-1: PASS
notes: internal/ui/picker.go — self-contained tea.Model: title, scrollable
rows with cursor/offset windowing (clampView keeps the cursor visible and
inside the filtered list, rowsAreaHeight accounts for the 4-line chrome plus
the optional filter line), up/down/k/j + home/end + g/G navigation, enter
submit, esc/q/ctrl+c cancel, tea.WindowSizeMsg resize with an 80×24 default,
RunPicker on the alt screen, PickerResult. Reuses the shared styles
(titleStyle/dimStyle/selectedStyle/accentStyle) and truncate() from the same
package — consistent with the existing TUI. picker_test.go drives it through
Update/View like agentspicker_test.go: navigation/clamping, submit/cancel,
empty-list enter, scroll state + render window, resize, and full render
assertions. Edge cases hold up under scrutiny: empty/nil rows, zero-height
terminals (rowsAreaHeight floors at 1), truncate with a negative width
returns the string unchanged (no panic), enter with no matches does not
submit.

TASK-2: PASS
notes: Type-to-filter is case-insensitive substring matching against
SearchKey; backspace edits the filter (rune-safe), esc clears it first and
only falls through to cancel once empty, the cursor is clamped into the
filtered list, a matching-nothing filter renders "no matches" and blocks
enter. The navigation-vs-input resolution (vi keys keep their roles while
the filter is empty; every printable key — including j/k/q/g — types once a
filter is active; arrows/home/end always navigate) is documented in the type
doc and footer and pinned by TestPickerFilterNavInterplay. The one known
limitation (a filter cannot *start* with g/j/k/q/G) is the documented
modeless trade-off, listed in implementation.md's known issues. No bug found
in the esc/backspace/clamp interaction paths.

TASK-3: PASS
notes: cmd/mg/jobs.go + cmd/mg/picker.go — rows are built from the same
ID/status/type/date/title + jobsBadge columns as the plain listing, search
key id + title; the picker runs through the injected pickerRunFunc seam
(following the existing confirm-func injection pattern), with ttyPicker
wired in main.go. Submit prints "→ Starting a session in <id>..." and
re-execs `mg --job <id> <passthrough>` via the pre-existing reexec; cancel
returns 0 quietly — exactly the deliberate UX change tasks.md's decisions
section asks to confirm (confirmed, and it is the only intentional
deviation from the old "quit" error path). The orphaned-worktree surfacing +
removal offer still runs before the picker; the bufio.Reader is now scoped
to just that confirm so no shared reader sits on stdin ahead of the picker
(an improvement over the old shared-reader flow). Non-TTY listing + refusal
strings are byte-identical (format verbs unchanged). Tests: non-TTY/empty/
orphan paths pass pickerStub, the launch test uses pickerChoice, and
TestJobsPickerGetsJobRows pins the exact rows and the quiet cancel; row-0
ordering matches job.Discover's date-desc sort.

TASK-4: PASS
notes: cmd/mg/agents.go mirrors TASK-3 — rows show name, description and the
"(project)"/"(project override)" tag (agentSource), search key name +
description, submit re-execs `mg --agent <name> <passthrough>`, cancel exits
0, non-TTY listing + refusal byte-identical. cli.Select removed; the `r`
parameter is now unused in runAgents (harmless, signature kept for symmetry).
Row-order assertions in TestAgentsPickerGetsAgentRows match
agentlist.Discover's ordering (globals sorted by name with overrides, then
project-only additions sorted) — verified deterministic.

TASK-5: PASS
notes: docs/AGENTS.md (internal/ui architecture bullet now mentions the
picker, the "mg agents" section's stale numbered-selection wording, and the
Commands list), README.md (commands table + Agents section), and the
`mg --help` text in cmd/mg/main.go are all synced. agents/*.md scanned — no
stale numbered-selection mention exists. project-template/docs/AGENTS.md
verified to contain no mg jobs/mg agents content at all (blank per-project
template), so nothing to update there; the AGENTS.md ↔ template sync rule
holds.

TASK-6: PASS
notes: Verification task — build/vet/test results claimed green and a manual
PTY smoke (navigation, filter, enter, esc-clear-then-cancel, non-TTY refusal
byte-identical, orphan offer, re-exec stopping at the expected missing-cred
error). Verified statically here; code and tests are internally consistent
and compile-clean on inspection. No TTY-path issues found in the alt-screen
restore / post-picker stdout ordering.

Commit discipline: one `[examine] TASK-N: ...` commit per task, plus a
separate `[examine] implementation: add summary` commit; working tree clean.
TASK-1's commit also carries the authored tasks.md (the analyst's deliverable
landing in the first task commit) — minor, not a blocker. No out-of-scope
changes: the only files touched beyond the task lists are cmd/mg/picker.go
(the seam the tasks explicitly call for) and the main.go wiring lines.

## Security

No security findings. The change is a TTY-only interactive selection layer;
it adds no new privilege surface (re-exec and confirmation paths are the
pre-existing ones), and the picker reads/writes only the terminal.

## Overall

APPROVED

All six tasks are implemented as specified, non-TTY behavior is preserved
byte-for-byte, tests are reworked through the injected seam without starting
real Bubble Tea programs, documentation is in sync, and no out-of-scope
changes or correctness bugs were found. The cancel-exits-0 change requested
for confirmation in tasks.md is implemented as intended.
