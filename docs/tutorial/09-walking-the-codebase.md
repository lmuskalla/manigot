# 09 — Walking the codebase

> Teaches: reading an unfamiliar Go codebase end to end. Grounded in:
> `internal/session/{session,root,docker}.go`, `internal/job/{create,finish,delete}.go`,
> `internal/orchestrate/orchestrate.go`, and their `cmd/mg/*.go` wrappers.

Everything so far has been individual concepts. This chapter is the
payoff: two full walks through the codebase — what happens when you run
`mg` (a session), and what happens across a job's whole life — following
the actual function calls. By the end you should be able to open any file
in this repo and orient yourself.

## Walk 1: a bare `mg` session

You type `mg` in a project directory. What runs? The call chain,
top to bottom:

```
cmd/mg/main.go:main()
  └─ runSession(args, os.Stdin, os.Stdout, os.Stderr)
       ├─ session.ParseArgs(args)          → Options
       ├─ session.ResolveProfile(opts)     → ProfileInfo (or error)
       ├─ session.ResolveRoot(opts)        → Root (or error)
       ├─ info.CheckAuth()                 → error?
       ├─ session.BuildDockerInvocation(...) → DockerInvocation
       └─ inv.Run(stdin, stdout, stderr)   → exit code
```

Each step is one function with one job. (Recall from chapter 07 why the
streams are parameters: every step becomes testable, and the command
returns an exit code instead of calling `os.Exit`.)

### Step 1: `session.ParseArgs` — what did the user ask for?

`internal/session/session.go`. `Options` is a struct holding everything the
session needs: `Agent`, `Job`, `Prompt`, `Tool`, `Profile`, `Print`, and
`Pass` (the passthrough args handed to the container CLI). Parsing is the
two-stage split from chapter 07: `splitFlags` extracts the known flags,
`flag.FlagSet` parses them, everything else lands in `o.Pass`.

### Step 2: `session.ResolveProfile` — which subscription?

Resolves the profile with the precedence chain the README documents:
`--profile` flag, else `--tool` (legacy alias), else `$MANIGOT_PROFILE`,
else `claude-pro`. The result is a `ProfileInfo`: which agent CLI to
launch, which credential keys to forward, which model. The old shell
script's logic, ported 1:1 — the package comment even notes the wording
of diagnostics is preserved.

### Step 3: `session.ResolveRoot` — where are we mounting?

`internal/session/root.go`. This is the most subtle step, and the one
where `--job` changes everything:

- Walk up from the cwd looking for a `docs/` directory
  (`job.FindProjectRootFrom`). Found → "initialized project". Not found →
  fall back to the git top-level, else `$PWD` — a plain session.
- If `--job <id>` was given, resolve the *job's own worktree*: match the id
  against local branch names (exact, then prefix; ambiguity is an error),
  then look up that branch's worktree via `git.WorktreeForBranch`. The
  mount root becomes the **worktree path**, not the project root — so the
  agent sees the job's branch, not `main`.
