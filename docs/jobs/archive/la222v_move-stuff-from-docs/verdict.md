# Verdict: move stuff from docs

id: la222v
status: in-progress
reviewer: reviewer (deepseek-v4-flash)
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed against the task breakdown produced by @analyst (the four-task list
from the analysis session; see note below about tasks.md) and the full diff
`git diff main...HEAD`.

TASK-1: PASS
notes: Commit `bce8e9a`. `docs/quotes.json` → `assets/quotes.json` is a 100%
rename — content verified byte-identical to `main`'s version (sha256
`307cea03…`). `scripts/run.sh` (lines 491, 495) repointed:
`QUOTES_FILE="$MANIGOT_ROOT/assets/quotes.json"`. Correct: the quote is
picked on the host under `$MANIGOT_ROOT` (manigot's own checkout), so the
new path resolves exactly where the file is read, and the container only
ever sees the selected quote via the `MANIGOT_QUOTE` env var
(`entrypoint.sh` line 115 confirms it never reads the file path). No
container-side or mount change needed.

TASK-2: PASS
notes: Commit `96369dc`. All three `docs/quotes.json` references in
`docs/NAMING.md` (flavor-text section lines 78–90, rap-sheet entry line
130) updated to `assets/quotes.json`. `git grep` confirms zero remaining
`docs/quotes.json` references in tracked files outside the job's own
records.

TASK-3: PASS
notes: No git commit — the deletions (`docs/package.json`,
`docs/package-lock.json`, `docs/node_modules/`, `docs/.gitignore`) were all
untracked/ignored files, so there was nothing git could record; the empty
commit was correctly avoided and the rationale documented in
implementation.md. Working tree verified: `docs/` now contains only
documents (`AGENTS.md`, `CLAUDE.md`, `NAME.md`, `NAMING.md`, `backlog.md`,
`jobs/`). The delete-vs-keep decision was the flagged decision point
(prior `kwd2ou` investigation had recommended keep); it was human-sanctioned
in this session before implementation, and nothing in the tracked repo
consumed the artifacts (no opencode plugin config exists anywhere). Note:
`git status`/diff cannot show these deletions — they live only in the
working tree, which is why `mg done`'s clean-tree check is unaffected.

TASK-4: PASS
notes: Verification documented in implementation.md and re-run during this
review: `bash -n scripts/run.sh` passes; quote-pick logic (grep/sed) parses
109 quotes from `assets/quotes.json`; `git status` clean; scope clean — the
diff touches only `assets/quotes.json`, `docs/NAMING.md`, `scripts/run.sh`,
and the job's own files.

Cross-checks: commit format `[la222v] TASK-N: …` and
`[la222v] implementation: …` respected; no secrets, no `.env`, no files
outside the brief's scope touched; `docs/jobs/archive/` left untouched.

Non-blocking observation (not a merge blocker): `tasks.md` was never filled
in — it still contains the scaffold placeholders, so the archived job
record will lack the analyst's task breakdown. Worth filling in from the
analysis session when convenient.

## Security

None — this change moves a static JSON asset and deletes untracked local
node artifacts. No credentials, no new code paths, no config changes. The
`@opencode-ai/plugin` SDK deletion removes unreferenced dependencies only
(no lockfile/package.json is tracked in the repo, so no supply-chain
surface is affected by the tracked diff).

## Overall

APPROVED

Nothing must change before this is merged. The four tasks fulfil the brief
— `docs/` is documents-only again (`quotes.json` moved to `assets/` with
its one consumer and its documentation repointed), the untracked node
artifacts are gone, and every verification passed.
