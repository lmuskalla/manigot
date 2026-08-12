# Verdict: improve code quality

id: z6p077
status: open
reviewer: deepseek-v4-flash
date: 2026-08-12

## Review

Fourth review round. Re-reviewed the full `main...HEAD` diff (59 files) in its
final on-branch state, each task's commit, and the round-3 blocker's fix
(commit 6ffcc20). Verified the end-state invariants from
docs/CODE_QUALITY_TASKS.md by grep, and ran the suite: `go build ./...`,
`go test -count=1 ./...` (14 packages, green), `gofmt -l` (clean), `go vet
./...` (clean). The only failure seen across runs is the pre-existing flaky
`TestJdiStartsResolvedCommandDetached` in internal/launch (see Notes).

TASK-1: PASS
notes: branch matching consolidated into internal/git/branch.go
(BranchTail/ExactBranchMatch/PrefixBranchMatches — exactly one definition
each, grep-verified); session/root.go, job/finish.go, job/delete.go and
cmd/mg/jdi.go's resolveJob all route through the primitives. resolveJob's fold
preserves the old resolution order (exact name → exact ID → name-prefix) and
the ambiguity wording; tests moved/merged into internal/git/branch_test.go.

TASK-2: PASS
notes: session.gitToplevel/gitRaw/configValue and init.go's gitToplevel
deleted; call sites use git.RevParseToplevel (error → $PWD fallback, keeping
the old "empty means not a repo" degrade) and git.ConfigUserName/ConfigEmail.
Verified zero `exec.Command("git")` in non-test code outside internal/git —
AGENTS.md's "only place that shells out to git" is true again and needed no
edit.

TASK-3: PASS
notes: internal/fs (IsDir/IsFile) is the single predicate home; the two
docs/jobs prefix scans unified into job.PrefixJobDirName (returns the name;
callers join the root), with the `archive/` exclusion in exactly one place.
session/root.go, job/delete.go, agentlist and cmd/mg/agents.go updated; no
isDir/isFile/isRegularFile/prefixJobDir* copies remain.

