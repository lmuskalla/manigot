# Verdict: skills

id: get
status: open
reviewer:
date:

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed on branch `feature/get_skills`, base `main` (from
`.manigot/manigot.json`). Diff `main...HEAD` cross-referenced against tasks.md
— the changed files are exactly the ones the tasks list (plus this job's own
docs); no out-of-scope changes, no unrelated refactors (`root_test.go` is
untouched).

TASK-0: PASS
notes: Layout decided and documented exactly as tasks.md recommends: global
`<home>/skills/<name>/SKILL.md` (mirrors `agents/`), project
`docs/skills/<name>/SKILL.md` (mirrors `docs/agents/`), project overrides
global, and manigot ships no skills of its own (mechanism only, provisioned
for user-supplied skills). Documented in `docs/AGENTS.md` (Stack bullet,
`internal/home` + `internal/session` bullets, Dockerfile bullet, `mg host`
bullet, hard rule) and `README.md` (repo/project layout, profile table,
per-profile paragraph, Host mode, new "## Skills" section). Consistent with
the "docs and project-template describe the same system" hard rule.

TASK-1: PASS
notes: `internal/session/skillconv.go` — `listSkills(srcDir)` enumerates only
immediate subdirectories containing a `SKILL.md`, returns `<name>/` → dir
pairs sorted by name, and treats a missing/empty source dir as an empty
result, not an error. `skillconv_test.go` covers discovery, sorting, non-skill
dirs, stray files, and the nested-`SKILL.md`-is-not-a-skill case.

TASK-2: PASS
notes: `stageGlobalSkills` stages every discovered skill (directory +
support files, recursively) into a fresh `manigot-skills-*` temp dir, returns
`("", false, nil)` when there is nothing to stage, removes the temp dir on
error, and leaves cleanup to the caller's Cleanup hook. `copyDir` copies
symlinks by content so a staged copy can never carry an out-of-tree link.
Tests cover staging of nested support files, no-skills, non-skill exclusion,
and source-tree immutability.

TASK-3: PASS
notes: `BuildDockerInvocation` mounts global skills read-only: Claude Code
gets the host's `skills/` verbatim at `/home/claude/.claude/skills:ro`;
OpenCode gets a staged copy at `/home/claude/.config/opencode/skills:ro`
(verified against opencode's documented global skills dir), cleaned up via
the invocation's combined Cleanup hook. Skips cleanly when the checkout has
no `skills/` dir (both tools). Existing argv pins are unaffected (their fake
checkouts have no `skills/`), and new tests pin the exact new mount strings
plus the no-skills negative cases.

TASK-4: PASS
notes: Project skills need no staging or shadow mount — the existing docs
bind (`docs/` → `/workspace/.claude` / `/workspace/.opencode`) lands
`docs/skills/` exactly at both CLIs' native project-skills locations
(`.claude/skills/`, `.opencode/skills/` — the opencode path confirmed against
opencode's documented project-skills location). The "project overrides global
of the same name" behavior is delegated to the CLIs' native precedence and was
not exercised in a live container (no docker in the dev environment) — the
developer flagged this honestly in implementation.md, and it is the
conservative, sanctioned choice per tasks.md's notes. The structural delivery
(project skills visible at the project skills location, overriding per CLI
semantics) is correct and pinned by `TestBuildProjectSkillsRideDocsMount`.

TASK-5: PASS
notes: `internal/session/host.go` — `installHostGlobalSkills(tool)` symlinks
each skill dir into `~/.claude/skills/` for Claude Code and copies each skill
dir into `~/.config/opencode/skills/` for OpenCode, never clobbering an
existing host skill name (Lstat skip), creating nothing when `skills/` is
absent, and skipping per-skill failures best-effort. Wired into
`BuildHostInvocation` with the same warn-only / "Installed : N global skill(s)"
diag discipline as `installHostGlobalAgents`. `host_test.go` covers symlinks,
copies, non-clobber of a user's own skill, no-skills (no side effects), and
both tools' `BuildHostInvocation` integration.

TASK-6: PASS
notes: `Dockerfile` pre-creates `/home/claude/.claude/skills` and
`/home/claude/.config/opencode/skills` (claude-owned via the existing chown,
opened to any session UID by the final `chmod -R o+rwX /home/claude`), so the
new read-only mounts can never land on root-owned parents. Comment updated to
cover agents + skills.

TASK-7: PASS
notes: `docs/AGENTS.md`, `README.md` and `project-template/docs/AGENTS.md` all
synced and mutually consistent (storage layout, mount targets, override
semantics, `mg host` delivery, "ships no skills"). No `agents/*.md` changes
were needed — verified: no agent file references the skills mechanism, and
skills are loaded globally by both CLIs independent of the active agent, which
is what makes them available to every agent and to agentless invocations.

## Security

No security findings. Global skills are mounted read-only in both container
paths (verbatim bind and staged copy are both `:ro`), so an agent can never
modify the host's `skills/` source tree; the host-side `installHostGlobalSkills`
writes only into the user's own CLI config dirs and never clobbers existing
content. No new environment or credential surface. The skills temp staging dir
is removed via the invocation's Cleanup hook.

## Overall

APPROVED

The skills mechanism fulfills the brief: global skills stored in the manigot
checkout (`<home>/skills/`), project skills in `docs/skills/`, both delivered
into every session (container mounts + `mg host` delivery) at the CLIs'
global skills locations, so every agent and every agentless invocation can use
them. All eight tasks are implemented as specified, with test coverage
pin-pinning the load-bearing docker argv and host-install behavior.

Non-blocking observations (no change required before merge):

1. `copyDir` copies symlinks by reading their target content; a broken symlink
   or a symlink pointing at a directory inside a skill makes `stageGlobalSkills`
   fail, which hard-errors the entire OpenCode container session launch —
   while `installHostGlobalSkills` (host path) skips such skills best-effort.
   The container path could align with the host path (skip + warn) for
   robustness; the current failure is loud and user-correctable, so it is not a
   blocker.
2. `copyDir` writes files with mode 0o644 — a skill support script that relied
   on an executable bit loses it in the staged/copied delivery (model-invoked
   `bash script.sh` still works).
3. Claude Code mounts `<home>/skills/` verbatim even when it contains no skill
   dirs, while OpenCode skips via `stageGlobalSkills` — cosmetic asymmetry,
   harmless in both cases.
4. TASK-4's project-overrides-global precedence is delegated to CLI-native
   behavior and was not exercised in a live container (already flagged in
   implementation.md); the failure mode if a CLI does not dedupe by name is
   benign (both skills advertised).