# 08 — Concurrency & context

> Teaches: `sync.Mutex`, `sync.Once`, `context`, and goroutines via Bubble
> Tea. Grounded in: `internal/markdown/markdown.go`, `internal/home/home.go`,
> `internal/git/git.go`, `internal/ui/app.go`, `internal/launch/launch.go`.

Go's concurrency story is famous for goroutines and channels — and this
codebase uses almost none of the latter. What it *does* use is the
practical, everyday side of Go concurrency: mutex-protected shared state,
one-time initialization, context-bounded child processes, and the
goroutines that a UI framework runs for you. This chapter is deliberately
honest about that: we'll show the concurrency that actually exists here,
and it's not a tour of channel patterns, because there are no raw channel
patterns in this code to teach. (If you want the classic channel
introduction, the Go Tour and "Go by Example" cover it — but you won't
find it in manigot.)

## The race that started it: `sync.Mutex` in the markdown package

`internal/markdown/markdown.go` maintains a cache of Glamour renderers,
one per terminal width. The cache is package-level shared state, so it is
protected by a mutex:

```go
var (
	rendererMu    sync.Mutex
	rendererCache = map[int]*glamour.TermRenderer{}
)

func rendererFor(width int) (*glamour.TermRenderer, error) {
	rendererMu.Lock()
	defer rendererMu.Unlock()
	if r, ok := rendererCache[width]; ok {
		return r, nil
	}
	r, err := glamour.NewTermRenderer(...)
	if err != nil {
		return nil, err
	}
	rendererCache[width] = r
	return r, nil
}
```

This is the textbook `sync.Mutex` shape:

- `rendererMu.Lock()` guards the map; `defer rendererMu.Unlock()` releases
  it when the function returns, *however* it returns — the defer is what
  makes this safe on every code path, including the error return.
- The check-then-set inside the lock is one atomic operation from other
  goroutines' point of view: two concurrent calls can't both build a
  renderer for the same width. (Lock the whole read-modify-write, not
  pieces of it.)
- Because `map` reads and writes are not safe for concurrent use in Go,
  *every* access to `rendererCache` goes through the mutex — including
  `SetStyle`, which replaces the whole map and clears the cache:

```go
func SetStyle(s string) {
	rendererMu.Lock()
	defer rendererMu.Unlock()
	if s == "" || s == style {
		return
	}
	style = s
	rendererCache = map[int]*glamour.TermRenderer{}
}
```

Why does this cache exist at all? The comment tells a genuinely wild
story — the kind of thing you only learn by debugging:

> glamour's "auto" style ... probes the terminal's background color via
> termenv.HasDarkBackground(), which writes an OSC query to the tty and then
> blocking-reads raw bytes directly off stdin until it sees the matching
> response — or until termenv.OSCTimeout (5s) elapses. That read races with —
> and can consume/discard — Bubble Tea's own raw-mode stdin reader.

In other words: building a renderer could stall the whole TUI for up to
five seconds *and* steal keystrokes. The cache bounds how often that can
happen (once per width instead of once per keypress), and the chapter-04
regression test (`TestRendererReusedPerWidth`) pins the cache to exactly
one renderer per width. The real fix lives in `DetectStyle`: the TUI picks
a concrete `"dark"`/`"light"`/`"notty"` style **once at startup** — before
Bubble Tea takes over the terminal — and `SetStyle` bakes it in, so the
dangerous auto-probe never runs while the TUI is live.

The general lessons, in case you ever touch this file:

1. **`sync.Mutex` + `defer Unlock`** is the default way to protect shared
   state in Go.
2. **Cache invalidation must be atomic too** — `SetStyle` clears the map
   under the same mutex.
3. **Comments that explain *why* a race exists** are worth their weight in
   gold. The markdown package's comments document a subtle, expensive bug
   that any refactor could reintroduce.

## `sync.Once`: run something exactly once, safely