TASK-4: PASS
notes: verdictOverallSection is the single extraction primitive and now owns
HTML-comment stripping (the old call-site strip in verdictApproved moved in,
same result); verdictOverallMatch is built on it. The line-5 corner is decided
(whole-section wins over finish-job.sh's -A5 window) and pinned by the moved
TestVerdictOverallMatch including the "status beyond line 5" and "scaffold
comment is not a status" cases; the decision is documented in the comment and
in implementation.md.

TASK-5: PASS
notes: one jobNotFoundError shared by FinishJob and DeleteJob, wording
unchanged (modulo TASK-6's prefix move).

TASK-6: PASS
notes: zero `fmt.Errorf("Error: ...")` under internal/ (grep-verified); the
`fmt.Errorf("%s", msg)` builders converted to errors.New; cmd/mg's domain-error
print paths (session/done/delete/job) all route through the single cliError
framing helper (grep-verified — no bare `Fprintln(stderr, err)` domain prints
remain). Rendered CLI output byte-identical by construction (domain errors went
bare, cliError re-adds "Error: "); the previously-bare "Invalid type '...'"
error is now consistently prefixed — deliberate and documented in
implementation.md. %w wrapping retained where the wrapped error carries meaning.

TASK-7: PASS
notes: list view extracted into internal/ui/list.go (listView: cursor,
recent-activity strip data, current branch, cursor keys,
render/renderJobRow/renderRecentActivity/recentActivityShown/listColumns/
listFooter); App keeps routing, refresh, cross-view state, spinner and jdi
bell-dedup as required; the jdi bell-dedup/spinner machinery was NOT moved.
list_test.go/refresh_test.go updated mechanically — rendered-output assertions
unchanged, only receiver/args moved. Cursor routing including g/G and the
j/k non-collision are behavior-identical (pinned by TestListJAndKNoLongerMoveCursor).

TASK-8: PASS
notes: context plumbing correct — runCtx/runEnvCtx return ctx.Err() on expiry
so errors.Is(err, context.DeadlineExceeded) works; WithContext variants
(Push/CommitFile/HeadCommit/CountVerdictCommits/LatestCommitIsVerdict) wrap
timeouts as ordinary errors. TUI push/commit bounded at 30s (hostGitTimeout);
mg-jdi probes at 10s (var jdiProbeTimeout, lowerable by tests); interactive
session and mg done/delete keep no timeout (documented). The round-2 blocker
is fixed in the final state: the post-agent stall probe uses its own fresh
context created after runner.Run, the pre-agent probeCtx is cancelled right
before the invocation (no deferred timers accumulate to function exit), and
TestRunStallProbeUsesFreshContext pins the backstop firing with a runner whose
Run outlives the lowered probe timeout. context_test.go also pins the
unbounded plain variants (TestPlainRunHasNoTimeout).

TASK-9: PASS
notes: all three review-round blockers are resolved in the final state.
runJob/runInit/runSetup/runProfiles/session.ParseArgs are on flag.FlagSet;
positionals after the title/flags are rejected with the old "Unknown argument:
<arg>" wording + exit 1 (TestRunJobRejectsPositionals, TestInitRejectsPositionals);
flagParseError restores the scripts' wording for unknown flags and missing
values; the runJob/runInit FlagSets use fs.SetOutput(io.Discard) so the flag
package's own diagnostics cannot leak — the unknown-arg and missing-value
tests now assert the full stderr with exact equality (TestRunJobUnknownArg,
TestRunJobMissingValue, TestInitUnknownArgument, TestInitMissingValue).
runSetup's "zai --check" (splitFlags), runProfiles' help aliases, and
ParseArgs' passthrough (unknown tokens land in Pass, order preserved) all keep
their pinned behavior. The round-3 fix also folded in the TASK-11 note's small
miss: cmd/mg/jdi.go's "output.go" references now say jdioutput.go (verified).

TASK-10: PASS
notes: internal/agents constants are the single source of truth;
orchestrate.Sequence, the new orchestrate.AgentTargetFile (kept next to the
sequence, as the task requires) and the TUI's agentMeta/agentOrder all key off
them; cmd/mg/jdi.go reads the mapping from orchestrate. No hardcoded agent-name
strings remain outside the constants — a rename breaks the build.

TASK-11: PASS
notes: dead-script references reduced to at most one-line "was <script>"
provenance notes (grep-verified across internal/ and cmd/); the stale
tui/-prefixed package paths and "stays on disk until Phase 5"-class claims
removed; AGENTS.md needed no edit because TASK-2 restored its claim.

TASK-12: PASS
notes: comment-only commit across 22 files; archaeology citations converted to
the plain reasoning they stand for; spot-checked detail.go/markdown.go/
launch.go/jdistatus.go — every why-comment retained (terminal-probe race, tmux
argv reasoning, launch failure-mode analysis). Net reduction with zero loss of
why.

TASK-13: PASS
notes: decision recorded in docs/CODE_QUALITY_TASKS.md §3.3 (keep the rule for
user-facing CLI wording; relax for internal shapes with semantic assertions),
landed after Phase 1/2 as required — it is the last of the original task
commits. The audit and the spot-check (broken absence-assertion in the TASK-17
docker test caught and fixed) are documented.

TASK-14: PASS
notes: rejection sampling with limit 252 (256 − 256%36) — every a-z0-9 char
exactly equally likely; rejection rate ~1.6%. Correct.

TASK-15: PASS
notes: writeScaffold iterates a {name, content} slice in the natural file
order; content byte-identical to the old map entries.

TASK-16: PASS
notes: executable-derived roots memoized with sync.Once; the MANIGOT_HOME env
check in Root() correctly stays uncached (Seed at startup and per-test env
overrides still work), documented in the comment.

TASK-17: PASS
notes: the four unconditional `-e CLAUDE_*` flags are gone from docker.go;
CheckAuth now forwards the claude-pro subscription keys into KeyEnv with the
same empty-value filter the opencode keys use. Both BuildDockerInvocation call
sites (cmd/mg/session.go, cmd/mg/jdi.go) run CheckAuth first (grep-verified),
so the claude-pro argv still carries the token while opencode profiles no
longer receive empty CLAUDE_* flags — absence asserted in docker_test.go and
session_test.go. Container-side behavior unchanged (entrypoint.sh treats
absent identically to empty); a strict security improvement.

## Notes (informational, not blockers)

- `TestJdiStartsResolvedCommandDetached` (internal/launch) remains the same
  pre-existing flaky test: it reads the stub's output file as soon as it
  exists, before the shell has flushed all lines (the record can be missing
  the arg=/cwd= lines). The launch package's changes on this branch are
  comment-only and launch_test.go is byte-identical to main, so this is not
  caused by the job; worth a follow-up, out of scope here.
- The TASK-9 `--` terminator edge remains: `mg job "Title" --` and
  `mg init --` silently succeed where the old loops errored "Unknown
  argument: --" (flag consumes "--" as the terminator). Likewise
  `mg job T --type=fix` is now accepted (flag's =value syntax), and
  `mg profiles --bogus` now errors with the flag wording instead of being
  treated as a profile name. Also, in runInit, `--profile` now always wins
  over `--tool` when both are given (old loop was last-wins). All are obscure
  inputs on unpinned surface, either behavior defensible, and were noted in
  the round-3 verdict; not blockers.
- The TASK-4 verdict-warning wording change for an unwritten scaffold verdict
  is a deliberate consequence of the documented whole-section decision, not a
  regression.

## Security

No security findings. The only secret-handling change (TASK-17) narrows which
credentials are forwarded into the container — claude-pro keys no longer leak
into opencode profiles' docker env. No .env or token material is committed.

## Overall

APPROVED

No blockers. All 17 tasks are implemented as specified, the end-state
invariants from docs/CODE_QUALITY_TASKS.md hold (grep-verified: one git
shell-out point, one definition per helper, zero prefixed domain errors, all
cmd/mg domain printers framed through one helper), the full suite is green
with `-count=1`, and gofmt/vet are clean. Commit discipline holds: every task
has its own `[z6p077] TASK-N:` commit, TASK-13 is the last original task
commit, and the three reviewer-fix rounds are committed as follow-ups on their
tasks. The only outstanding items are pre-existing and out of scope (the
internal/launch test flake) and the documented unpinned TASK-9 edge cases,
none of which block merge.
