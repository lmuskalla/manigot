# 10 — Your turn: contributing to manigot

> Teaches: putting it all together with concrete, graded exercises. No new
> concepts — this chapter is a workout.

You've read the whole codebase. Now change it. The exercises below are
deliberately small, grounded in the files you've already met, and ordered
by difficulty. Each one ends with the same verification loop:

```bash
make check     # go vet + go test — run it before you commit anything
```

If a test fails and you don't understand why, that's not a dead end —
it's the tutorial working as intended. `make check` is your safety net
while you experiment.

## Exercise 1: add a test to an existing package (easy)

The `internal/markdown` package has a `Mask`-like function... wait, no —
`internal/cli` does. `cli.Mask` renders a short safe form of a secret
(`"sk-ant-oat01-abcdefghijkl"` → `"sk-a…ijkl"`). Look at the existing
tests in `internal/cli/cli_test.go` — `TestMask` has four cases.

Your task: add two more cases to `TestMask`'s table:

1. exactly 8 characters (the boundary of the `<= 8` rule),
2. exactly 9 characters (just past it — check the `v[:4]` / `v[len-4:]`
   slices make sense).

Run `go test ./internal/cli/`. Then run `go test ./...` to make sure you
didn't break anything else.

**What you practiced:** the table-driven test shape (chapter 04), reading
a function's edge cases, and the `go test` loop.

## Exercise 2: add a flag to an existing subcommand (easy–medium)

`cmd/mg/job.go` parses `mg job` with a `FlagSet` (chapter 07). The script
it replaced accepted `--type` and `--base-branch`.

Your task: add a `--author` flag that overrides the `author` written into
the scaffolded `brief.md`. You'll need to:

1. Register it in `runJob`: `fs.String("author", "", ...)`.
2. Thread it through `job.CreateJob` — add an `Author string` field to
   `CreateOptions` in `internal/job/create.go`, and use it in place of
   `git.ConfigUserName(root)` when set.
3. Find the tests that pin `CreateJob`'s behavior (look in
   `internal/job/` and `cmd/mg/`) and add coverage for the override.
4. Run `make check`.

**What you practiced:** the `FlagSet` pattern, threading an option through
a struct instead of adding parameters (the `CreateOptions` struct exists
precisely so flags don't explode the signature), and extending tests
alongside code. Don't worry about changing the help text unless a test
demands it — the wording contract matters most where tests pin it.

## Exercise 3: add a subcommand by copying one (medium)

The dispatcher in `cmd/mg/main.go` makes adding a command a two-file
change (chapter 07). Copy `cmd/mg/profiles.go`'s shape to add `mg
versions` — a command that prints the version string and exits 0.

Concretely:

1. Create `cmd/mg/versions.go` with `runVersions(args []string, stdout,
   stderr io.Writer) int` that prints `version` (the package var from
   `main.go`, set at build time via `-ldflags`).
2. Add a `case "versions":` to the switch in `main.go`.
3. Write a small test `versions_test.go` asserting the output contains the
   version string and the exit code is 0.
4. `make check`, then `make mg && ./bin/mg versions`.

**What you practiced:** the dispatcher pattern, the `runX(args, out,
err) int` convention, and the per-command test shape. This is the exact
recipe the real subcommands were built with.

## Exercise 4: work a real job (the whole flow)

The ultimate exercise is to use the tool on itself. From the project
root, where `docs/` exists:

```bash
make mg
./bin/mg job "learn go by writing docs"    # creates branch + worktree
# fill in brief.md inside the new job dir
./bin/mg jdi --job <id>                     # analyst → developer → reviewer, unattended
./bin/mg done <id>                          # squash-merge + archive, when the verdict is APPROVED
```

Even simpler than that: **this very tutorial is such a job.** The
`docs/jobs/` directory holds real jobs with real `brief.md`, `tasks.md`,
`implementation.md`, and `verdict.md` files — open one and read it as a
worked example of everything in chapter 09. You're reading the product of
the exact workflow the code implements.

**What you practiced:** the whole arc — create, work, review, finish —
from the user side, which is the best context for later working on the
code that implements it.

## Where to go from here

- **`docs/AGENTS.md`** — the canonical description of the whole system.
  It's the project context every agent session loads; reading it is
  reading the maintainers' mental model. The hard rules at the bottom
  (never commit `.env`, never touch files outside `docs/`, keep
  `agents/*.md` in sync) are the project's constitution.
- **`agents/*.md`** — the agent definitions, including `developer.md` —
  the role you've effectively been practicing. They're Markdown
  instructions baked into the container image; they document how work is
  supposed to happen here.
- **The tests** — the ~100 `_test.go` files in this repo are the most
  detailed documentation you have. Every ported function's tests pin its
  exact behavior and wording. When you change behavior, the tests tell
  you what contract you're changing.
- **`make check`** — your permanent companion: `go vet` + `go test` (+
  shellcheck on the one bash script, when installed). Run it before every
  commit, and fix what it finds.
- **The job workflow** — for real changes, don't edit `main` directly.
  `mg job` cuts you a branch and a worktree; `@analyst` breaks down the
  work; `@developer` implements one task per commit; `@reviewer` and
  `@security` write the verdict; `mg done` merges it. That's the loop this
  whole codebase exists to serve — and now you can read every line of it.

Good luck. You now know enough to find your way around — the rest is
repetition.