- Two hard-failure modes are deliberate: a branch match with no worktree is
  an error (never silently fall back to the project root — "that would
  show the wrong job's content"), and so is a worktree that lacks the job's
  directory. Inconsistent states are loud, not quiet.

The result is a `Root` struct: `ProjectRoot` (the mount root, possibly
reassigned by `--job`), `InvocationRoot` (the original, for display),
`DocsInitialized`, `DocsDir`, `Job`, `GitCommonDir`.

### Step 4: `info.CheckAuth` — are we allowed?

Validates credentials for the resolved profile before touching docker —
the subscription-protection layer. Wrong or missing keys fail here with a
clear message instead of inside the container.

### Step 5: `BuildDockerInvocation` — assemble `docker run`

`internal/session/docker.go`. This function builds the *entire* docker
argv as a `[]string` — and its comment says the argument order and exact
strings are "pinned by the tests," because every one is load-bearing.
Walking through what it assembles:

- **Git identity** — `GIT_AUTHOR_NAME`/`GIT_AUTHOR_EMAIL` from env, else
  the project's git config, with a warning when neither exists.
- **Mounts** — the `docs/` mount (at `/workspace/.claude` for Claude Code,
  `/workspace/.opencode` for OpenCode), the context file mount
  (`AGENTS.md` or `CLAUDE.md`, read-only), and the `.env` *shadow* mounts:
  every `.env` file found in the project is mounted over `/dev/null`
  read-only, so secrets in the project never reach the container.
- **The prompt** — for a `--job` run, "Please work on the job at
  /workspace/docs/jobs/<id> — start by reading brief.md"; in `--print`
  mode, the `NEEDS-HUMAN-INPUT:` marker definition is appended.
- **Per-tool argument shapes** — Claude takes the prompt positionally;
  OpenCode gets `--prompt`. `--agent` becomes `--agent <name>`.
- **The banner** — the boxed "Entering safehouse" output, the flavor quote
  from `assets/quotes.json` (skipped in `--print` mode).

The returned `DockerInvocation` is a single `Argv []string`; `inv.Run`
executes it with stdio wired through and returns the container's exit
code.

### The shape of the walk

Notice what *didn't* appear: no docker command strings scattered anywhere
except `docker.go`, no git knowledge outside `git.go`, no credential
handling outside `config.go`/`session.go`. The seam discipline from
chapter 06 (one choke point per concern) is what makes the walk
possible at all — each step was one function call into one package.

## Walk 2: a job's life

The job workflow (from `docs/AGENTS.md` and the README):

```
mg job → fill brief.md → @analyst → @developer → @reviewer → mg done
```

Each step maps to real code.

### Birth: `mg job "title"`

```
cmd/mg/job.go:runJob
  └─ job.CreateJob(root, title, opts, stdout)
```

`internal/job/create.go`. `CreateJob` is the biggest single lifecycle
function, and its flow is:

1. **Validate type** — `feature` (default), `fix`, or `chore`; anything
   else is an error.
2. **Resolve settings** — `project.Load(root)` for the base branch
   (`BaseBranchValue()`, default `"main"`) and the optional
   `jobBranchPrefix`; `--base-branch` overrides for this one call.
3. **Generate identity** — a 6-char random id (`crypto/rand` with rejection
   sampling to avoid modulo bias — the comment explains *why*), a
   slugified title, today's date, and the git author name.
4. **Compose the branch name** —
   `[prefix/]feature|fix|chore/<id>_<slug>`.
5. **Git or not?** — decided by whether the repo has any local branches.
   With branches: check the base branch exists, check the branch namespace
   is free (`checkBranchNamespace` — git stores refs as filesystem paths,
   so a plain branch `feature` blocks the whole `feature/...` namespace),
   then create the **worktree**.
6. **Worktree layout** — a *sibling* directory by default
   (`<parent>/.manigot-worktrees/<project>/<id>_<slug>`); *nested* inside
   the project when the root is itself a mount point (a `stat` device
   comparison decides — a sibling would land on another filesystem), with
   the nested path excluded via `.git/info/exclude`.
7. **Scaffold + first commit** — the four files (`brief.md`, `tasks.md`,
   `implementation.md`, `verdict.md`) are written byte-identical to the
   old shell heredocs, then committed inside the job's *own* worktree:
   "Scaffold job <id>_<slug>". The commit happens in the worktree
   (`git -C <job-dir>`) — the project root is never touched.

Non-git projects take a simpler path: plain directory under
`docs/jobs/`, no branch, no worktree, and the brief's `branch:` field
says `(no git)`.

The key architectural fact: **every job is a branch in its own worktree.**
The project root stays on `main`; the job's branch is checked out in a
sibling directory. That's what lets many jobs run in parallel without
"wrong branch checked out" states.

### Life: agents

The job's four files are the contract between the agents
(`docs/AGENTS.md` documents the flow):

- `brief.md` — filled by a human: what and why.
- `tasks.md` — written by `@analyst`: the atomic task breakdown.
- `implementation.md` — written by `@developer`: what was implemented.
- `verdict.md` — written by `@reviewer`/`@security`: pass/fail per task,
  and an `## Overall` verdict of APPROVED / REJECTED / NEEDS WORK.

Agents run *inside the container* on the job's worktree mount (walk 1,
step 3: `--job` remounts to the worktree). They edit the job's files; each
stage is a separate agent session.

### Automation: `mg jdi` and `orchestrate.Next`

`mg jdi` drives `@analyst` → `@developer` → `@reviewer` unattended. The
brains are in `internal/orchestrate/orchestrate.go`, and the design is
beautifully stateless: `Next` is "a pure function of state already
visible on disk/in git." Its inputs:

- `job.Stage()` — which of the five stages the job's files say it's in
  (define → plan → implement → review → finished).
