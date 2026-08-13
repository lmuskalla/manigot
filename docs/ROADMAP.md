# Roadmap: what to build next

Prioritized recommendations from a product/user-perspective review of the
current state (2026-08-13). Where `backlog.md` records ideas deliberately
deferred during job scoping, this document is the opposite: what *should*
be picked up next, in order, with the evidence behind each call. Promote an
item to a real job (`mg job`) when it's actually going to be worked on.

## Current state, in one paragraph

The product is coherent and feature-dense: a single `mg` binary, three
subscription profiles (`claude-pro` / `zai` / `opencode-go`), real
filesystem isolation, a full job lifecycle on git worktrees, an autonomous
mode (`mg jdi`), a TUI, host mode, ntfy notifications, eleven agents, and a
strong test suite. The consolidation work of the last few days (bash → Go,
agent format, code quality) was the right move and has mostly landed.

The project is past the "add surface area" phase and into the "make the
core promise bulletproof" phase. The strongest signal in the repo — the
abandoned worktrees in `.manigot-worktrees/` — is itself a roadmap:

- `o3kk3n_jdi-is-broken`
- `a75hdc_opencode-jdi-issues`
- `6ro7eg_add-stage-to-overview`
- `sd62w9_add-jdi-in-overview`
- `7431d6_different-configurable-docker-images`

Combined with the archived history (`4i5tcx_jdi-does-not-work`,
`foycfl_jdi-for-opencode`, `gezlwy_attempts-in-jdi`,
`nrv5sa_multiple-jdi-instances`), one thing is unmistakable: **`mg jdi`
under the OpenCode profiles has been a recurring source of pain.** That's
the flagship autonomous feature, and it's the one that fails silently if
it's wrong.

## Recommended next work, in order

### 1. Prove `mg jdi` end-to-end on all three profiles, and fix what breaks

The current code (JSONL event parsing in `internal/orchestrate/signal.go`,
the retry-budget state machine in `internal/orchestrate/orchestrate.go`)
looks well-engineered — but the evidence says the OpenCode path has burned
us repeatedly, up to and including two scaffolded jobs literally named
"jdi is broken" and "opencode jdi issues" that were abandoned rather than
resolved.

The concrete risk from a user's perspective: a false "needs human" stop, a
stall, or a reviewer that isn't actually read-only under OpenCode — each of
which makes the autonomy promise untrustworthy. Success criteria: a real,
non-trivial job driven end-to-end under `zai` and `opencode-go`, with the
`run.log` and status sidecars checked for correctness — not just unit tests
of the parsers. If that passes cleanly, this is closed and the worry tax is
paid off for good.

### 2. Enforce read-only agents under OpenCode (the documented `tools:` caveat)

The README is honest about this: `@reviewer`, `@security`, `@analyst` and
`@owner` are read-only under Claude Code, but **not** under OpenCode,
because the `tools:` frontmatter key is stripped. This isn't cosmetic — it
is the integrity of the job workflow. In an `mg jdi` run under an OpenCode
profile, the reviewer can edit `implementation.md` or the code it's
supposed to be verifying, and nobody would know. The fix (expressing the
restriction as OpenCode `permission:` frontmatter) was already identified
as a follow-up; it should stop being a follow-up.

This is a correctness fix to the *exact* feature from item 1 — the two
halves of "can I trust an unattended run." Scope them together if possible.

### 3. Add the stage column to the TUI overview

`6ro7eg_add-stage-to-overview` was scaffolded and never built;
`sd62w9_add-jdi-in-overview` was effectively delivered by the status badges
(`[running @agent]`, `[finished]`, `[needs human]`). The jdi badge exists,
the detail view has the stage timeline — but the **list still shows
id / status / type / date / title, with no sense of where each job is in
the workflow.**

