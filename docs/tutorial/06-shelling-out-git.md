# 06 — Shelling out: the git package

> Teaches: `os/exec`, `exec.CommandContext`, capturing stderr, exit-code
> handling, and error wrapping. Grounded in: `internal/git/git.go` — 831
> lines of the clearest `os/exec` code you'll find.

manigot is a tool *about* git worktrees, and it doesn't use a git
library — it runs the `git` binary. The `internal/git` package is the
single place that happens, and it's a complete, readable course in
`os/exec`. Its package comment sets the design:

> It is the only place in the TUI that shells out to git, so the
> job/launch/ui packages ask about branches and worktrees through it rather
> than each shelling out ad-hoc.

**One choke point.** Every git invocation in the codebase funnels through
this package. That's a deliberate architecture decision: git's behavior
(exit codes, stderr wording, error classes) is normalized in exactly one
place, and every other package gets a clean Go API. When you shell out from
your own Go code, do the same: *put it in one package, with one set of
helpers, and never let the raw command leak out.*

## The core: `run`

Almost everything funnels through one helper (plus context/env variants):

```go
func run(root string, args ...string) ([]byte, string, error) {
	return runCtx(context.Background(), root, args...)
}

func runCtx(ctx context.Context, root string, args ...string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && ctx.Err() != nil {
		return out, stderr.String(), ctx.Err()
	}
	return out, stderr.String(), err
}
```

Deconstruct it:

1. **`exec.CommandContext(ctx, "git", args...)`** — the library's main
   entry point: run the program `git` with the given arguments. The
   variadic `args ...string` means callers pass flags positionally:
   `run(root, "for-each-ref", "--format=...", "refs/heads/")`.
2. **`-C root` is prepended** — git's `-C` flag changes directory for the
   command, so callers can hand over an absolute project root "without
   worrying about the process's own cwd" (as the package comment says).
   That's the *right* way to run git against a directory — never
   `chdir` a whole process just to run a child.
3. **`cmd.Stderr = &stderr`** — git's error output is captured into a
   `bytes.Buffer` instead of leaking to the terminal. `cmd.Output()`
   returns the child's *stdout*; its *stderr* is whatever you assigned to
   `cmd.Stderr`. Capturing stderr separately is what lets the package
   classify errors by their message (the `notARepo` test in chapter 05)
   and append git's explanation to its own error text.
4. **The return shape is `(out []byte, stderr string, err error)`** — raw
   stdout, raw stderr, and the exec error. Raw strings, no interpretation
   — callers parse (and the package normalizes) at the next layer.

Note the `CommandContext` vs `Command` split: `run` uses a
`context.Background()` — no timeout, because an interactive session
"waits on git as long as git needs." The `WithContext` variants exist for
the non-interactive callers (TUI background tasks, mg-jdi probes) that
want to bound the call; the timeout semantics are described in the `runCtx`
comment and surface as `context.DeadlineExceeded` via `ctx.Err()` (chapter
08 covers the context side). The point for now: **shelling out with a
context gives you cancellation and timeouts for free.**

## Normalizing errors: `wrapErr` and `notARepo`

Two helpers turn git's raw failure modes into a clean Go API.

`wrapErr` adds context to a real failure and preserves the cause:

```go
func wrapErr(prefix string, err error, stderr string) error {
	if msg := strings.TrimSpace(stderr); msg != "" {
		return fmt.Errorf("%s: %w: %s", prefix, err, msg)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
```

`%w` (chapter 03) keeps the chain intact for `errors.Is`/`errors.As`, the
prefix names the operation (`"git branch -D "+branch`), and git's own
stderr is appended so the user sees git's explanation. A call site shows
the pattern repeated:

```go
_, stderr, err := run(root, "branch", "-D", branch)
if err != nil {
	if notARepo(stderr, err) {
		return ErrNotARepo
	}
	return wrapErr("git branch -D "+branch, err, stderr)
}
return nil
```

