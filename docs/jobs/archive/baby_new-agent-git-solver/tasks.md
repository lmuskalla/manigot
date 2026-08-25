# Tasks: new agent: git solver

id: baby
status: open
analyst: claude
date: 2026-08-25

<!-- Produced by @analyst from brief.md. -->

## Context

The brief asks for one new global agent: a git expert who knows how to
resolve tricky git/git-worktree situations, merge safely, and clean up —
with read, write, and git (Bash) access. "Why" and "Out of scope" are empty,
but "What" is concrete enough to act on.

Established pattern for adding a global agent (precedent: `5trk01_create-new-
agents`, `shake_new-agent-chat`) is a single new file `agents/<name>.md` —
nothing else. A new file there is picked up automatically everywhere
(`mg agents`/`agentlist.Discover`, the TUI's `a` picker, the Dockerfile's
`COPY agents/` + OpenCode conversion loop) with no code change.

**Important architectural constraint the developer must respect, not work
around:** every container session — regardless of which agent is running —
gets the PATH-first git shim (`scripts/entrypoint.sh`) that refuses
`worktree`, `branch -d/-D`, `reset`, `clean`, `gc`, `prune`, `reflog`,
`push`, `fetch`, `pull`, `checkout`, `switch`, `restore`, `stash`, `remote`,
`update-ref`, destructive `tag`, `merge`/`rebase`, etc., and job-worktree
sessions additionally get read-only overlay mounts over the git-common-dir's
`hooks/` and every *other* job's worktree gitdir. These are deliberate,
project-wide isolation boundaries (see "Session git shim" and "Read-only git
mount for non-committing agents" in `docs/AGENTS.md`) — they are not
per-agent and this job must not attempt to carve out an exception for the
new agent, in its frontmatter or otherwise; that would be a much larger,
security-relevant architecture change and is out of scope here. Concretely:
inside a container session the new agent can diagnose problems, resolve
merge conflicts (edit files, `git add`/`git commit`), and inspect history —
exactly the same read + commit surface every other committing agent has —
but it **cannot** actually fix a broken worktree registration, force-remove
a stuck worktree, hard-reset, or delete a stray branch from inside the
container. The agent's own instructions must say so plainly, and point the
user at `mg host` (which runs the CLI directly on the host, unisolated, by
design — see the "`mg host`" bullet in `docs/AGENTS.md`) for that class of
operation. This keeps the new agent honest about what it can do in each
context instead of implying capabilities the platform deliberately blocks.

Two things deliberately NOT in scope (same precedent as `devops`/`sysadmin`/
`chat`):
- The TUI action bar (`internal/ui/agents.go` `agentMeta`/`agentOrder`) is a
  hardcoded five-agent job-flow subset (owner/analyst/developer/reviewer/
  security). Do not add the new agent there, and do not touch
  `internal/agents/agents.go` or `internal/orchestrate/` — this is a
  standalone utility agent, not part of the brief → tasks → implementation →
  verdict flow or the `mg jdi` sequence.
- The container image must be rebuilt (`make rebuild`) before the new agent
  exists inside running containers — an ops step, not a code change; note it
  in `implementation.md`'s "Known issues / follow-ups".

Naming: the brief calls it "a git expert" / "git solver"; this task list
uses the filename `agents/git-solver.md` (`name: git-solver`) to match the
job's own title. If the developer judges a different name reads better as a
global agent name, flag it rather than silently deviating — naming is a
product decision, not purely mechanical.

Doc-sync scope (verified): `README.md`'s Agents section ("Thirteen agents
are available globally..." + the agent table) and `docs/ROADMAP.md`'s
current-state paragraph ("thirteen agents") both enumerate/count the
roster and need updating. `docs/AGENTS.md` and `project-template/docs/
AGENTS.md` do not enumerate agents by name/count, so neither needs a roster
change.

## Task breakdown

TASK-1: Create `agents/git-solver.md`, a new global agent that is the git
expert: resolving tricky git/git-worktree states, merge conflicts, and
cleanup, and merging safely. Frontmatter as the siblings (`name: git-solver`
matching the filename; one-line `description:`; `tools: Read, Write, Edit,
Bash, Grep, Glob`, matching the brief's "read, write rights and git";
`commit: true`, since it edits files and makes commits like `developer`) plus
a `permission:` block for OpenCode identical in shape to `developer.md`'s
(broad `edit`/`bash` allow, with the same explicit deny list for `git
worktree*`, `git branch -d*`/`-D*`/`--delete*`/`--move*`/`--copy*`, `git
reset*`, `git clean*`, `git gc*`, `git prune*`, `git reflog*`, `git push*`,
`git fetch*`, `git pull*`, `git checkout*`, `git switch*`, `git restore*`,
`git stash*`, `git remote*`, `git tag -d*`, `git update-ref*`) — this agent
gets no special exemption from the platform's destructive-git-command
denylist. Body in the existing style (role statement, what it covers —
diagnosing detached HEADs/broken merges/conflicted rebases/stray worktree
registrations, how to approach a request, safe-merge and cleanup guidance),
and must explicitly state the container-shim limitation from the Context
section above: within a job/container session it can diagnose, resolve
conflicts by editing + committing, and inspect history, but destructive
worktree/branch/reset/clean/push operations are refused by the session's git
shim regardless of this agent's role — for that class of fix, tell the user
to re-run it via `mg host`. Also include the same "Branch" check section the
other job-aware agents have, and the standard hard rules (no push, no merge,
no touching other branches).
     files: `agents/git-solver.md` (new)
     depends: none
     risk: low — a new standalone file with an established schema; nothing
       in the repo (code or tests) enumerates the global roster by content,
       so nothing can break by adding a file. The one real risk is scope
       creep: writing agent instructions that imply it can bypass the git
       shim/worktree isolation, which would be misleading rather than
       merely stylistic — the task description above is explicit about this
       to keep it out.

TASK-2: Update `README.md`'s Agents section so the documented roster matches
the filesystem: change "Thirteen agents are available globally in every
project." to "Fourteen", and add a `@git-solver` row to the agent table
(role: a one-line summary of resolving tricky git/worktree situations and
cleanup; Tools (Claude Code): read + write), keeping the table's existing
column style, wording tone, and row ordering (new agents have so far been
appended at the end of the table).
     files: `README.md`
     depends: TASK-1
     risk: low — a mechanical doc edit; the only subtlety is keeping the
       stated count and the table row count consistent with each other.

TASK-3: Update `docs/ROADMAP.md`'s current-state paragraph ("...ntfy
notifications, thirteen agents, a strong test suite...") to "fourteen
agents", matching TASK-1's addition.
     files: `docs/ROADMAP.md`
     depends: TASK-1
     risk: low — a one-word count sync in a summary paragraph; no
       behavioral content.

TASK-4: Verify the change end to end: run `go test ./...` (all packages use
hermetic temp fixtures/checkouts, so a new agent file should not affect
them — this is verification only), confirm the new agent shows up via
`agentlist.Discover`/`mg agents`, and explicitly confirm the "not part of
the TUI workflow, no shim exception" constraints hold: no changes to
`internal/ui/agents.go` (`agentMeta`/`agentOrder`), `internal/agents/
agents.go`, `internal/orchestrate/`, `Dockerfile`, or
`scripts/entrypoint.sh`.
     files: none (verification only)
     depends: TASK-1, TASK-2, TASK-3
     risk: low — no production code is changed by this task itself; the
       only risk is discovering an unexpected dependency on the roster that
       the earlier analysis missed, which would be surfaced here and
       reported rather than worked around.
