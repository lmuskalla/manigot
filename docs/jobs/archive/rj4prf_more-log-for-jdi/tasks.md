# Tasks: more log for jdi

id: rj4prf
status: open
analyst: claude
date: 2026-08-11

<!-- Produced by @analyst from brief.md. -->

## Context

`mg jdi`'s current run.log/stdout output (`tui/cmd/jdi/main.go`'s `Run`,
`tui/cmd/jdi/output.go`) only ever writes one thing: a
`=== <timestamp> <agent> (attempt N) ===` block *after* an agent invocation
returns, containing its extracted final-response text
(`orchestrate.ResultText`). There is no line at all for: the run starting,
an agent being about to run (vs. having already finished — currently
indistinguishable, since the only header is written post-hoc), or the whole
job finishing. `logImmediateStop` is the one exception — it already logs a
reason when `Run` stops before any agent ever ran.

This job adds the four missing "keep me posted" events from the brief, all
timestamped and going through the same fan-out (`os.Stdout` +
`docs/jobs/.jdi-status/<job>/run.log`) `Run`/`main()` already use, plus a
dedup step so an agent's already-committed file content isn't printed twice.

All work is confined to `tui/cmd/jdi/` (`main.go`, `output.go`, their tests)
— no TUI changes are needed: `tui/internal/ui/detail.go`'s log tab just
displays `run.log`'s raw tail, with no header-specific parsing to update.

## Task breakdown

TASK-1: Log a "mg jdi started" event once, at the top of the run.
     files: tui/cmd/jdi/main.go (call site, right after `logDest` is
       finalized and before `Run` is invoked), tui/cmd/jdi/output.go (new
       `logStarted(w io.Writer, jobName, profile string)` helper, styled like
       `logImmediateStop`'s `=== <RFC3339 timestamp> ... ===` header),
       tui/cmd/jdi/output_test.go (new test)
     depends: none
     risk: low — additive log line, no control-flow change.

TASK-2: Log an "agent invoked" event immediately before each agent
     invocation, distinct from the existing post-run header.
     files: tui/cmd/jdi/main.go (`Run`, immediately before
       `out, runErr := runner.Run(...)`; reuse the same `i+1` attempt number
       `logInvocation` already uses so both headers agree), tui/cmd/jdi/output.go
       (new `logAgentInvoked(w io.Writer, agent string, attempt int)` helper),
       tests in tui/cmd/jdi/main_test.go / output_test.go
     depends: none (independent of TASK-1)
     risk: low — additive, single-threaded loop so ordering is guaranteed.

TASK-3: Reword the existing post-run header so it reads as a distinct
     "agent finished" event now that TASK-2 adds an "invoked" one (e.g.
     `=== <timestamp> <agent> finished (attempt N) ===`), so a reader sees an
     invoked/finished pair per agent call rather than one ambiguous header.
     Behavior otherwise unchanged for now — do not fold in TASK-5's dedup
     logic here.
     files: tui/cmd/jdi/output.go (`logInvocation`'s header line),
       tui/cmd/jdi/output_test.go (existing header-substring assertions,
       e.g. `TestLogInvocationPlainText`'s `"developer (attempt 1)"` check,
       need updating to the new wording)
     depends: TASK-2 (shares the invoked/finished pairing convention —
       land together or immediately after so the log reads consistently)
     risk: low — text-only change, but touches an already-tested function so
       existing assertions must be updated, not just added to.

TASK-4: Log a "job finished" event once, at loop exit — both the
     `StopFinished` and `StopNeedsHuman` outcomes.
     files: tui/cmd/jdi/main.go (`Run`'s `finish` closure), tui/cmd/jdi/output.go
       (new `logJobFinished(w io.Writer, kind orchestrate.Kind, reason string)`
       helper), tests in main_test.go
     depends: TASK-2, TASK-3 (shared timestamp/header style; also needs care
       against duplicating `logImmediateStop`'s reason line for the
       stop-before-any-agent-ran case — `logJobFinished` should still fire
       there too, but the two messages must not repeat the same reason text
       twice in a row)
     risk: low/medium — still just an additive log call in an already-tested
       closure, but needs the overlap with `logImmediateStop` resolved
       deliberately rather than left to print the same thing twice.

TASK-5: Skip re-printing an agent's output text when it duplicates what
     that agent already wrote to the job's own markdown file.
     Approach: after extracting `text := orchestrate.ResultText(raw)`, read
     the current content of the file the just-run agent is expected to have
     written (analyst → tasks.md, developer → implementation.md, reviewer →
     verdict.md; safe to read directly from `j.Dir` via `os.ReadFile` since
     `ensureOnBranch` guarantees `mg jdi` always operates on the job's own
     checked-out branch). If that file's trimmed content appears as a
     (trimmed, whitespace-normalized) *substring* of the agent's response
     text — not stricter exact-equality, since a real response typically
     pastes/echoes the file content plus its own surrounding commentary —
     treat it as a duplicate: still write the "finished" header, but replace
     the body with a short note (e.g. "(output matches tasks.md, omitted)")
     instead of the full text.
     files: tui/cmd/jdi/main.go (`Run` — needs to pass the just-run agent's
       target filename/content through to the logging call),
       tui/cmd/jdi/output.go (`logInvocation` signature/behavior change, new
       agent→filename map, new comparison helper), tests in main_test.go and
       output_test.go
     depends: TASK-3 (changes the same function this task also touches)
     risk: medium — a content-matching heuristic can under-suppress (agent's
       wording differs enough from the file that no match is found, so
       nothing is skipped) though not meaningfully over-suppress (containment
       is a strict condition); also changes `logInvocation`'s signature, so
       every call site and existing test needs updating, not just new ones.

TASK-6: Extend tests to cover the new events end-to-end, not just each
     helper in isolation.
     files: tui/cmd/jdi/main_test.go (extend/add a `TestRunReportsStatus`-
       style test asserting the full log sequence — started → invoked →
       finished [full or omitted body] → ... → job finished — for one
       happy-path run and one TASK-5 dedup case), tui/cmd/jdi/output_test.go
     depends: TASK-1, TASK-2, TASK-3, TASK-4, TASK-5
     risk: low — test-only.

## Open question for @developer

TASK-5's substring-containment rule is this analyst's best design call given
the brief's underspecified "if it's already the same ... skip it," not a
pinned requirement — if real agent output turns out not to match this
heuristic well in practice (e.g. because it's phrased as a diff/summary
rather than an echo of the file), treat the exact comparison strategy as
negotiable, but keep the overall behavior (full text by default, omitted
when clearly redundant) as the goal.
