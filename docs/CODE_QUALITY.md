# manigot code quality: an assessment

A full read-through of the Go codebase (all 14 packages under `internal/`,
every command in `cmd/mg/`, the TUI, and the test suite) conducted after the
bash → Go consolidation landed. Build, `gofmt`, and `go vet` are clean; the
full test suite passes. Coverage per package: 54.7% (`cmd/mg`) to 100%
(`editor`), with most packages between 75% and 95%.

## Verdict

This is not "messing things together." It is a disciplined, well-architected
codebase with one real weakness — medium-scale duplication — and a
distinctive stylistic trait: very heavy documentation.

The big-picture modularity is genuinely good; the fine-grained DRY is where
it slips. The consolidation killed the *large* duplication it was created to
kill (the brief itself notes `find_project_root` was copy-pasted into 4
scripts), but the Go port retains a family of *medium* duplications — most
visibly the branch-matching logic, which exists in three identical copies.

## What's genuinely good

**Package design.** The layout follows the AGENTS.md architecture faithfully:
`internal/git` as the (mostly) single shell-out point with a consistent error
taxonomy (`ErrNotARepo` + `wrapErr`), `internal/job` owning the lifecycle and
disk state, `internal/session` as the docker seam, `internal/orchestrate` as
a pure state machine. `cmd/mg/main.go` is a thin 108-line dispatcher;
essentially all logic lives in `internal/`.

**`orchestrate.Next` is a model of good design.** A pure function of
disk/git state, zero new state files, re-derivable after a crash mid-loop.
The reasoning around `verdictRounds` vs `latestCommitIsVerdict` (telling
"rejected-but-not-answered" apart from "fixed-since-verdict") is exactly the
kind of hard-won logic that should be written down — and is.

**Testability seams are excellent.** `CreateOptions.RandomID`/`DeviceCheck`,
`launch.ExeOverride`/`JdiExe`, `editor.LookPath`, `markdown.SetStyle`,
`ringBell`, `jdiNow`, `ConfirmFunc`, `StatusFunc`, the `AgentRunner`
interface for jdi — every OS boundary and time dependency is injectable, and
the tests use these seams. This is deliberate, consistent discipline.

**Error handling is consistent.** Sentinel errors where they matter
(`ErrNotARepo`, `ErrCancelled`, `ErrNotFound`), `errors.Is`/
`errors.As` used correctly, and graceful degradation followed as a policy
("never crash on host-command error"). The `cli.Confirm` `bufio.Reader`
reuse caveat is a real subtlety that was caught and documented.

**Testing.** Coverage is meaningful, not just line-hitting: the `isWritten`
scaffold-detection heuristic has tests for every edge (one substantive line,
TASK-lines, CRLF, unterminated comments), the jdi loop exercises all four
`orchestrate.Next` branch outcomes, and the docker argv is pinned by tests.
Git- and docker-adjacent tests run real git in temp dirs; the jdi loop tests
a fake `AgentRunner`. Table-driven where it fits, `t.Helper()` used
properly.

**No rot markers.** Zero `TODO`/`FIXME`/`HACK`. Every one of the 85
exported functions has a doc comment (verified mechanically).

## The DRY problem: concrete duplication

The one area where the codebase violates its own standards. All confirmed:

