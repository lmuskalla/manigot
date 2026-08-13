# Verdict: prevent worktree mess

id: leg
status: open
reviewer: reviewer
date: 2026-08-13

## Review

The four-layer defense is implemented as specified, both round-2 blockers from
the previous review are correctly fixed, and I found no remaining blockers.
Static review only: this environment's own session rules (the read-only
reviewer agent) restrict bash to git read/commit commands, so `go build` /
`go test` could not be executed here — every new test was traced by hand
against the code and the test helpers, and is coherent.

TASK-1: PASS
notes: scripts/entrypoint.sh lines 110-226. PATH-first git shim installed
      after the entrypoint's own `git config --global` calls (ordering the
      task required). Allowlists read + commit subcommands; refuses worktree,
      branch -d/-D/-m/-M/-c/-C/-f/--delete/--move/--copy/--track/
      --set-upstream/--unset-upstream/--edit-description, config writes
      (write-flags and >=2 positionals), symbolic-ref writes (>=2 positionals),
      and every non-allowlisted subcommand. Parses leading global options
      (-C/-c/--git-dir/--work-tree/--namespace/--config-env, both separated
      and =value forms) before locating the subcommand; execs the real git by
      absolute path (no recursion). Traced the deny paths by hand: log/status/
      diff/show/add/commit/-C forms pass, worktree/reset/branch -D/config
      write/symbolic-ref write are denied. Non-blocking notes (all previously
      flagged and accepted): `git branch <name>` creation is allowed (only the
      destructive forms denied), `git tag` (incl. reads) and `git --version`
      are denied wholesale, `git commit --amend` is allowed, `git branch -u`
      (upstream set) slips past the branch flag check — none affect the
      worktree-protection goal, and the shim is a documented soft layer
      (determined agents can exec the real git by absolute path).

TASK-2: PASS
notes: internal/session/agentgit.go resolves the active agent's file (project
      docs/agents/ override wins over the global agents/ file, mirroring
      agentlist.Discover) and reads the `commit:` marker; absent/unknown/no-
      agent/missing-file all default to writable so a committing agent is
      never broken. internal/session/docker.go mounts Root.GitCommonDir :ro +
      -e GIT_OPTIONAL_LOCKS=0 for non-committing agents, :z otherwise. I
      cross-checked every agent's marker against its own instructions: the
      only files instructing git add/commit are developer.md, reviewer.md and
      quality.md — all three are `commit: true`; the eight read-only agents
      (analyst, architect, designer, devops, mentor, owner, prompter,
      security) are `commit: false` and none of them are instructed to commit.
      No misclassification remains. Round-2 blocker verified fixed:
      agents/quality.md line 5 is `commit: true` (it commits quality.md like
      reviewer commits verdict.md). Tests pin ro/rw/project-override/unknown
      cases; agentconv_test.go verifies `commit:` survives the OpenCode strip.
      Docs synced (docs/AGENTS.md lists quality among the committing agents;
      project-template now describes the writable default correctly).

TASK-3: PASS
notes: agents/reviewer.md and agents/security.md dropped the broad
      `git branch *` allow (which matched `git branch -D <branch>`) and the
      bare `git branch` allow, replacing them with `git branch --show-current`
      (+ `*` form), and list the destructive-git denies after the allows
      (last-match-wins, matching the pre-existing `"*": deny` + allows
      pattern). agents/developer.md (previously unrestricted under OpenCode)
      got a `"*": allow` base with the same denies. analyst.md already had
      bash: deny — no change, as documented. agentconv_test.go exercises
      deny-rule passthrough. Non-blocking: the OpenCode deny set omits the
      short forms `-m`/`-c`/`-f` (only `--move*`/`--copy*`/`-d*`/`-D*` are
      denied) — but the git shim (which applies under OpenCode too) denies
      those, so the second layer is complete. The unknown `commit:` frontmatter
      key's acceptance by the installed opencode binary was live-verified per
      implementation.md (b7e6d88 documents the scratch-agent check).

