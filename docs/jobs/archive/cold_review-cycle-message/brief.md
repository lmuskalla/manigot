# Brief: review cycle message

status: done
type: feature
id: cold
branch: feature/cold_review-cycle-message
date: 2026-08-17
author: Leander Muskalla

## What

Currently, when a job in jdi mode comes back twice from the reviewer, we stop and write out "needs human". That's fine in itself, but "needs human" in our setup can mean any number of things. Sometimes things crash, then we don't know better.
But if we deliberately stop because we're going back and forth between developer/reviewer, let's add a message. E.g. needs human: review cycle".

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->

