# Implementation: jdi logs new line

id: adtros
status: open
developer: claude
date: 2026-08-11

<!-- Produced by @developer after implementation. -->

## Summary

mg-jdi's log output glued the agent's response text directly to the
`=== <timestamp> <agent> finished (attempt N) ===` line, so the output read
as starting on the very same line as the finished statement. `logInvocation`
now writes two blank lines between the "finished" header and the agent's
output, so a finished section reads:

```
=== 2026-08-11T18:30:00+02:00 analyst finished (attempt 1) ===


Wrote tasks.md with one task.
```

Both mg-jdi destinations (stdout for a direct CLI run and the sidecar
`docs/jobs/.jdi-status/<job>/run.log` for the TUI log tab) share the same
writer, so one change fixed both. The `invoked` header, the `mg jdi
finished` event, and the stop-reason bodies are untouched — the brief asked
only about the finished statement → output pair.

## Changes

TASK-1: `tui/cmd/jdi/output.go` — `logInvocation`'s header write now ends
with `\n\n\n` (the header's own trailing newline plus two blank lines)
instead of `\n`, so the agent's output starts two lines below the
"finished" statement. The separation is unconditional, applying to every
body variant (real text, `(no output)`, and the dedup
`(output matches <file>, omitted)` note). Doc comments updated:
`logInvocation`'s own doc now describes the two-blank-line separation, and
`sectionWriter`'s "must stay flush against its header" note now clarifies
that the writer itself never inserts blank lines inside a section — the
invocation body's separation is logInvocation's own write.

TASK-2: `tui/cmd/jdi/output_test.go` — the assertion in
`TestSectionWriterSeparatesHeadersButNotFirstRunEver` that rejected a blank
line between the finished header and its body (which encoded exactly the
behavior this brief reverses) was replaced with two assertions: the
finished header must be followed by exactly two blank lines before the body
(`finished (attempt 1) ===\n\n\nwrote tasks.md` present), and no more than
two (`\n\n\n\n` absent). The test's `\n\n\n===` separator counts (3 for 4
headers) and the sibling sectionWriter tests were unaffected, as traced.

TASK-3: verification only, no files changed. `go test ./...` from `tui/`
(the Go module root) passes for every package, `go vet ./...` is clean, and
a scratch program replicating main()'s `sectionWriter` + `logInvocation`
wiring rendered the finished section with the two blank lines as intended.

## Known issues / follow-ups

Interpretation note (also flagged in `tasks.md`): the brief's "add two new
lines" was read as two blank lines between the statement and the output
(three newlines total after the header's last character). If a single blank
line was intended, the change is a one-character diff
(`\n\n\n` → `\n\n`) in `logInvocation` plus the corresponding test strings.

Pre-existing quirk, not addressed: if an agent's response text itself began
with `===`, the `sectionWriter` would treat that body write as a header and
prefix it with the header separator — behavior predates this job and is
unrelated to the finished/body spacing.
