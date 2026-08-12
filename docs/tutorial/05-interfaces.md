# 05 — Interfaces & idiomatic Go

> Teaches: interfaces, implicit satisfaction, small interface types,
> sentinel errors, `errors.Is` / `errors.As`. Grounded in:
> `internal/ui/app.go` (the `tea.Model` interface), `internal/cli/cli.go`
> (`io.Reader`/`io.Writer`), `internal/git/git.go` (`ErrNotARepo`,
> `*exec.ExitError`).

Interfaces are Go's tool for abstraction, and they work differently from
most languages: a type **satisfies an interface implicitly** — it just has
to have the right methods, with no `implements` declaration anywhere. This
chapter shows the three ways manigot uses interfaces, each time with a real
example you can read in the code.

## What an interface is

An interface is a set of method signatures — a *contract*. Any type that
has those methods satisfies it automatically. The classic one, from the
standard library, is `io.Reader`:

```go
type Reader interface {
	Read(p []byte) (n int, err error)
}
```

Any type with a `Read(p []byte) (n int, err error)` method *is* an
`io.Reader`. `*os.File` is one, `strings.Reader` is one, a network
connection is one. You never see "implements io.Reader" written anywhere;
the relationship is structural.

## A big interface: `tea.Model`

The manigot TUI is built with Bubble Tea, a Go library whose whole model is
one interface. From `internal/ui/app.go`, the `App` type is the TUI's root:

```go
// App is the root Bubble Tea model.
type App struct {
	root  string
	jobs  []job.Job
	state appState
	// ...
}
```

Bubble Tea's `tea.Model` interface (from the library) is:

```go
type Model interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
	View() string
}
```

`App` satisfies it by having those three methods:

```go
// Init starts the program. No initial commands are needed — except the
// activity-indicator tick chain when an mg-jdi run is already active at
// startup ...
func (a *App) Init() tea.Cmd {
	return a.startSpinnerIfRunning()
}

// Update handles window resizing and routes key presses to the active view.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// remember the new size, resize the overlays
		// ...
	case editorDoneMsg:
		// a background editor process finished
		// ...
	}
	// ...
}

// View renders the current state as a string.
func (a *App) View() string {
	var content string
	switch a.state {
	case stateDetail:
		content = a.detail.render()
	case stateNewJob:
		content = a.newJob.render()
	case stateSettings:
		content = a.settingsView.render()
	// ...
	default:
		content = a.list.render(...)
	}
	return uiPaddingStyle.Render(content)
}
```

