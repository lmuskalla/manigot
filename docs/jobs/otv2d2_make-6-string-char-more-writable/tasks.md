# Tasks: Make 6 string char more writable

id: otv2d2
status: open
analyst:
date:

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

Replace the random 6-character job ID (a-z0-9) with an English word (e.g. `flower`),
so `mg --job flower` is easy to type, guaranteeing the word is never re-used —
including against archived jobs (confirmed decision: never re-use; if the word
list is exhausted, grow the dictionary, never fall back to a suffixed word).

TASK-1: Add an embedded English word list for job IDs
     files: internal/job/words.go (new), internal/job/words_test.go (new)
     depends: none
     risk: low — pure data + invariant tests, no behavior change
     Spec: ~1,500 curated common English words; lowercase a-z only (no digits,
     hyphens, apostrophes); length 1–12 chars; no exact duplicates; no
     profanity/offensive terms; prefix-pairs allowed (the retry loop in TASK-4
     neutralizes them). words_test.go asserts every invariant.

TASK-2: Replace randomID() with a uniform word picker
     files: internal/job/create.go
     depends: TASK-1
     risk: low — contained swap; tests inject IDs so they're unaffected
     Spec: wordID() picks a uniformly random index into the word list via
     crypto/rand (keep the rejection-sampling shape). Update the
     CreateOptions.RandomID doc comment ("6-char job id (a-z0-9)" -> word id).
     Injection seam unchanged.

TASK-3: Enumerate existing job IDs for the never-reuse check
     files: internal/job/create.go, internal/job/discover.go
     depends: none
     risk: medium — must cover all three homes of existing IDs without misses
     Spec: existingJobIDs(root) returns a set of IDs from: open jobs via
     Discover(root); archived jobs via ReadDir(<root>/docs/jobs/archive/) ->
     ReadJob(dir).ID; the non-git working-tree fallback (already inside
     Discover). Errors propagate. Add injectable ExistingIDs func() to
     CreateOptions (nil = real scan) for tests. Old random IDs in the archive
     are ordinary set members — mixed formats handled naturally.

TASK-4: Retry loop for uniqueness + prefix-collision in CreateJob
     files: internal/job/create.go
     depends: TASK-2, TASK-3
     risk: medium — prefix predicate and exhaustion behavior need precision;
     must never hang or spuriously error
     Spec: when opts.RandomID is nil, run uniqueJobID(root): up to 100
     attempts; a candidate is rejected if it (a) exactly matches an existing
     ID, (b) is a string-prefix of an existing ID, or (c) has an existing ID
     as its string-prefix (guards the mg --job <word> ambiguity regression).
     On exhaustion: clear error telling the user to add words to
     internal/job/words.go. A failed existing-ID scan fails the create
     (never proceed blind). Injected RandomID still bypasses uniqueness.

TASK-5: Widen the TUI list ID column so word IDs aren't truncated
     files: internal/ui/list.go, internal/ui/list_test.go
     depends: none
     risk: low — width feeds a title-column budget and test assertions
     Spec: listColumns() id: 8 -> 12; update the columnWidths comment; verify
     the titleColsWidth budget keeps the >=16 title floor; update list_test.go
     assertions that hardcode column widths.

TASK-6: Update docs and comments to describe word IDs
     files: README.md, internal/job/job.go, internal/job/create.go
     depends: TASK-2 (conceptually)
     risk: low — documentation only
     Spec: README.md job-workflow section ("named with a 6-character random
     ID"), the Job.ID field comment in job.go, stale comments in create.go.
     Optional (flag, don't block): rename project-template's
     docs/jobs/6-char-random-id_title-of-job/ example dir.

TASK-7: Tests for word-ID generation and never-reuse
     files: internal/job/create_test.go, internal/job/words_test.go,
            internal/ui/list_test.go
     depends: TASK-1, TASK-3, TASK-4, TASK-5
     risk: low — test-only, but the archive-scan integration needs a scratch
     repo with an archived job
     Spec: picker returns only list members; retry avoids exact + prefix
     collisions via injected RandomID/ExistingIDs (existing "flower" rejects
     "flowerbed"; existing "flowerbed" rejects "flower"); exhaustion produces
     the documented error. Integration: scratch repo with an open job + an
     archived job -> the real scan never mints an existing or prefix-colliding
     word (deterministic via injected picker sequence). TUI renders a >8-char
     word untruncated. All existing tests keep passing.