| # | Duplicated logic | Copies |
|---|---|---|
| 1 | `exactBranchMatch` / `prefixBranchMatches` / `branchTail` | **3 identical copies**: `internal/session/root.go:160-188`, `internal/job/finish.go:245-273`, `internal/job/delete.go` (the last two reuse the same trio). A 4th sibling pattern — `resolveJob` in `cmd/mg/jdi.go` (exact name / exact ID / prefix) — is the same matching family against `job.Job` instead of branch strings. |
| 2 | `configValue` (reads `git config` key) | 2 copies: `internal/git/git.go:763` and `internal/session/docker.go:235`. |
| 3 | `isDir` / `isFile` | 4 copies: `session/root.go`, `job/delete.go:237`, `agentlist/agentlist.go:151`, `cmd/mg/agents.go:94` (`isRegularFile`). |
| 4 | `prefixJobDir` (session) vs `prefixJobDirName` (job/delete) | 2 near-identical scans of `docs/jobs` with the archive/ exclusion. |
| 5 | The `git -C <root>` exec plumbing | 3 copies: `git.run`/`runEnv`, `session.gitRaw` (`root.go:215`), plus direct `exec.Command("git", ...)` in `session.gitToplevel` and `cmd/mg/init.go:162`. |
| 6 | Verdict-section text extraction | 2 variants in the *same package*: `verdictOverallMatch` (`job/finish.go:280`, grep-`-A5` window) vs `verdictOverallSection` (`job/stage.go:146`, whole-section). They share the regexes but not the extraction. |
| 7 | "job not found among local branches" error block | Identical ~10-line builder in `finish.go` and `delete.go`. |

Two deserve special comment:

- **#1 is the one that will bite.** Branch matching is exactly the logic the
  `jobBranchPrefix` setting touches. The moment someone changes matching
  semantics, three files must change in lockstep — and the tests won't catch
  a miss, because each copy has its own passing tests. The natural home is a
  small branch helper in `internal/git`, with `resolveJob`'s name/id
  matching folded in.
- **#5's justification is stale.** The comment on `gitRaw` says it exists
  "so the session package doesn't need the git package's error
  classification" — but `session/root.go` already imports `internal/git`
  (for `LocalBranches`, `WorktreeForBranch`, `GitCommonDir`), and git does
  not import session, so there is no cycle. `session.gitToplevel` likewise
  duplicates the exported `git.RevParseToplevel`, and `init.go`'s
  `gitToplevel` is a third copy. This also quietly breaks the AGENTS.md
  claim that `internal/git` is "the only place that shells out to git" —
  the session launcher and `mg init` both shell out directly. The doc needs
  correcting, or the code needs to route through git (preferably both).

## Design smells (minor but real)

1. **Presentation coupled into domain errors.** `job.CreateJob` returns
   `fmt.Errorf("Error: base branch '%s' does not exist...")` — the domain
   layer formats errors with a CLI presentation prefix. Every layer above
   prints them verbatim. This is faithful to the scripts (a deliberate port
   artifact), but it means the TUI, `mg jdi`, and any future consumer
   inherit "Error: " strings, and several errors are built as
   `fmt.Errorf("%s", msg)` with no `%w`, killing wrapability. The clean end
   state would be domain errors that say *what* happened, with the CLI
   adding the "Error:" prefix — but this is a judgment call given the
   "output wording is the contract" rule.

2. **The App god-file.** `internal/ui/app.go` is 1366 lines: state routing,
   list rendering, refresh/polling, bell dedup, spinner chain, jdi launch
   guard, agent launches, key handling, commit/push commands. Internally
   well-organized (section comments, small methods), and Bubble Tea
   tolerates a fat root model — but note the asymmetry: detail/settings/
   newjob/agentspicker/confirm are each separate view structs, while the
   *list* view lives inline in App.

