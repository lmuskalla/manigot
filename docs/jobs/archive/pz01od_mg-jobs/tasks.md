# Tasks: mg jobs

id: pz01od
status: open
analyst: systems-architect
date: 2026-08-13

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Add `mg jobs` to the dispatcher switch and to the `mg -h` help text
     files: cmd/mg/main.go
     depends: none
     risk: low — mechanical switch-case + help-block edit; no existing test
       passes "jobs" as argv[0] (verified), so dispatch is unambiguous
     notes: switch case calls runJobs(args[1:], os.Stdin, os.Stdout,
       os.Stderr, cli.IsTerminal(os.Stdin)) — same shape as runAgents. Help
       block: "mg jobs  List jobs and pick one to start a session in" (place
       it near mg job, matching the existing one-line command style).

TASK-2: Implement runJobs in a new cmd/mg/jobs.go — list open jobs with
        state, pick one interactively on a TTY, then launch a session for it
     files: cmd/mg/jobs.go (new)
     depends: TASK-1
     risk: medium — new command surface; follow the locked decisions below
       and mirror mg agents / mg job wording exactly
     notes: behavior spec is "Locked decisions" below. Column widths can
       follow the TUI's listColumns (id 8, status 6, type 8, date 12) with a
       plain-space layout (no styling — this is a plain-text CLI list).

TASK-3: Add tests for runJobs in cmd/mg/jobs_test.go, mirroring
        agents_test.go's hermetic fixtures
     files: cmd/mg/jobs_test.go (new)
     depends: TASK-2
     risk: medium — fixtures must exercise job.Discover's working-tree
       fallback (non-git temp dir with docs/jobs/<name>/brief.md); git-
       worktree-backed discovery is already covered by the job package's own
       tests
     notes: cover: list rendering (columns + jdi badge when a
       .manigot/jdi-status/<name>/status sidecar exists), non-TTY refusal,
       TTY selection → re-exec launch line, empty list, missing-project
       error. Reuse the agents_test.go pattern of asserting on output
       substrings and exit codes; the re-exec assertion expects the go test
       binary to reject the flags and exit non-zero (same as
       TestAgentsSelectWritesChosenAndLaunches).

TASK-4: Sync docs — docs/AGENTS.md (dispatcher blurb + Commands list) and
        README.md (installed-commands table) with the new command; verify
        agents/*.md and project-template/docs/AGENTS.md need no change
     files: docs/AGENTS.md, README.md
     depends: TASK-1, TASK-2
     risk: low — mechanical doc edits; agents/*.md and the project template
       do not enumerate the command surface (verified — only `mg done`
       appears there)
     notes: docs/AGENTS.md: add `jobs` to the dispatcher line's command
       enumeration and a `mg jobs` bullet to the Commands list. README.md:
       add a `mg jobs` row to "The installed commands" table (after `mg
       job`). Keep wording in sync with the `mg -h` help text from TASK-1.

## Locked decisions (confirmed with the user)

These are settled — implement them as specified, no further user input needed.

- "list of jobs with state": mirror the TUI's list row — ID, status
  (open/done from brief.md), type, date, title, plus a plain-text mg-jdi
  activity badge ([running @<agent>] / [finished] / [needs human]) via
  job.ReadJDIStatus when present. Done jobs (docs/jobs/archive/) are
  excluded by job.Discover, same as the TUI. The workflow Stage
  (define/plan/implement/review/finished) is deliberately NOT shown — it
  stays a TUI detail-view concept.
- "select one for work": numbered menu (cli.Select, the mg agents pattern),
  then re-exec `mg --job <chosen-id> <passthrough>` in the foreground via
  the existing reexec helper — the session launcher mounts the job's
  worktree and prompts with its brief.md. passthrough is args after "jobs"
  (e.g. --agent / --profile), exactly like runAgents. Launch line wording:
  "→ Starting a session in <id>..." (mirrors agents' "→ Starting a session
  in @<agent>...").
- Non-TTY: print the list, then refuse to pick with an error, matching
  mg agents: "Error: mg jobs needs an interactive terminal to select a
  job." and exit 1.
- No project root (no docs/): error wording mirrors mg job: "Error: could
  not find project root (no docs/ directory found)." and exit 1.
- Empty list: print "No jobs yet — run 'mg job \"<title>\"' to create
  one." and exit 0.

## Verification

- `make mg && ./bin/mg jobs` from this repo's root (a job worktree with
  docs/): this job (pz01od_mg-jobs) must appear in the list; selecting it
  must re-exec the session launcher with --job pz01od.
- `go vet ./... && go test ./...` green.
- Non-TTY: `./bin/mg jobs < /dev/null` prints the list then the
  non-TTY refusal (exit 1).

