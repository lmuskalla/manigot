# Verdict: fully autonomous mode

id: vu33rn
status: open
reviewer: @reviewer
date: 2026-08-09

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `docs/AGENTS.md` gets the one-bullet architecture note defining the
`NEEDS-HUMAN-INPUT:` marker as a `--print`-path prompt addition, not a rule
in `agents/*.md`. `agents/analyst.md`/`developer.md`/`reviewer.md` and
`project-template/docs/AGENTS.md` are correctly untouched (verified via
`git diff main...HEAD -- agents/ project-template/` — empty).

TASK-2: PASS
notes: `scripts/run.sh` adds `--print` (drops `-it`, rejects `--tool
opencode` with a clear error, redirects its own diagnostics to fd 3 so
`--print`'s stdout is exactly the agent's output, appends the
`NEEDS-HUMAN-INPUT:` sentence to `JOB_PROMPT` only when `--print`+`--job`).
`scripts/entrypoint.sh` execs `claude --dangerously-skip-permissions --print
--output-format json` under `MANIGOT_PRINT=true`, interactive path
unchanged. Matches TASK-1's convention exactly.

TASK-3: PASS
notes: `tui/internal/git/git.go` — `CountVerdictCommits`,
`LatestCommitIsVerdict`, and `HeadCommit` (added while wiring TASK-6, as
documented) all degrade gracefully (non-repo, missing branch, unparseable
message) per their own doc comments; well covered in `git_test.go`
(zero/one/many commits, unparseable message, missing branch, not-a-repo for
each).

TASK-4: PASS
notes: `tui/internal/orchestrate/orchestrate.go`'s `Next` matches the
decision table in tasks.md exactly, including the `StageImplement` tip-flag
disambiguation. Table-tested against every stage and the exact 0/1/≥2
verdict-round boundaries, plus a guard (`TestNextStagesCoverEveryDefinedStage`)
against a future `job.Stage` silently falling into the default case.

TASK-5: PASS
notes: `tui/internal/orchestrate/signal.go`'s `DetectSignal` is anchored,
case-sensitive, prefers the `--output-format json` `"result"` field to avoid
false-positiving on incidental tool-call output, and falls back to raw text.
Well covered (JSON vs. plain text, no-match, empty input, mid-output, case/
anchor sensitivity, empty reason).

TASK-6: PASS
notes: `tui/cmd/jdi/main.go`'s `Run` loop matches the spec: re-derives
`Stage()`/verdict-round state every iteration, checks off-branch checkout
before looping (`ensureOnBranch`), the stall backstop compares Stage+HEAD
before/after each invocation and only trips on a *second* consecutive no-op
for the same agent, `maxIterations` is a documented independent backstop.
`main_test.go` exercises a real temp git repo end to end: happy path,
one-bounce-then-approved, budget-exhausted (asserts no third developer
bounce and exactly 2 verdict commits), the marker, the stall backstop, and a
runner error — matches the task's required coverage list exactly.

TASK-7: PASS
notes: `tui/cmd/jdi/output.go`'s `logInvocation` fans out to the
`io.MultiWriter` built in `main()` (stdout + `run.log`), writes the
extracted final-response text (not raw JSON), and the honesty caveat about
`--print` only returning final-response text is carried into
`entrypoint.sh`'s comment, `output.go`'s doc comment, and README/AGENTS.md
(TASK-13). `run.log` is append-only across runs (`openRunLog`, tested).

TASK-8: PASS
notes: `tui/internal/job/jdistatus.go` — `JDIState`/`WriteJDIStatus`/
`ReadJDIStatus`, with the 30-minute staleness degrade for `running` (never
for a terminal `stopped:*` status) exactly as specified. `renderJobRow`
gets the `[mg-jdi: ...]` badge. `docs/jobs/.jdi-status/` added to manigot's
own `.gitignore`. Well covered in `jdistatus_test.go` and `list_test.go`.

TASK-9: PASS
notes: `detail.go` gets a fifth `isLog` tab (key `5`) reading
`job.ReadJDIRunLogTail` (256 KiB tail, not load-everything, with a
truncation banner), a distinct no-run-yet placeholder, and is never
editable. Covered in `detail_test.go`.

