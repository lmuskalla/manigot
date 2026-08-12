# 07 — CLI architecture

> Teaches: subcommand dispatch, `flag.FlagSet`, custom flag splitting,
> prompt primitives, and the `runX(args, stdin, stdout, stderr)` pattern.
> Grounded in: `cmd/mg/main.go`, `cmd/mg/job.go`, `cmd/mg/profiles.go`,
> `cmd/mg/flags.go`, `internal/session/session.go`, `internal/cli/cli.go`.

The `mg` binary implements eleven commands in one process: `mg` (session),
`profiles`, `setup`, `agents`, `job`, `done`, `delete`, `init`, `tui`,
`jdi`. This chapter walks the three layers that make that work: the
dispatcher, the per-command parsing, and the shared prompt primitives —
and the one pattern that ties them together.

## Layer 1: the dispatcher

`cmd/mg/main.go` (chapter 01) is the whole router. `os.Args[1:]` is
switched on its first element; each case calls a `runX` function; the
result is the process exit code:

```go
switch args[0] {
case "-h", "--help", "help":
	printHelp()
	os.Exit(0)
case "profiles":
	os.Exit(runProfiles(args[1:], os.Stdin, os.Stdout, os.Stderr, cli.IsTerminal(os.Stdin)))
case "job":
	os.Exit(runJob(args[1:], os.Stdout, os.Stderr))
case "jdi", "made-man":
	os.Exit(runJDI(args[1:], os.Stdout, os.Stderr))
// ...
default:
	os.Exit(runSession(args, os.Stdin, os.Stdout, os.Stderr))
}
```

The architecture rule is visible in the `args[1:]` on every line: **each
`runX` receives the *remaining* arguments, never the full `os.Args`.** The
command name is consumed by the switch; the command parses the rest. This
is the "dispatcher hands off" shape — each command is a self-contained
function, and adding a new command means adding one `case` and one file.

