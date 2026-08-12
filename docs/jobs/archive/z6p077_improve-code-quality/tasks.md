# Tasks: improve code quality

id: z6p077
status: open
analyst: deepseek-v4-flash
date: 2026-08-12

Produced by @analyst from brief.md + the three CODE_QUALITY docs. Scope is the
CODE_QUALITY_TASKS.md breakdown, verbatim in structure, ordered by its
recommended phase sequencing. Every task is refactor-only: no behavior change;
the existing test suite is the safety net. The bash → Go port is done and
untouched — "restore" below refers to making the *Go* code conform to the
architecture AGENTS.md already documents for the Go world (one shell-out point,
one definition per helper), not to reintroducing any scripts.

Scope: the full breakdown is in scope — all four phases, TASK-1 through
TASK-17, per the owner's decision ("let's do everything"). Execute in phase
order; TASK-13 is explicitly last. Each phase is independently shippable, so
the commit history can land phase by phase.

## Task breakdown

### Phase 1 — Eliminate duplication; bring the Go code in line with the Go architecture documented in AGENTS.md

TASK-1: Consolidate the three identical branch-matching helper copies (exactBranchMatch / prefixBranchMatches / branchTail) into internal/git and replace all call sites, folding resolveJob's name/ID matching onto the same primitives.
     files: internal/git/ (new helpers + merged tests), internal/session/root.go, internal/job/finish.go, internal/job/delete.go, cmd/mg/jdi.go, tests for all four
     depends: none
     risk: medium — resolution paths for --job, mg done, mg delete, and mg jdi all route through this; ambiguity-error wording is pinned by tests and is user-facing contract.

TASK-2: Route every git exec through internal/git — replace session.gitToplevel, session.gitRaw, session.configValue and init.go's gitToplevel with the exported git.RevParseToplevel / ConfigUserName / ConfigEmail, and delete the private copies.
     files: internal/session/root.go, internal/session/docker.go, cmd/mg/init.go, AGENTS.md (only if deliberate exceptions remain)
     depends: none (independent of TASK-1)
     risk: low — pure replacement of identical behavior; docker argv tests and init tests pin observable behavior, not the helper.

TASK-3: Consolidate the four isDir/isFile/isRegularFile predicates and unify prefixJobDir (session) with prefixJobDirName (job/delete) into one docs/jobs scan (returning the matched name; callers join the root).
     files: internal/session/root.go, internal/job/delete.go, internal/agentlist/agentlist.go, cmd/mg/agents.go
     depends: none
     risk: low — one-liner predicates, but the docs/jobs scan must end up as exactly one definition.

