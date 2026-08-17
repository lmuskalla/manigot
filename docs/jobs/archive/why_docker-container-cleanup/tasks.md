# Tasks: docker container cleanup

id: why
status: open
analyst:
date: 2026-08-17

<!-- Produced by @analyst from brief.md. -->

## Decisions this breakdown locks in

The brief's open question — "by which rule and at which point?" — is resolved
here, with reasoning, rather than left for `@developer` to invent
mid-implementation.

**1. Rule — remove only EXITED containers whose name matches the manigot
prefix.** Every session runs `docker run --rm --name manigot-<project>-<pid>`
(`internal/session/docker.go`), so containers self-remove on normal exit;
orphans are purely the residue of abnormal ends (killed client, killed
pane/window, host/daemon restart, hung CLI). Cleanup therefore removes only
exited containers whose name matches the manigot prefix (`docker ps -aq
--filter name=^/manigot- --filter status=exited` + `docker rm`). Running
manigot containers are never touched — a killed pane/window can leave an
unattended agent that is still working — and foreign containers are out of
scope. This mirrors `docker container prune` semantics, scoped to manigot's
own containers.

**2. Point — automatically before every container launch, plus an explicit
`mg prune` command.** (a) Every launch path — bare mg, mg init's @prompter
hand-off, mg agents/mg jobs launches, TUI-launched sessions, and every mg jdi
invocation — funnels through `runSession` (cmd/mg/session.go) or
`commandAgentRunner.Run` (cmd/mg/jdi.go), so pruning there self-heals any
crash residue on the next invocation. (b) An explicit `mg prune` subcommand
covers on-demand or cron use. A background timer was considered and rejected:
mg is the only creator of these containers, so launch-time cleanup is both
regular and sufficient, and a timer would add host machinery the codebase
does not have.

## Task breakdown

TASK-1: Add a PruneOrphans helper in internal/session that lists exited
     containers whose name matches the manigot prefix (`docker ps -aq
     --filter name=^/manigot- --filter status=exited`) and removes them with
     `docker rm`, returning the removed and running counts; docker missing or
     the daemon down degrades to a warning, never an error; running manigot
     containers and all foreign containers are never touched; expose an
     overridable docker-exec seam for tests (the cmd/mg/diff.go tigLookPath
     pattern).
     files: internal/session/prune.go, internal/session/prune_test.go
     depends: none
     risk: medium — new host-side docker shell-out; the launch path must never
     break when docker is unavailable

TASK-2: Wire the prune into every container-launch path before the run:
     runSession (bare mg, mg init's prompter re-exec, mg agents/mg jobs
     re-exec, TUI-launched sessions) and commandAgentRunner.Run (mg jdi) call
     PruneOrphans with their diag writer; a prune failure only warns on stderr
     and never aborts the launch. mg host is unaffected — it never creates a
     container (BuildHostInvocation).
     files: cmd/mg/session.go, cmd/mg/jdi.go, cmd/mg/session_test.go (new),
            cmd/mg/jdi_test.go
     depends: TASK-1
     risk: low — two additive call sites behind a fail-soft helper

TASK-3: Add the explicit `mg prune` subcommand: new cmd/mg/prune.go, dispatch
     + help entry in cmd/mg/main.go; runs PruneOrphans, prints the removed
     count and "nothing to prune" when empty, reports (without removing) how
     many running manigot containers exist, and exits 1 with a clear error
     when docker is missing or the daemon is down.
     files: cmd/mg/prune.go, cmd/mg/prune_test.go, cmd/mg/main.go
     depends: TASK-1
     risk: low — new subcommand reusing the tested helper; only the
     docker-unavailable exit-code branch is new

TASK-4: Sync the user-facing docs with the new behavior: docs/AGENTS.md
     (Commands list plus a short container-cleanup note in the session-launch
     section) and README.md (command table row + usage line).
     project-template/docs/AGENTS.md's Commands section is per-project
     placeholders ([test command] etc.), NOT a mirror of mg's command list —
     nothing to add there.
     files: docs/AGENTS.md, README.md
     depends: TASK-2, TASK-3
     risk: low — doc-only, mirrors the orphaned-worktree cleanup doc precedent

TASK-5: Verify with go build ./..., go vet ./..., and the full go test ./...
     suite, and sanity-check the docker filters against a real daemon: exited
     manigot-* containers are removed, running manigot-* and foreign
     containers are left untouched.
     files: (none — verification)
     depends: TASK-2, TASK-3
     risk: low

TASK-6: Investigate the stray implementation.md at the worktree root — a
     leftover from the prior "mg done dirty worktree" job (its content
     documents that fix, and it is not in .gitignore). Check with git
     status/log whether it is tracked; if untracked in this worktree, remove
     it so mg done's clean-tree check can pass; if committed on the base
     branch, leave it and note it as out of scope.
     files: /workspace/implementation.md (possibly deleted)
     depends: none
     risk: low — housekeeping, but directly relevant to this job's own finish
     path
