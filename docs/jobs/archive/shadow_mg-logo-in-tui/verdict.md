# Verdict: mg logo in tui

id: shadow
status: open
reviewer: @reviewer
date: 2026-08-27

## Review

Reviewed the full `main...HEAD` diff (21 files) against tasks.md, plus the
surrounding code (home.Root, pickQuote, entrypoint.sh, Dockerfile build
context, TUI height-budget accounting, App's WindowSizeMsg padding).

TASK-1: PASS
notes: `assets/manigot.txt` is now a 4-line × 36-column printable-ASCII mark
evolving the pre-existing three-window motif with the original `*#@*` glyphs
(decisions D1/D2/D3 recorded in tasks.md). Width verified line by line: all
four lines are exactly 36 columns (line 1 `.`+34×`-`+`.`; lines 2/3
`|`+3sp+6col-window+3sp+6col-window+3sp+6col-window+7sp+`|`; line 4
`'`+34×`-`+`'`) — the `|*#@*|` inner windows are 6 wide, so their `|` borders
land exactly on the `.----.` frame corners on the adjacent row. Plain text,
no markdown, no ANSI, no trailing whitespace. Matches the README fenced block
byte for byte.

TASK-2: PASS
notes: `internal/brand/logo.go` mirrors `pickQuote` exactly (home.Root() →
"" guard → os.ReadFile, "" on any error). No import cycle (brand → home;
session and ui both import brand). Tests cover found-file → exact content and
missing-file → "" (logo_test.go, using t.Setenv("MANIGOT_HOME", ...), which
home.Root() reads first).

TASK-3: PASS
notes: shared `printLogo(diag)` helper (internal/session/docker.go) called
from both `BuildDockerInvocation` and `BuildHostInvocation` — the two banners
are byte-identical by construction (the logo block + box both come from the
same code path; the pre-existing safehouse/host-mode content differences are
unchanged). Logo prints above the box, plain uncolored ASCII, to the diag
writer (always stderr — the `--print` stdout contract is untouched).
TrimSuffix+split handles the trailing newline; a missing asset prints nothing.
Tests: `checkout` stub asset (session_test.go) + logo-presence assertions in
both docker_test.go and host_test.go; the pinned `║           manigot`
assertion is unchanged and still passes (logo is additive above the box).

TASK-4: PASS
notes: logo rendered above the title in `listView.render`, styled with
`logoStyle` (accent #7D56F4, non-bold — reasonable: bold would thicken every
glyph). Height budget verified: `logoShown` gates on
`height >= 7 + len(logo) + jobCount + 1` and on `width >= logoWidth` (a wider
logo is omitted — horizontal overflow treated like vertical), and
`recentActivityShown` subtracts exactly `len(v.logo)` from spare when shown.
Accounting cross-checked against App's WindowSizeMsg handler
(a.height = terminalHeight − 2 for uiPaddingY): with or without the logo the
render total is terminalHeight − 1, so the logo folds in exactly and never
pushes job rows down or overflows — the graceful-degrade path is correct, not
just present. Missing asset → logo nil → header renders exactly as before
(the no-error convention matches printLogo). Tests cover shown-above-title,
omitted-on-full-list (all 20 rows still render), strip shrinks by exactly the
logo height, omitted on a narrow terminal, plus the existing render tests are
additive-safe (logo is nil in them, MANIGOT_HOME unset).

TASK-5: PASS
notes: README.md gains the ASCII logo in a fenced block after the PNG plus the
PNG-regen follow-up note and an `assets/` entry in the repo tree; docs/AGENTS.md
mentions the logo in the `internal/home` and Dockerfile bullets (the natural
spots the task's sync check asked for); docs/NAMING.md's rap sheet lists
`assets/manigot.txt`. Sync check verified: neither `agents/*.md` nor
`project-template/docs/AGENTS.md` asserts anything about the logo (the
project-template file is a user-project context template, not a mirror of the
repo's docs/AGENTS.md), so no further sync changes are needed.

TASK-6: PASS
notes: Dockerfile `COPY --chown=claude:claude assets/manigot.txt
/home/claude/assets/manigot.txt` — build context is the repo root (no
.dockerignore; existing COPYs already reference repo paths), and the final
`chmod -R o+rwX /home/claude` covers the file for any session UID. entrypoint.sh
prints it above the flavor quote, guarded on `-f "$HOME/assets/manigot.txt"`
(stale image → skip, no error under `set -euo pipefail`) and on
`MANIGOT_PRINT != "true"` (—print stdout stays clean, mirroring the quote's
guard). `$HOME` resolution is consistent with the existing `~/.claude.json`
usage in the same file. Syntax is valid bash; verified by inspection (the
Docker build itself can't run in this session — noted in implementation.md).

TASK-7: PASS
notes: verification claim is consistent with inspection — the changed Go files
compile-clean by review (imports, signatures, callers all consistent:
`recentActivityShown`'s new width param updated at both call sites and in the
tests; no import cycles; `logoStyle` references the in-scope `accent`). The
review session cannot run `make mg`/`gofmt`/`go test` (tooling restricted to
git read+commit), so this task rests on the developer's reported run plus code
inspection; no issue found that would break the build or tests. The two
pre-existing gofmt violations the developer names (internal/git/commitall_test.go,
internal/ui/tig_test.go) are indeed outside this job's diff.

Non-blocking observations (not blockers, no action required):
- `internal/session/root_test.go` has a 1-line gofmt fix of a pre-existing
  violation — outside the task file lists, but benign and consistent with the
  session-package test work this job did.
- The 36-wide logo sits over the 38-wide banner box left-aligned (2-column
  right gap). This is the exact design the tasks chose (D3 ≤36, logo above the
  box, least churn) — cosmetic, per spec.
- The TUI logo is loaded once at `newListView`; a logo appearing later (e.g.
  relocated checkout) isn't picked up mid-process — consistent with the
  documented "loaded once" behavior and the no-error convention.

## Security

No security findings. The only new container-surface change (TASK-6) is a
static asset `cat` to stdout in the interactive branch, guarded on
`MANIGOT_PRINT`; it cannot influence the agent CLI's stdin, argv, or --print
stdout. The banner/TUI logo paths are host-side read-only file reads with a
missing-file → "" convention (no error surface, no injection vector).

## Overall

APPROVED

All seven tasks are implemented as specified and the diff contains nothing
outside the task scope beyond two benign one-line test-file touch-ups
(session_test.go stub asset — a supporting change for TASK-3's assertions —
and the root_test.go gofmt fix). The width/height constraints of the design
decisions (36×4, ASCII-only) hold exactly, both banner builders share one
byte-identical logo path, the TUI height budget preserves the pre-existing
fit with graceful degradation, and the docs/container surfaces are consistent
with the canonical asset. Nothing must change before merge.