TASK-4: PASS
notes: internal/session/docker.go mounts <GitCommonDir>/hooks :ro after the
      gitdir mount in job-worktree sessions, skipped when the source is
      missing (docker would otherwise create an empty root-owned dir). Only
      job sessions carry overlays (root.GitCommonDir is set only in the --job
      path, internal/session/root.go). Nested ro-over-ro / ro-over-z mounts
      follow the codebase's existing contextMount pattern. Tests: hooks
      present, missing, and plain non-job sessions.

TASK-5: PASS
notes: internal/git/git.go WorktreeGitDirs enumerates every linked worktree's
      gitdir via `git worktree list --porcelain` (the same parsing
      WorktreeForBranch uses) plus a per-worktree
      `git rev-parse --path-format=absolute --git-dir` (pre-2.31 relative
      fallback), excluding the caller's own worktree (currentPath) and the
      main worktree (gitdir == common dir), and skipping unresolvable/
      prunable entries. docker.go mounts each other job's gitdir :ro (skipping
      missing sources), leaving the current job's gitdir writable for commits.
      The current-worktree exclusion is robust: root.go stores
      filepath.Clean(WorktreeForBranch's porcelain path), so the comparison is
      byte-identical in the normal case. Tests cover
      current/main exclusions, prunable skip, not-a-repo, and the docker argv
      pins (other overlaid ro, current never, common dir never). Enumeration
      once at launch — a job created mid-session is not covered until the next
      launch, documented in docs/AGENTS.md.

Doc sync: PASS
notes: docs/AGENTS.md describes all four layers (shim, ro mount + commit:
      marker, OpenCode deny set, overlay mounts) consistently with the
      implementation. Round-2 blocker verified fixed: project-template/docs/
      AGENTS.md now states the writable default for no-agent/missing/unknown
      marker, matching the implementation and the main docs/AGENTS.md. The
      root AGENTS.md overlay reflects the same text. The known-issues section
      of implementation.md owns the accepted residuals (mg host isolation,
      soft shim, stale overlay list, branch-creation allow).

Scope: PASS — every changed file is within the task scope (agents/*.md,
      docs/AGENTS.md, project-template, scripts/entrypoint.sh,
      internal/session/{docker,agentgit,docker_test,agentconv_test}.go,
      internal/git/{git,git_test}.go); no unrelated refactors (home.go,
      agentlist, host.go untouched, as the out-of-scope mg host note
      requires). Commit discipline: PASS — one `[leg] TASK-N:` commit per
      task in order (597c11d..4fdec4b), separate `[leg] implementation:`
      commits, a `[leg] verdict: needs work` commit, and the two round-2 fix
      commits (acb27e5, 534972b) plus the implementation doc update (b7e6d88).

## Security

The layered design is sound: a hard read-only git-common-dir boundary for the
non-committing agents (TASK-2), the soft shim for the committing agents
(TASK-1), and the ro overlays shrinking the gitdir blast radius for the ones
that must keep it writable (TASK-4/5). The two round-2 blockers were
functional regressions, not security holes, and both are fixed. Residual,
accepted risks are documented in docs/AGENTS.md and implementation.md: mg
host sessions have no isolation; a determined agent can exec the real git by
absolute path or write the mounted gitdir; the current job's own worktree
gitdir and the main worktree's HEAD/index stay writable for committing agents
(a determined agent could still corrupt its *own* worktree registration —
out of TASK-5's stated scope, which covers other jobs' registrations only).

## Overall

APPROVED

No blockers. The two round-2 blockers are verified fixed (quality.md
`commit: true`; project-template writable-default wording), and my review
found no new issues requiring changes before merge.

Non-blocking observations for the record (none merge-blocking):
- The analyst is now read-only per TASK-2's explicit design ("only developer
  and reviewer get commit: true"). Consequence: tasks.md written by @analyst
  stays uncommitted and is swept into the developer's first `git add -A`
  commit (as happened in this very job — tasks.md entered via the TASK-1
  commit). Stage detection reads disk, not git, so mg jdi is unaffected.
- The shim's allowlist slightly overstates "branch read-only" in docs
  (branch creation is allowed) — already owned by implementation.md's known
  issues.
- Verification gap (environmental): `go test`/`go build` could not be run in
  this session; tests were reviewed statically and traced against the helpers.
