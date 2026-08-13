# Brief: prevent worktree mess

status: done
type: feature
id: leg
branch: feature/leg_prevent-worktree-mess
date: 2026-08-13
author: Leander Muskalla

## What

Often times when I run my agents against a job, while testing stuff, they prune or remove worktrees.
There are two questions here:
1. Can we deny them the possibility to do so? The agents should only ever be able to read the git log and make commits, not mess with anything else
2. Is this especially stupid due to the way where we save worktrees and how we mount them into the container?

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->

