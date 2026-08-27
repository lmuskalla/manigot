# Tasks: mg logo in tui

id: shadow
status: open
analyst: @analyst
date: 2026-08-27

Produced by @analyst from brief.md.

## Summary

Design an ASCII logo for manigot and show it in the two user-facing surfaces
the `mg` binary owns: the session info banner (the terminal — printed on
every `mg` / `mg host` launch) and the TUI job list header (`mg tui`). The
logo's canonical source is `assets/manigot.txt`, which already holds a
hand-drawn ASCII logo (a rounded box containing three windows) that is
currently referenced by **no code at all** — this job designs the final logo
and wires it into both render sites, keeping the design in one place so the
two surfaces can never drift.

## Design direction (proposed by @analyst — review before implementing)

The brief is open ("think about our whole project and design a logo"), so the
following constraints pin the design down; they come from the existing brand,
not new invention (per `agents/designer.md`: "do not invent a brand identity —
work with what exists"):

1. **Motif**: evolve the existing `assets/manigot.txt` motif — a box of three
   windows — rather than inventing a new identity. It already exists, it
   reads as "three panes / three orchestrated agent CLIs", and it matches the
   product (one orchestrator launching isolated sessions). The Sopranos
   flavor (`docs/NAMING.md`, `assets/quotes.json`) is a tone layer, not a
   graphic; a fedora-take is an option only if it stays as recognizable as
   the three-window mark at 36 columns.
2. **Charset**: printable ASCII only (0x20–0x7E). The session banner already
   uses box-drawing unicode (`╔═╗`), but the logo itself must render
   identically in a plain captured terminal, a tmux pane, and the colorized
   TUI — ASCII-only guarantees that. The existing `assets/manigot.txt` is
   already ASCII-only; keep it that way.
3. **Size**: the logo must fit the session banner's inner width (the box is
   currently 38 columns wide with 36 usable inside the `║`s) and be at most
   4 lines tall. The existing draft is 37 columns wide — one too many for the
   current box — so either the final logo is ≤36 wide (box unchanged) or the
   banner box widens to fit it (TASK-3 decides; TASK-1 must state the chosen
   width).
4. **Where it renders**: (a) the host-side session info banner — both
   `BuildDockerInvocation` (`internal/session/docker.go`) and
   `BuildHostInvocation` (`internal/session/host.go`) print the same banner
   shape and must stay byte-consistent; (b) the TUI job list header
   (`internal/ui/list.go`), colored with the existing accent `#7D56F4`.
   Deliberately NOT in `mg --help` — technical output stays technical, per
   `docs/NAMING.md` ground rule 4.
5. **The README PNG**: `assets/manigot.png` (README header) is out of the
   ASCII-logo scope; regenerating it to match the new ASCII is a follow-up
   unless the developer finds a trivial path (there is no in-repo tool to
   render ASCII → PNG; `shot` renders URLs). TASK-5 documents the ASCII logo
   as text instead and flags the PNG.

## Task breakdown

TASK-1: Finalize the ASCII logo design and write it to `assets/manigot.txt` — the single canonical source. Decide and record: the motif (evolving the three-window mark, decision D1), the exact glyph set (printable ASCII only, D2), and the final width/height (≤36 cols to fit the current banner box, ≤4 lines tall, D3). The file must keep its current plain-text format (no markdown, no ANSI) so a renderer can read and print it verbatim; replace the `*#@*` window contents or keep them per the decided motif — the choice is recorded here in tasks.md so the reviewer can check it against the rendered output.
     files: assets/manigot.txt
     depends: none
     risk: low — an asset file only, no code; the only risk is a design that doesn't fit the two render surfaces, which is why the width/height constraints are pinned here first

TASK-2: Add a tiny shared logo loader, `internal/brand` (new package) — `Logo() string` reading `assets/manigot.txt` via `home.Root()`, returning "" on a missing/unreadable file, mirroring `pickQuote`'s no-error convention (`internal/session/docker.go`). One source of truth for both render sites; without it the logo string would be duplicated in Go and drift. Include a small test (found file → exact content; missing file → "").
     files: internal/brand/logo.go (new), internal/brand/logo_test.go (new)
     depends: TASK-1
     risk: low — a new additive package following the established `pickQuote`/`home.Root()` pattern; no existing code touched

