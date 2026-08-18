# Tasks: hashtag before job id

id: habit
status: open
analyst:
date: 2026-08-18

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

Brief: job ids are now English words (e.g. "fun"), which reads as prose
inside the job list. Add a `#` prefix wherever a job id is *displayed* to the
user, so the list shows `#fun` instead of `fun`.

Scope boundary (do not change): machine-consumed id usages stay raw —
`PickerRow.ID`, the `--job`/`-j` CLI argument, `launch.Jdi`/`launch.Agent`
calls, git commit subjects (`[id] type: summary` is a documented agent
convention in agents/*.md and shows bracketed in git log anyway), and the
id_slug job *name* (e.g. "fun_alpha") shown by mg done / mg delete / the
session banner. Only the bare id word rendered to the user gets the `#`.

TASK-1: Prefix the TUI job list's ID column with `#` and widen the id column from 12 to 13 so the longest word id (12 chars, e.g. "unemployment") plus the `#` still renders untruncated
     files: internal/ui/list.go, internal/ui/list_test.go
     depends: none
     risk: medium — the column-width change (12 → 13) shifts layout, and the existing TestRenderListWordIDNotTruncated must be updated to use a 12-char word with the `#` prefix to keep pinning non-truncation

TASK-2: Prefix the TUI detail view's meta line id with `#` (the "id · status · type · date" line becomes "#id · …"), and pin it in a test
     files: internal/ui/detail.go, internal/ui/detail_test.go
     depends: none
     risk: low — a one-place display change; existing detail tests assert on branch/content, not the bare id

TASK-3: Prefix `#` to the id in the `mg jobs` plain listing rows and the interactive picker label, and to the picker SearchKey (so typing `#fun` filters); keep `PickerRow.ID` raw — the picker's chosen ID resolves the job and re-execs `mg --job <id>`
     files: cmd/mg/jobs.go, cmd/mg/jobs_test.go
     depends: none
     risk: medium — several pinned test strings (listing "1) def02", picker SearchKey "def02 Beta Job") must be updated; the raw-`ID` field is load-bearing for the launch path and must not change

TASK-4: Prefix `#` to the id in `jobsLaunchLine` (the "→ Starting a session … for <id>…" line before a picked job launches)
     files: cmd/mg/jobs.go, cmd/mg/jobs_test.go
     depends: TASK-3 (same file — do together to avoid edit conflicts)
     risk: low — display-only wording; the re-exec args built by jobsLaunchArgs keep the raw id
