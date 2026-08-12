# CODE_QUALITY_TASKS: from "well above average" to "totally solid design"

Raw material for turning the findings in `docs/CODE_QUALITY.md` into
implementation work. Free-flow on purpose — the pipeline will reformat this
into a task breakdown. Every item below is **refactor-only: no behavior
change**. The existing test suite (which pins exact output, docker argv, and
the TUI's rendered text) is the safety net — if a change to wording is
unavoidable, the tests will say so, and the change must be deliberate.

The guiding principle throughout: **make the code look like the architecture
already documented in AGENTS.md.** Most of these tasks exist because the code
drifted from the stated design (the "only place that shells out to git"
invariant, the "one seam" model, DRY). Restoring the code to the documented
design is the definition of "totally solid" here.

Suggested ordering: Phase 1 first (biggest hazard, restores the documented
invariants), then Phase 2 (design smells), then Phase 3 (docs/contract), then
Phase 4 (trivia). Each phase is independently shippable; Phase 1 could stand
alone as its own job if a smaller cut is preferred.

---

## Phase 1 — Kill the duplication, restore the documented architecture

### 1.1 Consolidate branch matching into `internal/git` (highest priority)

`exactBranchMatch`, `prefixBranchMatches`, and `branchTail` exist in **three
identical copies**: `internal/session/root.go:160-188`, `internal/job/
finish.go:245-273`, and `internal/job/delete.go` (which reuses the finish
copies). A fourth sibling — `resolveJob` in `cmd/mg/jdi.go` (exact name,
exact ID, prefix) — is the same matching family against `job.Job` structs
instead of branch strings.

This is the most dangerous duplication in the codebase: branch matching is
exactly the logic the `jobBranchPrefix` setting touches. A semantic change
to job-branch resolution (prefixes, new type segments, id format) currently
requires three files to change in lockstep, and the tests won't catch a
miss — each copy has its own passing tests.

Do:
- Add a small branch-matching surface to `internal/git` (e.g.
  `BranchTail`, `ExactBranchMatch`, `PrefixBranchMatches` — or one
  `ResolveBranch(root, arg)` that returns exact-or-unambiguous-prefix),
  with the existing per-copy tests moved and merged there.
- Replace all three call sites. `session/root.go` and `job/finish.go` /
  `job/delete.go` should import git for this instead of defining it.
- Fold `resolveJob`'s name/ID matching onto the same primitives where the
  shapes allow (a `Job` name is `id_slug`; branch tail is the same
  `id_slug` — they are the same concept).
- Keep the ambiguity-error wording identical (it is pinned by tests and is
  user-facing contract).

Files: `internal/git/` (new), `internal/session/root.go`,
`internal/job/finish.go`, `internal/job/delete.go`, `cmd/mg/jdi.go` + tests
Risk: medium — the resolution paths for `--job`, `mg done`, `mg delete`,
and `mg jdi` all route through this. The tests for all four exist and are
good; run the full suite.
Verify: `go test ./internal/git ./internal/session ./internal/job ./cmd/mg`
green; `grep` shows exactly one definition of each helper.

### 1.2 Route every git exec through `internal/git` (restores the documented invariant)

AGENTS.md says `internal/git` is "the only place that shells out to git."
It isn't: `internal/session/root.go` shells out directly (`gitToplevel` at
line 151, `gitRaw` at line 215), `internal/session/docker.go` has its own
`configValue` over that `gitRaw`, and `cmd/mg/init.go` has a third
`gitToplevel` copy. The `git -C <root>` exec plumbing now exists in three
places, and the session's private copies are not even justified by an
import cycle — `session` already imports `internal/git`.

Do:
- Replace `session.gitToplevel` and `init.go`'s `gitToplevel` with the
  existing exported `git.RevParseToplevel` (it returns the same value;
  `init.go`'s version also swallows the error, which `RevParseToplevel`
  classifies as `ErrNotARepo` — keep the "empty means not a repo" degrade
  at the call site).
- Replace `session.configValue` with `git.ConfigUserName` /
  `git.ConfigEmail` (they exist and are exported — `docker.go` currently
  re-implements them for no reason).
- Delete `session.gitRaw` once its two users are gone.
- Update the AGENTS.md sentence to match reality, or — better — let it
  become true again. (If a few deliberate exceptions remain, say so in
  AGENTS.md explicitly rather than letting the doc drift.)

