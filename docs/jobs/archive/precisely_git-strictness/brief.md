# Brief: git-strictness

status: done
type: feature
id: precisely
branch: feature/precisely_git-strictness
date: 2026-08-25
author: Leander Muskalla

## What

We have two issues with out manigot workflow considering git.
1. Most of the time, when I try to set it to done, it fails due to uncommitted stuff.
2. The reviewer often flags things as 'NEEDS WORK' just because a git commit guideline wasn't fully considered.

I'm not sure what needs to be done so every agent always commits his changes.
What I do want to change for sure is a less strict git guideline. When we do mg done, everything is merged in one commit anyway.
It's nice to have clear commits while working on a job, but in the end it doesn't matter as much and we don't want to invoke a whole new agent round just because a commit wasn't done right.
As long as functionally and quality-wise everything is correct, let's just skip very strict git hygiene. Like I said, it'll merge into one commit anyway.
And if there is something we can do about agents always committing changes, let's do that as well.
Also, I have a feeling if I create a job and let it run via mg jdi, there'll always end up files not commited because no agents feels responsible.

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->

