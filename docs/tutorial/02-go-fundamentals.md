# 02 — Go fundamentals through this repo

> Teaches: packages, import paths, the `internal/` convention, exported vs
> unexported identifiers, constants, and how `main()` dispatches.
> Grounded in: `internal/fs/fs.go`, `internal/agents/agents.go`,
> `cmd/mg/main.go`.

Every Go program is a collection of **packages**. If you understand
packages, imports, and the naming rules, you can read any Go file. This
chapter uses three small real files from this repo to teach exactly that.

## Packages: the file header

Every `.go` file starts with a `package` clause. Look at the first lines of
`internal/fs/fs.go`:

```go
// Package fs holds the tiny filesystem predicates shared across the host-side
// packages (session, job, agentlist, cmd/mg) — each used to have its own
// one-line isDir/isFile copy. One definition per predicate, one place to
// change the semantics.
package fs

import "os"
```

Three things to note:

1. **`package fs`** — all `.go` files in the same directory belong to the
   same package and share one namespace. A directory of files all declaring
   `package fs` is a package; other directories declaring `package session`,
   `package git`, etc. are separate packages.
2. The **comment above `package`** is the package doc comment. `go doc` (and
   editors) show it as the package's documentation. This codebase documents
   every package this way, and the comments are unusually honest about
   *why* — read them.
3. **`import "os"`** — only what's used gets imported. Go refuses to compile
   an unused import, which keeps files tidy.

A rule of Go naming that shows up immediately: **`fs` is the package name,
`fs.IsDir` is how other packages call it.** Package names are short (one or
two words), all lowercase, and the package's purpose is carried in the
caller-side name.

## The `internal/` convention

Now look at where these packages live on disk:

```
internal/
  fs/         package fs
  agents/     package agents
  git/        package git
  session/    package session
  ...
cmd/
  mg/         package main
```

The directory named `internal/` is special. Go's rule: **packages under an
`internal/` directory can only be imported by code rooted at the directory
that contains that `internal/`.** Here, `internal/` sits at the module root,
so only this module's own packages can import `github.com/lmuskalla/manigot/internal/...` —
nobody outside the module can. It's an *enforced* visibility boundary, not
a convention: someone importing `manigot/internal/session` from another
module gets a compile error.

Why does manigot use it? The `internal/` packages are the tool's guts —
session launching, git operations, the UI. They're not a public API anyone
else should depend on. `internal/` lets the maintainers change them freely
without breaking outside consumers. (The one thing that is public — the
`mg` binary — lives in `cmd/mg`.)

## Exported vs unexported: capitalization is the visibility rule

Go has no `public`/`private` keywords. Visibility is decided by the first
letter of an identifier's name:

- **Uppercase** = exported = visible outside the package.
- **Lowercase** = unexported = visible only inside the package.

`internal/fs/fs.go` is a perfect example:

```go
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
```

`IsDir` and `IsFile` start with uppercase `I`, so any package can call
`fs.IsDir("docs")`. If a helper were lowercase — say `isDir` — only code
inside `package fs` could use it. Note the package doc comment in that file
explains exactly why these two exist: every package used to have its own
one-line `isDir`/`isFile` copy, so they were centralized — "one definition
per predicate, one place to change the semantics."

The same rule applies to everything: functions, types, struct fields,
constants, variables. One consequence: **an unexported identifier is
reusable.** Every package may have its own `func main`, its own `err`
variable, its own `init` helper — they never collide because they're
package-scoped and invisible outside.

## Constants: `internal/agents/agents.go`

The whole of `internal/agents/agents.go` is a good study in Go idioms:

```go
package agents

// The agent names, as launched with --agent <name>.
const (
	Analyst   = "analyst"
	Developer = "developer"
	Reviewer  = "reviewer"
	Owner     = "owner"
	Security  = "security"
)
```

- **A `const` block** groups related constants. `const` values are
  compile-time; they can't change at runtime.
- **Exported constants** (`Analyst`) give other packages a name for a value
  that is otherwise a magic string. Elsewhere in the code the TUI and
  mg-jdi use `agents.Analyst` instead of writing `"analyst"` by hand.
- The package comment explains the *design*: the agent names appear in many
  places, so they're defined once, by construction, instead of by
  convention — a renamed agent then breaks the build everywhere it's
  referenced rather than silently breaking behavior.

This is idiomatic Go in miniature: small packages, exported names for
anything shared, and a doc comment that says why the package exists.

## `main`: the program starts here

A Go binary is just a package named `main` with a `func main()`.
`cmd/mg/main.go` is that function. The interesting part:

```go
func main() {
	home.Seed()

	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(runSession(args, os.Stdin, os.Stdout, os.Stderr))
	}

	switch args[0] {
	case "-h", "--help", "help":
		printHelp()
		os.Exit(0)
	case "profiles":
		os.Exit(runProfiles(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin)))
	case "setup":
		os.Exit(runSetup(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin)))
	case "agents", "crew":
		os.Exit(runAgents(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin)))
	case "job":
		os.Exit(runJob(args[1:], os.Stdout, os.Stderr))
	// ...
	default:
		os.Exit(runSession(args, os.Stdin, os.Stdout, os.Stderr))
	}
}
```

Go concepts on display:

- **`os.Args`** is a `[]string` — the command line. `os.Args[0]` is the
  program name; `[1:]` is a slice containing the rest. Slicing `[1:]`
  means "from index 1 to the end".
- **`switch args[0]`** — Go's `switch` has no `break` needed; each case
  ends at the next `case`, not at a `break` statement. Multiple values per
  case (`case "agents", "crew":`) is idiomatic.
- **`os.Exit(code)`** ends the process immediately with the given exit
  status. Convention: `0` success, non-zero failure. Every branch converts
  its `runX` result into the process exit code.
- **Calling other packages** — `home.Seed()` and `cli.IsTerminal(...)` are
  calls into `internal/home` and `internal/cli`. That's what exported names
  are for.

The `default` case is worth noticing: `mg` with any *unknown* first
argument — including no argument at all — starts a session, because the
session launcher accepts flags like `--profile zai` that must not be
consumed by the switch. The first argument isn't a command name; it's data.

## Exercise

Read `internal/fs/fs.go` and `internal/agents/agents.go` — both are under
20 lines — and answer:

1. Why is the function `IsDir` (uppercase) rather than `isDir`?
2. What happens if you try to import `github.com/lmuskalla/manigot/internal/session`
   from a *different* module? (Try it in a scratch directory if you like.)
3. In `cmd/mg/main.go`, what does the `default:` case do, and why is it
   structured that way?

**Next:** [03 — Types, structs & errors](03-types-and-errors.md) — the data
structures that carry manigot's configuration, and how Go handles failure.
