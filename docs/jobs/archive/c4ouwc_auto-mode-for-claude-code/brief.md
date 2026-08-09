# Brief: Auto mode for claude code

status: done
type: feature
id: c4ouwc
branch: feature/c4ouwc_auto-mode-for-claude-code
date: 2026-08-09
author: Leander Muskalla

## What

When launching sc (either an agent via TUI, agent via cli or just generally via cli), if using claude code, claude code should start in auto mode and already assume the directory is safe.

## Why

Right now you have to confirm the safety of the dir every time which makes no sense in the context of safecode, we already specifically launch an isolated environment. Then you always have to switch to auto mode to let the agent do its thing.

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->
