# Brief: Set default branch in settings

status: done
type: feature
id: 6kbt43
branch: feature/6kbt43_set-default-branch-in-settings
date: 2026-08-11
author: Leander Muskalla

## What

Currently, all committing and merging behaviour is based on the fixed assumption that main is always the origin branch. However, in many projects you'll work on features in the development branch, etc. If that is the case, you basically cannot use the TUI at all.

Therefore I suggest that we make this configurable. Instead of merging from and to main (also the m shortcut on the dashboard), this should be configurable in the settings section. It can still default to 'main'.

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->