For the person juggling several parallel jobs (which the worktree model
explicitly enables), "which of my five jobs is stuck in review" at a glance
is the single most useful piece of overview information, and `job.Stage()`
already computes it — this is a rendering change, not a data one. Small,
high-value, and it honors the intent of a job already scoped.

### 4. Close the lifecycle hole: orphaned-worktree detection and cleanup

The five dead directories in `.manigot-worktrees/` (~3.5 MB each, `.git`
files pointing at gitdirs that no longer exist) are the proof. A job
scaffolded and then abandoned leaves no branch, no worktree registration,
no entry in `mg jobs` — and no way to clean it up through the tool. The
user is left `rm -rf`-ing directories by hand and wondering whether they're
safe to delete.

`mg jobs` or `mg delete` should surface orphans (a registered worktree
whose metadata is gone, or vice versa) and offer to remove them — mirroring
`git worktree prune` semantics, but with the confirmation discipline
`mg delete` already has. This is the tool not cleaning up after itself, and
it quietly erodes trust in the lifecycle.

### 5. Add the `@owner` gate and `@security` pass to `mg jdi` — after 1–4 land

The docs describe the ideal flow as `@owner` → `@analyst` → `@developer` →
`@reviewer` → `@security`, and `mg jdi` drives only the middle three. That
is the biggest gap between the documented workflow and the autonomous one —
the exact thing a user means by "don't babysit the whole thing."

But it is correctly in the backlog with a known wrinkle (`@owner` never
writes to disk; it needs its own verdict signal convention — the drafted
"PO-VERDICT:" marker from the `vu33rn` scoping is a reasonable starting
point). Pull it in only after the core loop is proven reliable on all
profiles; extending a loop that is still being stabilized just multiplies
the failure surface.

### 6. Housekeeping that unblocks everything else

- **CODE_QUALITY_TASKS Phase 1** (branch-matching logic in three copies,
  git exec in three places, fs helpers in four). The stage-in-overview work
  and any future `--job` resolution change will touch exactly this
  duplicated logic. Consolidate first so new features land on one
  definition instead of three.
- **Timeouts on the jdi loop's git probes** (Phase 2.3). A stalled probe
  hangs a whole autonomous run silently — a user-facing reliability bug
  dressed as an internal one.
- **The error-prefix inconsistency** (Phase 2.1) — "Error: ..." on some
  messages, bare on others, in the same commands. Visible to the user every
  time something goes wrong.
- **Decide the fate of `feature/3ro17g_go-instructions`** — ten chapters of
  teaching material sitting unmerged on main. Either it's a deliberate
  long-running branch (then say so) or it should land or be cut. Invisible
  open branches are how the orphaned-worktree problem starts.

## What not to build next

- **In-TUI embedded terminal** (backlog): big Bubble Tea/PTY commitment for
  window-management convenience. The tmux split-pane already covers the
  "less window chaos" need for tmux users.
- **Event-streaming subsystem** (backlog): the polling model delivers real
  value; this is a wire-format commitment that should wait until polling is
  provably insufficient.
- **Headless/cron execution** (backlog): genuinely valuable for the VPS use
  case, but the entire launch model assumes an attended terminal. It needs
  its own design pass — not a bolt-on.
- **Configurable Docker images** (`7431d6`): more moving parts in the
  isolation story for a need that isn't evidenced. The single-image model is
  a feature, not a limitation.

## Bottom line

The next jobs, in order: **(1) verify and harden `mg jdi` on the OpenCode
profiles**, **(2) enforce read-only agents under OpenCode**, **(3) stage
column in the overview**, **(4) orphaned-worktree cleanup**, then **(5)
extend the autonomous sequence with `@owner`/`@security`**. Items 1 and 2
are the same job if scoped together. Everything else in the backlog stays
where it is until the core loop is boring.

One concrete thing to do regardless of job ordering: delete the five
orphaned worktree directories (or land item 4 first and let the tool do
it). They are the clearest signal in the repo that something in the
lifecycle isn't finished.
