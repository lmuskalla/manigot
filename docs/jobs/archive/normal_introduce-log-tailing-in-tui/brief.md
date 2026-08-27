# Brief: introduce log tailing in tui

status: done
type: feature
id: normal
branch: feature/normal_introduce-log-tailing-in-tui
date: 2026-08-27
author: Leander Muskalla

## What

Recently, we've introduced caputing all logs from non-interactive agent sessions.
In the tui, e.g. if you'd ran mg jdi, we want to introduce an option to tail the currently running agent.
Check what shortcuts are still available in a job detail view. I'd imagine t for tail, but I'm not sure it's still available.
If you do press t (or whatever), I imagine a tmux split window (like with invoking agents, tig, etc.) to spawn which tails the logs of the current running agent.

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->

