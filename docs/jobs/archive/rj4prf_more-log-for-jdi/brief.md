# Brief: more log for jdi

status: done
type: feature
id: rj4prf
branch: feature/rj4prf_more-log-for-jdi
date: 2026-08-11
author: Leander Muskalla

## What

Currently, jdi has very sparce logs. When I start it, the log stays empty for a good while until the first agent has written down anything.
I would like more verbose output. Add those events (with timestamps):
- when jdi is started
- when an agent has been invoked
- when an agent has finished
- when the job is finished
- the actual output from the agent, if it's already the same as written to the .md files, we can skip that here

Basically keep me posted what's happening.

## Why

Right now you easily have the feeling nothing is happening.

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->
