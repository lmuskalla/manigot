# Tasks: add best practice skills

id: soil
status: open
analyst:
date: 2026-08-27

<!-- Produced by @analyst from brief.md. -->

## Background

`get_skills` (archived, APPROVED) built the skills mechanism and deliberately
shipped **no** skills of its own: global skills live in the manigot checkout at
`<home>/skills/<name>/SKILL.md`, project skills in `docs/skills/<name>/SKILL.md`,
both delivered into every session — read-only container mounts at the CLI-global
skills locations (`~/.claude/skills/` for Claude Code, `~/.config/opencode/skills/`
for OpenCode, staged copy for OpenCode), and for `mg host` symlinked/copied into
the host CLI's skills dir, never clobbering a user's own skill of the same name.
Both CLIs read `SKILL.md` and its frontmatter (`name:`, `description:`) natively;
a skill's **name is its directory name** (the identifier both CLIs address the
skill by). The only shipped skill today is the example `skills/job-brief/`.

This job is the explicit follow-up the `get_skills` implementation flagged as
"a natural follow-up": **research the ecosystem and ship a small set of
high-quality skills.** The brief is deliberately open about which skills ("Do
online research … most respectable, most proven and most competent … not as many
as possible, but only those who are high quality"), with one hint from the
author: `ui-ux-pro-max-skill`. The selection is therefore delegated to the
developer, but against explicit criteria (below) so the reviewer can check the
choice against the brief.

## Scope and constraints

In scope: researching, curating and shipping 2–5 high-quality skills into
`skills/<name>/` (global, shipped with manigot), plus the doc-sync those
shipments make necessary.

Out of scope (explicitly NOT doing): any change to the skills delivery
mechanism (`internal/session` docker/host mounts, staging, `mg host` installs —
all shipped and approved in `get_skills`); any change to agents (`agents/*.md`);
any new Go code except an optional validation unit test; project skills in
`docs/skills/` (that is for user projects, not the manigot checkout).

Selection criteria (the developer applies these; the reviewer checks them):

1. **Provenance**: from a reputable, actively-maintained source — Anthropic's
   official `anthropics/skills` repo, the most-established community indexes
   (`awesome-claude-skills` et al.), or the skill's own well-known repo.
   Prefer original/official sources over third-party mirrors.
2. **Transferable craft value**: skills that make agents better at real work
   (design/UI-UX, web app testing, code review, security review, debugging,
   technical writing, architecture, git). Do NOT add skills that merely
   restate manigot's own conventions — job workflow, git discipline and code
   quality are already covered by `docs/AGENTS.md` and the `job-brief` skill.
3. **Tool-agnostic**: must work under BOTH Claude Code and OpenCode. No
   Claude-only `/` slash commands, no `@agent` mentions, no OpenCode-only
   syntax; plain markdown instructions relying on the standard tool set
   (Read/Grep/Glob/Write/Edit) both CLIs provide.
4. **Self-contained**: a skill is its directory — any scripts/templates must
   live inside `skills/<name>/` (only the skill dir is delivered). Avoid
   skills that assume tools absent from the container image.
5. **License**: permissive (MIT/Apache-2.0/CC-BY-* family) and compatible
   with shipping in the manigot checkout; provenance + license recorded per
   skill. Reject anything unclear or incompatible.
6. **Few, not many**: ~2–5 skills. Every skill must earn its place; when in
   doubt, leave it out.

The author's named example `ui-ux-pro-max-skill` must be evaluated against
these criteria explicitly — include it if it passes (verify its license and
Claude-specificity; adapt the body if the adaptation is small), otherwise
document why it was rejected.

## Task breakdown

TASK-1: Research the skill ecosystem and curate the shortlist
     files: none committed by this task itself — the output (the shortlist)
         drives TASK-2's `skills/<name>/` dirs; the selection rationale is
         recorded in implementation.md and in TASK-3's provenance notes
     depends: none
     risk: medium — the judgment-heavy core of the brief; the criteria above
         must be applied consistently because the reviewer will check the
         choice against "quality over quantity"

TASK-2: Vendor each selected skill into `skills/<name>/` as a normalized,
     tool-agnostic, self-contained SKILL.md directory
     files: `skills/<name>/SKILL.md` (+ any support files) for each adopted
         skill; frontmatter `name:` must exactly match the directory name and
         `description:` must be one line (both CLIs read these natively; the
         name is the identifier)
     depends: TASK-1
     risk: medium — the content quality of these directories IS this job's
         deliverable; they must load under both CLIs, stay self-contained, and
         not duplicate manigot's own conventions

TASK-3: Record provenance and license for every adopted skill
     files: `skills/<name>/PROVENANCE.md` per adopted skill (upstream source
         URL, license, what was adapted from the original); keep provenance
         OUT of the SKILL.md frontmatter (both CLIs read frontmatter natively —
         do not rely on unknown keys being ignored)
     depends: TASK-2
     risk: low — record-keeping; the only risk is an incompatible license
         slipping through, which the review/security stage re-checks

TASK-4: Verify the shipped skills load cleanly and keep the build green
     files: `skills/**` (fixes surfaced by validation); optionally a small
         unit test in `internal/session/` following the existing fake-home
         patterns if it fits cleanly
     depends: TASK-2, TASK-3
     risk: medium — a live-container smoke test may be impossible in this
         environment (the predecessor job had no docker), so verification may
         be limited to static checks + `go test ./...`; if so, flag it
         explicitly in implementation.md exactly as `get_skills` did

TASK-5: Sync the documentation that currently says manigot ships no/one skill
     files: `docs/AGENTS.md` (Stack bullet and hard-rule phrasing),
         `README.md` (skills line + "## Skills" section); verify
         `project-template/docs/AGENTS.md` and `agents/*.md` need no changes
     depends: TASK-2 (final skill list)
     risk: low — documentation only, but must stay consistent with the hard
         rule that docs, README, project-template and agents describe the same
         system

## Notes for the developer

- The analyst has no network access (`webfetch`/`websearch` denied); the
  research is yours. The container has full egress (the `network:` guardrail
  is not yet enforced).
- Research sources to start from (not exhaustive — the research decides):
  Anthropic's official `anthropics/skills` repo, the `awesome-claude-skills`
  community index, OpenCode's own skills documentation/examples, and
  `ui-ux-pro-max-skill` (the author named it).
- The frontmatter contract is minimal and load-bearing: `name:` must equal the
  directory name and `description:` must be a single line — that is what the
  CLIs' skill pickers surface. Put everything else in the body or a support
  file.
- Security: adopted skills are injected into every session, so vet each one
  for prompt-injection or harmful instructions before shipping; @security
  double-checks in review.
- Keep `go test ./...` green at each step. The docker-argv and host-install
  behavior are test-pinned and must not change (TASK-4 is validation-only).
- If the brief's latitude on skill choice feels too wide at any point, prefer
  the conservative reading (fewer, better-vetted skills) and say so in
  implementation.md rather than expanding scope.