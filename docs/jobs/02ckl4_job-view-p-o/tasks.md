# Tasks: job view: p -> o

id: 02ckl4
status: open
analyst: @analyst
date: 2026-08-13

<!-- Produced by @analyst from brief.md. -->

## Problem as understood

The TUI's job detail view launches the Owner agent with the key `p` — a
mnemonic left over from when the role was still called "product owner". The
role has since been renamed to plain "owner" (`agents/owner.md`,
`internal/agents/agents.go`), so the detail-view key should be `o`.

The binding lives in exactly one place: the `agentMeta` table in
`internal/ui/agents.go` (`agents.Owner: {key: "p", display: "Owner"}`). Both
the action-bar rendering (`renderActionBar`, `internal/ui/detail.go`) and the
key dispatch (`agentForKey`, `internal/ui/app.go`) derive from that table, so
changing the single entry propagates everywhere.

`o` collides with no detail-view binding (tab/1-5, `e`, `D`, `j`, `x`/`del`,
`P`, `esc`/`q`, `ctrl+r`, plus the other agent keys `a`/`d`/`r`/`s`). The
TUI's only other `o` is the *list view's* quick session (`updateList`,
`internal/ui/app.go`), which is a separate view state — key routing in
`App.Update` is strictly per-state, so there is no fallthrough and no
collision.

## Task breakdown

TASK-1: Change the Owner agent's action-bar key from `"p"` to `"o"` in the
`agentMeta` table.
     files: internal/ui/agents.go
     depends: none
     risk: low — a single key string in the data map that both rendering and
     key dispatch derive from; the only other `o` binding in the TUI is the
     list view's quick session, a separate state with no fallthrough.

TASK-2: Update the existing test assertions that expect the owner key `"p"`:
the `wantByKey` map (line ~75), the `"[p] Owner"` unified-format button (line
~161), and the `"[p]"` 80-column truncation key list (line ~184).
     files: internal/ui/agents_test.go
     depends: TASK-1
     risk: low — mechanical test updates; the truncation test's width math is
     unaffected (`[o]` is the same width as `[p]`).

TASK-3: Add a regression test mirroring `TestAgentForKeyVNoLongerResolves`
(the existing precedent for Developer's old `"v"` key) asserting `"p"` no
longer resolves to any agent now that Owner uses `"o"`.
     files: internal/ui/agents_test.go
     depends: TASK-1
     risk: low — follows an established test pattern for a key removal.

TASK-4: Verify with `go test ./internal/ui/...` and `go build ./...` that the
change compiles and the ui suite passes, including the updated/added tests.
     files: none (verification only)
     depends: TASK-1, TASK-2, TASK-3
     risk: low — baseline ui suite is already green; the change is one map
     entry plus test updates.

## Explicitly out of scope

- The list view's `o` quick-session binding (`updateList` in
  `internal/ui/app.go`) — a different view state; intentionally unchanged.
- The Owner agent definition itself (`agents/owner.md`,
  `internal/agents/agents.go`) — already renamed to "owner"; only the TUI key
  is stale.
- The stale "b switch branch" mention in `agents.go`'s comment — pre-existing
  and unrelated to this key change.

## Suggested order

TASK-1 first; TASK-2 and TASK-3 are independent of each other once TASK-1
lands; TASK-4 last as verification.

## Note for the developer

At analysis time the job worktree's git registration was missing
(`/home/leo/code/manigot/.git/worktrees/02ckl4_job-view-p-o` does not exist
while the branch ref `feature/02ckl4_job-view-p-o` does), so `git` commands
from `/workspace` fail. Re-check `git status` before relying on per-task
commits; if still broken, flag it to the host rather than working around it.
