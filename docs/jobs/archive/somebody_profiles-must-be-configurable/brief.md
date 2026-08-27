# Brief: profiles must be configurable

status: done
type: feature
id: somebody
branch: feature/somebody_profiles-must-be-configurable
date: 2026-08-22
author: Leander Muskalla

## What

We've just resolved an issue with profiles.
That brought up a new issue altogether.
manigot should work with each and every combination.
It's fine if it brings defaults, but it needs to enable the user create/delete profiles that apply to him (or not).
So currently I think we have 5 profiles. But I should be able to do a sort of mg settings command to add e.g. OpenCode Zen with a different model. Or OpenCode Go or z.Ai Coding Plan, etc.
Basically you have credentials for an API provider (opencode, claude). Those you can use with different models. Each combination is a profile.
But right now we are telling people what profiles we can have. Instead, given the set up credentials users should create and manage all kinds of profiles.
Otherwise you have to do manual code changes and then rebuild stuff which is not good at all.

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->