TASK-4: Unify the two verdict-section extractions in internal/job (verdictOverallMatch's -A5 window vs verdictOverallSection) onto one primitive, pinning the "status beyond line 5" corner with a test and documenting which behavior wins.
     files: internal/job/stage.go, internal/job/finish.go, internal/job tests
     depends: none
     risk: low-medium — the behavioral edge (line-5 cutoff) must be decided and pinned by a test before the consolidation, not discovered after.

TASK-5: Deduplicate the identical ~10-line "job '%s' not found among local branches" error builder in finish.go and delete.go into one helper.
     files: internal/job/finish.go, internal/job/delete.go
     depends: TASK-1 (lands mostly for free once the resolve helper owns the message; do standalone if TASK-1 is deferred)
     risk: low — wording pinned by tests.

### Phase 2 — Design hardening

TASK-6: Decouple presentation from domain errors — audit every error returned by internal/job and internal/session by style, remove the "Error: " prefix from domain layers (CLI owns framing in one place), standardize bare errors deliberately as test-pinned output changes, and prefer %w wrapping over %s concatenation.
     files: internal/job/create.go, internal/job/finish.go, internal/job/delete.go, internal/session/session.go, internal/session/root.go, cmd/mg/* (error printing), internal/ui/app.go (cmdErrorText callers)
     depends: none
     risk: medium — the "wording is the contract" rule means every touched message is test-pinned; this is the one Phase-2 item that will churn tests.

TASK-7: Split the 1366-line App god-file — extract the list view (cursor/rendering/key handling, renderList, renderJobRow, renderRecentActivity, column-width helpers, empty-state copy) into its own listView model; App keeps routing, refresh, and cross-view state; do NOT move the jdi bell-dedup/spinner machinery.
     files: internal/ui/app.go, internal/ui/list.go (new), possibly internal/ui/list_test.go additions
     depends: none
     risk: low-medium — pure refactor with strong rendering tests (list_test.go must pass unchanged); the risk is scope creep, resist extracting more than the list.

TASK-8: Thread context.Context / timeouts through internal/git's exec plumbing and apply them to non-interactive callers only — the TUI's push/commit cmds and the jdi loop's probes (HeadCommit, CountVerdictCommits, LatestCommitIsVerdict); interactive session and mg done/delete keep no timeout; surface timeouts as ordinary wrapped errors.
     files: internal/git/git.go, internal/ui/app.go, cmd/mg/jdi.go, internal/git tests
     depends: none
     risk: low — a new failure mode added deliberately; PATH-faking git tests run fast and must not trip the timeout.

TASK-9: Unify argument parsing — move the remaining hand-rolled parsers (session.ParseArgs, runJob, runInit, runSetup, runProfiles) to flag.FlagSet, preserving exact behavior (passthrough, --check + profile arg, --tool legacy alias).
     files: internal/session/session.go, cmd/mg/job.go, cmd/mg/init.go, cmd/mg/setup.go, cmd/mg/profiles.go
     depends: none
     risk: low — behavior pinned by command tests; mechanical.

TASK-10: Create one source of truth for Go-side agent names (constants in internal/orchestrate or a small internal/agents package) and key Sequence, agentTargetFile, and agentMeta/agentOrder off it; keep the agent→target-file mapping next to the sequence.
     files: internal/orchestrate/, internal/ui/agents.go, cmd/mg/jdioutput.go, internal/ui/app.go
     depends: none
     risk: low — a rename should break the build by construction.

### Phase 3 — Documentation and contract

TASK-11: Sweep for stale architecture claims — fix AGENTS.md's "only place that shells out to git" sentence (verify after TASK-2), and remove stale references to the dead scripts (run.sh, new-job.sh, finish-job.sh, delete-job.sh, profiles.sh, setup.sh, init.sh, agents.sh, tui.sh, jdi.sh, scripts/lib/) from Go doc comments, keeping at most a one-line provenance note.
     files: AGENTS.md, doc comments across internal/ and cmd/
     depends: TASK-2 (doc should match reality; do the AGENTS.md edit in the same job at the latest)
     risk: low — comments only.

TASK-12: Trim comment bloat — keep every why/context comment, trim what-restating prose (worst offenders: 40–60-line doc comments on 15-line functions), and convert job-ID/brief archaeology citations into the plain reasoning they stand for; zero loss of why.
     files: doc comments across internal/ and cmd/
     depends: none (cleanest done after TASK-1/TASK-2 so comments describe final code)
     risk: low — but the biggest judgment-call surface; review-diff it.

TASK-13: Decide and write down the fate of the "output is the contract" rule — keep it for user-facing CLI wording, relax it for internal shapes (docker argv contents, TUI rendered substrings) with semantic assertions; do AFTER Phase 1/2 land so consolidation happens under the strict regime.
     files: tests across cmd/mg, internal/session, internal/ui, and a note in AGENTS.md or CODE_QUALITY docs
     depends: TASK-6, TASK-7 (must be the last thing, not the first)
     risk: medium — relaxing tests reduces the safety net by design.

### Phase 4 — Trivia and micro-hardening

TASK-14: Fix randomID's modulo bias on crypto/rand bytes in internal/job/create.go (rejection sampling or math/rand/v2 rand.IntN with the crypto reader).
     files: internal/job/create.go
     depends: none
     risk: low — negligible bias today; correctness-shaped micro-fix.

TASK-15: Make writeScaffold's file iteration deterministic by replacing the map with a slice of {name, content} pairs.
     files: internal/job/create.go
     depends: none
     risk: low — harmless today (independent files), deterministic order is tidier.

TASK-16: Cache home.Root() with sync.Once so os.Executable + EvalSymlinks run once per process instead of on every config/env access.
     files: internal/home/home.go, internal/config (verify EnvValue hot path)
     depends: none
     risk: low — micro-win; removes syscalls from EnvValue in the setup wizard.

TASK-17: Emit only the active profile's docker env keys — stop passing the CLAUDE_* -e flags unconditionally for opencode profiles (empty values today), matching KeyEnv's existing pattern.
     files: internal/session/docker.go, internal/session tests
     depends: none
     risk: low — docker argv is pinned by tests; update them to the new expected set.
