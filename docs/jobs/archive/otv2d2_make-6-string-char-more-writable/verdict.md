# Verdict: Make 6 string char more writable

id: otv2d2
status: open
reviewer: opencode-go
date: 2026-08-13

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed against tasks.md on branch `feature/otv2d2_make-6-string-char-more-writable`
(diff `main...HEAD`, 18 files, all job-owned). `go build ./...`, `go vet ./...`,
and `go test ./...` are green. Per-task:

TASK-1: PASS — `internal/job/words.go` embeds 2,341 curated words (lowercase
a-z, ≤12 chars, no duplicates, no profanity — verified by re-scanning the
committed file); `words_test.go` asserts every invariant plus a ≥1000 floor.

TASK-2: PASS — `wordID()` replaces `randomID()` with a uniform crypto/rand
pick (uint16 rejection sampling: limit = 65536 − 65536%2341, so every word is
equally likely). `CreateOptions.RandomID` doc updated; injection seam intact.

TASK-3: PASS — `existingJobIDs(root)` covers all three homes of existing IDs:
open worktrees via `Discover`, archived via `docs/jobs/archive/` scan, and the
non-git working-tree fallback inside `Discover`. Set semantics make
double-visits harmless; missing `docs/jobs` or `archive` degrades to empty.
`CreateOptions.ExistingIDs` seam is injectable and defaults to the real scan.

TASK-4: PASS — `uniqueJobID` retries up to 100 draws, rejecting exact matches
and prefix relationships in both directions (verified both branches with
tests: existing "flower" rejects "flowerbed"; existing "flowerbed" rejects
"flower"). Exhaustion returns the documented "grow the dictionary" error; a
scan failure fails the create (never proceeds blind). Wired into `CreateJob`'s
default path only — no production caller passes `RandomID` (grep-verified), so
uniqueness is never bypassed outside tests.

TASK-5: PASS — TUI id column 8 → 12 in `listColumns()`; title-column budget
(63 fixed ≤ 72−16 floor) holds; all existing list tests rebuild cells via
`pad()`/`listColumns()` so nothing hardcodes the old width.

TASK-6: PASS — README job-workflow section and `Job.ID` comment updated;
template example dir renamed to `word-id_title-of-job` (no code referenced the
old name). No stale "6-char"/"random" references remain in `cmd/`, `internal/`,
or README (grep-verified).

TASK-7: PASS — picker membership, exact/prefix retry, exhaustion, scan-error,
real-scan wiring, archived-word never-reuse, and untruncated TUI rendering
all covered; every pre-existing test passes unchanged (`fixedID` injection
untouched).

Notes (non-blocking):
1. `docs/CODE_QUALITY.md` / `docs/CODE_QUALITY_TASKS.md` still describe the
   removed `randomID` modulo-bias item (4.1). They are a historical analysis
   backlog and were outside TASK-6's explicit scope; a follow-up doc touch
   would tidy them.
2. Parallel `mg job` invocations could theoretically mint the same word
   (~1/2341 per pair) → ambiguous `--job`; the same race shape existed with
   random IDs (far lower probability). No registry lock exists by design.
3. Environmental (not a code defect): the job worktree's git admin metadata
   (`.git/worktrees/otv2d2_...`) was wiped twice mid-session by external
   host-side activity (main advanced; orphaned-worktree tooling merged). All
   commits were safe in the shared object store; the registration was
   recreated by hand. If this recurs before `mg done`, finish will fail with a
   clear "no git worktree" error (not silent corruption) — re-register the
   worktree as described in implementation.md before merging.
4. Full-word `--job` lookups are always unambiguous by construction (the
   prefix rule prevents ID-to-ID prefix pairs); partial-input ambiguity
   against slugs is pre-existing prefix-match behavior ("type more" UX).

## Security

No new attack surface. Entropy remains crypto/rand; the word list is static,
curated data (no external input); generated IDs are constrained to `[a-z]{1,12}`
by the picker and enforced by the invariant test, so they cannot inject into
branch/dir names; no new input parsing or file writes beyond the existing
scaffold path. Pass.

## Overall

APPROVED — all 7 tasks implemented as specified, per-task commits in the
correct `[otv2d2] TASK-N:` format, clean tree, full suite green. Nothing
blocks merge.
