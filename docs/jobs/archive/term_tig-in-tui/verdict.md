# Verdict: tig in tui

id: term
status: open
reviewer: @reviewer
date: 2026-08-16

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed the full branch diff `main...HEAD` (13 files, +845/−7) against
tasks.md. Branch `feature/term_tig-in-tui` matches brief.md; base branch is
`main` (from `.manigot/manigot.json`). Every changed file maps to a task —
no out-of-scope changes. Static review only (the session git shim allows
git read/commit commands only, so `go test` could not be run here; the
tests were verified by reading them against the existing test
infrastructure they reuse).

TASK-1: PASS
notes: `internal/launch/launch.go` — `var TigLookPath = exec.LookPath`
(exported seam, mirrors `ExeOverride`/`JdiExe`) and `func TigAvailable() bool`
(LookPath probe, no version/config scan). `errors` import added and used.
Matches the task exactly.

TASK-2: PASS
notes: `internal/launch/launch.go` — `Tig(jobID, projectRoot, terminal)` and
`tigShellCommand(manigotPath, jobID, projectRoot)`. Ordering correct:
`TigAvailable()` check first (the authoritative backstop, returning the
not-installed error synchronously), then `ExeOverride()`, then
`launchDetached`. Inner command is `cd '<root>' && '<exe>' diff '<name>' --tig`
— cd-first, shellQuote-everything, `holdOnFailure`-wrapped, no
`--profile`/`--agent`/`--job` (verified `mg diff` in cmd/mg/diff.go takes
none of those flags; it resolves the job via `resolveJobBranch` →
`git.ExactBranchMatch` on the branch tail, and `job.Name` equals that tail
exactly, so passing `job.Name` is unambiguous). Reuses the existing tmux
split-pane / terminal-override / replace-policy machinery via `launchDetached`
— no new terminal code, matching the brief's "just like agents".

TASK-3: PASS
notes: `internal/launch/launch_test.go` — five new test groups cover the
shell-command format (prefix + holdOnFailure wrap), absence of session flags,
path-with-spaces quoting, embedded-quote escaping, `TigAvailable` true/false
via stubbed `TigLookPath`, `Tig`'s tig-missing-before-spawn path (proving the
availability check precedes `ExeOverride`), `Tig`'s `ExeOverride` failure
path, and the success path through the existing `tmuxStub` (desc "tmux pane",
pane recorded, split-window carries the inner command, select-pane tagging
asserted). Helpers (`newTmuxStub`, `stubTigLookPath`) match the package's
existing conventions; no name collisions.

TASK-4: PASS
notes: `internal/ui/detail.go` — `tigAvailable bool` field on `detailView`,
set in `newDetailView` via `launch.TigAvailable()` (re-checked per job open,
cached per view — matches the task). Footer hint `· t tig` added only when
`d.tigAvailable && d.job.Branch != ""`, mirroring the conditional `e edit`
hint. Existing footer tests assert via `strings.Contains`, so the added hint
cannot break them (the zero-value default keeps them green regardless).

TASK-5: PASS
notes: `internal/ui/app.go` — `"t"` case in `updateDetail`, placed before the
`agentForKey` fallthrough (no agent uses key "t" — verified agentMeta keys
a/o/d/r/s). Three paths exactly as specified: not-installed status
("tig is not installed on the host — install it, or use the diff tab") when
`!tigAvailable`; the same "no branch known for this job" guard the `P` key
uses when branch-less; otherwise `launch.Tig(a.detail.job.Name, a.root,
a.settings.Terminal)` with `"→ tig in " + desc` / `cmdErrorText(err)` status.
`internal/ui/agents.go` doc comment updated to include `t tig` in the
non-colliding binding list. The full runtime flow was traced end to end:
shell cds to `a.root` (project root), `mg diff` re-finds the project from
$PWD, resolves the branch by Name, and `runTig` runs the three-dot range —
all consistent.

TASK-6: PASS
notes: `internal/ui/tig_test.go` — covers the footer hint (shown when
available + branch set; omitted when unavailable or branch-less), the `t`
key when unavailable (status set, `ExeOverride` proven not consulted via a
marker stub), the branch-less guard, the `ExeOverride` resolution-failure
path (proving routing to `launch.Tig`), and the success path with a locally
replicated tmux stub on PATH + `$TMUX` asserting the `diff <name> --tig`
inner command and select-pane tagging. Traced the success test: NewApp's git
calls fail gracefully with the stub-only PATH (`git.CurrentBranch`/
`RecentCommits` errors are swallowed), `launchDetached` takes the tmux branch,
split-window prints %100, status lands "→ tig in tmux pane". No parallel
tests in the package, so the PATH/`$TMUX` mutations are safe.

TASK-7: PASS
notes: README.md detail-view Keybindings table gets the `t` row (after `P`);
`docs/AGENTS.md` TUI section gains the matching sentence (confirmed live in
the mounted `/workspace/AGENTS.md`); `project-template/docs/AGENTS.md`
comment block gains the same sentence — the hard-rule sync pair is
consistent. Three descriptions agree with each other and with the code.

Commit discipline: each TASK-N has its own `[term] TASK-N: ...` commit and
implementation.md has its own commit, per the required format. Minor
observation (not a blocker): commit 6f35ebb, labeled TASK-1, also bundles the
analyst's full tasks.md rewrite — the tasks.md change would ideally be its
own commit, but it is cosmetic and is lost anyway in the `mg done` squash
merge.

## Security

No security-relevant surface: the new code shells out only to `tmux` /
`exec.LookPath` (both via the existing, already-reviewed launch machinery)
and builds a shell string with the package's existing `shellQuote` escaping
(verified: embedded quotes in projectRoot/jobID are escaped, not injected —
`TestTigShellCommandQuoteEscape` pins this). No new credentials, no new
mounts, no agent permissions touched. The launched pane runs `mg diff --tig`
as the host user in the project root only, identical to existing agent
launches.

## Overall

APPROVED

All seven tasks are implemented as specified, with no missing pieces, no
out-of-scope changes, and no bugs found in the traced runtime and test
paths. The only observation (TASK-1's commit bundling the tasks.md rewrite)
is cosmetic and does not block merge.
