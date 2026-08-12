# 01 — Getting started: build & run manigot

> Teaches: running a Go project, `go.mod`, the Go build toolchain, and the
> `Makefile` targets that wrap it. Grounded in: `go.mod`, `Makefile`,
> `cmd/mg/main.go`, `README.md`.

Before any Go code, let's get the project building and running. You'll meet
the two things every Go project has — a module and a build toolchain — and
see how this project's `Makefile` wraps them.

## What is this project?

manigot is a tool that runs AI coding agents in an isolated Docker
container per project. It is written almost entirely in Go:

- one Go binary, `mg`, is the entire command-line tool (starting sessions,
  managing jobs, a terminal UI);
- a single bash script (`scripts/entrypoint.sh`) runs inside the container;
- the `Dockerfile` builds that container image.

Everything else — the ~23,000 lines of Go under `cmd/` and `internal/` — is
what this tutorial teaches you to read and eventually write.

## `go.mod` — the module file

Every Go project is a **module**: a named collection of packages with a
declared Go version and a list of dependencies. The module file is
`go.mod` at the repository root:

```go
module github.com/lmuskalla/manigot

go 1.23

require (
	github.com/charmbracelet/bubbles v0.20.0
	github.com/charmbracelet/bubbletea v1.2.4
	github.com/charmbracelet/glamour v0.8.0
	github.com/charmbracelet/lipgloss v1.0.0
	golang.org/x/term v0.22.0
)
```

Three things to notice:

1. **`module`** is the module's name — the import path that all of its
   packages share as a prefix. Any package inside this module is imported as
   `github.com/lmuskalla/manigot/<path>`, e.g. `github.com/lmuskalla/manigot/internal/session`.
   Chapter 02 covers imports in detail.
2. **`go 1.23`** declares the Go version the code is written against. If you
   run a different version, the toolchain tells you instead of silently
   behaving differently.
3. **`require`** lists the module's direct dependencies (the second
   `require` block lists *indirect* ones — dependencies of dependencies).
   All of them are Charmbracelet libraries (terminal UI: Bubble Tea,
   Lip Gloss, Glamour) plus `golang.org/x/term`. `go.sum` holds
   cryptographic checksums of these dependencies so builds are reproducible.

`go.mod` and `go.sum` are the first files you look at in any unfamiliar Go
project: they tell you its name, its Go version, and what it depends on.

## The Go toolchain: build and test

Go ships one toolchain with subcommands, most relevantly `go build`, `go
test`, `go vet`, and `go run`. All of them take a package path or a file
list:

```bash
# Build the mg binary into ./mg (named after the package directory)
go build ./cmd/mg

# Build to a specific output path
go build -o bin/mg ./cmd/mg

# Compile everything and check it's all valid
go build ./...

# Run every test in the module
go test ./...

# Static analysis: catch suspicious constructs
go vet ./...
```

The `./...` pattern means "this directory and everything below it", so `go
build ./...` builds every package in the module — the compiler checks that
all of it type-checks and compiles, even if you only care about one binary.

## The `Makefile` — wrapping the toolchain

manigot's `Makefile` wraps the raw `go` commands so you don't have to
remember them. The targets you'll use most:

| target | runs | used for |
|---|---|---|
| `make mg` | `go build -trimpath -ldflags "-X main.version=..." -o bin/mg ./cmd/mg` | build the host-side `mg` binary into `bin/mg` |
| `make check` | `go vet ./...` + `go test ./...` | the verification loop — always run this before committing |
| `make build` | `docker build -t manigot .` | build the container image (needs Docker) |
| `make install` | symlink `bin/mg` to `/usr/local/bin/mg` | put `mg` on your PATH |

The `ldflags` in `make mg` are worth a look — they inject the version
string at build time:

```make
-ldflags "-X main.version=$(VERSION) -X main.tuiVersion=$(VERSION) -X main.jdiVersion=$(VERSION)"
```

`-X main.version=...` tells the linker to set the value of the package
variable `main.version` (declared in `cmd/mg/main.go` as
`var version = "0.1.0-dev"`) to the value of `$(VERSION)` — which comes from
`git describe`. Same binary source, version baked in at build time. This is
a common Go technique for build-time metadata (version, commit hash, build
date).

Try it now:

```bash
make mg          # builds bin/mg
./bin/mg --help  # prints usage — a real working Go binary
make check       # go vet + go test — everything should pass
```

## What `mg` does when you run it

Run `./bin/mg` with no arguments and it starts a manigot session in the
current directory. The entry point is `cmd/mg/main.go` — 108 lines, the
smallest and most important file in the repo. It does three things:

1. `home.Seed()` — makes sure `$MANIGOT_HOME` points at this checkout so
   config and agent files resolve correctly no matter where the binary was
   invoked from.
2. `args := os.Args[1:]` — `os.Args` is a `[]string` of everything on the
   command line; `os.Args[0]` is the program name, so `[1:]` is the actual
   arguments.
3. A `switch` on the first argument that dispatches to a subcommand:
   `profiles`, `setup`, `agents`, `job`, `tui`, `jdi`, `done`, `delete`,
   `init` — or, for anything else (including nothing), starts a session.

Each branch calls a `runX` function and passes its exit code to
`os.Exit`. The pattern `os.Exit(runSession(args, os.Stdin, os.Stdout,
os.Stderr))` shows two Go idioms you'll see everywhere in this codebase:

- **Explicit dependency injection** — `runSession` receives the standard
  streams as parameters rather than reading `os.Stdin` directly. That makes
  it testable: a test can pass a `strings.Reader` and a `bytes.Buffer`
  instead of real terminal streams. Chapter 04 and 07 dig into this.
- **Explicit exit codes** — the `runX` functions return an `int` (0 for
  success, non-zero for failure) and `main` translates it to the process
  exit status with `os.Exit`.

Don't worry if the details of each subcommand are unclear yet — chapter 07
walks the whole dispatcher, and chapter 09 traces a session end to end.

## What you should take away

- A Go project is a **module** (`go.mod`): name, Go version, dependencies.
- `go build`, `go test`, `go vet` are the daily commands; `./...` means
  "everything".
- `make mg` builds this project's binary; `make check` is its verification
  loop.
- The whole tool is one binary whose `main()` dispatches to subcommands —
  a very common Go CLI shape, and the shape of everything to come in this
  tutorial.

**Next:** [02 — Go fundamentals](02-go-fundamentals.md) — packages, imports,
the `internal/` convention, and exported vs unexported names.
