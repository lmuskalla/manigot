# Verdict: add jq to tailing

id: break
status: open
reviewer: @reviewer
date: 2026-08-28

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `JqLookPath`/`JqAvailable` added in src/internal/launch/launch.go,
exactly mirroring the existing `TigLookPath`/`TigAvailable` seam. Purely
additive, covered by `TestJqAvailableTrueWhenJqResolves` /
`TestJqAvailableFalseWhenJqMissing`.

TASK-2: PASS
notes: `jqTailShellCommand` added alongside the untouched `tailShellCommand`;
`Tail` now picks `jqTailShellCommand` when `JqAvailable()`, else falls back
to the plain command — matches the analyst's decision
(`tail -f '<path>' | jq -R -r 'fromjson? // .'`). Verified the jq filter is
the correct idiom: `fromjson?` suppresses the error on non-JSON lines
(section headers, blank lines) with no output, and `// .` falls through to
the raw line printed unquoted via `-r`; valid JSONL lines parse and
pretty-print (jq's default). Neither shell-command builder is wrapped in
`holdOnFailure`, preserving the Ctrl+C-closes-cleanly behavior on both
branches (asserted by `TestJqTailShellCommandFormat`'s `ec=$?` check).
`shellQuote` still protects the log path; the jq filter itself is a fixed
literal, no injection surface. Confirmed by re-running
`go build ./...`, `go vet ./...`, and the full `go test ./...` in this
review's own sandbox: everything is clean, including the packages the
developer reported as failing in their own sandbox
(internal/git, internal/job, internal/session, cmd/mg, non-tail internal/ui)
— that failure was specific to the developer's session and is not present
here, so it does not block this review; not a finding, just noting the
discrepancy since implementation.md flagged it.

TASK-3: PASS
notes: `stubJqLookPath`/`jqResolves`/`jqMissing` added to
src/internal/ui/tail_test.go, mirroring `stubTigLookPath`.
`TestTailKeyLaunchesTailInTmuxPane` now stubs jq available and asserts the
exact piped command; the new `TestTailKeyFallsBackToPlainTailWhenJqMissing`
covers the other branch and asserts no "jq" substring leaks into the
fallback command. Both pass. Minor nit (non-blocking): the file ends
without a trailing newline (`gofmt -l` flags it) — cosmetic only, doesn't
affect build/vet/test.

TASK-4: PASS
notes: docs/AGENTS.md and project-template/docs/AGENTS.md both updated to
describe the jq pipe and its fallback, matching the actual `Tail` behavior.
Confirmed no `agents/*.md` file mentions tailing (grep), so correctly left
untouched. docs/ROADMAP.md still describes the "l" tail as a plain
`tail -f` pane, but that's a historical status entry for a different,
already-finished job (`guest_fix-tailing`) — out of this job's task list
and not something this job's implementation.md claimed to touch, so not a
finding.

TASK-5: PASS
notes: `go build ./...`, `go vet ./...`, and the targeted
`go test ./internal/launch/... ./internal/ui/... -run 'Jq|Tail'` all pass in
this review's sandbox (verified independently, not just taken on the
developer's word). Full `go test ./...` is also clean here (see TASK-2
note on the sandbox discrepancy). The manual jq smoke test against a real
jq binary could not be run in this sandbox either (no jq installed, no
package-manager privileges) — the developer disclosed this honestly in
implementation.md's "Known issues" rather than papering over it, and the
filter's correctness was independently re-verified by hand above (jq
semantics of `fromjson?` / `//` are unambiguous and this is a well-known
idiom), so the gap doesn't block approval.

## Security

None. No new external input reaches the jq filter (it's a fixed literal);
the log path continues through the existing `shellQuote`. No new
credentials, no new network surface, no change to the git shim or file
permissions.

## Overall

APPROVED

No blockers. Two non-blocking nits for a future pass, not required before
merge:
- src/internal/ui/tail_test.go is missing a trailing newline (gofmt nit).
- docs/ROADMAP.md's "l" description could optionally be refreshed to
  mention the jq pipe, but it's describing a past job's status entry, not
  current-behavior reference docs, so leaving it alone is defensible.
