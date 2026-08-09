# Safecode — Development Todo

This document tracks planned features and improvements.
Work on each item should follow the standard job workflow (sc-job → brief → tasks → implement → review).

---

## Priority 2: Agent scaffolding (`new-agent`)

A script to create new agents without having to manually write frontmatter.

- [ ] Create `scripts/new-agent.sh`
- [ ] Prompt for: name, description, role, tools (checklist), scope (global or project)
- [ ] Generate correctly formatted `.md` file with frontmatter and placeholder system prompt
- [ ] Place in `safecode/agents/` (global) or `your-project/docs/agents/` (project-specific)
- [ ] Print path to created file so it can be opened immediately
- [ ] Update README

---

## Priority 4: Skills

Reusable prompt fragments that agents can reference — e.g. "how we write Laravel controllers", "our Svelte component conventions".

- [ ] Define what a skill looks like in this context (a `.md` file? frontmatter + content?)
- [ ] Decide on directory: `safecode/skills/` (global) and `your-project/docs/skills/` (project)
- [ ] Bake global skills into image alongside agents
- [ ] Mount project skills into container alongside agents
- [ ] Update agent prompts to mention skills are available and where to find them
- [ ] Create `new-skill` script analogous to `new-agent`
- [ ] Update README

---

## Priority 5: MCP server support

Give agents access to external context via MCP — database schemas, git history, documentation, APIs.

- [ ] Research which MCP servers are most useful for a Laravel + Svelte project
- [ ] Define how MCP servers are configured per project (likely `docs/mcp.json` or similar)
- [ ] Pass MCP config into container at runtime
- [ ] Ensure MCP server access is scoped appropriately — blast radius implications
- [ ] Consider per-agent MCP access (reviewer gets DB read-only, developer gets more)
- [ ] Update README

---

## Ongoing / housekeeping

- [ ] Rename the project (name TBD — see naming discussion)
- [ ] Publish to GitHub with public README
- [x] Add `make install` target that sets up symlinks automatically (bvi7n6)
- [ ] Consider a `sc list-jobs` command for quick overview without TUI
- [ ] Git user config inside container (name + email) so developer agent commits don't fail
- [ ] Update example job files to reflect `docs/processes/` path (currently says `docs/templates/processes/`)
