# Implementation: improve code quality

id: z6p077
status: open
developer: deepseek-v4-flash
date: 2026-08-12

Produced by @developer from tasks.md (the CODE_QUALITY_TASKS.md breakdown).
All four phases landed, TASK-1 through TASK-17, refactor-only: no behavior
change beyond the deliberate, test-pinned ones called out below. The full
suite is green after every task; `gofmt` and `go vet` are clean.

## Summary

Executed the full code-quality breakdown in phase order (TASK-1–17, with
TASK-13 last as the task list requires). Phase 1 killed the medium-scale
duplication the assessment called the top hazard (branch matching ×3, git
exec plumbing ×3, fs predicates ×4, the docs/jobs scan ×2, verdict
extraction ×2, the not-found error builder ×2) and restored the documented
"one shell-out point" architecture. Phase 2 hardened the design: domain
errors no longer carry presentation prefixes, the App god-file lost its list
view, non-interactive git calls are timeout-bounded, argument parsing is
uniform, and agent names agree by construction. Phase 3 swept stale
architecture claims, trimmed comment archaeology, and recorded the
"output-is-the-contract" decision. Phase 4 landed the four micro-fixes.

## Changes

TASK-1: Consolidated the three identical branch-matching helper copies
(`exactBranchMatch`/`prefixBranchMatches`/`branchTail`) into new exported
`git.BranchTail`/`git.ExactBranchMatch`/`git.PrefixBranchMatches`
(`internal/git/branch.go`), replaced the call sites in session/root.go,
job/finish.go, job/delete.go, and folded `resolveJob`'s name/ID matching
(cmd/mg/jdi.go) onto the same primitives. Tests moved/merged into
`internal/git/branch_test.go`. Exactly one definition of each helper now
exists; ambiguity-error wording is unchanged (test-pinned).

TASK-2: Routed every git exec through `internal/git`. Deleted session's
`gitToplevel`/`gitRaw`/`configValue` and init.go's `gitToplevel`;
`session.ResolveRootFrom` and `runInit` now use `git.RevParseToplevel`,
docker.go uses `git.ConfigUserName`/`git.ConfigEmail`. AGENTS.md's "only
place that shells out to git" is true again (verified: zero
`exec.Command("git")` in non-test code outside internal/git).

TASK-3: Created the tiny `internal/fs` package (`IsDir`/`IsFile`) as the
single home for the four isDir/isFile/isRegularFile predicate copies, and
unified `prefixJobDir` (session) with `prefixJobDirName` (job/delete) into
one `job.PrefixJobDirName(root, prefix)` docs/jobs scan returning the
matched name. Updated session/root.go, job/delete.go, agentlist, cmd/mg
agents.go, and create_test.go.

TASK-4: Unified the two verdict-section extractions onto one primitive,
`verdictOverallSection` (which now owns HTML-comment stripping; both moved
to stage.go). `verdictOverallMatch` is reimplemented on top of it. Decision
on the corner: the whole-section behavior wins over finish-job.sh's `-A5`
window — a genuine status beyond line 5 is recognized (it previously
spuriously warned "could not determine"); the scaffold's comment guidance
is no longer read as a verdict. Pinned by the moved/expanded
`TestVerdictOverallMatch` in stage_test.go.

TASK-5: Deduplicated the identical "job not found among local branches" +
branch-list error builder into one `job.jobNotFoundError` shared by
FinishJob and DeleteJob.

TASK-6: Decoupled presentation from domain errors. Removed the "Error: "
prefix from every `fmt.Errorf` in internal/job and internal/session (zero
remain), converted the `fmt.Errorf("%s", msg)` builders to `errors.New`,
and added one CLI framing helper `cmd/mg/errors.go: cliError` that all
domain-error print sites (runSession/runDone/runDelete/runJob) go through.
Rendered CLI output is byte-identical where it was already consistent; the
previously-bare `Invalid type '...'` error is now consistently prefixed
(deliberate, test-pinned change). Domain tests updated to expect the bare
wording.

TASK-7: Split the App god-file (1366 → 1141 lines). Extracted the list view
into `internal/ui/list.go` (`listView`: cursor, recent-activity strip data,
current branch, cursor-key handling, render/renderJobRow/renderRecentActivity/
recentActivityShown/listColumns/listFooter). App keeps routing, refresh,
cross-view state (status, spinner, jdi bell-dedup) and the viewport size,
passing it to `listView.render`. list_test.go/refresh_test.go updated
mechanically; all rendered-output assertions unchanged.

