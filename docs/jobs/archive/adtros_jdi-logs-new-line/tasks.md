# Tasks: jdi logs new line

id: adtros
status: open
analyst: claude
date: 2026-08-11

<!-- Produced by @analyst from brief.md. -->

## Context

mg-jdi's log output (both its stdout for a direct CLI run and the sidecar
`docs/jobs/.jdi-status/<job>/run.log`, which share one writer — see
`tui/cmd/jdi/main.go`'s `io.MultiWriter` and the `sectionWriter` wrapper) is
produced by the `log*` functions in `tui/cmd/jdi/output.go`. Each agent
invocation writes a `=== <timestamp> <agent> finished (attempt N) ===` header
followed immediately — flush, on the very next line — by the agent's
extracted response text (`logInvocation`, output.go:256-269). The
`sectionWriter` deliberately inserts a blank line *before* every
`=== ... ===` header so events don't crumble together, but the *body* is
passed through untouched and stays glued to its header.

The brief: after "agent xy has finished", the agent's output starts right
away with no separation, which reads (on a wrapped or narrow terminal, and
in the TUI's log tab which renders the raw run.log — `detail.go`'s log tab
is `job.ReadJDIRunLogTail`'s raw content, no reformatting) as the output
beginning on the same line as the finished statement. The fix is to add two
new lines between the "agent finished" statement and the agent's output, so
a finished section reads:

```
=== 2026-08-11T18:30:00+02:00 analyst finished (attempt 1) ===


Wrote tasks.md with one task.
```

(statement line, then two blank lines, then the output). Interpretation note:
"add two new lines" is read as *two additional newline characters* after the
header's own terminating newline → two blank lines of separation. If the
intent were a single blank line, @reviewer should flag it; the change is a
one-character diff either way.

Scope: only the agent *output* after the `finished` header. The `invoked`
header, the `mg jdi finished` event, and the stop-reason lines
(`logJobFinished`/`logImmediateStop`) keep their current flush bodies —
the brief asks only about the finished statement → output pair.

## Task breakdown

TASK-1: In `logInvocation` (`tui/cmd/jdi/output.go`), emit two blank lines
between the `=== <ts> <agent> finished (attempt N) ===` header and the
agent's output text, so the output starts two lines below the finished
statement instead of on the line right after it. Applies to every body
variant (real text, `(no output)`, and the dedup `(output matches <file>,
omitted)` note) — the separation is between header and body, unconditional.
Simplest form: extend the header's `fmt.Fprintf` format string to end with
`\n\n\n` (header line + two blank lines), or write `\n\n` as its own write
after the header — both are equivalent through `sectionWriter` (the blank
lines land before the body's own write, which still passes through
untouched). Also update the `logInvocation` doc comment (output.go:230-255)
to describe the two-blank-line separation, and adjust `sectionWriter`'s
"must stay flush against its header" note (output.go:135-138) so it
clarifies that logInvocation itself now writes the separating blank lines
(the writer still passes non-header writes through unmodified).
     files: tui/cmd/jdi/output.go
     depends: none
     risk: low — a one-spot formatting change in the single function every
       agent-output write funnels through; stdout and run.log share the
       writer, so both destinations get the fix together.

TASK-2: Update `TestSectionWriterSeparatesHeadersButNotFirstRunEver`
(`tui/cmd/jdi/output_test.go`, lines 295-299): its assertion that the
invocation body "must stay flush against its finished header" (rejects any
`finished (attempt 1)\n\nwrote tasks.md`) now encodes the exact behavior
this brief reverses. Replace it with an assertion that the finished header
is followed by exactly two blank lines before the body (e.g.
`finished (attempt 1)\n\n\nwrote tasks.md` must be present), and update the
comment. The test's `\n\n\n===` separator counts (3 for 4 headers) are
unaffected — verified by trace: the new blank lines sit between header and
body, never before a subsequent `===` header. No other test asserts the
flush behavior (grep for `flush|blank line leaked|\\n\\n\\n` in tui/ shows
only this one).
     files: tui/cmd/jdi/output_test.go
     depends: TASK-1
     risk: low — a single inverted assertion in one test, with the
       separator-count expectations in the same test (and in
       TestRunLogSeparationAcrossRuns / TestSectionWriterJoinsPriorRunWithBlankLine
       / TestSectionWriterPassesThroughNonHeaders) unchanged.

TASK-3: Verify the change end to end: run `go test ./...` from `tui/` (the
module root — `go.mod` lives there, not at the repo root), plus `go vet
./...`, and render a simulated finished section (e.g. a tiny scratch
program mimicking main()'s `sectionWriter` + `logInvocation` wiring, or a
throwaway test) to confirm the output reads as the brief wants: finished
statement on its own line, two blank lines, then the agent output.
     files: none (verification only)
     depends: TASK-1, TASK-2
     risk: low — pure verification; the only plausible surprise would be an
       unseen test depending on the old flush format, which the grep in
       TASK-2 already rules out.