`notARepo` classifies the one *expected* failure — not a git repository, or
git not installed — so it can become the `ErrNotARepo` sentinel instead of
a wrapped noise error. Its comment documents a real subtlety:

> The match is case-insensitive: `git diff` reports "warning: Not a git
> repository" (capital N, exit 129) while the plumbing commands report
> "fatal: not a git repository" (lowercase, exit 128) — both are the same
> signal.

Different git commands spell the same failure differently — so the
classifier matches case-insensitively. That's the kind of detail you only
learn by testing against real behavior, and the comment is what makes it
safe to rely on.

## Exit codes are data

Shelling out means dealing with the child's exit status. The most
instructive use is `WorkingTreeDirty`, which checks for uncommitted
changes:

```go
func WorkingTreeDirty(root string) (bool, error) {
	dirty := false
	for _, args := range [][]string{{"diff", "--quiet"}, {"diff", "--cached", "--quiet"}} {
		_, stderr, err := run(root, args...)
		if err != nil {
			if notARepo(stderr, err) {
				return false, ErrNotARepo
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
				// Exit 1 from `git diff --quiet` is the "differences exist"
				// signal, not a failure.
				dirty = true
				continue
			}
			return false, wrapErr("git "+strings.Join(args, " "), err, stderr)
		}
	}
	return dirty, nil
}
```

This is the essence of exit-code handling: **git uses exit status as a
*result channel*, not just a success/failure bit.** `git diff --quiet`
exits 0 when there are no differences and 1 when there are. `exec` turns
any non-zero exit into an error, so the code must distinguish "error" from
"meaningful non-zero exit":

- `errors.As(err, &exitErr)` asks "is this an `*exec.ExitError`?" (chapter
  05).
- `exitErr.ExitCode() == 1` reads the actual status.
- Commenting the *meaning* of that status (`"differences exist" signal,
  not a failure`) is what keeps the next reader from "fixing" it.

The same `ExitCode()` inspection appears in `RefExists` (a non-zero exit
from `git rev-parse --verify --quiet` means "ref doesn't exist" — not an
error) and in the detached-HEAD detection in chapter 05. General lesson:
**a non-zero exit is not automatically an error. Read the code, know what
the command uses it for, and comment that knowledge.**

## Parsing output: one command, one parse

When a function needs data from git, it runs one command and parses its
stdout. `LocalBranches` splits lines; `RecentCommits` uses a custom field
separator (`\x1f`, the ASCII Unit Separator) "which — unlike a comma,
space or pipe — never legitimately appears in a commit subject." The
`RecentCommits` comment explains *why* the command is shaped the way it is
(one `git log --source` union-traversal instead of per-branch queries)
with the care of a design document. These long comments are the package's
superpower: every non-obvious choice is explained in place.

## Security note: no string interpolation

One thing you'll never see in `git.go` is a shell string. `exec.Command`
takes the program and arguments as separate strings — no shell involved,
so no injection. `run(root, "branch", "-D", branch)` can't be tricked into
running extra commands even if `branch` contains spaces or shell
metacharacters (a real risk with branch names, which people love to make
adversarial). **Use `exec.Command`, never `sh -c "git ..."`. That's not a
style choice; it's the injection-safe way to shell out.**

## What you should take away

- One package, one `run` helper, and a clean Go API — the single choke
  point pattern for all external commands.
- `exec.CommandContext` + prepend `-C root` for "run against this dir".
- Capture stderr in a `bytes.Buffer`; return it alongside stdout.
- Classify expected failures as sentinels, wrap everything else with
  `%w` + context.
- Non-zero exits are often meaningful data (`ExitCode()`), not errors —
  comment what each code means.
- Never build shell strings; `exec.Command` is the injection-safe path.

**Next:** [07 — CLI architecture](07-cli-architecture.md) — how the `mg`
binary turns `os.Args` into a command.
