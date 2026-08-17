# Verdict: review cycle message

id: cold
status: open
reviewer: @reviewer
date: 2026-08-17

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: internal/orchestrate/orchestrate.go:116 — the `verdictRounds >= 2`
branch of `Next` now returns `Reason: "needs human: review cycle — retry
budget exhausted: 2 verdict commits already, still not approved"`, exactly
the wording tasks.md specified (leads with the brief's requested phrase while
keeping the 2-verdicts/retry-budget context). The branch comment was extended
(lines 112–115). Verified the diff: no other stop path changed — the
brief-not-written (line 90) and unknown-stage (line 130) reasons are
untouched, and the marker/runner-error/stall/maxIterations reasons live in
cmd/mg/jdi.go's Run and are unchanged. The reason reaches the human everywhere
tasks.md said it should: runJDI's final stdout line (jdi.go:161), run.log's
`mg jdi finished: stop-needs-human` line (logJobFinished via finish()),
and the ntfy needs-attention body (notifyStop uses result.Reason verbatim,
jdi.go:199). No code parses the reason programmatically (grep confirmed) —
string-only, human-facing change.

TASK-2: PASS
notes: internal/orchestrate/orchestrate_test.go — TestNext gained a
`wantReasonContains` field (asserted at lines 120–122) and all three
`verdictRounds >= 2` subtests (lines 76–97: "2 verdicts, tip is the verdict",
"2 verdicts, developer responded again", "3+ verdicts") assert the Reason
contains "review cycle"; the `strings` import was added and is used.
cmd/mg/jdi_test.go:325–327 — TestRunStopsAfterOneBounceExhausted now asserts
`got.Reason` contains "review cycle"; `strings` was already imported, and the
reason genuinely flows end to end (Next → Run's finish() → LoopResult.Reason,
jdi.go:449–453/490), so the pin is meaningful. Struct-field additions are safe
(all initializers keyed). No existing test asserted the old exact reason text
(grep for "retry budget exhausted" finds no test hits), so nothing else can
break.

TASK-3: PASS
notes: Verification claims are consistent with the static evidence. The old
reason string "retry budget exhausted" survives only in orchestrate.go's own
updated string/comment, internal/job/jdistatus.go:44's generic class comment
(which stays accurate — the new Reason keeps the "retry budget exhausted"
tail), this job's own tasks.md/implementation.md, and one archived job's
tasks.md (docs/jobs/archive/vu33rn_fully-autonomous-mode/tasks.md) — no living
(non-archive) code or docs reference the old string. The changes are
statically compile-safe: the new `strings` import in orchestrate_test.go is
used, and no unused imports result in either test file. Note: `go build
./...` / `go test ./...` could not be re-run in this review session (bash is
restricted to git-only commands), but the diff is a string change plus
additive test assertions with no new dependencies — low risk, and the
developer's explanation for needing the shim removed from PATH (test helpers
run `git init`) is plausible.

## Security

none — a string-only change to a stop reason plus additive test assertions;
no new inputs, no filesystem/network surface added. The ntfy body and run.log
now carry the new text, both already human-facing sinks with no parsing.

## Overall

APPROVED
The change does exactly what the brief asked: the deliberate
developer/reviewer back-and-forth stop in `mg jdi` now reports "needs human:
review cycle — …" instead of the generic "retry budget exhausted: …", so this
deliberate stop reads distinctly from a crash. Scope is tight (only
orchestrate.go, its test, and jdi_test.go touched beyond the job's own docs),
commit discipline is correct ([cold] TASK-1 / TASK-2 / implementation each in
their own commit, TASK-3 verification-only with no commit expected), and the
wording is pinned in tests at both the orchestration layer and the full Run
loop. Nothing to change before merge.