`internal/home/home.go` locates the manigot checkout. Locating it involves
`os.Executable()` and `filepath.EvalSymlinks` — not cheap, and
process-constant (the binary doesn't move while running). So it's computed
once and memoized:

```go
var (
	executableRootsOnce  sync.Once
	executableRootsCache []string
)

func executableRoots() []string {
	executableRootsOnce.Do(func() {
		executableRootsCache = computeExecutableRoots()
	})
	return executableRootsCache
}
```

`sync.Once` guarantees the function runs exactly once per process — even if
a hundred goroutines call `executableRoots()` simultaneously, exactly one
executes the body and the rest wait for it. It's Go's answer to "lazy
initialization that is safe under concurrency" without any mutex ceremony.

The comments make the design explicit:

> os.Executable and filepath.EvalSymlinks are process-constant and not
> cheap, and Root() sits on config's env-read hot path — deriving them once
> per process is the win. The MANIGOT_HOME env check in Root() stays
> uncached: it is cheap and must be read fresh (Seed sets it at startup,
> and tests set it per-test).

Note the careful boundary: the *expensive* lookup is memoized, the *cheap
env read* is not — because caching would break tests that set the env var
per test. Deciding what *not* to cache is part of the design.

## `context`: deadlines for child processes

Chapter 06 showed `runCtx` — `exec.CommandContext(ctx, "git", ...)` —
which ties a child process's lifetime to a `context.Context`. When the
context fires, `CommandContext` kills the child; `runCtx` then returns the
context's own error (`context.DeadlineExceeded` / `context.Canceled`) so
callers can `errors.Is` against it (there's a test for exactly that in
`internal/git/context_test.go`).

The TUI uses it to bound an operation that could otherwise hang forever.
From `internal/ui/app.go`, the "P" push-to-origin action:

```go
func (a *App) pushCmd(branch string) tea.Cmd {
	root := a.root
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), hostGitTimeout)
		defer cancel()
		err := git.PushWithContext(ctx, root, branch)
		return pushMsg{branch: branch, err: err}
	}
}
```

The pattern, which you'll use constantly:

```go
ctx, cancel := context.WithTimeout(context.Background(), hostGitTimeout)
defer cancel()   // always cancel — releases the timer even on the happy path
err := someCall(ctx, ...)
```

`context.WithTimeout` returns a context that fires after the timeout and a
`cancel` function that releases the timer early. The `defer cancel()` is
not optional ceremony — without it, the timer leaks until it fires.
Passing the context down lets the callee (the git call) observe
cancellation mid-flight instead of blocking forever on a stalled network.
The comment in `app.go` states the goal: "a stalled network can't hang the
TUI's command channel forever."

## Goroutines without `go` statements: Bubble Tea commands

Here's the surprising part: the TUI runs *concurrently*, but you won't see
a `go` statement in `app.go` — Bubble Tea runs goroutines for you. Recall
from chapter 05 that `tea.Model.Update` returns `(tea.Model, tea.Cmd)`,
where a `Cmd` is just a function:

```go
type Cmd func() tea.Msg
```

When `Update` returns a `Cmd`, Bubble Tea runs it **in its own goroutine**
and delivers the resulting `tea.Msg` back to `Update` on the next loop.
So `runDoneCmd` in `app.go`:

```go
func (a *App) runDoneCmd() tea.Cmd {
	root := a.root
	jobName := a.detail.job.Name
	return func() tea.Msg {
		_, err := job.FinishJob(root, jobName, yesConfirm, io.Discard)
		return doneMsg{err: err}
	}
}
```

does a full `job.FinishJob` — git operations, prompts, the works — off the
UI thread, and the TUI keeps rendering while it runs. When it finishes,
the returned `doneMsg` flows back through `Update`'s type switch (chapter
05) and the UI shows the result. The pattern is:

1. **Capture** everything the background work needs in a closure
   (`root`, `jobName`).
2. **Return** the closure as a `tea.Cmd`.
3. **Report** the outcome as a message (`doneMsg{err: err}`) that `Update`
   handles.

This is the idiomatic way to do background work in a Bubble Tea app — and
the deeper lesson is that *the framework is the concurrency*. You write
single-threaded-looking `Update` code; the framework hands work to
goroutines and funnels results back through one message channel. The
`pushCmd` example above shows the same shape with a context deadline
layered in.

The one *explicit* goroutine in the launch code is honest about what it
does. `internal/launch/launch.go` spawns `mg jdi` as a detached child
process and reaps it asynchronously:

```go
if cmd.Process != nil {
	go func() { _ = cmd.Wait() }()
}
```

`cmd.Start()` launches the process; `cmd.Wait()` waits for it and reaps the
zombie — but `mg jdi` can run for minutes, and the comment says waiting
for it "must not block." So the wait happens in a goroutine: the process
is started, the caller returns immediately, and the `Wait` goroutine does
the reaping in the background. The `_ =` deliberately discards the exit
error — nothing needs it. This is the everyday "fire and forget with
reaping" goroutine, and the comment earns its place by saying *why* the
wait is async.

Also in `launch.go`, the mutex used to serialize tmux pane management is
worth a nod:

```go
// tmuxMu serializes the find-kill-split sequence in launchTmuxPane so two
// concurrent launches (Agent/Quick can be invoked from Bubble Tea command
// goroutines) can never interleave ...
var tmuxMu sync.Mutex
```

"Bubble Tea command goroutines" — there it is again: the framework's
background `Cmd` execution is the reason the launch path needs a mutex at
all. The TUI's "concurrency" is mostly Bubble Tea's, and the code guards
the shared resources (the tmux pane id) against it.

## The honest summary

manigot's concurrency toolbox, in order of how often it's used:

| tool | used for | where |
|---|---|---|
| `context` + `WithTimeout` | bounding child processes and network calls | `git.go`, `ui/app.go` |
| `sync.Mutex` + `defer Unlock` | protecting shared maps/state | `markdown.go`, `launch.go` |
| Bubble Tea `tea.Cmd` closures | background work off the UI thread | `ui/app.go` |
| `sync.Once` | safe one-time initialization | `home.go` |
| `go func()` | explicit fire-and-forget (reaping a child) | `launch.go` |
| channels | — | *none — not used in this codebase* |

If you're learning concurrency, learn the table top-down: contexts and
mutexes are what you'll actually write; channels are a tool you reach for
when you have real producer/consumer or pipeline structure — which manigot
doesn't have, and didn't force.

## Exercise

1. In `internal/markdown/markdown.go`, why must `SetStyle` take the mutex,
   and why does it replace the whole map instead of clearing it in place?
2. In `internal/home/home.go`, why is `Root()`'s `MANIGOT_HOME` check *not*
   memoized? What would break if it were?
3. In `internal/ui/app.go`, find `pushCmd` and `runDoneCmd`. What does each
   capture in its closure, and what message type does each report back?
4. In `internal/launch/launch.go`, what invariant does `tmuxMu` protect,
   and which goroutines could race without it?

**Next:** [09 — Walking the codebase](09-walking-the-codebase.md) — trace a
bare `mg` session and a job's life end to end.
