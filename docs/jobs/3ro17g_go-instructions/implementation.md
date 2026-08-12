# Implementation: go instructions

id: 3ro17g
status: open
developer: @developer
date: 2026-08-12

## Summary

Wrote a full Go tutorial based on the manigot codebase so the user can go
from reading to working on the project themselves. The deliverable is a
10-chapter tutorial living under `docs/tutorial/` (index at
`docs/tutorial/README.md`), with a one-line link added to the root
`README.md`. Every chapter is grounded in real files of this repository
(package/struct/function references were read and verified during
writing); no `.go` files were touched.

Because `tasks.md` was empty when the job was started, the task breakdown
was first produced by the @analyst agent (reflecting the job workflow's
@analyst → @developer sequence) and committed to `tasks.md` before any
implementation.

## Changes

TASK-1: Tutorial entry point — `docs/tutorial/README.md` (prerequisites,
         how to use, chapter index) + `docs/tutorial/01-getting-started.md`
         (`go.mod`, Go toolchain, `Makefile` targets, `cmd/mg/main.go`),
         plus a one-line link from the root `README.md`.
TASK-2: `docs/tutorial/02-go-fundamentals.md` — packages, import paths,
         the `internal/` visibility convention, exported vs unexported
         identifiers, constants, `os.Exit`, the subcommand dispatcher —
         grounded in `internal/fs/fs.go`, `internal/agents/agents.go`,
         `cmd/mg/main.go`.
TASK-3: `docs/tutorial/03-types-and-errors.md` — structs, JSON tags, value
         receivers, the zero value, `%w` wrapping, `os.IsNotExist` degrade
         paths — grounded in `internal/project/settings.go`,
         `internal/config/config.go`.
TASK-4: `docs/tutorial/04-testing.md` — `TestXxx`, table-driven tests,
         subtests, io.Reader/io.Writer injection, regression tests,
         `make check` — grounded in `internal/orchestrate/orchestrate_test.go`,
         `internal/git/branch_test.go`, `internal/cli/cli_test.go`,
         `internal/markdown/markdown_test.go` (all four files verified to
         exist before prose was written).
TASK-5: `docs/tutorial/05-interfaces.md` — interfaces, implicit
         satisfaction, `tea.Model` (`Init`/`Update`/`View` on `App`),
         small interface parameters, sentinel errors + `errors.Is` /
         `errors.As` — grounded in `internal/ui/app.go`,
         `internal/cli/cli.go`, `internal/git/git.go`.
TASK-6: `docs/tutorial/06-shelling-out-git.md` — `os/exec`,
         `exec.CommandContext`, `-C` root, stderr capture via
         `bytes.Buffer`, exit-code-as-data (`WorkingTreeDirty`,
         `RefExists`), `wrapErr`, the single-choke-point design, and the
         no-shell-string injection note — grounded in `internal/git/git.go`.
TASK-7: `docs/tutorial/07-cli-architecture.md` — dispatcher, per-subcommand
         `flag.FlagSet`, `splitFlags`/passthrough, prompt primitives in
         `internal/cli/cli.go`, and the `runX(args, stdin, stdout, stderr)
         int` pattern — grounded in `cmd/mg/main.go`, `cmd/mg/job.go`,
         `cmd/mg/profiles.go`, `cmd/mg/flags.go`,
         `internal/session/session.go`.
TASK-8: `docs/tutorial/08-concurrency-and-context.md` — `sync.Mutex` cache
         guard + the glamour OSC-probe race story (`internal/markdown`),
         `sync.Once` memoization (`internal/home`), context deadlines for
         child processes (`git.go` `runCtx`/`PushWithContext`), Bubble Tea
         background `tea.Cmd` goroutines (`ui/app.go`) and the explicit
         `go func() { _ = cmd.Wait() }()` reap in `internal/launch`.
         Chapter explicitly states there are no raw channel examples in
         this codebase and does not invent any.
TASK-9: `docs/tutorial/09-walking-the-codebase.md` — end-to-end traces of a
         bare `mg` session (ParseArgs → ResolveProfile → ResolveRoot →
         CheckAuth → BuildDockerInvocation → Run) and a job's life
         (CreateJob → agents → orchestrate.Next → FinishJob/DeleteJob),
         with a "mental map" table of which package answers which question.
TASK-10: `docs/tutorial/10-your-turn.md` — four graded exercises (add a
         test case to `cli.Mask`, add an `--author` flag to `mg job`,
         add a `versions` subcommand by copying `profiles.go`, work a real
         job end to end), plus pointers to `docs/AGENTS.md`, `agents/*.md`,
         and `make check` as the verification loop.

Also (pre-implementation): `tasks.md` filled with the @analyst-produced
10-task breakdown, committed as `[3ro17g] tasks: add analyst task
breakdown for Go tutorial`.

## Known issues / follow-ups

- The tutorial is prose/documentation only — none of the exercises were
  executed against the code, so Exercise 2 (adding `--author` to `mg job`)
  has not been validated against the existing `cmd/mg`/`internal/job`
  tests. The chapter points at the right files but a future implementer
  should run the exercises once to confirm the exercise steps stay
  accurate.
- Chapter 05's `tea.Model` interface listing is accurate as of Bubble Tea
  v1.2.4 (the version in `go.mod`); a major Bubble Tea version bump could
  change the interface shape and would need the chapter's example updated.
- The root `README.md` gained only a one-line link to the tutorial, per
  the task's scope; a fuller "Learning Go" section could be a follow-up if
  the tutorial grows.