The `default` case is the interesting one: it's the session launcher.
Anything that isn't a known command — including nothing at all — starts a
session, because the session's own flags (`--profile zai --job a3f9k2
--agent analyst ...`) must not be consumed by the dispatcher. The
dispatcher only recognizes its *commands*; session flags pass through.

## Layer 2: per-command parsing with `flag.FlagSet`

Go's standard library provides `flag` for command-line parsing. Each
subcommand creates its own `FlagSet` with `flag.ContinueOnError` so
parsing failures become errors it can handle itself. `cmd/mg/job.go` is a
full example:

```go
fs := flag.NewFlagSet("mg job", flag.ContinueOnError)
fs.SetOutput(io.Discard)   // don't let flag print its own diagnostics
fs.Usage = func() {}
jobType := fs.String("type", "", "job type: feature, fix, or chore")
baseBranchOverride := fs.String("base-branch", "", "branch to cut the job branch from")
if err := fs.Parse(args[1:]); err != nil {
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(stderr, "Unknown argument: --help")
		return 1
	}
	fmt.Fprintln(stderr, flagParseError(err))
	return 1
}
```

Things worth copying:

- **`fs.String("type", "", ...)` registers a flag and returns a pointer**
  to its value; after `Parse`, `*jobType` holds `"feature"`, `"fix"`, or
  `""` if unset.
- **`flag.ErrHelp` is handled as a special case** — the code comments that
  the old shell script had no help case, so `--help` was just an unknown
  argument, and the port reproduces that wording exactly.
- **`flagParseError` translates flag's messages into the CLI's
  user-visible contract.** `cmd/mg/flags.go` exists because the original
  scripts printed "Unknown argument: <flag>" and the tests pin that
  wording: flag reports errors as `"flag provided but not defined: -x"`
  (single dash), so the helper rewrites them to `"Unknown argument: --x"`.
  This is a recurring theme in this codebase — the CLI's exact wording is
  a contract, preserved 1:1 from the scripts it replaced.
- **`fs.Args()` after `Parse` holds the positionals.** `flag` stops at the
  first non-flag argument; `job.go` explicitly rejects any leftovers with
  "Unknown argument: %s", restoring the old script's strictness.

`cmd/mg/profiles.go` shows the same shape with a twist — its `-h` prints a
custom help to *stdout* via `fs.Usage = func() { fmt.Fprint(stdout,
profilesHelp) }` and returns 0. Each command tunes flag's behavior to its
own contract.

## When `flag` isn't enough: passthrough splitting

The session command has a requirement `flag` can't meet directly: unknown
flags and bare words are *passthrough* — handed verbatim to the agent CLI
inside the container (`mg --profile zai --agent analyst --job a3f9k2` is
`mg`'s own syntax, but `mg --some-opencode-flag xyz` should forward
`--some-opencode-flag xyz` into the container). The comment in
`internal/session/session.go` explains why that breaks `flag`:

> flag stops at the first non-flag argument and treats unknown flags as
> errors, neither of which matches "everything unknown goes through".

So `ParseArgs` first separates the args with `splitFlags`, then feeds only
the known flags to a `FlagSet`:

```go
func ParseArgs(args []string) Options {
	var o Options
	fs := flag.NewFlagSet("mg", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.Agent, "agent", "", "")
	fs.StringVar(&o.Job, "job", "", "")
	// ... --prompt, --tool, --profile ...
	fs.BoolVar(&o.Print, "print", false, "")

	var flagArgs []string
	flagArgs, o.Pass = splitFlags(args, sessionValueFlags, sessionBareFlags)
	_ = fs.Parse(flagArgs)
	return o
}
```

`splitFlags` (in `cmd/mg/flags.go` and mirrored in `session.go`) walks the
args once, recognizing the known flags:

```go
func splitFlags(args []string, valueFlags, bareFlags map[string]bool) (flagArgs, rest []string) {
	for i := 0; i < len(args); i++ {
		switch {
		case bareFlags[args[i]]:            // --print: no value
			flagArgs = append(flagArgs, args[i])
		case valueFlags[args[i]]:           // --agent etc.: take the next token as value
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		default:                            // unknown flag or bare word → passthrough
			rest = append(rest, args[i])
		}
	}
	return flagArgs, rest
}
```

This "known flags first, everything else through" pattern is the answer to
a problem every CLI hits once: **Go's `flag` is designed for tools whose
args are entirely their own.** When your tool wraps another program
(docker, git, an agent CLI), you need a two-stage parse: extract what's
yours, forward the rest. `flag` parses the extraction; a simple loop does
the forwarding.

## Layer 3: shared prompt primitives

Interactive commands (`profiles`, `setup`, `agents`, `done`, `delete`)
need to ask the user questions. The old shell scripts did this with
`read -rp`; the port lives in `internal/cli/cli.go` as five testable
functions (chapter 04). The catalog:

| function | behavior | script it replaces |
|---|---|---|
| `Confirm(prompt, r, w)` | `[y/N]` — true only for y/yes | the scripts' default-no confirmations |
| `PromptSecret(label, current, r, w)` | masked current value, Enter keeps | `setup.sh`'s secret prompts |
| `PromptValue(label, current, def, r, w)` | shown default, Enter keeps | `setup.sh`'s `prompt_value` |
| `Select(prompt, n, allowEmpty, r, w)` | numbered menu 1..n, q quits | `profiles.sh`/`agents.sh` menus |
| `IsTerminal(f)` | `[[ -t 0 ]]` — can we prompt at all? | the scripts' TTY guards |

Two design points visible in `cli.go`:

1. **Every prompt takes `io.Reader` + `io.Writer`** — the injection pattern
   from chapters 04–05, which is what made porting the scripts' *exact
   wording* testable. The package comment is explicit that "the exact
   wording of every prompt and error message matches the script it
   replaces, 1:1 (that wording is the CLI's user-visible contract)."
2. **One `bufio.Reader` per sequence of prompts.** The package comment
   warns that a command asking several prompts in sequence should wrap its
   input in *one* `bufio.Reader` and pass that down — `readLine` reuses
   it, "but a fresh wrap per prompt would lose whatever the previous
   bufio.Reader buffered past its newline." (`Select`'s loop is the
   visible case: it creates `br := bufio.NewReader(r)` once before the
   loop.)

## The pattern that ties it together

Look at every `runX` signature:

```go
runSession(args []string, stdin *os.File, stdout, stderr io.Writer) int
runJob(args []string, stdout, stderr io.Writer) int
runProfiles(args []string, r io.Reader, stdout, stderr io.Writer, tty bool) int
```

The shape is always: **`(remaining args, streams..., bool flags) int`**.
It's the single most important convention in this codebase, and it buys
three things:

1. **Testability.** `cmd/mg/*_test.go` files call `runJob([]string{"x"},
   &outBuf, &errBuf)` with zero filesystem or terminal setup — chapter
   04's entire premise.
2. **One place to decide "is there a terminal?"** — `tty` is passed as a
   plain bool, computed once in `main` via `cli.IsTerminal(os.Stdin)`,
   rather than each command probing stdin itself.
3. **Explicit exit codes.** Every command returns an `int`; `main` maps it
   to `os.Exit`. A command never calls `os.Exit` itself (so it stays
   testable), and it never forgets to report its status.

If you take one thing from this chapter into your own CLIs: **write every
command as `runX(args, in, out, err) int` and keep `main` a pure
dispatcher.** The pattern scales to any number of subcommands and makes
the whole surface testable.

## Exercise

1. In `cmd/mg/flags.go`, `flagParseError` maps two flag error prefixes.
   What do they correspond to, and why does it re-add `--` (double dash)?
2. In `cmd/mg/profiles.go`, why is `fs.Usage` set to print to `stdout`
   rather than letting flag print its default usage to stderr?
3. Trace what `mg --agent analyst --job a3f9k2 --some-unknown-flag x`
   does: which parts does `main`'s switch consume, what does
   `session.ParseArgs` extract, and what ends up in `Options.Pass`?

**Next:** [08 — Concurrency & context](08-concurrency-and-context.md) —
the goroutines, mutexes, and contexts that actually exist in this code.
