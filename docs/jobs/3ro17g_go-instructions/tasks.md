# Tasks: go instructions

id: 3ro17g
status: open
analyst: @analyst
date: 2026-08-12

## Task breakdown

The deliverable is a full Go tutorial written in Markdown, grounded in this
codebase (manigot), living under `docs/tutorial/` — one file per chapter plus
a `README.md` index. The user wants to go from reading to being able to work
on the project themselves.

TASK-1: Write the tutorial entry point — `docs/tutorial/README.md` (how to use the tutorial, prerequisites, chapter index with links) plus chapter 01 "Getting started: build & run manigot", explaining `go.mod` (module path, go 1.23, dependency classes), the Go toolchain commands behind `Makefile` targets (`go build ./cmd/mg`, `make mg`, `make check` = `go vet` + `go test`), and what `mg` does when run; add a link from the root README to the tutorial.
     files: docs/tutorial/README.md, docs/tutorial/01-getting-started.md, README.md (one-line link)
     depends: none
     risk: low — pure documentation grounded in go.mod/Makefile/README, nothing behavioral

TASK-2: Write chapter 02 "Go fundamentals through this repo": packages and `package` clauses, import paths (`github.com/lmuskalla/manigot/internal/...`), the `internal/` visibility convention, exported vs unexported identifiers, constants, `os.Exit` and the single-binary subcommand dispatcher in `cmd/mg/main.go`.
     files: docs/tutorial/02-go-fundamentals.md
     depends: TASK-1
     risk: low — grounded in small, well-commented files (internal/fs/fs.go, internal/agents/agents.go, main.go)

TASK-3: Write chapter 03 "Types, structs & errors": structs with JSON tags, value-receiver methods, zero-value-as-defaults philosophy, and idiomatic error handling (`%w` wrapping, `os.IsNotExist` degrade paths), using `internal/project/settings.go` and `internal/config/config.go` as the worked examples.
     files: docs/tutorial/03-types-and-errors.md
     depends: TASK-2
     risk: low — both source files are self-contained and heavily documented

TASK-4: Write chapter 04 "Testing with Go's testing package": basic `TestXxx` functions, table-driven tests with subtests, and the io.Reader/io.Writer injection pattern that makes the `cli` package testable — grounded in `internal/orchestrate/orchestrate_test.go`, `internal/markdown/markdown_test.go`, `internal/git/branch_test.go`, `internal/cli/cli_test.go`, and how `make check`/`go vet` fit in. Every referenced test file must be verified to exist before prose is written.
     files: docs/tutorial/04-testing.md
     depends: TASK-3
     risk: low — test files are concrete and easy to quote; only risk is describing a test that doesn't exist, mitigated by verification

TASK-5: Write chapter 05 "Interfaces & idiomatic Go": the `tea.Model` interface (`Init`/`Update`/`View` on `App` in `internal/ui/app.go`), interface-valued parameters (`io.Reader`/`io.Writer` in `internal/cli/cli.go`), sentinel errors + `errors.Is`/`errors.As` (`ErrNotARepo`, `exec.ExitError` in `internal/git/git.go`).
     files: docs/tutorial/05-interfaces.md
     depends: TASK-4
     risk: medium — app.go is the largest file in the repo (1154 lines) and Bubble Tea's `tea.Cmd` semantics are subtle; the author must read it carefully and not overgeneralize

TASK-6: Write chapter 06 "Shelling out: the git package": `os/exec`, `exec.CommandContext`, capturing stderr via `bytes.Buffer`, exit-code handling, error wrapping with `wrapErr`, and why `internal/git/git.go` is deliberately the single choke point that shells out.
     files: docs/tutorial/06-shelling-out-git.md
     depends: TASK-5
     risk: low — git.go is extremely well-commented and reads almost like prose already

TASK-7: Write chapter 07 "CLI architecture": the `switch` dispatcher in `cmd/mg/main.go`, per-subcommand files, `flag.FlagSet` + custom passthrough splitting (`session.ParseArgs`/`splitFlags`), the interactive prompt primitives in `internal/cli/cli.go`, and the `runX(args, os.Stdin, os.Stdout, os.Stderr)` dependency-injection pattern that makes CLI commands testable.
     files: docs/tutorial/07-cli-architecture.md
     depends: TASK-5
     risk: low — grounded in main.go/flags.go/profiles.go/cli.go, all compact and documented

TASK-8: Write chapter 08 "Concurrency & context": the concurrency actually used in this codebase — `sync.Mutex` cache guard in `internal/markdown/markdown.go` (incl. the glamour OSC-probe race story), `sync.Once` memoization in `internal/home/home.go`, context deadlines for child processes (`runCtx`/`WithContext` variants in `internal/git/git.go`), and Bubble Tea's background `tea.Cmd` goroutines in `internal/ui/app.go` and `internal/launch/launch.go`. Must be honest that there is no raw channel example here; do not invent patterns.
     files: docs/tutorial/08-concurrency-and-context.md
     depends: TASK-6
     risk: medium — concurrency is the sparsest and most subtle area of the codebase; the markdown race explanation is intricate and easy to get wrong

TASK-9: Write chapter 09 "Walking the codebase": an end-to-end trace of a bare `mg` session (root resolution → profile resolution → auth → docker argv build → run) and the job lifecycle (`mg job` → worktree creation → agents → `mg done`/`mg delete`, plus the `mg jdi` state machine), following the actual call paths across `internal/session/{root,session,docker}.go`, `internal/job/*.go`, `internal/orchestrate/orchestrate.go` and the corresponding `cmd/mg/*.go` wrappers.
     files: docs/tutorial/09-walking-the-codebase.md
     depends: TASK-7
     risk: medium — largest scope; the git-worktree model and the `--job` root-reassignment are genuinely tricky concepts that must be described accurately

TASK-10: Write chapter 10 "Your turn: contributing to manigot": concrete next-step exercises grounded in the repo (add a test to an existing package, add a new `--flag` to an existing subcommand, add a new subcommand by copying an existing one, create and work a real job via `mg job`), plus pointers to `docs/AGENTS.md`, `agents/*.md`, the job workflow, and `make check` as the verification loop.
     files: docs/tutorial/10-your-turn.md
     depends: TASK-9
     risk: low — exercises are suggestions, not code changes; no behavioral risk to the repo

All tasks are documentation-only: none touch `internal/`, `cmd/`, or any `.go`
file — the deliverable lives entirely under `docs/tutorial/` plus a one-line
README cross-link.