TASK-8: Threaded `context.Context` through internal/git's exec plumbing
(`runCtx`/`runEnvCtx`, returning `ctx.Err()` on expiry so
`errors.Is(err, context.DeadlineExceeded)` works) with `WithContext`
variants for the non-interactive functions: `PushWithContext`,
`CommitFileWithContext`, `HeadCommitWithContext`, `CountVerdictCommitsWithContext`,
`LatestCommitIsVerdictWithContext`. The TUI's push/commit cmds use a 30s
timeout (`hostGitTimeout`); mg-jdi's per-iteration probes use a 10s
timeout. Interactive session and mg done/delete keep no timeout. New
`internal/git/context_test.go` covers the timeout path with a stubbed slow
git and confirms the plain variants stay unbounded.

TASK-8 (reviewer fix): the jdi loop's stall backstop was silently defeated
in production. The per-iteration probeCtx (10s timeout) was created at the
top of the loop and reused for the post-agent `HeadCommitWithContext` probe
— but `runner.Run` (a minutes-long docker/LLM invocation) sits between
them, so by the time the probe ran the context was long expired and the
probe returned "" — `headAfter == headBefore` was always false and the
documented "same agent makes no progress on two consecutive runs" stop
condition never fired (a stuck agent ran up to maxIterations=20 instead of
stopping after 2). Fixed in cmd/mg/jdi.go: the post-agent probe now gets
its own fresh bounded context created after `runner.Run`, the in-loop
`defer cancelProbes()` is gone (contexts are cancelled at their point of
use, so no timers accumulate to function exit), and `jdiProbeTimeout`
became a var so tests can lower it. Pinned by the new
`TestRunStallProbeUsesFreshContext`, which lowers the timeout and uses a
fake runner whose Run outlives it (but writes nothing) — verified it fails
with the old probe-reuse behavior (20 invocations, "exceeded 20
iterations") and passes with the fix (2 invocations, "no progress").

TASK-9: Unified argument parsing on `flag.FlagSet` for runJob, runInit,
runSetup, runProfiles and session.ParseArgs, preserving pinned behavior:
passthrough, `--check` after a profile name, the `--tool` legacy alias, and
the exact "Unknown argument: <flag>" / usage wording (via a shared
`flagParseError` mapper and `splitFlags` extraction where flags must be
parseable in any position — Go's flag stops at the first positional).

TASK-9 (reviewer fix): the first pass silently ignored positional arguments
after the title/flags — `flag.FlagSet` stops at the first non-flag argument
and leaves the remainder in `fs.Args()`, which neither runJob nor runInit
checked (`mg job Add Gallery` created a job titled "Add"; `mg init extra`
proceeded). The script's hand-rolled loops rejected any such positional as
"Unknown argument: <arg>" + exit 1. Restored that: after `fs.Parse`, both
runJob (cmd/mg/job.go) and runInit (cmd/mg/init.go) now reject a non-empty
`fs.Args()` with the old wording and exit 1. Pinned by new
TestRunJobRejectsPositionals (unquoted-title word, stray after --type,
stray after --base-branch) and TestInitRejectsPositionals (bare word, word
after --tool).

TASK-9 (third-round fix): the FlagSets for runJob and runInit were created
with `fs.SetOutput(stderr)`, so the flag package printed its own diagnostic
("flag provided but not defined: -bogus", "flag needs an argument: -type")
ahead of the pinned "Unknown argument: <flag>" line — the old hand-rolled
loops printed exactly one line, on exactly the surface TASK-13 declares
contract. Both FlagSets now use `fs.SetOutput(io.Discard)` (the mapped line
is the only output; the --help path is unaffected because fs.Usage is a
no-op and the handler prints "Unknown argument: --help"). The unknown-arg
and missing-value tests now assert the full stderr with exact equality
(TestRunJobUnknownArg, TestRunJobMissingValue, TestInitUnknownArgument,
TestInitMissingValue), so any leaked flag-package line fails the build.
Also folded in the TASK-11 note's small miss: the two doc references to
"output.go" in cmd/mg/jdi.go now say jdioutput.go.

TASK-10: Created `internal/agents` as the single source of truth for the
Go-side agent names. `orchestrate.Sequence` and the new
`orchestrate.AgentTargetFile` (moved next to the sequence) key off the
constants, as do the TUI's `agentMeta`/`agentOrder`; cmd/mg/jdi.go reads the
mapping from orchestrate. A rename now breaks the build.

TASK-11: Swept stale architecture claims. AGENTS.md's "only place that
shells out to git" was verified accurate (TASK-2 restored it). Removed
dead-script references that described run.sh/new-job.sh/etc. as live or
future work ("stays on disk until Phase 5", "strangler stage 0",
"kept in sync with scripts/..."), the stale `tui/`-prefixed package paths
left over from the consolidation, and multi-line citations — keeping at
most one-line "was <script>" provenance notes.

TASK-12: Trimmed comment bloat across 22 files: converted ~70
job-ID/brief/TASK-N archaeology citations into the plain reasoning they
stand for (or dropped them where the reasoning was already stated), and
trimmed what-restating prose on several functions. Every why-comment kept
(the terminal-probe race, the tmux argv reasoning, the launch failure-mode
analysis). Net −20 lines with zero loss of why.

TASK-13 (last): Decided and wrote down the fate of the "output is the
contract" rule in docs/CODE_QUALITY_TASKS.md §3.3: keep the rule for
user-facing CLI wording; relax it for internal shapes (docker argv
contents, TUI rendered substrings) with semantic assertions. The audit
found the tests already practice the relaxed regime; the spot-check
confirmed the semantic assertions still fail on a deliberate regression —
and caught a broken absence-assertion in the TASK-17 docker test, which was
fixed.

TASK-14: Fixed `randomID`'s modulo bias with rejection sampling (draw a
byte until it lands in [0, 252), the largest multiple of 36 below 256, so
every a-z0-9 char is exactly equally likely).

TASK-15: Made `writeScaffold`'s file iteration deterministic by replacing
the map with a slice of {name, content} pairs in the natural file order.

TASK-16: Cached the expensive executable-root derivation in
`home.executableRoots` with `sync.Once` (os.Executable + EvalSymlinks run
once per process instead of on every config env read). The MANIGOT_HOME env
check stays uncached — it is cheap and must be read fresh (Seed sets it at
startup, tests set it per-test).

TASK-17: Emit only the active profile's docker env keys. The four
unconditional `-e CLAUDE_*` flags in docker.go are gone; CheckAuth now
forwards the claude-pro subscription keys into `KeyEnv` (with the same
empty-value filter the opencode keys use), so an opencode profile's docker
argv no longer carries `-e CLAUDE_*==""` noise. docker/session tests
updated to the new expected set and assert the absence of foreign keys.

## Known issues / follow-ups

- The TASK-8 stall-backstop fix landed after the reviewer's second NEEDS WORK
  verdict (the sole blocker); all other tasks were approved as committed.
- The TASK-9 positional-rejection fix (the first round's blocker) is
  unchanged and remains in place.
- The TASK-9 stderr-leak fix (the third round's blocker) makes `mg job` and
  `mg init` print exactly one error line on unknown-flag / missing-value
  inputs, as the scripts did; pinned by exact-equality assertions.
- `splitFlags` (the flags-in-any-position pre-extraction for `flag.FlagSet`)
  exists in two copies: `internal/session/session.go` and `cmd/mg/flags.go`.
  They serve different layers (internal/session vs the CLI) and a shared
  home would be a worse abstraction for 15 lines; a future `internal/cli`
  arg-parsing package could absorb it if a third consumer appears.
- TASK-6's cliError framing covers the domain-error print paths
  (session/done/delete/job); the other subcommands' hardcoded validation
  messages (init/agents/profiles/setup usage errors) still print inline
  rather than through the one helper — they are CLI-owned framing already,
  not domain errors.
- The TASK-4 verdict decision (whole-section wins over the -A5 window) is a
  deliberate behavior change for verdicts written more than five lines below
  the "## Overall" heading; pinned by TestVerdictOverallMatch.
- docs/CODE_QUALITY_TASKS.md and docs/CODE_QUALITY.md are the project's own
  assessment docs, not edited by this job beyond the §3.3 decision record.
