# Brief: agents location

status: done
type: feature
id: major
branch: feature/major_agents-location
date: 2026-08-27
author: Leander Muskalla

## What

Can we try to move the agents out of the docker image?
The task of the docker image should purely be isolation for workspaces.
Agents should live somewhere else, so you can use them with mg host as well.
Especially since we're going to do hooks, skills, etc. They need to live somewhere on the system and be mounted.

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->