3. **No timeouts or context on any subprocess.** Every `exec.Command` and
   docker run is context-free. The interactive session is fine
   (user-driven), but `git.Push` runs in a `tea.Cmd` goroutine from the TUI
   — a stalled network can hang it forever (`GIT_TERMINAL_PROMPT=0` covers
   only the credential case). A `context.WithTimeout` on the non-interactive
   paths (push, jdi's git probes) would be cheap hardening.

4. **Three ad-hoc argument-parsing styles in one binary.** `runJDI`/
   `runTUI` use `flag.FlagSet`; `session.ParseArgs`, `runJob`, `runInit`,
   `runSetup`, `runProfiles` hand-roll loops. Defensible given 1:1 port
   fidelity, but a reader has to learn two conventions.

5. **Stringly-typed agent names across layers.** `ui/agents.go`
   (`agentMeta`/`agentOrder`), `orchestrate.Sequence`, `jdioutput.go`'s
   `agentTargetFile`, the `agents/*.md` files, and the entrypoint all
   hardcode the same names independently. The file-based ones are
   necessarily data-driven; the three Go-side lists could share constants.

6. **Trivia.** `randomID` uses modulo on crypto/rand bytes (negligible bias,
   fine for job IDs; rejection sampling or `rand.IntN` would be cleaner);
   `writeScaffold` iterates a map (nondeterministic write order — harmless,
   a slice would be tidier); `home.Root()` re-runs `os.Executable` +
   `EvalSymlinks` on every config access (micro-cost, fine).

## Documentation: the distinctive trait

The comments are the most opinionated thing about this codebase, and they
cut both ways.

**Where they're excellent:** the *why*-comments are frequently outstanding —
the glamour terminal-probe race in `markdown.go`, the tmux
`argv-vs-single-string` login-shell explanation in `launch.go`, the "why
`--quiet` + empty stderr means detached HEAD" in `git.go`, the recent-commits
`--source` single-traversal trick. This is hard-won knowledge that would be
lost without the comments.

**Where they overreach:** many comments restate what the code already says,
at length, and — distinctively — they cite the job-workflow archaeology
("see 207bfu_git-worktrees, Decision 2", "per TASK-7 review"). There are doc
comments of 40–60 lines on functions whose bodies are 15. For a solo
maintainer this is a personal knowledge base; for anyone else it reads like
meeting minutes, and nothing enforces the accuracy of job-ID references once
those jobs are archived and the docs they cite are edited.

Recommendation: keep every *why*/context comment, but trim the
*what*-restating prose, and convert job-ID citations into plain reasoning
("the retry budget is one bounce" beats "see brief scope item 2"). This is
the single largest maintainability liability — not because it's wrong, but
because it will rot first.

## Testing assessment

- Coverage is genuinely strong (54.7–100%, most packages 75–95%) and
  meaningful, not just line-hitting.
- The per-package `git -C` test helper duplication is standard Go practice
  (test helpers don't cross package boundaries) — not a problem.
- The one tension: tests assert byte-exact output where the contract is
  "the old script's wording." That is the right call *during* a strangler
  migration (it proves fidelity), but it makes any future wording change
  expensive and pins refactoring. Once the scripts are fully gone,
  relaxing exact-output assertions in favor of semantic ones should be on
  the table.

## Risks and open questions

1. **The three-copy branch matcher is the top hazard** — any semantic
   change to job-branch resolution (prefixes, new type segments, id-format
   changes) must touch three files in lockstep, silently.
2. **Doc drift is already happening** — AGENTS.md says git is the only
   shell-out point; session and init contradict it. The codebase's own
   accuracy bar is high, so this kind of drift compounds.
3. **The "output is the contract" rule** — deliberate and useful now; it
   becomes a tax later. Decide consciously when to retire it.
4. **No timeouts on subprocesses** — the TUI push path can hang on a dead
   network.
5. **Comment archaeology** — job-ID/brief references have no enforcement
   mechanism; they'll rot and mislead.

## Bottom line

Quality is solidly above average, with genuine architectural discipline:
the layering, error taxonomy, injection seams, and testing show deliberate
design, not accretion. The honest failings are (a) the reintroduced
medium-scale duplication (branch matching ×3, git plumbing ×3, helpers ×4),
(b) presentation-coupled error strings, (c) a 1366-line App file, and (d)
comment bloat that will rot. None of these are "messing things together" —
they are the predictable residue of a faithful, script-fidelity-first port.

The highest-leverage cleanup, if one is done: consolidate the branch-matching
and git-exec plumbing into `internal/git` and delete the private copies —
it both reduces the biggest DRY violation and restores the "one shell-out
point" architecture documented as the intended design.
