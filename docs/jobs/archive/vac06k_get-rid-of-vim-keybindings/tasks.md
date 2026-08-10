# Tasks: Get rid of vim keybindings

  id: vac06k
  status: open
  analyst: claude
  date: 2026-08-10

  <!-- Produced by @analyst from brief.md. -->

  ## Task breakdown

  TASK-1: In the detail view's key handling, drop the `h`/`l` aliases for tab
  switching and the `j`/`k` aliases for scrolling, leaving `tab`/`shift+tab`,
  `left`/`right`, and `up`/`down` (plus the already-non-vim `pgup`/`pgdown`/
  `g`/`G`/`1`-`5`) as the only ways to do those two things.
       files: tui/internal/ui/detail.go (detailView.update's `"tab", "l", "right"`
         / `"shift+tab", "h", "left"` cases; fileTab.scroll's `"down", "j"` /
         `"up", "k"` cases)
       depends: none
       risk: low — purely removes dead-simple key aliases from two switch
         statements; arrow-key/tab equivalents already exist and are untouched,
         so no navigation capability is lost, only the vim spelling of it.

  TASK-2: Rebind the detail view's "run mg-jdi" action from `J` to `j` (now
  free after TASK-1 removes it as a scroll-down alias): the key-dispatch case
  in `updateDetail`, and the two places that render or describe that key — the
  `[J]`/`mg-jdi` action-bar button and the footer hint's `J run mg-jdi` text.
       files: tui/internal/ui/app.go (updateDetail's `case "J":`),
         tui/internal/ui/detail.go (renderActionBar's stageLine, renderFooter's
         hint string)
       depends: TASK-1 (must land together or after, so `j` is not
         simultaneously bound to both scroll-down and launch-mg-jdi — the two
         switch statements are mutually exclusive at runtime either way, but
         sequencing keeps the diff easy to review as one coherent rebind)
       risk: low — a straight rename of a single-key binding; the target key
         (`j`) is confirmed free (agentMeta's five agent keys are a/p/d/r/s,
         nothing else claims it) both before and after TASK-1.

  TASK-3: Delete the footer's `j/k scroll` hint text entirely (per the brief:
  "completely get rid of the hint for j/k"), and update the doc comment above
  `agentMeta` (agents.go) that lists the detail view's full key set so it no
  longer mentions `h`/`l` file nav or `j`/`k` scroll, and refers to the mg-jdi
  launch key as lowercase `j`.
       files: tui/internal/ui/detail.go (renderFooter's `hint` string, currently
         `"tab/1-5 files · j/k scroll"`), tui/internal/ui/agents.go (comment
         above agentMeta)
       depends: TASK-1, TASK-2
       risk: low — comment/string-literal only, no behavior change.

  TASK-4: Update the two test files that exercise the mg-jdi launch key by
  literal `"J"` to use `"j"` instead — `keyMsg("J")` call sites and the
  surrounding doc comments that describe "the J flow" / "TestBranchGuardBlocksJdi
  verifies 'J' refuses...".
       files: tui/internal/ui/jdilaunch_test.go
         (TestJdiKeyLaunchesDetachedAndSeedsBellDedup,
         TestJdiKeyReportsResolutionFailure), tui/internal/ui/branchguard_test.go
         (TestBranchGuardBlocksJdi)
       depends: TASK-2
       risk: low — mechanical test-literal update; no new behavior under test.

  TASK-5: Add regression coverage in the detail-view tests confirming `j`, `k`,
  `h`, and `l` are now inert in the file body/tab context (no scroll, no tab
  switch) — e.g. asserting scroll position and `d.cur` are unchanged after each
  of those four keys, distinct from confirming `up`/`down`/`left`/`right`/
  `tab`/`shift+tab` still work.
       files: tui/internal/ui/detail_test.go
       depends: TASK-1
       risk: low — additive test-only change.

  TASK-6: Update README.md's documented keybindings and prose to match: the
  detail view table's scroll row loses `j`/`k` (leaving `pgup`/`pgdn`, `g`/`G`),
  its `J` row (run mg-jdi) becomes `j`, the `b` row's "needed before
  e/D/J/x/agent keys" list becomes "...D/j/x...", the `make jdi` build-step
  comment ("needed for the TUI's \"J\" key") becomes `"j"`, and both mentions of
  `J` in the "mg-jdi status & log" section ("Press `J` in the detail view",
  "A `J`-launched run") become `j`.
       files: README.md
       depends: TASK-2
       risk: low — documentation only, but several separate occurrences to
         catch; missing one would leave the docs self-inconsistent rather than
         break anything functionally.

  TASK-7: Update this repo's own `docs/AGENTS.md` architecture bullet for
  `tui/cmd/jdi`, which parenthetically says "a TUI-launched run (`J` in the
  detail view) has no terminal of its own at all" — change `J` to `j` there too,
  per the hard rule that `docs/AGENTS.md` and `agents/*.md` /
  `project-template/docs/AGENTS.md` stay in sync (the latter two don't mention
  this detail, so no change needed there — confirmed by search).
       files: docs/AGENTS.md
       depends: TASK-2
       risk: low — single-word documentation fix.

  ## Out of scope (confirmed, not tasks)

  - The job **list** view's own `k`/`j` (move selection) and `l` (open detail,
    alongside `enter`/`right`) bindings: the brief explicitly scopes this job to
    "the job detail view" only. List view is untouched.
  - `g`/`G` (top/bottom) and `pgup`/`pgdown`/space in the detail view: not vim
    *letter* mnemonics in the same sense the brief calls out (`g`/`G` doubles as
    `home`/`end`, already has non-letter equivalents), and the brief's own
    examples are specifically "j/k (and hl)" — leaving these alone unless told
    otherwise.
  - Archived job docs under `docs/jobs/archive/**` that mention the old `J` key
    (e.g. `vu33rn_fully-autonomous-mode`): historical records of past jobs, not
    live documentation — not edited.
