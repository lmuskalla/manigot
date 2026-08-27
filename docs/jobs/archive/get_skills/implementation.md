# Implementation: skills

id: get
status: open
developer:
date:

<!-- Produced by @developer after implementation. -->

## Summary

Implemented the skills mechanism for manigot, mirroring how agents are
mounted/stored: global skills live in the manigot checkout at
`<home>/skills/<name>/SKILL.md`, project skills in `docs/skills/<name>/SKILL.md`,
and both are delivered into every session (container mounts + `mg host`
delivery) at the CLI-global skills locations both CLIs load at startup — so a
skill is available to every agent and to invocations that name no agent,
exactly as the brief asks. Because a skill is a *directory* (SKILL.md plus
support files) rather than a single file, new directory-level helpers were
added (`listSkills`/`stageGlobalSkills`/`copyDir`) instead of reusing the
single-file agent conversion machinery. Skills need no frontmatter conversion
(both CLIs read SKILL.md natively), so delivery is a plain copy/symlink.

## Changes

TASK-0: Decided and documented the storage layout — global `<home>/skills/`,
project `docs/skills/` (mirroring `agents/` and `docs/agents/`), project
overrides global, and manigot ships no skills of its own (mechanism only).
Documented in `docs/AGENTS.md` (Stack bullet, `internal/home` bullet) and
`README.md` (repo/project layout entries, new "## Skills" section).

TASK-1: `internal/session/skillconv.go` (new) — `listSkills(srcDir)` enumerates
every immediate subdirectory containing a `SKILL.md` as a `<name>/` → dir pair,
sorted by name; missing/empty dirs are not errors. Tests in
`internal/session/skillconv_test.go` (new).

TASK-2: `internal/session/skillconv.go` — `stageGlobalSkills(srcDir)` copies
every skill (directory + support files, recursively) into a fresh
`manigot-skills-*` temp dir, returning it for a read-only container mount; the
caller removes it via the invocation's Cleanup hook, and the host source is
never modified. `copyDir(dst, src)` is the recursive directory copy (symlinks
copied by content, never recreated, so a staged copy can't carry an
out-of-tree link). Mirrors `convertAgents`' temp-dir lifecycle. Tests added.

TASK-3: `internal/session/docker.go` — global skills mounted read-only in
`BuildDockerInvocation`: Claude Code gets the host's `skills/` dir verbatim at
`/home/claude/.claude/skills:ro`; OpenCode gets a staged copy at
`/home/claude/.config/opencode/skills:ro` with the temp dir cleaned up via the
invocation's Cleanup hook (now combined with the agent cleanups). Skips
cleanly when the checkout has no `skills/` dir. Tests in
`internal/session/docker_test.go` pin the exact argv strings.

TASK-4: `internal/session/docker.go` — project skills (`docs/skills/`) need no
staging or shadow mount: the existing docs bind mount maps the whole `docs/`
dir, so `docs/skills/` already lands at the project skills location
(`/workspace/.claude/skills` for Claude Code, `/workspace/.opencode/skills` for
OpenCode), where both CLIs natively prefer a project skill over a global one of
the same name. Documented in a comment block and pinned by a test
(`TestBuildProjectSkillsRideDocsMount`) asserting no separate skills mount is
added.

TASK-5: `internal/session/host.go` — `installHostGlobalSkills(tool)` delivers
the checkout's global skills to the host CLI for `mg host`: symlinked skill
dirs for Claude Code (`~/.claude/skills/`), copied dirs for OpenCode
(`~/.config/opencode/skills/`), never clobbering an existing host skill name,
nothing created when `skills/` is absent, best-effort per-skill failures that
warn and never block the session. Wired into `BuildHostInvocation` with an
"Installed : N global skill(s)" diag line mirroring the agents install. Tests
in `internal/session/host_test.go`.

TASK-6: `Dockerfile` — pre-create `/home/claude/.claude/skills` and
`/home/claude/.config/opencode/skills` (claude-owned, opened by the final
`chmod -R o+rwX /home/claude`), mirroring the existing agents dirs block, so
the new mounts never land on root-owned parents; comment updated to cover
agents + skills.

TASK-7: Synced documentation — `docs/AGENTS.md` (internal/session bullet
describes skills staging/mounting, Dockerfile bullet mentions `skills/`, `mg
host` command lists skills delivery, hard rule mentions the skills mechanism),
`README.md` (profile table gains Global/Project skills rows, per-profile
paragraph on skills mounts + override, Host mode gains a Skills bullet,
per-project-setup sentence, Skills section), and
`project-template/docs/AGENTS.md` (skills paragraph mirroring the agents one).
No `agents/*.md` changes were needed — skills are loaded globally by the CLIs,
transparent to agents.

## Known issues / follow-ups

- **manigot ships no skills of its own** (TASK-0 decision, conservative): the
  checkout has no `skills/` dir, so the feature is dormant until a user drops a
  skill into `<home>/skills/` or `docs/skills/`. Every path skips cleanly when
  absent. Shipping built-in skills (e.g. a `commit`-message or `review` skill)
  is a natural follow-up.
- The OpenCode container mount stages a copy per session rather than a verbatim
  bind (mirrors the agents pattern and keeps the CLI's skills dir a fresh,
  disposable snapshot). The Claude Code path is a verbatim read-only bind, so
  edits to `<home>/skills/` are reflected immediately for Claude sessions.
- TASK-4 relies on the CLIs' native project-skills location inside the docs
  mount; this was verified against the documented skill locations of both CLIs
  and pinned by argv tests, but not exercised inside a live container (no
  docker available in this environment).
- `internal/session/root_test.go` has a pre-existing gofmt deviation (one
  function signature sharing a line) — untouched, out of scope.