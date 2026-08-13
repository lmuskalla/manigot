# Tasks: aliases

id: qs7mfg
status: open
analyst: @analyst
date: 2026-08-13

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Verify the `mg host` (primary) / `mg wild` (alias) rename is complete — it was already shipped by the archived `in-the-wild` job (`hnhn1s`) and is on the base branch: `cmd/mg/main.go` dispatches `case "host", "wild":` to `runHost`, help text lists `mg host` with "(thematic alias: mg wild)", and README/docs/AGENTS.md are consistent. Confirm no "wild-first" wording remains anywhere; no functional change expected.
     files: cmd/mg/main.go, README.md, docs/AGENTS.md (read-only check)
     depends: none
     risk: low — verification only; the rename already landed

TASK-2: Add `-a`/`-j` short flag aliases to the session flag parser: in `internal/session/session.go` `ParseArgs`, register `fs.StringVar(&o.Agent, "a", ...)` and `fs.StringVar(&o.Job, "j", ...)` on the same target fields as the long forms (last-given wins, same as duplicated long flags), AND add `-a`/`-j` to `sessionValueFlags` so `splitFlags` extracts them with their values instead of leaking them into the container-CLI passthrough. Preserve the existing passthrough rule: only the exact single-dash short tokens are consumed; `--a`/`--j` (double dash) and `-a=value` remain passthrough, consistent with how the long flags already behave. This automatically covers `mg host` (shared parser) and the `mg jobs`/`mg agents` re-execs.
     files: internal/session/session.go
     depends: none
     risk: medium — touches the flag-extraction seam; a mistake silently routes `-a`/`-j` to the container CLI as passthrough instead of parsing them

TASK-3: Add unit tests for the short flags in `internal/session/session_test.go`: value consumption with passthrough intact, mixed long/short last-wins precedence, a lone `-a`/`-j` leaving the field unset (matching the long-form silent-ignore behavior), and unknown flags/bare words still landing in `Pass` verbatim.
     files: internal/session/session_test.go
     depends: TASK-2
     risk: low — pure test addition

TASK-4: Document `-a`/`-j` in the `mg` help text and its test: update the Usage/Commands sections of `printHelp()` in `cmd/mg/main.go` (e.g. `mg --agent/-a <name>` / `mg --job/-j <id>`, preserving the description column alignment), and extend the help-listing assertion in `cmd/mg/host_test.go` (`TestPrintHelpListsHost`) to check the short-form lines appear.
     files: cmd/mg/main.go, cmd/mg/host_test.go
     depends: TASK-2
     risk: low — text + assertion only

TASK-5: Update `README.md` for the short flags: short-form example lines in the session quick-reference block (comment column matching the block's alignment), a sentence in the "Three ways to seed a session's initial prompt" paragraph, and `(-a)`/`(-j)` in the Host mode "same session machinery" bullet.
     files: README.md
     depends: TASK-2
     risk: low — docs only

TASK-6: Update `docs/AGENTS.md` for the short flags: the session-launch bullet should list `--agent`/`-a`, `--job`/`-j`. Edit only `docs/AGENTS.md` — `/workspace/AGENTS.md` is a read-only overlay and must not be touched. Re-verify `agents/*.md` and `project-template/docs/AGENTS.md` need no sync (they reference none of these flags).
     files: docs/AGENTS.md
     depends: TASK-2
     risk: low — docs only

TASK-7: (Open question, needs confirmation) `mg jdi -j` — whether `mg jdi --job` should also accept `-j`. The brief's "--job should also work as -j" is unqualified and `mg jdi --job` is the only other user-facing `--job` flag, so the literal reading supports implementing it: add a `j` flag sharing the `jobArg` variable in `cmd/mg/jdi.go`'s flag set (last-given wins), update the jdi usage line and doc comment, the `mg jdi` help line in `cmd/mg/main.go`, the README quick-reference, and the `docs/AGENTS.md` command entry, and add tests pinning that `-j` is accepted (gets past flag parsing) versus an unknown flag (exit 2). If the author intended the short forms for the session launcher only, skip this — do not guess.
     files: cmd/mg/jdi.go, cmd/mg/jdi_test.go, cmd/mg/main.go, README.md, docs/AGENTS.md
     depends: none (independent of TASK-2 — separate flag set)
     risk: low — isolated to jdi's own flag set
