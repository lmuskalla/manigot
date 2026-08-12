# 04 — Testing with Go's testing package

> Teaches: `TestXxx` functions, table-driven tests, subtests, and the
> dependency-injection pattern that makes code testable. Grounded in:
> `internal/orchestrate/orchestrate_test.go`, `internal/git/branch_test.go`,
> `internal/cli/cli_test.go`, `internal/markdown/markdown_test.go`, and
> `make check`.

Go ships a built-in testing framework — no external framework, no
assertion library, no test doubles framework required. A test is just a
function that uses the `testing` package and runs your code. This chapter
shows the three shapes of tests used across manigot, using the real test
files as examples.

## The basics: `go test` and `TestXxx` functions

A test file lives next to the code it tests: `branch.go` has
`branch_test.go`, `markdown.go` has `markdown_test.go`, and so on. Every
test is a function whose name starts with `Test` and which takes a single
argument, `*testing.T`:

```go
package markdown

import (
	"strings"
	"testing"
)

func TestRenderProducesNonEmpty(t *testing.T) {
	src := "# Title\n\nSome **bold** text and a paragraph.\n"
	out := Render(src, 60)
	if strings.TrimSpace(out) == "" {
		t.Fatal("Render returned empty output")
	}
	// Glamour renders the heading; the word "Title" must survive.
	if !strings.Contains(out, "Title") {
		t.Errorf("rendered output missing heading text; got:\n%s", out)
	}
}
```

Notes on the mechanics:

- The test lives in the **same package** (`package markdown`, not
  `package markdown_test`) — that's how it can call `Render` directly, and
  also peek at unexported details like `v.offset` (which `markdown_test.go`
  does, since unexported identifiers are visible inside the package).
- `t.Fatal` stops the test immediately; `t.Errorf` records a failure but
  keeps going. Both print the message with file/line info.
- `go test ./...` runs every test in every package. Run it — this repo's
  suite should pass.
- The output-checking convention here is plain `if` + `t.Errorf` with an
  explicit `got`/`want` message. No assertion library — the message format
  `got %q, want %q` is the idiomatic convention.

## Table-driven tests with subtests

The most important testing idiom in Go is the **table-driven test**: list
your cases as a slice of structs, then loop over them. It makes adding a
case a one-line change and keeps the test logic in one place.

`internal/git/branch_test.go` is a compact example:

```go
func TestBranchTail(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{"feature/abc123_hello", "abc123_hello"},
		{"feature/irw320_tui", "irw320_tui"},
		{"fix/abc123_x", "abc123_x"},
		{"abc123_hello", "abc123_hello"},                // no "/" — the whole name
		{"", ""},                                        // empty input
		{"feature/", ""},                                // trailing slash
		{"prefix/feature/abc123_hello", "abc123_hello"}, // nested prefix
	}
	for _, tt := range tests {
		if got := BranchTail(tt.branch); got != tt.want {
			t.Errorf("BranchTail(%q) = %q, want %q", tt.branch, got, tt.want)
		}
	}
}
```

The anonymous struct `[]struct { branch string; want string }` is the
"table". Each row is one case, including edge cases — empty input, trailing
slash, nested prefixes. Comments on the edge-case rows explain *why* they
matter. Adding a case is literally one line.

For more complex functions, manigot goes one step further and uses
**subtests** — `t.Run(name, func(t *testing.T){...})` — which give each
case its own identity in the output. `internal/orchestrate/orchestrate_test.go`
shows the full pattern. Its table has six fields and twelve rows describing
the mg-jdi state machine:

```go
tests := []struct {
	name                  string
	stage                 job.Stage
	verdictRounds         int
	latestCommitIsVerdict bool
	wantKind              Kind
	wantAgent             string
}{
	{
		name:     "define: brief.md not written",
		stage:    job.StageDefine,
		wantKind: StopNeedsHuman,
	},
	{
		name:      "plan: tasks.md not written",
		stage:     job.StagePlan,
		wantKind:  RunAgent,
		wantAgent: "analyst",
	},
	// ...
}

for _, tc := range tests {
	t.Run(tc.name, func(t *testing.T) {
		got := Next(tc.stage, tc.verdictRounds, tc.latestCommitIsVerdict)
		if got.Kind != tc.wantKind {
			t.Errorf("Next(...).Kind = %v, want %v", got.Kind, tc.wantKind)
		}
		// ...
	})
}
```

Two things worth copying from this file:

1. **Case names are prose.** `"plan: tasks.md not written"` reads like a
   spec. With subtests, a failure shows up as
   `--- FAIL: TestNext/plan:_tasks.md_not_written` — you know instantly
   which behavior broke.
2. **A guard test protects the table itself.**
   `TestNextStagesCoverEveryDefinedStage` loops over `job.Stages` and fails
   if any stage is missing from the coverage map — so someone adding a new
   stage *cannot* silently fall into the "unknown" default. The test
   comments even name the real bug it guards against (the "jdi does not
   work" job). This "test the tests" pattern is rare and worth emulating:
   it turns a future mistake into a loud failure instead of a subtle one.

## Making code testable: inject the streams

The hardest part of testing isn't writing tests — it's designing code that
can be tested. The most common testability pattern in this codebase is
**dependency injection of `io.Reader`/`io.Writer`**.

Look at `internal/cli/cli.go`'s signature (seen in action in
`cli_test.go`):

