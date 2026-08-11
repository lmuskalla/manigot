# Verdict: jdi logs new line

id: adtros
status: open
reviewer: claude
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `tui/cmd/jdi/output.go` — `logInvocation`'s header write now ends
`=== ... finished (attempt N) ===\n\n\n` (own trailing newline + two blank
lines) instead of `\n`, so the agent's output starts two lines below the
"finished" statement. Unconditional across every body variant (real text,
`(no output)`, dedup omission note). Doc comments updated: `logInvocation`
's own doc and `sectionWriter`'s "flush against its header" note both now
describe that the writer passes non-header writes through unmodified while
logInvocation itself emits the two separating blank lines. Matches the
tasks.md spec exactly.

TASK-2: PASS
notes: `tui/cmd/jdi/output_test.go` — the assertion in
`TestSectionWriterSeparatesHeadersButNotFirstRunEver` that previously
rejected a blank line between the finished header and its body (the exact
behavior this brief reverses) was replaced with: `finished (attempt 1)
===\n\n\nwrote tasks.md` must be present, and `\n\n\n\n` absent (no more
than two blank lines). The `\n\n\n===` separator counts (3 for 4 headers)
and the sibling sectionWriter/run-log tests are unchanged and pass.

TASK-3: PASS
notes: verification-only, no files changed. I independently re-ran `go test
./...` from `tui/` (all 13 packages pass) and `go vet ./...` (clean). A
scratch render of main()'s `sectionWriter` + `logInvocation` wiring shows
the finished section with two blank lines before the output, as the brief
asked.

Interpretation confirmed: "add two new lines" as two blank lines of
separation is the right reading, and it fixes the reported symptom at its
actual mechanism — the TUI log tab renders run.log through the glamour
markdown viewer (`tui/internal/markdown`), where the old flush format
(`... ===\nWrote tasks.md ...`) was a single markdown paragraph rendered on
ONE line ("the finished statement ends and on the very same line the output
begins"). The new two-blank-line format splits it into separate paragraphs,
so the output renders on its own line below the statement. Raw log/stdout
gets exactly two blank lines as requested; the TUI collapses the two source
blank lines to one visual blank line (inherent markdown behavior) while
still fully resolving the same-line symptom.

## Security

none — the change alters only mg-jdi's log formatting (two newline
characters); no secrets, no new code paths, no input handling.

## Overall

APPROVED

The change does exactly what the brief asked (two new lines between the
"agent finished" statement and the agent output), is scoped to the two
files tasks.md specified, and is covered by an updated unit test plus the
full existing suite. Two non-blocking observations for the record:

- `tasks.md` (the analyst artifact) was swept into the TASK-1 commit rather
  than committed on its own; harmless, since it had to land in the branch
  before implementation anyway and its content matches what was built.
- In the TUI log tab, glamour renders the two source blank lines as one
  visual blank line (markdown paragraph collapse); the literal two-newline
  requirement holds in the raw run.log and in mg-jdi's stdout, which is
  where the brief's symptom lived.
