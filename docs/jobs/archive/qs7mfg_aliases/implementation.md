# Implementation: aliases

id: qs7mfg
status: open
developer:
date:

<!-- Produced by @developer after implementation. -->

## Summary

Introduced the requested aliases.

- **`mg host` / `mg wild`**: the rename was already shipped by the archived
  `in-the-wild` job (`hnhn1s`) — `mg host` is the primary name, `mg wild` its
  thematic alias (both dispatch to the same `runHost`). TASK-1 verified the
  current code and docs present `host` as primary with `wild` as alias
  everywhere; no change was needed.
- **Session short flags**: `--agent` now also works as `-a` and `--job` as
  `-j` on the session launcher (bare `mg`, `mg jobs`/`mg agents` re-execs,
  and `mg host`, which shares the same flag parser).
- **`mg jdi -j`**: since the brief's "--job should also work as -j" is
  unqualified, `mg jdi --job`'s own flag set accepts `-j` too (judgment call
  flagged below under Known issues).

## Changes

TASK-1: Verified the `mg host` (primary) / `mg wild` (alias) rename is fully
in place — `cmd/mg/main.go` dispatcher (`case "host", "wild":`), help text,
README, and `docs/AGENTS.md` all treat `host` as primary. No code change
required; no commit.

TASK-2: `internal/session/session.go` — registered `a` and `j` as aliases of
the existing `agent`/`job` flags in `ParseArgs`'s flag set (shared target
variables, so last-given wins, same as duplicated long flags), and added
`-a`/`-j` to `sessionValueFlags` so `splitFlags` extracts them with their
values instead of leaking them into the container-CLI passthrough. This
covers the bare `mg` session path and `mg host` automatically (shared
parser).

TASK-3: `internal/session/session_test.go` — added `TestParseArgsShortFlags`
(value consumption + passthrough intact), `TestParseArgsShortAndLongLastWins`
(mixed long/short precedence), `TestParseArgsShortFlagWithoutValue` (lone
`-a`/`-j` leaves the field unset, matching the long-form behavior), and
`TestParseArgsShortFlagsDoNotSwallowPassthrough` (unknown flags/bare words
still pass through verbatim).

TASK-4: `cmd/mg/main.go` — help Usage now shows `mg --agent/-a <name>` and
`mg --job/-j <id>`; `cmd/mg/host_test.go` — `TestPrintHelpListsHost` now
asserts the two short-form lines appear in `mg --help`.

TASK-5: `README.md` — added `-a`/`-j` example lines to the quick-reference
session block, a sentence in the "Three ways to seed a session's initial
prompt" paragraph, and `(-a)`/`(-j)` in the Host mode "same session
machinery" bullet.

TASK-6: `docs/AGENTS.md` — the session-launch bullet now lists
`--agent`/`-a`, `--job`/`-j` (only `docs/AGENTS.md` was edited; the mounted
`/workspace/AGENTS.md` overlay was left untouched). Re-verified `agents/*.md`
and `project-template/docs/AGENTS.md` reference none of these flags, so the
sync rule needs no changes there.

TASK-7: `cmd/mg/jdi.go` — `runJDI`'s flag set now accepts `-j` as a short
form of `--job` (shared variable, last-given wins), usage line and doc
comment updated. `cmd/mg/main.go` help jdi line, README quick-reference, and
`docs/AGENTS.md` command entry updated to `--job/-j`.
`cmd/mg/jdi_test.go` — added `TestRunJDIJobShortFlagAccepted` (an unknown
flag exits 2; `-j` gets past flag parsing and fails cleanly at project
resolution) and `TestRunJDIUnknownFlagRejected`.

Verified with `go build ./...`, `go test ./...` (all packages pass), and a
binary smoke test: `mg -j xyz --print`, `mg -a analyst`, and
`mg jdi -j xyz` each consume their short flag and proceed to the expected
resolution/auth error (no unknown-flag rejection).

## Known issues / follow-ups

- **`mg jdi -j` scope decision**: TASK-7 was flagged in the analysis as an
  open question — the brief pairs `--agent`/`--job` as session flags, and
  `--agent` has no jdi counterpart. Implemented as the literal reading of
  "--job should also work as -j" (it is the only other user-facing `--job`
  flag); revert the `cmd/mg/jdi.go` flag-set change if the author intended
  the short forms for the session launcher only.
- The short forms are single-dash only (`-a`, `-j`); `--a`/`--j` (double
  dash) and `-a=value`/`-j=value` forms remain passthrough to the container
  CLI, consistent with how the long flags already behave (`--agent=value` was
  never parsed either).