```go
Confirm(prompt string, in io.Reader, out io.Writer) (bool, error)
```

`Confirm` doesn't read the terminal directly — it *receives* its input and
output streams. In production, `main.go` passes `os.Stdin`/`os.Stdout`; in
tests, it passes a `strings.Reader` and a `strings.Builder`:

```go
func TestConfirmAcceptsYesAnswers(t *testing.T) {
	for _, in := range []string{"y", "Y", "yes", "YES", "y\n"} {
		var w strings.Builder
		got, err := Confirm("Continue? [y/N] ", strings.NewReader(in), &w)
		if err != nil {
			t.Fatalf("Confirm(%q): %v", in, err)
		}
		if !got {
			t.Errorf("Confirm(%q) = false, want true", in)
		}
		if w.String() != "Continue? [y/N] " {
			t.Errorf("prompt written = %q, want %q", w.String(), "Continue? [y/N] ")
		}
	}
}
```

This single test exercises *everything*: each accepted answer (`y`, `Y`,
`yes`, ...) returns `true`, and the prompt text written to the output
stream is exactly right. No terminal, no user, no flakiness — pure,
deterministic function calls.

The file shows the pattern at full power:

- **Input**: `strings.NewReader("9\nabc\n3\n")` feeds a fake user who types
  garbage, then a valid choice — testing the reprompt loop.
- **Output**: `strings.Builder` captures exactly what was written, so tests
  assert on the *prompt wording* (e.g. `"Select an agent [1-3]: Enter a
  number between 1 and 3.\n"` repeated per iteration).
- **Edge cases**: empty input (EOF) defaults to "no"; `errors.Is(err,
  io.EOF)` checks that a raw EOF is wrapped in a descriptive message rather
  than leaking out.

You saw this same idea in chapter 01: every `runX(args, os.Stdin,
os.Stdout, os.Stderr)` in `cmd/mg/main.go` exists so the commands can be
tested exactly this way. **If a function needs a stream, make it a
parameter.** That single habit is what makes most of manigot testable
without mocks.

## Testing unexported state (and why it's OK)

`markdown_test.go` shows something that surprises newcomers: tests reaching
into unexported fields and calling unexported helpers.

```go
rendererMu.Lock()
rendererCache = map[int]*glamour.TermRenderer{}
rendererMu.Unlock()

Render("# a\n", 42)
Render("# b\n", 42)

rendererMu.Lock()
defer rendererMu.Unlock()
if len(rendererCache) != 1 {
	t.Fatalf("expected exactly one cached renderer for width 42, got %d entries", len(rendererCache))
}
```

This is a **regression test** — its job is to pin down a specific past
bug. The comment says so: "TestRendererReusedPerWidth is a regression test
for TASK-2: Render must not build a brand-new glamour.TermRenderer (and
re-trigger its WithAutoStyle terminal probe) on every call." Because the
test is in the same package, it can reset the cache, call `Render` twice,
and assert the cache holds exactly one entry. (Chapter 08 tells the race
story this test protects.) White-box tests like this are idiomatic in Go
for exactly this purpose: asserting on *internals* that would be
impossible to observe from outside.

## `make check`: the verification loop

The Makefile's `check` target is the project's quality gate:

```make
check: ## Run go vet + go test, and shellcheck on scripts/entrypoint.sh (when installed)
	go vet ./...
	go test ./...
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck scripts/entrypoint.sh; \
	else \
		echo "shellcheck not installed — skipping scripts/entrypoint.sh check"; \
	fi
```

- `go test ./...` — the whole suite, every package.
- `go vet ./...` — a static analyzer for suspicious constructs (wrong
  `Printf` verbs, unreachable code, copy locks, etc.). `vet` doesn't run
  your code; it inspects it.
- `shellcheck` — a *linter* for the one remaining bash script, run only if
  installed (the target degrades gracefully).

This is the loop you'll use constantly while working on manigot: change
something, `make check`, and it tells you whether anything is broken — 
compilation, tests, or obvious static-analysis bugs.

## What you should take away

- A test is a `TestXxx(t *testing.T)` function in a `_test.go` file next
  to the code; `go test ./...` runs them all.
- Table-driven tests (slice-of-struct cases + a loop) are the default
  shape; add `t.Run` subtests with prose names for real behavior specs.
- Code that receives its streams (`io.Reader`/`io.Writer`) as parameters is
  trivially testable with `strings.Reader`/`strings.Builder` — design for
  this from the start.
- Regression tests may legitimately inspect unexported state to pin a
  specific bug.
- `make check` = `go vet` + `go test` (+ shellcheck) is the project's
  verification loop. Run it before committing.

**Next:** [05 — Interfaces & idiomatic Go](05-interfaces.md) — the
language's most distinctive feature, through `tea.Model`, `io.Reader`, and
sentinel errors.
