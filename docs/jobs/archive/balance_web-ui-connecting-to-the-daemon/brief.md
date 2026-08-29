# Brief: web-ui: connecting to the daemon..

status: done
type: fix
id: balance
branch: fix/balance_web-ui-connecting-to-the-daemon
date: 2026-08-29
author: Leander Muskalla

## What

Some issues with the new web tui.
After connection setup, it doesn't show the projects in the dropdown top-left, only after reload. (yes i know, on hist host i currently have no projects set up but thats not the issue)
Then, several pages just show "Connecting to the daemon.." and nothing else. The daemon health page e.g. shows it, even though the API requests returns validly.
There is a bug with displaying this message and not grabbing actual responses. If the response is an error, we need to indicate that to the user.

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->