The Bubble Tea runtime (the library, in `main`'s `tea.NewProgram(&app{...})`)
drives the whole UI through this one interface: it calls `Init` once at
startup, then loops forever calling `Update` with every event (keypress,
window resize, timer tick, background-process result) and `View` to redraw.
`App` never knows or cares how the terminal works — it just implements the
three-method contract.

The design is worth pausing on:

- **State is data, UI is a function of it.** `View()` has no side effects;
  it just renders `a`'s current fields to a string. Every event goes into
  `Update`, which returns the new state. This "state in, string out" model
  is why the whole TUI is unit-testable (chapter 04).
- **The `tea.Msg` type switch is the event loop.** `Update` does
  `switch msg := msg.(type)`, a **type switch** — Go's way of dispatching
  on an interface's dynamic type. Each `case` handles one event type
  (`tea.WindowSizeMsg`, `editorDoneMsg`, ...). Notice `editorDoneMsg` etc.
  are tiny structs defined in the same file — *events are data*, defined
  next to the code that handles them.
- **One method can be added without touching the interface.** Because
  satisfaction is structural, `App` could grow a hundred helper methods and
  `tea.Model` still means "these three methods exist".

## Small interfaces: `io.Reader` / `io.Writer` as parameters

The flip side of a big interface is a tiny one used as a parameter type.
Chapter 04 showed `Confirm(prompt string, in io.Reader, out io.Writer)` —
the parameter types *are* interfaces, and that's what makes the function
testable. `internal/cli/cli.go` spells out the philosophy in its package
comment:

> Every prompt takes an io.Reader for input and an io.Writer for output, so
> tests can drive them with a strings.Reader and assert on a strings.Builder.

The Go convention visible here: **accept the smallest interface you need.**
`Confirm` doesn't need a file, a terminal, or a buffer — it needs
*something it can read a line from* and *something it can write a prompt
to*. `io.Reader`/`io.Writer` are the smallest contracts that cover that.
Production passes `os.Stdin`/`os.Stdout`; tests pass
`strings.Reader`/`strings.Builder`; nobody has to change `Confirm`.

This "accept interfaces, return concrete types" rule is one of the most
quoted bits of Go advice, and this codebase follows it consistently — look
at the `runX(args, os.Stdin, os.Stdout, os.Stderr)` calls in
`cmd/mg/main.go` from chapter 01: the streams are passed as
`*os.File` at the top, but the functions underneath accept them as
`io.Reader`/`io.Writer` where they only need the narrow contract.

## Sentinel errors: errors as values you can compare

An error in Go is just a value. The simplest meaningful error is a
package-level variable — a **sentinel** — created once:

```go
// internal/git/git.go
// ErrNotARepo is returned when root is not inside a git repository or the
// git binary itself cannot be found. Callers that want to degrade
// gracefully ... test for it via errors.Is.
var ErrNotARepo = errors.New("not a git repository (or git not installed)")
```

`internal/cli/cli.go` has another:

```go
// ErrQuit is returned by Select when the user answers q/quit/exit, ...
var ErrQuit = errors.New("quit")
```

Sentinel errors let callers distinguish *kinds* of failure with `errors.Is`
instead of string matching:

```go
// internal/git/git.go
if notARepo(stderr, err) {
	return nil, ErrNotARepo
}
```

and from `internal/job/delete.go`:

```go
if errors.Is(err, git.ErrNotARepo) {
	// not a git project — treat as a plain directory delete
}
```

Why `errors.Is` and not `err == ErrNotARepo`? Because errors get **wrapped**
(chapter 03: `fmt.Errorf("%s: %w", path, err)`). `errors.Is(err, target)`
walks the whole chain of wrapped errors and reports whether *any* layer is
the target — so a wrapped `ErrNotARepo` still matches. `errors.Is` is the
Go-idiomatic "is this the kind of failure I care about?" test, and it's
everywhere in this codebase: every git test asserts with
`errors.Is(err, ErrNotARepo)`, every prompt test with `errors.Is(err, ErrQuit)`.

## `errors.As`: matching a *type* of error

`errors.Is` matches a specific sentinel value. `errors.As` matches a
*type* of error — it's for when the type itself carries information. From
`internal/git/git.go`, detecting a detached HEAD:

```go
// An empty stderr with a non-zero exit is the detached-HEAD case:
// --quiet suppressed the message, so treat it as "no branch".
var exitErr *exec.ExitError
if errors.As(err, &exitErr) && strings.TrimSpace(stderr) == "" {
	return "", nil
}
```

`exec.Command` returns an `*exec.ExitError` when the child process exits
non-zero. The code uses `errors.As(err, &exitErr)` to ask "is this error an
`*exec.ExitError`?" and, if so, binds `exitErr` to it — with access to
`exitErr.ExitCode()` if needed. The pattern's shape is
`errors.As(err, &target)` where `target` is a pointer to the type you're
looking for. Where `errors.Is` says "is this *that specific error*?",
`errors.As` says "is this *that kind of error*, and if so, give it to me".

The comment above it is a masterclass in documenting intent: *why* an empty
stderr with a non-zero exit is the signal for detached HEAD, and what the
distinction protects against.

## The idiomatic arc

Put the pieces together and you get the error-handling pattern this
codebase repeats on nearly every git call:

```go
func LocalBranches(root string) ([]string, error) {
	out, stderr, err := run(root, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		if notARepo(stderr, err) {
			return nil, ErrNotARepo          // 1. classify the expected case as a sentinel
		}
		return nil, wrapErr("git for-each-ref", err, stderr) // 2. wrap the rest with context
	}
	// ...parse out...
}
```

1. Expected, ignorable failure → a sentinel (`ErrNotARepo`) the caller can
   `errors.Is` against and handle gracefully.
2. Real failure → wrapped with context (`wrapErr("git for-each-ref", err,
   stderr)`) so the message says *what* failed, and `%w` preserves the
   cause for `errors.Is`/`errors.As` up the chain.

Design the *kind* of failure as a value, preserve the *cause* of the
failure as a chain, and let callers match either one. That's idiomatic Go
error handling.

## Exercise

1. In `internal/ui/app.go`, find the `tea.Msg` type switch in `Update`.
   List three message types it handles and where each comes from.
2. In `internal/cli/cli.go`, `readLine` does `r.(*bufio.Reader)` — a **type
   assertion** (`x.(T)`). What does the `ok` boolean in `br, ok := r.(*bufio.Reader)`
   tell you, and what does the function do in each case?
3. In `internal/git/git.go`, find three functions that return
   `ErrNotARepo`, and one that uses `errors.As` with `*exec.ExitError`.
   Explain what each is classifying.

**Next:** [06 — Shelling out: the git package](06-shelling-out-git.md) —
the whole `os/exec` story, through the package that runs git.