TASK-10: PASS
notes: `scripts/jdi.sh` mirrors `scripts/tui.sh` exactly; `make jdi` and the
`mg-jdi:jdi.sh` `LINKS` entry added to the `Makefile`.

TASK-11: PASS
notes: CLI path rings `\a` unconditionally after `Run` returns
(`main.go`). TUI path: `app.go`'s `pollJDIBell`, called from `refreshJobs`,
dedups via `a.jdiSeen` keyed by job name — first observation of any job only
seeds the map (never rings), a later stopped-state transition rings once,
repeated polls of the same stopped state don't re-ring. Covered thoroughly
in `refresh_test.go` (first-observation no-ring, single ring on transition,
no re-ring on repeat poll, no-status no-op).

TASK-12: PASS
notes: `resolve.Jdi()` (`MANIGOT_JDI_BIN`), `launch.Jdi` starts `mg-jdi
--job <id>` via `cmd.Start()` with no terminal emulator at all and
async-reaps the process. Detail view's `J` key goes through the same
`branchGuard` as every other mutating action (tested in
`branchguard_test.go`), seeds `a.jdiSeen` as running immediately on launch
(tested in `jdilaunch_test.go`, including the resolution-failure path).
Action bar and footer hint updated.

TASK-13: PASS
notes: `README.md` gets a full "Autonomous mode (`mg-jdi`)" section (sequence,
retry bound, marker, honesty caveat) and an "mg-jdi status & log" section
(badge, log tab, two-path notification), plus keybindings/installed-commands/
file-tree updates. `docs/AGENTS.md` (repo root) updated at the same
one-bullet-per-script granularity as the rest of the file.
`project-template/docs/AGENTS.md` correctly left untouched.

TASK-14: PASS
notes: `go build ./...`, `go vet ./...`, and `go test ./...` all clean,
verified independently during this review (also `gofmt -l .` clean). The
real bug found during manual verification — the sidecar leaking into a
target project's own `git add -A` because only manigot's own `.gitignore`
excluded it, not the driven project's — is fixed via `ensureSidecarIgnored`
(`tui/cmd/jdi/output.go`, called once at `mg-jdi` startup, idempotent,
appends to `.git/info/exclude` rather than mutating the tracked
`.gitignore`), with a real regression test (`TestEnsureSidecarIgnoredActuallyWorksWithGit`)
that runs a real `git add -A` and asserts the sidecar never gets staged
while a real job file still does. Documented in both `tasks.md`'s Finding
and `implementation.md`'s Known issues, consistent with the "fixed, not just
noted" framing.

## Security

None run — @security is explicitly out of scope for this job's driven
sequence and was not invoked separately. Nothing in this diff stood out as
security-sensitive beyond what a normal review already covers: all new
`exec.Command` invocations (`tui/cmd/jdi/main.go`'s `commandAgentRunner`,
`tui/internal/launch/launch.go`'s `Jdi`) pass arguments as separate argv
entries, not through a shell, so there's no injection surface from job
names/agent names. `ensureSidecarIgnored` only ever appends to
`.git/info/exclude`, never the tracked `.gitignore`, matching its own stated
intent of staying local-only.

## Overall

APPROVED

All 14 tasks are implemented as specified in tasks.md, each with its own
correctly-formatted commit (`[vu33rn] TASK-N: ...`), plus a separate
`implementation: add summary` commit. `go build`/`go vet`/`go test ./...`
and `gofmt -l .` are all clean under `tui/`. Test coverage is thorough and
targeted at the actual risk areas the task breakdown itself flagged (the
one-bounce boundary, the stall backstop, the marker's JSON-vs-plain-text
false-positive risk, the sidecar-leak regression). Scope matches the brief:
`agents/*.md` and `project-template/docs/AGENTS.md` are untouched,
`@product-owner`/`@security` are not added to the driven sequence, nothing
auto-merges, and OpenCode is cleanly rejected rather than silently
mishandled.

One pre-existing, out-of-band commit on this branch (`3365d32 "Fix for
ptyxis"`, authored by the human author before `@analyst`'s breakdown even
started) touches `tui/internal/launch/launch.go` and `README.md` to add
`ptyxis` terminal-emulator support and isn't part of any task or
`implementation.md` entry — noted here for the record since it shows up in
`git diff main...HEAD`, but it predates and is unrelated to the developer's
own work on this job, so it is not a blocker.

No changes required before merge.
