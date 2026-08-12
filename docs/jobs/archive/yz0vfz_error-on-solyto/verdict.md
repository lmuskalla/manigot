# Verdict: error on solyto

id: yz0vfz
status: open
reviewer: deepseek-v4-flash
date: 2026-08-12

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: internal/ui/app.go:474-491 — the `n > len(a.recentCommits)` clamp was hoisted out of the
height > 0 branch so it runs on the `a.height == 0` path too; the zero-height fallback now degrades
to 0 against an empty commit cache while still returning the floor (1) when commits exist (len >= 1
so the clamp is a no-op). Doc comment updated (app.go:469-473). Height > 0 path is bit-for-bit
unchanged (`clamp(spare, floor, max)` then clamp to len). `clamp` itself (app.go:494) is fine.

TASK-2: PASS
notes: internal/ui/app.go:1246-1250 — defense-in-depth clamp of `n` to `len(a.recentCommits)`
immediately before `a.recentCommits[:n]` at app.go:1251; no behavior change for in-range counts.

TASK-3: PASS
notes: internal/ui/list_test.go:260-279 — `TestRenderListZeroHeightNoCommitsDoesNotPanic` combines
the two previously-missed halves: a fresh repo with no commits and an App with `a.height == 0`.
Verified in a scratch checkout of the pre-fix commit (243fec7) that this test fails with the exact
panic from the brief — `runtime error: slice bounds out of range [:1] with capacity 0` at
`renderRecentActivity` app.go:1236 via `renderList` app.go:1204 — and passes on the fixed code.
Faithful regression pin.

TASK-4: PASS
notes: independently re-ran `go test ./internal/ui/` (ok), `go test ./...` (all packages ok),
`go build ./...` (exit 0), and `go vet ./internal/ui/` (clean).

## Security

none — no security surface involved; the change is a slice-count clamp in TUI rendering code.

## Overall

APPROVED

The root cause analysis in tasks.md is accurate (confirmed by reproduction, identical panic message
and stack site), both tasks match their specification, the regression test genuinely pins the crash
combination, and there is no behavior change for repos with commits. No blockers.

Non-blocking observation (no change required): the finalized tasks.md content (analyst output) was
committed inside the TASK-1 commit rather than as its own commit; purely cosmetic commit hygiene,
no impact on the code or the fix.
