# Implementation: Make 6 string char more writable

id: otv2d2
status: open
developer: opencode-go
date: 2026-08-13

<!-- Produced by @developer after implementation. -->

## Summary

Replaced the random 6-character job ID (a-z0-9, e.g. `sd9yxa`) with an
English word (e.g. `flower`) so `mg --job flower` is easy to type. The word
is drawn uniformly from a new embedded ~2,300-word list and is **never
re-used**: the default ID path in `CreateJob` runs a uniqueness retry loop
that scans every existing job ID — open jobs in worktrees, archived jobs in
`docs/jobs/archive/`, and the non-git working-tree fallback — and rejects a
candidate that exactly matches, is a prefix of, or has as prefix any existing
ID (guarding the `--job` prefix-match ambiguity for word IDs). Exhausting the
list (100 attempts) is a clear error pointing at `internal/job/words.go`
rather than a suffixed fallback, per the confirmed "grow the dictionary"
decision.

Backward compatible: no code path validated the 6-char format, so old random
IDs (open and archived) keep working as ordinary members of the existing-ID
set, and every resolution path (`--job`, `mg done`, `mg delete`, commit
subjects, TUI) flows word IDs through unchanged.

## Changes

TASK-1: Added `internal/job/words.go` — an embedded slice of ~2,300 curated
common English words (lowercase a-z only, ≤12 chars, no duplicates, no
profanity; prefix-pairs allowed). `words_test.go` asserts every invariant.

TASK-2: Replaced `randomID()` with `wordID()` in `internal/job/create.go` — a
uniform crypto/rand pick over the word list (uint16 rejection sampling so
every word is equally likely). Updated the `CreateOptions.RandomID` doc. The
injection seam is unchanged.

TASK-3: Added `existingJobIDs(root)` in `internal/job/discover.go` — the set
of every job ID in use, covering open jobs via `Discover`, archived jobs via a
`docs/jobs/archive/` scan, and the non-git fallback inside `Discover`. Added
the injectable `CreateOptions.ExistingIDs` for tests.

TASK-4: Added `uniqueJobID` + `idAvailable` in `internal/job/create.go` —
the never-reuse retry loop (exact + prefix-collision rejection, 100-attempt
cap with a "grow the dictionary" error, scan failure fails the create) and
wired it into `CreateJob`'s default path. Injected `RandomID` still bypasses
uniqueness (test-only role).

TASK-5: Widened the TUI job-ID column from 8 to 12 in `internal/ui/list.go`
so words up to the 12-char list cap render untruncated (previously `pad()`
cut them).

TASK-6: Updated `README.md` (job-workflow section), the `Job.ID` comment in
`internal/job/job.go`, and renamed the project-template example dir
`6-char-random-id_title-of-job` → `word-id_title-of-job` (no code referenced
the old name).

TASK-7: Added tests — `TestWordIDReturnsListMembers`,
`TestCreateJobArchivedWordNotReused`, `TestCreateJobDefaultPathAvoidsTakenWord`
(real default path against a real scan), `TestRenderListWordIDNotTruncated`,
plus the TASK-4 unit tests (`idAvailable`, retry, prefix rejection,
exhaustion, scan-error). All existing tests pass unchanged (`fixedID`
injection is untouched).

## Known issues / follow-ups

- **Word list source**: the ~2,300 words were curated by hand (there is no
  system dictionary in the build environment). Quality is good but not
  frequency-ranked; a future pass could swap in a vetted frequency list. The
  list lives in `internal/job/words.go` and is embedded — growing it means
  editing that file and rebuilding (`make mg`), per the confirmed decision.
- **Namespace ceiling**: with the never-reuse policy (archived IDs included),
  a single project is capped at the list size minus prefix-locked words. At
  realistic job counts this is a non-issue; the documented remedy is growing
  `jobWords`.
- **`internal/ui/app.go` has a pre-existing gofmt nit** (a double blank line
  before `jdiStatusBadge`) that predates this job (last touched by `5c10509`
  on the base branch) — left untouched per the scope rules.
- **Mid-implementation incident**: the job worktree's git admin metadata
  (`$GIT_DIR/worktrees/otv2d2_...`) vanished mid-session (external to this
  work — all commits were safe in the shared object store). It was recreated
  manually (`gitdir`/`commondir`/`HEAD`) and the index rebuilt with
  `git reset`; the working tree was never touched. Worth a look if the
  host-side `mg` tooling prunes worktrees aggressively.
