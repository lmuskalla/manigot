## Summary

Job ids are now English words (e.g. "fun"), which reads as prose inside the
job list. This job adds a `#` prefix wherever a job id is *displayed* to the
user, so the list shows `#fun` instead of `fun`. Machine-consumed id usages
stay raw: `PickerRow.ID`, the `--job`/`-j` CLI argument, `launch.Jdi`/
`launch.Agent` calls, git commit subjects, and the id_slug job *name* shown by
`mg done` / `mg delete` / the session banner.

## Changes

TASK-1: Prefixed the TUI job list's ID column with `#` and widened the id
column from 12 to 13 so the longest word id (12 chars, e.g. "unemployment")
plus the `#` still renders untruncated.
- `internal/ui/list.go`: `listColumns()` id 12 → 13; `renderJobRow` pads
  `"#"+j.ID`; updated the column-width doc comment.
- `internal/ui/list_test.go`: `TestRenderListWordIDNotTruncated` now uses the
  12-char word "unemployment" and pins `#unemployment` rendering untruncated.

TASK-2: Prefixed the TUI detail view's meta line id with `#` (the
"id · status · type · date" line becomes "#id · …").
- `internal/ui/detail.go`: meta line now renders `#<id> · …`.
- `internal/ui/detail_test.go`: added
  `TestDetailViewMetaLinePrefixesIDWithHashtag` pinning
  "#fun · open · feature · 2026-01-01" in the rendered meta line.

TASK-3: Prefixed `#` in the `mg jobs` plain listing rows and the interactive
picker label, and to the picker SearchKey (so typing `#fun` filters). Kept
`PickerRow.ID` raw — the picker's chosen ID resolves the job and re-execs
`mg --job <id>`.
- `cmd/mg/jobs.go`: listing row format `#%-8s`; picker label `#%-8s`;
  `SearchKey: "#" + j.ID + " " + j.Title`; `ID: j.ID` unchanged.
- `cmd/mg/jobs_test.go`: updated pinned listing strings ("1) #def02",
  "2) #aaa01"), picker SearchKey "#def02 Beta Job", and the label assertion
  to `#def02`.

TASK-4: Prefixed `#` to the id in `jobsLaunchLine` (the
"→ Starting a session … for <id>…" line before a picked job launches). The
re-exec args built by `jobsLaunchArgs` keep the raw id.
- `cmd/mg/jobs.go`: `jobsLaunchLine` renders `#<id>` (both the agent-ful and
  agent-less variants).
- `cmd/mg/jobs_test.go`: updated `TestJobsLaunchLine` and every
  `TestJobsSelectStageLaunchOutput` wantLine to the `#`-prefixed form.

## Known issues / follow-ups

none
