# Verdict: job view: p -> o

id: 02ckl4
status: open
reviewer: @reviewer
date: 2026-08-13

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: internal/ui/agents.go:20 — `agents.Owner` key changed `"p"` → `"o"` in
the `agentMeta` table, the single source of truth that both the action-bar
rendering (`renderActionBar`, detail.go) and the key dispatch (`agentForKey`,
app.go) derive from. Verified `o` collides with no detail-view binding
(tab/1-5, `e`, `D`, `j`, `x`/`del`, `P`, `esc`/`q`, `ctrl+r`, plus agent keys
`a`/`d`/`r`/`s`); the list view's `o` quick-session key is a separate view
state routed strictly by `App.state`, so no fallthrough or collision exists.

TASK-2: PASS
notes: internal/ui/agents_test.go — all three `"p"` assertions updated: the
`wantByKey` map entry (now `"o": "owner"`), `"[o] Owner"` in
`TestRenderActionBarUnifiedFormat`, and `"[o]"` in
`TestRenderActionBarTruncatesLabelsAt80ColsKeysStayIntact`. Width math
unchanged (`[o]` is the same width as `[p]`); all updated tests pass.

TASK-3: PASS
notes: internal/ui/agents_test.go — `TestAgentForKeyPNoLongerResolves` added,
mirroring the `TestAgentForKeyVNoLongerResolves` precedent; asserts `"p"` no
longer resolves to any agent. Passes.

TASK-4: PASS
notes: `go build ./...` exits 0; `go test ./internal/ui/...` and the full
`go test ./...` suite pass. Re-ran the ui key/render tests independently —
all PASS, including the new regression test.

## Security

none — a single TUI keybinding change; no security surface touched.

## Overall

APPROVED

The brief's single ask — the TUI detail page's Owner shortcut must be `o`, not
`p` — is implemented exactly as specified: one-line key change in the
`agentMeta` table, matching test updates, a regression test for the removed
key, and a clean build + full test run. One commit per task in
`[02ckl4] TASK-N: …` format on `feature/02ckl4_job-view-p-o`
(HEAD = 34c326a, verified in the shared repo), no out-of-scope changes.

Nothing must change before merge.

Non-blocking context (pre-existing, host-side, not introduced by this job and
not part of its scope): this job's git worktree registration is missing —
`.git/worktrees/02ckl4_job-view-p-o` does not exist while the branch ref
does, and `/workspace` is currently registered to the concurrent
`otv2d2_make-6-string-char-more-writable` job, so ordinary `git` commands
fail from `/workspace`. All commits are durably on the branch in the shared
repo (ref verified directly). `mg done` on this job will require the host-side
`mg` tooling to re-register the worktree first — see implementation.md's
Known issues.
