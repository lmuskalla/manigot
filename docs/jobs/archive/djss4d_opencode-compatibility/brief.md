# Brief: opencode-compatibility

status: done
type: feature
id: djss4d
branch: feature/djss4d_opencode-compatibility
date: 2026-08-08
author: Leander Muskalla

## What

Make this tool vendor-agnostic. It can run with claude code or opencode.

## Priority 1: OpenCode support

Make safecode work with OpenCode as an alternative to Claude Code.

- [ ] Add `--tool` flag to `run.sh` (values: `claude-code` (default), `opencode`)
- [ ] Install OpenCode binary alongside Claude Code in Dockerfile
- [ ] Bake global agents into both `~/.claude/agents/` and `~/.config/opencode/agents/` in Dockerfile
- [ ] Branch entrypoint.sh on tool — OpenCode needs no onboarding wizard bypass, just API key env vars
- [ ] Mount target changes per tool: `docs/` → `/workspace/.claude` for Claude Code, `/workspace/.opencode` for OpenCode
- [ ] Add OpenCode provider API key support to `.env` and `run.sh`
- [ ] Update README with OpenCode setup and usage


## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->
