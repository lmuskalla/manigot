# Implementation: add best practice skills

## Summary

Researched the agent-skills ecosystem and shipped four curated, high-quality
skills into the manigot checkout's `skills/` directory, each vendored as a
normalized, tool-agnostic, self-contained `<name>/SKILL.md` directory with a
`PROVENANCE.md` license/adaptation record. Also synced the documentation that
previously said manigot ships no/one skill.

## Selection rationale (TASK-1)

Researched: Anthropic's official `anthropics/skills` repo, the
`ComposioHQ/awesome-claude-skills` index, `obra/superpowers` (the largest
battle-tested community collection), and the author-named
`ui-ux-pro-max-skill`. Applied the brief's six criteria (provenance, transferable
craft value, tool-agnostic, self-contained, permissive license, few-not-many).
Chose the conservative reading: 4 skills, each from a reputable actively
maintained source, each covering a craft area the brief lists and manigot's own
docs do not already mandate.

Adopted (one-line rationale):

1. **frontend-design** (anthropics/skills, Apache-2.0) — UI/UX design craft:
   distinctive, intentional visual direction. Pure markdown, fully
   tool-agnostic, zero runtime dependencies. The most reputable design skill
   available.
2. **webapp-testing** (anthropics/skills, Apache-2.0) — web app testing with
   Playwright (decision tree, server-lifecycle helper script, example scripts).
   The manigot container ships Python 3 + a Playwright browser, so it is
   self-contained up to a one-line `pip install playwright` (recorded as a
   prerequisite note in the skill body).
3. **test-driven-development** (obra/superpowers, MIT) — red-green-refactor
   craft with the "watch it fail" iron law and a good-tests reference. Pure
   markdown + one support file; manigot's docs do not currently mandate TDD.
4. **systematic-debugging** (obra/superpowers, MIT) — root-cause-first
   debugging (four phases, support files for backward tracing, defense in
   depth, condition-based waiting). Pure markdown + scripts.

Rejected (with reasons):

- **ui-ux-pro-max-skill** (author's named example; nextlevelbuilder, MIT,
  ~121k stars) — evaluated explicitly against the criteria and rejected:
  (1) **Claude-specific**: the SKILL.md's command paths use the Claude-plugin
  `${CLAUDE_PLUGIN_ROOT}` variable and assume the plugin/CLI install layout
  (`uipro init`); the adaptation is *not* small — every invocation path and the
  whole search-engine workflow would need rewriting. (2) **Not self-contained
  in manigot's sense**: it is a heavy data engine (hundreds of files: style/
  palette/typography CSV databases, a Python search script, design-system
  persistence) rather than a lean instruction set. (3) Its frontmatter
  `description:` is a multi-line run-on, violating the one-line contract both
  CLIs' pickers surface. (4) Its design-craft role is covered by the lighter,
  fully tool-agnostic, official `frontend-design` from anthropics/skills. The
  author's evident intent — "skills *like* ui-ux-pro-max-skill", i.e. a
  top-caliber design skill — is satisfied by that adoption.
- **doc-coauthoring** (anthropics) — interactive conversational workflow built
  on Claude-specific artifacts (`create_file`), Claude Code tool names
  (`str_replace`), and sub-agents; requires user answers at every section.
  Not tool-agnostic without substantial rewriting.
- **brand-guidelines** (anthropics) — applies Anthropic's own brand; not
  transferable craft for manigot users.
- **canvas-design / theme-factory / algorithmic-art / slack-gif-creator**
  (anthropics) — art/brand/slide/GIF production, lower transferable dev craft
  than the adopted four.
- **superpowers requesting/receiving-code-review, brainstorming,
  writing-plans/executing-plans, subagent-driven-development,
  dispatching-parallel-agents, using-git-worktrees, finishing-a-development-
  branch** — Claude Code subagent/slash-command-centric (fail tool-agnostic);
  code review is already covered by manigot's own `@reviewer` agent and docs.
- **software-architecture** (NeoLabHQ context-engineering-kit) — could not
  verify license or fetch the skill (404 on the advertised path); per
  "when in doubt, leave it out".

## Changes

TASK-1: research + curation. No files committed by this task itself; the
shortlist above drove TASK-2, and the rationale is recorded here and in each
skill's `PROVENANCE.md`.

TASK-2: vendored four skills as normalized, tool-agnostic, self-contained
directories under `skills/` (each `name:` frontmatter exactly matches its
directory name; each `description:` is one line):
- `skills/frontend-design/` — SKILL.md + LICENSE.txt, verbatim from
  anthropics/skills except the `license:` frontmatter key removed (manigot's
  frontmatter contract is `name:` + one-line `description:`; license lives in
  LICENSE.txt + PROVENANCE.md).