Files: `internal/session/root.go`, `internal/session/docker.go`,
`cmd/mg/init.go`, `AGENTS.md` (only if exceptions remain)
Risk: low — pure replacement of identical behavior; `session`'s docker
argv tests and `init`'s tests pin the observable behavior, not the helper.
Verify: full `go test ./...`; `grep -rn 'exec.Command("git"'` shows hits
only in `internal/git/` (and tests).

### 1.3 Consolidate the tiny filesystem helpers

`isDir`/`isFile` are reimplemented in four places (`session/root.go`,
`job/delete.go`, `agentlist/agentlist.go`, `cmd/mg/agents.go`), and
`prefixJobDir` (`session/root.go:194`) vs `prefixJobDirName`
(`job/delete.go:211`) are the same `docs/jobs` scan with the `archive/`
exclusion, differing only in return type.

Do:
- One home for the two file predicates — either a tiny shared internal
  package or, cleaner, as small exported helpers on an existing
  filesystem-adjacent package (`internal/job` already owns `JobsRelDir`/
  `ArchiveDirName`, so `prefixJobDir`-style scanning naturally belongs
  there; the pure `isDir`/`isFile` predicates fit a `internal/fs`-sized
  package or can be inlined as `os.Stat` calls where they're used once).
- Unify `prefixJobDir`/`prefixJobDirName` into one function (returning the
  matched directory name; callers join the root themselves).
- Judgment call, not dogma: `isDir`/`isFile` are one-liners, so the win is
  consistency and single-point-of-change for the `archive/` exclusion, not
  lines saved. If the four call sites end up with different enough needs,
  accept two homes max — but the `docs/jobs` scan must be one.

Files: `internal/session/root.go`, `internal/job/delete.go`,
`internal/agentlist/agentlist.go`, `cmd/mg/agents.go`
Risk: low.
Verify: full suite green; each helper has exactly one definition.

### 1.4 Unify verdict-text extraction in `internal/job`

`verdictOverallMatch` (`finish.go:280`, grep-`-A5` window: heading + five
lines, first status line) and `verdictOverallSection` (`stage.go:146`,
whole section up to the next `##`) are two extractions of the same
"## Overall" region, in the same package, sharing the same regexes but not
the extraction. The `-A5` window is a fidelity artifact of finish-job.sh's
`grep -A5`; the section version is the natural expression.

Do:
- Make one extraction primitive (the section version — it subsumes the
  window: a status line in the first five lines is also in the section).
- Reimplement `verdictOverallMatch` on top of it if finish semantics truly
  need the window (they may not — verify against finish-job.sh's actual
  behavior and the tests), or delete it and use the section everywhere.
- Watch the corner: a verdict line *after* the fifth line currently
  doesn't match for finish but does for stage. That difference may be a
  latent bug or a deliberate fidelity choice — decide, pin it with a test,
  and document the decision.

Files: `internal/job/stage.go`, `internal/job/finish.go`
Risk: low-medium — behavioral edge (see above) must be pinned by a test
before the consolidation, not discovered after.
Verify: `go test ./internal/job`; add an explicit test for the
"status beyond line 5" case documenting which behavior wins.

### 1.5 Deduplicate the "job not found" error builder

`finish.go` and `delete.go` build the identical ~10-line "job '%s' not
found among local branches" + branch-list message. This lands mostly for
free once 1.1 lands (the resolve helper can own the message); if 1.1 is
deferred, do it here anyway — one helper, two call sites.

Files: `internal/job/finish.go`, `internal/job/delete.go`
Risk: low (wording pinned by tests).
Verify: full suite green.

---

## Phase 2 — Design hardening

### 2.1 Decouple presentation from domain errors

Today the domain layer returns display-ready strings: 23 errors across
`internal/job` and `internal/session` are built as `fmt.Errorf("Error:
...")`, while others in the same files are bare (`"Invalid type '%s'..."`)
or properly `%w`-wrapped (`"generating job id: %w"`). Three styles in one
function file (`create.go` alone has all three). The CLI prints them all
verbatim, so the *rendered* output is already inconsistent — some errors
show "Error:", some don't.

Do:
- Audit: list every error the domain packages return, sorted by which
  style it uses today. This audit is itself the first deliverable.
- Decide the end state: domain packages return errors that say *what*
  happened (typed where callers branch on them — see the existing
  sentinels `ErrNotARepo`/`ErrCancelled` as the model), with no
  presentation prefix; the CLI layer (`cmd/mg`) owns the "Error: "
  prefix and any user-facing framing, in exactly one place.
- Keep the rendered output byte-identical *where it's already consistent*;
  where it's inconsistent today (the bare errors), standardizing to the
  prefixed form is an intentional, test-pinned output change — call it out
  in the task notes rather than smuggling it.
- Prefer `%w` wrapping over `%s` concatenation everywhere the wrapped
  error carries meaning (some `fmt.Errorf("%s", msg)` sites currently
  discard the underlying error entirely).
- Consider typed errors where callers genuinely branch: e.g. a
  `NotApproved`/`UncommittedChanges` distinction for the finish/delete
  flows is currently inferred from message text by nothing (good) but the
  TUI shows raw domain strings in status lines — typed errors let the TUI
  format its own way without parsing.

Files: `internal/job/create.go`, `internal/job/finish.go`,
`internal/job/delete.go`, `internal/session/session.go`,
`internal/session/root.go`, `cmd/mg/*` (error printing), TUI status
paths (`internal/ui/app.go`'s `cmdErrorText` callers)
Risk: medium — the "wording is the contract" rule means every touched
message is test-pinned. This is the one Phase-2 item that will churn
tests; budget for it.
Verify: full suite green; `grep 'fmt.Errorf("Error:'` under `internal/`
returns zero; every `cmd/mg` error printer goes through one framing helper.

### 2.2 Split the App god-file

`internal/ui/app.go` is 1366 lines: state routing, list rendering, refresh/
polling, bell dedup, spinner chain, jdi launch guard, agent launches, key
handling, commit/push commands. The asymmetry: detail/settings/newjob/
agentspicker/confirm are each separate view structs, but the list view
lives inline in App.

Do:
- Extract the list view into its own model (a `listView` struct mirroring
  `detailView`'s shape: owns cursor/rendering/key handling, takes the
  data it needs). App keeps routing, refresh, and cross-view state only.
- Move the render functions that belong to the list (`renderList`,
  `renderJobRow`, `renderRecentActivity`, the column-width helpers, the
  empty-state copy) into it. Keep the shared styles and `clamp`/`truncate`
 /`pad` helpers where they are (they're already shared).
- Do **not** move the JDI bell-dedup/spinner machinery — it is genuinely
  App-level state (cross-view). The goal is a smaller App and a list view
  as testable as the others, not a purist split.
- Existing list tests (`list_test.go`) must pass unchanged — they pin
  rendered output, which is the safety net that this is a pure move.

Files: `internal/ui/app.go`, `internal/ui/list.go` (new), maybe
`internal/ui/list_test.go` additions
Risk: low-medium — pure refactor with strong rendering tests. The risk is
scope creep: resist extracting more than the list.
Verify: `go test ./internal/ui` green with zero test edits (except
additions).

### 2.3 Add context/timeouts to non-interactive subprocesses

Every `exec.Command` and docker run is context-free. Interactive paths are
fine (user-driven), but `git.Push` runs in a `tea.Cmd` goroutine from the
TUI — a stalled network can hang the TUI's command channel forever
(`GIT_TERMINAL_PROMPT=0` covers only the credential case). The jdi loop's
git probes (`HeadCommit`, `CountVerdictCommits`, `LatestCommitIsVerdict`)
are per-iteration and can stall a whole run.

Do:
- Thread `context.Context` through `internal/git`'s exec plumbing
  (variadic `run`/`runEnv` taking an optional context, or a small
  `WithContext` method on a git client struct).
- Apply timeouts to the non-interactive callers: the TUI's push/commit
  cmds and the jdi loop's probes (e.g. 10-30s); the interactive session
  and `mg done`/`mg delete` keep no timeout (user waits on git).
- Surface timeout failures as ordinary wrapped errors, not panics — the
  TUI's `cmdErrorText` path already handles them.

Files: `internal/git/git.go`, `internal/ui/app.go` (push/commit cmds),
`cmd/mg/jdi.go` (probe calls)
Risk: low — new failure mode (timeout) added deliberately; tests that
fake git via PATH must not trip the timeout (they run fast, fine).
Verify: full suite green; a unit test for the timeout path with a
stubbed slow `git`.

### 2.4 Unify argument parsing

Three styles in one binary: `runJDI`/`runTUI` use `flag.FlagSet`;
`session.ParseArgs`, `runJob`, `runInit`, `runSetup`, `runProfiles`
hand-roll loops. Two conventions to learn, two ways for a flag typo to
misbehave.

Do:
- Move the remaining hand-rolled parsers to `flag.FlagSet` (or one shared
  tiny parser if the session passthrough semantics genuinely don't fit
  flags — document which and why).
- Preserve exact behavior: `session.ParseArgs`'s passthrough (`Pass`),
  `runSetup`'s `--check` + profile argument, `runInit`'s `--tool` legacy
  alias mapping. The command tests pin these; they must pass unchanged.

Files: `internal/session/session.go`, `cmd/mg/job.go`, `cmd/mg/init.go`,
`cmd/mg/setup.go`, `cmd/mg/profiles.go`
Risk: low — behavior pinned by tests; mechanical.
Verify: full suite green; `grep` shows one parsing style.

### 2.5 Shared agent-name constants

The agent names are hardcoded in four Go-side places: `ui/agents.go`
(`agentMeta`/`agentOrder`), `orchestrate.Sequence`,
`jdioutput.go`'s `agentTargetFile`, and the key dispatch in `app.go`'s
`agentForKey`. The `agents/*.md` files are necessarily data-driven, but the
Go lists should agree by construction, not by convention — a renamed agent
today silently breaks `mg jdi`'s target-file mapping or the TUI's key
dispatch.

Do:
- One source of truth for the Go-side agent names (constants in
  `internal/orchestrate` or a small `internal/agents` package), with
  `Sequence`/`agentTargetFile`/`agentMeta` keyed off it.
- Keep the mapping of agent → target job file (`agentTargetFile`) next to
  the sequence that uses it.
- Defensive: `agentForKey`/`logInvocation` should already tolerate an
  unknown name gracefully (they do); keep that.

Files: `internal/orchestrate/`, `internal/ui/agents.go`,
`cmd/mg/jdioutput.go`, `internal/ui/app.go`
Risk: low.
Verify: full suite green; renaming a constant in one place breaks the
build (that's the point).

---

## Phase 3 — Documentation and contract

### 3.1 Fix the stale architecture claims

AGENTS.md says `internal/git` is "the only place that shells out to git."
Session and init contradict it today (fixed by 1.2 — but the doc should be
correct *before* the code, or at least in the same job). Same family:
several doc comments describe behavior that has since changed (e.g.
comments citing `finish-job.sh`/`run.sh` as live scripts — they're gone;
`git.Checkout`'s "currently unused" note is honest, keep that pattern).

Do:
- Sweep for stale references to the dead scripts (`run.sh`,
  `new-job.sh`, `finish-job.sh`, `delete-job.sh`, `profiles.sh`,
  `setup.sh`, `init.sh`, `agents.sh`, `tui.sh`, `jdi.sh`,
  `scripts/lib/`) in Go doc comments. The port is done — comments should
  say what the Go does, with at most a one-line "was <script>" note where
  the provenance matters (a few already do this well).
- Where a comment says "kept in sync with <dead script>" (e.g. the
  `legacyOpenCodeKeys` note), verify the sync target still exists
  (`scripts/entrypoint.sh` does) and reword the rest.

Files: `AGENTS.md`, doc comments across `internal/` and `cmd/`
Risk: low — comments only.
Verify: `grep -rn 'run\.sh\|new-job\.sh\|finish-job\.sh\|delete-job\.sh'`
under `internal/ cmd/` returns only provenance notes.

### 3.2 Trim comment bloat; convert archaeology to reasoning

The why-comments are the codebase's best asset and must stay. But many
comments restate the code at length, and several cite job-workflow
archaeology ("see 207bfu_git-worktrees, Decision 2", "per TASK-7 review",
"corrected by TASK-2 of the 'jdi does not work' job"). Nothing enforces
those references; they will rot.

Do:
- Keep every *why*/context comment. Trim the *what*-restating prose (the
  code says what it does; the comment should say why it does it).
- Convert job-ID/brief citations into the plain reasoning they stand for
  ("the retry budget is one bounce" beats "see brief scope item 2"). Where
  the reasoning is genuinely lost, reconstruct it from the code — that's
  part of this task.
- Target the worst offenders first: the 40-60-line doc comments on
  15-line functions (session resolution, the docker argv builder, the
  markdown renderer cache). A good bar: the comment should fit the
  function's complexity, not the history of its changes.
- Taste call, not a rule — some long comments (the tmux argv-vs-string
  explanation, the terminal-probe race) are worth every line. The goal is
  a net *reduction* with *zero* loss of why.

Files: doc comments across `internal/` and `cmd/`
Risk: low (comments only) — but the biggest judgment-call surface in this
whole document. Review-diff it.
Verify: no comment loses a "why"; line counts of the worst offenders drop
meaningfully.

### 3.3 Decide the fate of the "output is the contract" rule

Tests pin byte-exact output (CLI wording, docker argv, TUI render). That
was the right call during the strangler migration — it proved fidelity.
It's now a tax: any wording change costs test churn, and it pins
refactoring (2.1 and 2.2 both brush against it).

Do:
- Make the decision explicit and write it down (in AGENTS.md or this
  doc's conclusion): keep the rule for *user-facing CLI wording* (it's
  the documented contract), but relax it for *internal* shapes where a
  semantic assertion is stronger — e.g. assert on docker argv *contents*
  (presence/order of flags that matter) rather than the full exact vector,
  and on rendered-TUI *substrings* rather than full snapshots where the
  full snapshot only exists to catch regressions.
- Do this *after* Phase 1/2 land, so the consolidation happens under the
  strict regime and the relaxation is a separate, deliberate step.

Files: tests across `cmd/mg`, `internal/session`, `internal/ui`
Risk: medium — relaxing tests reduces the safety net by design; it must be
the last thing, not the first.
Verify: full suite green after relaxation; spot-check that the relaxed
assertions still fail on a deliberate regression.

---

## Phase 4 — Trivia and micro-hardening

Small, independent, low-risk. Pick up when the phases above are done or
as filler tasks:

- **4.1** `randomID` (`internal/job/create.go:261`): modulo bias on
  crypto/rand bytes. Negligible for job IDs, but rejection sampling (or
  `rand.IntN` from `math/rand/v2` with the crypto reader) is the correct
  shape and costs nothing.
- **4.2** `writeScaffold` iterates a map (`internal/job/create.go:369`):
  nondeterministic write order. Harmless (four independent files), but a
  slice of {name, content} pairs is deterministic and equally simple.
- **4.3** `home.Root()` re-runs `os.Executable` + `EvalSymlinks` on every
  call, and `config` calls it on every env read. Cache it (sync.Once) —
  micro-win, removes syscalls from hot-ish paths like `EnvValue` in the
  setup wizard.
- **4.4** The `-e CLAUDE_CODE_OAUTH_TOKEN=...` docker env flags are
  emitted unconditionally even for opencode profiles
  (`internal/session/docker.go:196-199`), passing empty values when the
  key is unset. Emit only the profile's own key set — matches `KeyEnv`'s
  existing pattern, less noise in `docker inspect`.

Files: as listed per item
Risk: negligible.
Verify: full suite green.

---

## Definition of "totally solid" — the end-state invariants

Each one is grep-able or test-checkable, so the next review can verify them
in minutes instead of reading the whole tree:

1. **One shell-out point:** every `exec.Command("git", ...)` in non-test
   code lives in `internal/git`.
2. **One definition per helper:** `branchTail`, `isDir`/`isFile`, the
   `docs/jobs` prefix scan, the verdict-section extraction — each appears
   exactly once.
3. **Errors are domain facts, presentation is CLI's job:** zero
   `fmt.Errorf("Error: ...")` under `internal/`; every `cmd/mg` error
   printer frames through one place; `%w` used wherever the wrapped error
   carries meaning.
4. **Every non-interactive subprocess has a timeout** (and the
   interactive ones are documented as intentionally unbounded).
5. **One arg-parsing style** across all subcommands.
6. **Agent names agree by construction**, not by convention.
7. **Doc comments state why, not what;** no live references to dead
   scripts; AGENTS.md matches the code.
8. **Full suite green after every item**, and coverage does not drop.

When all eight hold, the code matches the documented architecture — which
is the actual definition of solid here, not any particular style choice.
