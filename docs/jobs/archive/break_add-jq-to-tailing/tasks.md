# Tasks: add jq to tailing

id: break
status: open
analyst: deepseek-v4-flash (read-only analyst)
date: 2026-08-28

<!-- Produced by @analyst from brief.md. -->

## Analysis summary

The "l" key in the TUI detail view spawns a host-side pane running a plain
`tail -f '<jobdir>/session.log'` — `launch.Tail` → `tailShellCommand` →
`launchDetached` (src/internal/launch/launch.go), wired from the "l" handler
in src/internal/ui/app.go, gated on session.log existing (detail.go
`sessionLogExists`). The pane is a tmux split / new terminal **on the host**,
so any formatting tool must exist on the host, not in the container.

session.log is NOT pure JSON: mg-jdi writes `=== <RFC3339> <agent> (attempt
N) ===` section headers and blank lines around each invocation's raw stream
(src/cmd/mg/jdioutput.go `openSessionLog`), and the raw stream itself is
JSONL for BOTH CLIs (opencode `--format json`; Claude `--output-format
stream-json`). A naive `tail -f x | jq .` would error on the header lines.

Decision (the brief invited an opinion): pipe through jq with the
mixed-content filter `jq -R -r 'fromjson? // .'` — each line is parsed as
JSON when possible (pretty-printed, jq's default), and non-JSON lines
(headers, blanks) fall through raw. jq is soft-gated exactly like tig
(`launch.TigLookPath`/`TigAvailable`): when jq is missing on the host the
tail falls back to the plain `tail -f` instead of erroring — strictly better
than the status quo. Rejected alternatives: hard-requiring jq ("readily
available" ≠ guaranteed, and an error where today there is working behavior
is a regression), a Go-side formatter subcommand (over-engineering for what a
one-line pipe does), python3 (not guaranteed on the host).

Out of scope: the serve API's session.log tail (internal/serve/api.go
`readFileTail` — plain text over HTTP; formatting is the client's concern),
`mg jdi`'s direct-CLI live terminal stream (different surface, not the "l"
key), and the TUI log tab (run.log is prose, not JSON). No Dockerfile change
— the pane is host-side.

## Task breakdown

TASK-1: Add a jq availability seam to the launch package
     files: src/internal/launch/launch.go, src/internal/launch/launch_test.go
     depends: none
     risk: low — mirrors the existing TigLookPath/TigAvailable seam
     (an exported `JqLookPath = exec.LookPath` var + `JqAvailable()`),
     purely additive; no behavior change.

TASK-2: Pipe the tail through jq when available
     files: src/internal/launch/launch.go, src/internal/launch/launch_test.go
     depends: TASK-1
     risk: medium — the pane command becomes
     `tail -f '<path>' | jq -R -r 'fromjson? // .'`: the filter must pass
     the `=== ... ===` headers and blank lines through raw while
     pretty-printing JSONL lines, the command string is re-parsed by
     bash -lc / osascript so quoting must hold, and the deliberate
     no-holdOnFailure behavior (Ctrl+C exit 130 closes the pane cleanly)
     must be preserved for both forms. Prefer keeping `tailShellCommand`
     unchanged and adding a parallel `jqTailShellCommand` builder (the
     file's separate-function-per-path convention); `Tail` picks on
     `JqAvailable()`, silently falling back to plain `tail -f` when jq is
     missing. No TUI code change needed — the launch path owns the decision.

TASK-3: Make the TUI tail tests environment-independent
     files: src/internal/ui/tail_test.go
     depends: TASK-2
     risk: low — stub `launch.JqLookPath` (mirror stubTigLookPath in
     tig_test.go) so TestTailKeyLaunchesTailInTmuxPane asserts the exact
     jq-piped command when jq resolves and the plain command when it is
     missing, instead of depending on whether the test machine has jq.

TASK-4: Update the docs describing the "l" tail
     files: docs/AGENTS.md (canonical, mounted at /workspace/AGENTS.md),
     project-template/docs/AGENTS.md (mentions the `l` key, lines ~60-66);
     agents/*.md do NOT mention tailing (verified — no change needed there)
     depends: TASK-2
     risk: low — doc-only; "a plain `tail -f`" becomes "a `tail -f` piped
     through jq when available on the host, falling back to plain tail -f".

TASK-5: Verify
     files: none (verification only)
     depends: TASK-2, TASK-3
     risk: low — `go build ./...` and `go test ./internal/launch
     ./internal/ui` (plus the rest of the module as feasible), and a manual
     smoke of the pipeline against a sample session.log with headers +
     JSONL: printf '=== 2026-08-28T00:00:00Z analyst (attempt 1)
     ===\n{"type":"message","content":"hi"}\n\n' | jq -R -r 'fromjson? // .'
     — headers/blank lines must pass through raw, JSON lines pretty-printed.