- `skills/webapp-testing/` — SKILL.md + `scripts/with_server.py` +
  `examples/{element_discovery,static_html_automation,console_logging}.py` +
  LICENSE.txt. Adaptations: `license:` key removed; a short prerequisite note
  added (`pip install playwright` if the Python client is missing); the two
  examples' Claude.ai-specific `/mnt/user-data/outputs/` paths replaced with
  `/tmp/`.
- `skills/test-driven-development/` — SKILL.md + `writing-good-tests.md`.
  Adaptation: dropped the `(superpowers:writing-skills)` cross-skill pointer
  (that skill is not shipped).
- `skills/systematic-debugging/` — SKILL.md + `root-cause-tracing.md`,
  `defense-in-depth.md`, `condition-based-waiting.md` and the two files those
  reference (`find-polluter.sh`, `condition-based-waiting-example.ts`).
  Adaptations: `superpowers:test-driven-development` → `test-driven-development`
  (shipped); the `superpowers:verification-before-completion` pointer replaced
  with a plain "verify the fix before claiming success" instruction; upstream
  internal development artifacts (`CREATION-LOG.md`, `test-*.md`) dropped.

TASK-3: `PROVENANCE.md` per adopted skill — upstream URL (with fetched commit
SHA), license, and exactly what was adapted. License texts: Apache-2.0
(anthropics skills, vendored as LICENSE.txt) and MIT (superpowers repo license,
Copyright (c) 2025 Jesse Vincent).

TASK-3 fix (post-review, NEEDS WORK): the review flagged that
`skills/systematic-debugging/PROVENANCE.md` did not record what was adapted in
`find-polluter.sh`. Re-verified byte-for-byte against the pinned upstream
commit (`b36e0829…`): the `${TEST_PATTERN#./}` strip and the dual `-path` with
the `**/`-collapsed fallback (lines 21-27) are **present upstream-verbatim**,
not manigot modifications — so the PROVENANCE.md now states that explicitly
instead of the vague "kept the support files" wording. Also recorded the one
real adaptation in that directory that was previously undocumented:
`root-cause-tracing.md` invokes `bash find-polluter.sh` rather than
`./find-polluter.sh` (staged copies strip the +x bit).

TASK-4: validation. Static checks: every `skills/<name>/` has SKILL.md at the
top level (the `listSkills` loader contract), frontmatter `name:` == directory
name, one-line `description:`; Python example/helper files compile; the bash
helper passes `bash -n`. `go test ./...` is green for all 15 packages (run
with the real git on PATH). Also fixed a runtime footgun surfaced by
validation: `root-cause-tracing.md` now invokes `find-polluter.sh` via
`bash find-polluter.sh`, because staged skill copies strip the +x bit.
No Go code changes; no new unit test added (the optional test was skipped in
favor of the conservative reading — the loader behavior is already
test-pinned and unaffected by skill content).

TASK-5: doc sync.
- `docs/AGENTS.md` — Stack bullet now lists the shipped skills; the hard rule
  "(even though manigot ships no skills of its own)" reworded.
- `README.md` — "## Skills" section now lists the shipped set with one-line
  roles.
- Verified `project-template/docs/AGENTS.md` (describes the mechanism
  generically, no count claims) and `agents/*.md` (no mechanism references)
  need no changes.

## Known issues / follow-ups

- **No live-container smoke test**: docker is not available in this
  environment, so the skills were not smoke-tested inside a real container
  session (same limitation the predecessor `get_skills` job flagged). Static
  validation + `go test ./...` passed instead; a future session with docker
  should launch a container and confirm the four skills surface in both CLIs'
  pickers.
- **`go test ./...` inside an agent container**: with the session git shim on
  PATH, many packages' test fixtures fail at `git init` (shim refuses it);
  this is a pre-existing environment artifact of running tests inside the
  agent container, unrelated to this job. Run tests with the real git on PATH
  (e.g. `PATH=/usr/bin:$PATH go test ./...`).
- **webapp-testing Python client**: the container ships Python 3 + a Playwright
  browser but not the Python `playwright` package; the skill records the
  one-line install (`pip install playwright`, network egress available) as a
  prerequisite.
- **Skill updates are manual**: vendored skills are snapshots at the recorded
  upstream commits; keeping them current (or pinning via a future mechanism)
  is a follow-up, not part of this job.