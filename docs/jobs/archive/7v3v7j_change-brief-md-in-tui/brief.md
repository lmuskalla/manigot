# Brief: Change brief.md in TUI

status: done
type: feature
id: 7v3v7j
branch: feature/7v3v7j_change-brief-md-in-tui
date: 2026-08-09
author: Leander Muskalla

## What

When you create a new job in the TUI, it'll show you the brief.md file. However, if I have just created that file, it's only the template. If I wanted to edit it, I'd have to close the TUI and manually navigate to the .md file. The TUI should let me change the brief.md (e.g. via nano or vim) via a shortcut.

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

Scope decision (resolves tasks.md's "one file or all four?" open question):
the edit shortcut is scoped to `brief.md` only, not all four job files. The
brief's own trigger case is exclusively about editing a freshly-created
`brief.md` template; `tasks.md`/`implementation.md`/`verdict.md` are
agent-written outputs and should stay effectively read-only from the TUI for
now. The underlying wiring (editor resolution, `tea.ExecProcess`,
reload-on-return) is generic per-tab and gated by a single `editable` flag,
so extending the shortcut to the other tabs later is a small follow-up, not
a rework, if that's wanted.