- `verdictRounds` — how many `"[ID] verdict: ..."` commits exist
  (`git.CountVerdictCommits`).
- `latestCommitIsVerdict` — whether the branch tip *is* the verdict commit
  (`git.LatestCommitIsVerdict`), which distinguishes "reviewer rejected,
  developer hasn't responded yet" from "developer already fixed it."

`Next` is a `switch` over the stage with careful sub-cases for the
implement stage:

```go
case job.StageImplement:
	switch {
	case verdictRounds == 0:
		// first pass — developer's turn
		return Decision{Kind: RunAgent, Agent: agents.Developer, ...}
	case verdictRounds >= 2:
		// retry budget exhausted — hand to a human
		return Decision{Kind: StopNeedsHuman, ...}
	case latestCommitIsVerdict:
		// rejected once, developer hasn't answered — the one allowed bounce
		return Decision{Kind: RunAgent, Agent: agents.Developer, ...}
	default:
		// developer committed since the verdict — re-review
		return Decision{Kind: RunAgent, Agent: agents.Reviewer, ...}
	}
```

Because every decision re-derives from committed state, "re-running
mg-jdi against the same job after it was killed mid-loop is always safe"
— there's no in-memory state to trust or lose. That's the design lesson
of the whole package: **when you can derive state instead of storing it,
derive it.** (Chapter 04's `TestNext` and the coverage-guard test exist to
pin this exact function.)

`mg jdi` stops when `Next` says `StopFinished` (verdict APPROVED — it
never auto-merges) or `StopNeedsHuman` (brief not written, retry budget
exhausted, or an agent printed the `NEEDS-HUMAN-INPUT:` marker).

### Death: `mg done` and `mg delete`

`internal/job/finish.go` (`FinishJob`) archives a finished job, and its
structure mirrors `create.go` in reverse:

1. **Resolve** the branch + worktree from the id (same exact-then-prefix
   matching as everywhere else).
2. **Verify** — verdict status (warns when missing or not approved, with
   confirmations), the worktree is on the right branch, the tree is clean.
   Each check has the old script's exact wording.
3. **Archive inside the job's own worktree** — move `docs/jobs/<id>_<slug>`
   to `docs/jobs/archive/<id>_<slug>`, rewrite `status: open` → `status:
   done` in the archived brief, commit: "archive: <name>".
4. **Squash-merge** — switch the main worktree to the base branch,
   `git merge --squash <branch>`, and commit the whole job as one commit
   with message `<title>\n\nJob: <name>`.
5. **Cleanup** — remove the worktree (skipped if the job's branch was
   checked out in the main worktree itself) and delete the branch.

`internal/job/delete.go` (`DeleteJob`) is the destructive cousin: same
resolution, then a "This cannot be undone." confirmation (with a
dirty-worktree warning), force-remove the worktree, force-delete the
branch — no merge, no archive. Non-git projects get a plain directory
delete. Both share helpers (`askConfirm`, `jobNotFoundError`,
`briefTitle`) — the shared-wording discipline again.

Both functions take a `ConfirmFunc` (`func(prompt string) (bool, error)`)
so the CLI wires the interactive `cli.Confirm` while the TUI passes its
own pre-approved prompt — the interface pattern from chapter 05 in
production.

## The mental map

After these two walks, you should be able to navigate the repo by
question:

| "What does X do?" | Look in |
|---|---|
| "How does a session start?" | `internal/session/` + `cmd/mg/session.go` |
| "How do git things work?" | `internal/git/` (the only place) |
| "How are jobs created/archived/deleted?" | `internal/job/` |
| "How does mg-jdi decide what's next?" | `internal/orchestrate/` |
| "Where are settings stored?" | `internal/config/` (personal), `internal/project/` (project) |
| "Where's the TUI?" | `internal/ui/` (Bubble Tea models) |
| "How does a command get its flags?" | `cmd/mg/` per-subcommand files + `cmd/mg/flags.go` |

One more habit worth naming: **the comments in this codebase tell you
what the script it replaced did** ("run.sh's ...", "new-job.sh's ...",
"finish-job.sh's ..."). Every ported function names its ancestor. When you
see one of those, you know the behavior was inherited deliberately and
pinned by tests — don't "simplify" it casually.

**Next:** [10 — Your turn: contributing to manigot](10-your-turn.md) —
concrete exercises to start changing this code yourself.