TASK-3: Render the logo in the session info banner (the terminal surface): print `brand.Logo()` in `BuildDockerInvocation` (`internal/session/docker.go`, around the current `║           manigot` line) and identically in `BuildHostInvocation` (`internal/session/host.go`). Either widen the 38-col box to fit the logo (per TASK-1's decided width) or print the logo above the box — pick the option with the least churn and keep both banners byte-identical. Plain uncolored ASCII; the banner already goes to the diag writer (stderr), so the `--print` stdout contract is untouched. Update the pinned assertions: `internal/session/docker_test.go`'s `strings.Contains(diag.String(), "║           manigot")` and `internal/session/host_test.go`'s banner-presence check; add a logo-presence assertion to both.
     files: internal/session/docker.go, internal/session/host.go, internal/session/docker_test.go, internal/session/host_test.go
     depends: TASK-2
     risk: medium — the fixed-width boxed banner is test-pinned and appears in two builders; widening it touches every box line plus two test files, but the change is contained to the diag writer and never touches argv/mounts

TASK-4: Render the logo in the TUI job list header: in `listView.render` (`internal/ui/list.go`), print the logo above the `manigot - <project> - on <branch>` title, colored with a new `logoStyle` (accent `#7D56F4`) in `internal/ui/styles.go`. Height budget is the crux: the list's vertical layout is exact (`dashboardFixedChrome` = 7 rows, `recentActivityShown` scales the activity strip into spare room) — the logo's line count must be folded into the fixed chrome and the logo must degrade gracefully (omitted entirely) when there isn't room, so it can never push job rows down or overflow the terminal, on the same philosophy as `recentActivityShown`. Update `internal/ui/list_test.go` and any render tests that pin the list surface (the existing ones assert substrings, so a prepended logo should be additive — verify).
     files: internal/ui/list.go, internal/ui/styles.go, internal/ui/list_test.go
     depends: TASK-2
     risk: medium-high — the list layout budget is deliberate and test-pinned; adding vertical chrome on short terminals is exactly the failure mode the existing code guards against, so the graceful-degradation path is the part to get right

TASK-5: Update docs: README.md's header currently shows only `assets/manigot.png` — add the ASCII logo in a fenced code block right there (so the repo's logo is visible as text) and note the PNG regeneration as a follow-up; `docs/AGENTS.md` gets a one-line mention of the logo asset only if a sync check finds a natural spot (it documents `assets/` as the source of quotes.json etc. — verify); `docs/NAMING.md`'s "full rap sheet" can list the logo as a brand artifact (one line, optional). Keep it minimal — no new documentation files.
     files: README.md, docs/AGENTS.md (only if the sync check finds a spot), docs/NAMING.md (optional one-liner)
     depends: TASK-1 (accurate wording once the design exists; TASK-3/TASK-4 only if describing the rendered placement)
     risk: low — documentation only; the sync rule (agents/*.md, project-template/docs/*) needs a check that no doc claims a logo-less state, but none should

TASK-6 (optional — include only if it stays low-risk): print the logo inside the container session too — `scripts/entrypoint.sh` prints the random quote right before exec'ing the agent CLI; a logo line above it would be the in-agent-terminal variant of "show in the terminal". This needs the Dockerfile to `COPY assets/manigot.txt` into the image and entrypoint.sh (the single, self-contained, shellcheck-pinned bash file) to render it. If it destabilizes entrypoint.sh or the Dockerfile in any way, skip it and record as a follow-up in implementation.md — the host-side banner (TASK-3) already satisfies the brief's "terminal" surface.
     files: scripts/entrypoint.sh, Dockerfile
     depends: TASK-1 (asset content), TASK-3 (decided placement precedent)
     risk: medium — entrypoint.sh is the one bash file and the image build is `GOTOOLCHAIN=local`-sensitive; a new baked asset must not break the build or the shim generation

TASK-7: Verify: `make mg` builds, `gofmt` is clean, and `go test ./...` passes with the new package and the updated banner/TUI tests.
     files: none (verification only)
     depends: TASK-2, TASK-3, TASK-4 (TASK-5, TASK-6 if taken)
     risk: low — verification only, but it is the gate that catches a TASK-4 height-budget regression or a TASK-3 banner-assertion miss

---

## TASK-1 design decisions (recorded by @developer for the reviewer)

- **D1 — motif**: evolve the existing three-window box. A single rounded box
  (`...---...`) containing three windows, each carrying the original `*#@*`
  terminal-content glyph — the same "three orchestrated agent CLIs" reading
  as the previous draft, not a new identity. No fedora.
- **D2 — charset**: printable ASCII only (0x20–0x7E). The file is plain text,
  no markdown, no ANSI, so a renderer can print it verbatim.
- **D3 — size**: exactly **36 columns × 4 lines**, fitting the session
  banner's 36-column inner width unchanged. This lets TASK-3 print the logo
  *above* the existing box (least churn) rather than widening it.

The window content `*#@*` is kept from the original draft (not replaced) so
the rendered output stays true to the pre-existing motif.