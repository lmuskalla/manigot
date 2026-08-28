# Brief: Fix tailing

status: done
type: fix
id: guest
branch: fix/guest_fix-tailing
date: 2026-08-28
author: Leander Muskalla

## What

When I try the new tail feature in a job detail view, it shows no stream. It seems buffered, so I'd get exactly what I get in the log view. No tail at all, no improvement.
I'm currently using opencode, so as I understood it, opencode does support tailing.
Check if we're actually grabbing the output or not.
Also, there should be a difference between the log (agent invoked, agent result, next agent invoked, etc.) and tailing. The full tail should also be written to a log, but to a different one, a more verbose one.

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->

