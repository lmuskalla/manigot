# Verdict: more log for jdi

id: rj4prf
status: open
reviewer: claude
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

This is a second review pass, after a prior NEEDS WORK round flagged that
`tasks.md` was never committed to the branch. That has since been fixed
(`8713ee5`) and confirmed clean below.

TASK-1: PASS
notes: `logStarted(w, jobName, profile string)` added in `output.go`
(`tui/cmd/jdi/output.go:109`), writes a `=== <RFC3339> mg jdi started: job
<name>, profile <profile> ===` line, styled like `logImmediateStop`. Called
in `main()` at `main.go:140`, immediately after `logDest` is finalized and
before `Run` is invoked — the first line written to both stdout and
run.log. Tested (`TestLogStarted`).

TASK-2: PASS
notes: `logAgentInvoked(w, agent string, attempt int)` added
(`output.go:117`), called in `Run` at `main.go:353`, immediately before
`runner.Run(...)`, reusing the same `i+1` attempt number the post-run header
uses. Tested (`TestLogAgentInvoked`, `TestRunLogsAgentInvoked`).

TASK-3: PASS
notes: `logInvocation`'s header reworded to `=== <timestamp> <agent>
finished (attempt N) ===` (`output.go:206`). Existing assertions in
`output_test.go` (`TestLogInvocationPlainText`, etc.) updated to the new
wording, not just left alone.

TASK-4: PASS
notes: `logJobFinished(w, kind, reason string, includeReason bool)` added
(`output.go:130`), called from every exit point of `Run`'s `finish` closure
(`main.go:339`) — verified all `return finish(...)` call sites in `Run` route
through it, no bypass. `includeReason` is correctly `false` only for the
stop-before-any-agent-ran path, where `logImmediateStop` already printed the
same reason a line above; `TestRunImmediateStopDoesNotDuplicateReason`
explicitly asserts the reason string appears exactly once in that case.
`TestRunLogsJobFinishedOnNormalStop` covers the normal-stop path.

TASK-5: PASS
notes: `logInvocation` gained `targetFile`/`targetContent` params
(`output.go:206`); `Run` reads the just-run agent's target file fresh off
disk via the new `agentTargetFile` map and `os.ReadFile(filepath.Join(j.Dir,
targetFile))` (`main.go:369-378`) and passes it through. `isDuplicateOutput`
does a trimmed, whitespace-normalized substring check (`output.go:158`); an
empty file content correctly never counts as a match, so an unreadable/
missing file degrades to a no-op rather than a false-positive omission.
Matches `tasks.md`'s open-question framing (heuristic, not exact-equality).
Well tested: `TestIsDuplicateOutput` table (all four cases: exact substring,
whitespace differences, no match, empty-file-never-matches),
`TestLogInvocationOmitsDuplicateOutput`,
`TestLogInvocationKeepsDistinctOutput`,
`TestLogInvocationSkipsDedupWhenTargetFileEmpty`.

TASK-6: PASS
notes: `TestRunFullLogSequenceHappyPath` and
`TestRunFullLogSequenceDedupsMatchingOutput` in `main_test.go`, using the new
`assertLogOrder` helper, assert the complete started → invoked → finished
[full or omitted body] → ... → job finished sequence appears in strict order
for both a normal run and the dedup case — genuinely end-to-end, not just
each helper tested in isolation.

All code changes are confined to `tui/cmd/jdi/` (`main.go`, `output.go`,
`main_test.go`, `output_test.go`), matching `tasks.md`'s stated scope — no
TUI changes, correctly, since `internal/ui/detail.go`'s log tab has no
header-specific parsing to update. `go build ./...`, `go vet ./...`, and `go
test ./...` all pass (verified directly in this review). `gofmt -l .` shows
one finding, `internal/ui/settings.go`, confirmed via `git diff
main...HEAD -- tui/internal/ui/settings.go` to be untouched by this branch
(pre-existing, not introduced here).

## Commit discipline

Every task has its own `[rj4prf] TASK-N: ...` commit
(`4e11788`..`709b5e1`), plus `[rj4prf] implementation: add summary`
(`9647659`). The prior round's blocker — `tasks.md`'s real analyst content
(Context, TASK-1..6, open question) sitting uncommitted in the working tree,
which would have reverted to the blank scaffold on merge — is fixed:
`8713ee5` commits it, confirmed via `git diff main...HEAD --stat` showing
`tasks.md` with the full 124-line breakdown present in the branch's actual
diff, not just the working tree. `implementation.md` documents the fix
(`bfce337`).

## Working-tree note (non-blocking for this job)

`docs/NAMING.md` still has an uncommitted, unrelated "Parking lot" change
sitting in the working tree (`git status` shows it modified). It predates
this job, is not part of `brief.md`'s scope, and — because it was never
committed to any commit on this branch — is not part of `git diff
main...HEAD` and will not be merged in either way; it's working-tree state,
not branch content. The developer's call to leave it alone rather than
commit or discard someone else's unrelated change is reasonable and not this
job's responsibility to resolve. Flagging only so a human notices it's still
sitting there for whenever they next touch that file directly.

## Security

none

## Overall

APPROVED

All six tasks are correctly implemented, well tested (including genuine
end-to-end coverage of the full log sequence), and properly scoped to
`tui/cmd/jdi/`. The prior round's blocker — `tasks.md` never committed — is
resolved and verified. `go build`/`go vet`/`go test` all pass. No further
changes needed before merge.
