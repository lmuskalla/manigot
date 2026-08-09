# Brief: adjust tui to be dynamic

status: done
type: feature
id: bvi7n6
branch: feature/bvi7n6_adjust-tui-to-be-dynamic
date: 2026-08-08
author: Leander Muskalla

## What

The TUI currently expects soft links to safecode, new-job, etc. However, first of all, not everybody wants to do the ln -s route. Second of all, if they were, new-job would be a way too generic command. It needs to be safecode-specific. E.g. I'm currently using ~/.zshrc aliases resulting in sc, sc-job, sc-done.

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->
