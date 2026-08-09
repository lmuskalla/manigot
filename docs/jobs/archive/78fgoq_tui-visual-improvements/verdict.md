# Verdict: tui visual improvements

id: 78fgoq
status: open
reviewer: claude
date: 2026-08-09

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `app.go` — `recentActivityFloor`/`recentActivityCeiling`, decoupled
fetch (`refreshRecentCommits` always fetches the ceiling) from render-time
sizing (`recentActivityShown`), guarded for `a.height == 0`. Verified the
`spare := a.height - 5 - len(a.jobs)` accounting against the actual chrome
(`renderList`) line-by-line — it's correct and doesn't overflow. Confirmed
via `go test` and by manually exercising `renderActionBar`/`renderList` at a
range of heights.

TASK-2: PASS
notes: `renderJobSummary` + `spareHeaderRoom`, additive, no borders, only
shown when room remains and the list is non-empty. Counts bucket everything
non-"done" as "open", a reasonable simplification of "open/done counts" per
the brief's own wording.

TASK-3: PASS
notes: `list_test.go` covers sparse-scales-up, full-list-keeps-floor, and
zero-height-no-panic, matching the task exactly. Existing tests re-derived
against the new sizing math rather than left passing by accident.

TASK-4: PASS
notes: `footer()` and `renderFooter()` both now concatenate hint + status;
the multi-line `cmdErrorText` exception preserved as scoped.

TASK-5: PASS
notes: coexistence tests added for both list and detail footers, plus a
regression test pinning the multi-line-status exception.

TASK-6: PASS
notes: `stripLeadingFrontmatter` correctly bounds stripping to the leading
block terminated by the first blank line; verified against all four real
`project-template` scaffolds and this job's own filled-in files. `d`/`D`-style
colon-shaped body content (e.g. `TASK-1: ...` inside `## Task breakdown`) is
correctly left alone. `filePlaceholder` heading removal is correct.

TASK-7: PASS
notes: end-to-end test across all four file types plus unit tests for the
fallback and placeholder cases. `TestDetailViewMetaLineShowsBranch`'s
coincidental-pass assertion was correctly identified and fixed to assert
against the chrome's actual off-branch format.

TASK-8: PASS
notes: `d`→`v` rename for Developer; unified `[key] Label` format for all six
buttons via `actionButton`; 80-column truncation verified by hand across a
width sweep (0–200 cols) — never panics, keys are never truncated, stays on
one line as documented. Truncation (not wrapping) was the right call given
`bodyHeight`'s single-line action-bar assumption, and that tradeoff is
documented in the function's doc comment as required.

TASK-9: PASS
notes: all three hardcoded-`"d"` tests updated to `"v"`; new tests for
`"d"` no longer resolving, unified format, and 80-column truncation all
present and passing.

TASK-10: PASS
notes: empty-state copy leads with `n`, never mentions quitting on that line;
footer below still legitimately lists `q quit`.

TASK-11: PASS
notes: isolates the empty-state line and asserts both the `n` mention and the
absence of "quit" on it.

All 11 tasks build (`go build ./...`), pass `go vet`, are `gofmt`-clean, and
the full `go test ./...` suite passes. Scope is clean — `git diff main...HEAD`
touches only the files named in tasks.md, no unrelated refactors, no stray
debug output.

## Process / commit-discipline finding (blocker)

`tasks.md` and part of `brief.md` were never committed to this branch's git
history, even though the working tree (and everything the developer built)
depends on them:

- `git diff main...HEAD -- docs/jobs/78fgoq_tui-visual-improvements/tasks.md`
  shows only the original empty scaffold (18 lines, the `new-job.sh`
  template) — the analyst's entire "Scope confirmation" + 11-task breakdown
  that this review and the developer's 11 per-task commits are built on
  exists **only as an uncommitted, unstaged working-tree change**
  (`git status` shows `M docs/jobs/.../tasks.md`). No commit in `git log --
  tasks.md` ever added analyst content — there is no `[78fgoq] tasks: ...`
  commit at all.
- `brief.md` has one commit (`a83d347 Updated brief`) that added the
  "Unresolved as of this revision" note, but the later "Decided: revisiting
  qge358's recent-activity strip" Notes section and the revised priority-1
  wording (the actual resolution TASK-1 implements) are **also only in the
  uncommitted working tree**, not in that commit or any other.

Practically: anyone who clones this branch fresh, or whose working tree gets
reset, loses the analyst's task breakdown and the brief's resolution notes
entirely — despite every developer commit message referencing `TASK-1`
through `TASK-11` by number and `implementation.md` (which *is* committed)
describing work against a `tasks.md` that git has no record of. This is a
break in the brief → tasks → implementation chain the job workflow depends
on, not a cosmetic gap.

This is independent of the code review above (which is clean) but must be
fixed before merge: commit the current working-tree state of `tasks.md` and
`brief.md` under this job's own commit convention (e.g.
`[78fgoq] tasks: adaptive strip + status coexistence + dedup + action bar +
empty-state task breakdown`, and a follow-up brief commit for the Notes
section) so the job's documentation trail is reproducible from git alone.

## Security

None run — no security-sensitive surface touched (pure TUI rendering, no new
I/O, no new external commands, no new parsing of untrusted input beyond the
existing local job-file reads `loadTab` already performed before this job).

## Overall

NEEDS WORK

The code itself is APPROVED — all 11 tasks match tasks.md, are correctly
implemented, well-tested, in-scope, and commit-disciplined at the task level.
One blocker before merge:

1. `tasks.md` (the analyst's full task breakdown) and the remainder of
   `brief.md`'s Notes section are uncommitted on this branch — commit them.
   Nothing in the code needs to change; this is purely a missing-commit gap
   in the job's own documentation trail.
