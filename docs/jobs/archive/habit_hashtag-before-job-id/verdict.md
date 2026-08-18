# Verdict: hashtag before job id

id: habit
status: open
reviewer: deepseek-v4-flash
date: 2026-08-18

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed `git diff main...HEAD` on branch `feature/habit_hashtag-before-job-id`
(base branch `main` per `.manigot/manigot.json`). The diff contains exactly the
four task files plus their tests plus the job's own docs (brief/tasks/
implementation/verdict scaffold) — nothing out of scope.

TASK-1: PASS
notes: internal/ui/list.go:143 — `listColumns()` id 12 → 13; list.go:278 —
`pad("#"+j.ID, cols.id)`; doc comment updated. Width math holds: the word-id
pool contract is pinned to `^[a-z]{1,12}$` (internal/job/words_test.go:11,
with "unemployment" in the list), so `#` + 12-char id = 13 chars fits the 13
column untruncated. list_test.go:559 `TestRenderListWordIDNotTruncated` now
uses the 12-char "unemployment" and pins `#unemployment`. Other list tests
assert on id-free substrings (status/stage/type cells) and remain valid under
the 1-char layout shift; no test pins the bare unpadded id.

TASK-2: PASS
notes: internal/ui/detail.go:661 — meta line renders `#%s · …`; the `#` sits
inside the single `dimStyle.Render(meta)` unit, so the asserted substring is
contiguous in the output. detail_test.go:384 adds
`TestDetailViewMetaLinePrefixesIDWithHashtag` pinning
"#fun · open · feature · 2026-01-01"; the fixture's explicit `id: fun`
frontmatter wins over the dir-name-derived id (job.go:159). Existing
`TestDetailViewMetaLineShowsBranch` still passes (asserts only the branch
segment).

TASK-3: PASS
notes: cmd/mg/jobs.go:67 — listing row `#%-8s`; jobs.go:111 — picker label
`#%-8s`; jobs.go:117 — `SearchKey: "#" + j.ID + " " + j.Title` (typing `#fun`
filters via the picker's case-insensitive substring match, and typing the raw
`def02` still matches as a substring — no filter regression); jobs.go:116 —
`ID: j.ID` kept raw, and the picker's chosen ID feeds `jobs[i].ID == id`
(jobs.go:140) and the re-exec. jobs_test.go pinned strings updated ("1) #def02",
"2) #aaa01", SearchKey "#def02 Beta Job", label "#def02").

TASK-4: PASS
notes: cmd/mg/jobs.go:209-214 — `jobsLaunchLine` renders `#%s` in both the
agent-ful and agent-less variants; the re-exec args `jobsLaunchArgs` (jobs.go:224)
still build `--job <raw id>`. jobs_test.go updated (`TestJobsLaunchLine` and
all seven `TestJobsSelectStageLaunchOutput` wantLines).

Scope boundary respected: `PickerRow.ID`, `--job`/`-j`, `launch.Jdi`/`launch.Agent`
(app.go:729/981/1081), git commit subjects (app.go:1203/1264), and the id_slug
job *name* surfaces (session banner session/docker.go:275 `root.Job` = branch
tail; mg done/mg delete; TUI confirm dialogs) all stay raw — verified unchanged
in the diff.

Commit discipline: one commit per task in `[habit] TASK-N: <desc>` format
(eccf252, 431b0e1, 02b7baf, 1e1d4f2) plus the separate implementation commit
(6405624); each commit touches only its task's files.

Caveat: I could not execute `go test` — this session's git shim restricts bash
to read+commit git commands. Verification is static; no compile/runtime issues
found (all helpers/functions referenced exist, format strings valid, tests
self-consistent with the changed code).

## Security

none — display-only changes; no new input surfaces, no privilege paths touched.

## Overall

APPROVED
