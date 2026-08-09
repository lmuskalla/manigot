# Verdict: git view and switch

id: qge358
status: open
reviewer: claude
date: 2026-08-09

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review (post-fix re-review)

This is a re-review after the developer's fix for the previously-blocking
TASK-5 finding (recent-activity strip pushing job rows down). TASK-1 through
TASK-4 and TASK-6 were already PASS in the prior verdict and are unchanged by
the fix commits (`3c436eb`, `7395666`) — re-confirmed below by re-reading the
current diff, not just carried over.

TASK-1: PASS
notes: `git.RecentCommits` (`tui/internal/git/git.go`) — single multi-ref
`git log --source -n <n> <branches...>` call, current-branch-first tie-break,
`ErrNotARepo` / empty-slice degrade. Unchanged since prior review.

TASK-2: PASS
notes: `tui/internal/git/recentcommits_test.go` covers dedup, ordering,
truncation, fewer-than-n, empty repo, non-repo. Unchanged, still green.

TASK-3: PASS
notes: `App.currentBranch` refreshed at all three named call sites, rendered
in `renderList`'s header, empty on non-repo/detached HEAD. Unchanged.

TASK-4: PASS
notes: `m` key in `updateList`, dispatches `checkoutCmd("main")`, status
reported for both real-switch and already-on-main no-op, refusal path
surfaces `cmdErrorText`. Footer hint updated. Unchanged, tests green.

TASK-5: PASS (fix verified)
notes: The fix (`3c436eb`) applies the brief's own named fallback exactly as
the prior verdict required: `recentActivityCount` dropped 5 → 1, and
`renderList` now has the activity strip take over the header's existing blank
spacer line (`if activity != "" { write activity } else { write "\n" }`)
instead of appending after it. Verified this is not just a smaller version of
the same bug but an actual fix of the "must not push job rows down"
constraint:
  - Read `renderList` (`tui/internal/ui/app.go:627-689`) and confirmed the
    header is always exactly one "safecode ... branch" line + one
    spacer-or-activity line before the column header, in both the
    empty-strip and non-empty-strip cases.
  - Diffed against the pre-TASK-5 baseline (`git show 70d635f:.../app.go`):
    the original header was `b.WriteString("\n\n")` after the branch line —
    i.e. exactly one blank line before the column header. The fixed code
    produces the same one-line footprint whether that line is blank or holds
    the single activity entry.
  - Empirically verified with a standalone test rendering both a fresh empty
    repo and a repo with commits: the column header lands on the same output
    line index (line 2) in both cases — zero net lines added, confirming the
    "identical footprint" claim in the implementation notes rather than just
    trusting it.
  - `n=1` means no multi-line growth risk regardless of history length;
    `subjectW` still clamps to a floor of 12 at narrow widths, so no
    wrapping risk either.
  - `list_test.go`'s `TestRenderListRecentActivityShowsMostRecentAcrossBranches`
    was correctly updated for the 1-entry behavior: it now asserts the single
    shown entry is the most recent commit across all local branches (not just
    the checked-out branch), and that the older shared commit does not also
    render — this still meaningfully exercises the cross-branch/dedup path
    end-to-end through the UI layer, it just can't assert multi-entry
    ordering anymore (that exhaustive coverage still lives at the git-package
    level in TASK-2, which is correct — no coverage was lost, just relocated
    to where it was always primarily asserted).
  - This is exactly the brief's own explicitly pre-approved fallback
    ("the fallback is to shrink it further (e.g. last commit only)"), so no
    separate product sign-off is needed — the brief already anticipated and
    blessed this exact resolution.

TASK-6: PASS
notes: `checkout_test.go` list-view checkout tests unaffected by the TASK-5
fix (only `list_test.go` changed). All three list-checkout edge cases
(switch, no-op already-on-main, refused checkout) still pass.

## Build/test verification

- `go build ./...`, `go vet ./...`, `gofmt -l .` — clean.
- `go test ./...` — all packages pass, including
  `tui/internal/git` and `tui/internal/ui`.
- `git status` clean on `feature/qge358_git-view-and-switch`; no uncommitted
  changes.

## Scope check

`git diff main...HEAD --stat` shows only the four job docs
(`brief.md`/`tasks.md`/`implementation.md`/`verdict.md`) and the four
expected source files (`git.go`, `recentcommits_test.go`, `app.go`,
`checkout_test.go`, `list_test.go`). No refactors or changes outside the
brief's scope. Commit discipline correct: one commit per task in
`[qge358] TASK-N: ...` format, a fix-up commit for the flagged TASK-5 issue,
a matching `implementation:` doc-update commit, and this verdict commit —
all present and properly attributed.

## Security

None run — no security-sensitive surface introduced (branch names passed as
argv elements, not shell-interpolated; unchanged from prior review).

## Overall

APPROVED

All six tasks pass. The previously-blocking TASK-5 layout regression is
fixed and independently verified (not just re-asserted from the developer's
notes): the recent-activity strip now reclaims the header's pre-existing
blank spacer line, so its footprint is byte-for-byte identical to the
pre-TASK-5 baseline whether or not there's a commit to show. No blockers
remain. Ready to merge.
