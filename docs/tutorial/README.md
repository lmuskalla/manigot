# Learn Go with manigot

A full Go tutorial that teaches the language through the code you already
have here. Every chapter takes a real piece of manigot — a Go codebase of
about 23,000 lines across `cmd/mg` and `internal/*` — and uses it to explain
a Go concept. You finish each chapter with code you can read, run, and
eventually change.

No previous Go experience is assumed. You should be comfortable with the
basic ideas of programming (variables, functions, conditions, loops) in any
language. Some of the later chapters also assume you can use a terminal and
have `git` and `docker` installed.

## How to use this tutorial

- Read the chapters in order. Each one builds on the previous.
- Keep the repository open next to the tutorial. The chapters reference
  files by path — open them and read along.
- Run the commands. Every chapter has commands you can run yourself
  (`go build`, `go test`, `make check`, ...). Doing them is how the
  concepts stick.
- The tutorial never asks you to modify the code while learning. Chapter 10
  is where you get to change things.

## Prerequisites

- **Go 1.23+** (this project's `go.mod` says `go 1.23`) — install from
  [go.dev/dl](https://go.dev/dl).
- **git** — the project's git integration is itself a tutorial chapter.
- **Docker** — only needed for the parts that build the container image
  (`make build`). Reading and building the `mg` binary itself only needs Go.
- A terminal you're comfortable with.

## Chapter index

| Chapter | Topic | Teaches |
|---|---|---|
| [01 — Getting started: build & run manigot](01-getting-started.md) | What this project is, `go.mod`, the Go toolchain, `Makefile` targets | Running a Go project, the build toolchain |
| [02 — Go fundamentals](02-go-fundamentals.md) | Packages, imports, the `internal/` convention, exported names, `os.Exit`, the subcommand dispatcher | How a Go program is structured |
| [03 — Types, structs & errors](03-types-and-errors.md) | Structs, JSON tags, value receivers, the zero value, error wrapping | Defining data and handling failure |
| [04 — Testing](04-testing.md) | `testing` package, table-driven tests, subtests, dependency injection | Writing tests Go's way |
| [05 — Interfaces & idiomatic Go](05-interfaces.md) | Interfaces, small interface types, sentinel errors, `errors.Is`/`errors.As` | Designing with interfaces |
| [06 — Shelling out: the git package](06-shelling-out-git.md) | `os/exec`, `CommandContext`, stderr capture, exit codes | Running other programs |
| [07 — CLI architecture](07-cli-architecture.md) | Subcommand dispatch, `flag.FlagSet`, prompts, the `runX(args, stdin, stdout, stderr)` pattern | Building a command-line tool |
| [08 — Concurrency & context](08-concurrency-and-context.md) | `sync.Mutex`, `sync.Once`, `context`, goroutines via Bubble Tea | Concurrency as it's actually used here |
| [09 — Walking the codebase](09-walking-the-codebase.md) | End-to-end trace of `mg` and the job lifecycle | Reading an unfamiliar Go codebase |
| [10 — Your turn: contributing](10-your-turn.md) | Concrete exercises to change manigot yourself | Putting it all together |

## How this project is organized (30 seconds)

```
cmd/mg/        the single `mg` binary: main.go dispatches to subcommands
internal/      the logic, as importable packages (session, job, git, ui, ...)
scripts/       the one shell script that runs inside the Docker container
Dockerfile     builds the container image the sessions run in
Makefile       build / install / check targets
docs/          all documentation, including this tutorial
```

The rest of the tutorial explains what each of these does and why.
