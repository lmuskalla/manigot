# Verdict: show mg diff in tui

id: pick
status: open
reviewer: @reviewer
date: 2026-08-14

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Reviewed on branch `feature/pick_show-mg-diff-in-tui`, base `main`
(`git diff main...HEAD`). Cross-referenced the diff against tasks.md, traced
every changed line and every new test against the code and real git
behavior. Note: the review shell only allows git read/commit commands, so
`go test` could not be executed — the verdict below is from static analysis
plus manual runs of the exact git commands the diff tab issues.

TASK-1: PASS
notes: internal/ui/detail.go — 6th "diff" tab (isDiff, never editable)
      appended after log in newDetailView; loadTab routes to loadDiff, which
      computes the quick eyeball via git.LogOneline + git.DiffStat over
      <base>...<branch> and degrades to placeholders: no branch → exists=false
      placeholder; git error → exists=false "_could not compute the diff: ..._";
      undiverged → exists=true "No changes on <branch> relative to <base>."
      (byte-identical text to cmd/mg/diff.go). diffBaseBranch mirrors
      cmd/mg/diff.go's chain exactly: project.Load(root).BaseBranch, falling
      back to git.SymbolicRefHead(root) — deliberately not BaseBranchValue().
      detail_test.go — TestDetailViewHasFiveTabsIncludingLog renamed to
      TestDetailViewHasSixTabsIncludingLogAndDiff (6 tabs, tabs[5].label
      "diff", not editable).

TASK-2: PASS
notes: internal/ui/detail.go — "6" → d.cur = 5 in detailView.update (reaches
      it via app.go's updateDetail fall-through; no collision with agent keys
      a/o/d/r/s or D/j/x/P/e/ctrl+r); footer hint "tab/1-5 files" →
      "tab/1-6 files". internal/ui/agents.go comment synced. detail_test.go —
      TestDetailViewDiffTabKeyBindingSwitchesTab added.

TASK-3: PASS
notes: All five content tests present on real scratch repos. Traced each
      against the code and git semantics: diverged branch renders log
      (ZZDIFFCOMMIT) + stat (jobfile.txt); undiverged branch (worktree added,
      job dir uncommitted) gets the exact "No changes on feature/df02_u
      relative to main."; branchless working-tree fallback gets the no-branch
      placeholder with exists=false; configured baseBranch "trunk" is
      respected (proven via the error placeholder naming trunk, since the
      branch doesn't exist); reload picks up a new commit (d.reload() is the
      same loadTabs path App.refresh drives). gitInitRepo now pins `-b main`
      with a clear comment — correct, since SymbolicRefHead's no-origin/HEAD
      fallback is "main"; helper still reads the branch back, contract
      unchanged. Manual runs of `git log --oneline main...feature/...` and
      `git diff --stat main...feature/...` on this repo confirm the output
      shape the tests assert.

TASK-4: PASS (dropped — explicitly optional)
notes: The full-patch toggle was optional per tasks.md ("can be deferred or
      dropped") and the brief only asks to "see the changes". Dropped rather
      than guessed at, documented in implementation.md's known issues. Correct
      call.

TASK-5: PASS
notes: README.md keybindings table (`tab`/`1`-`6`, tab list gains `diff`),
      `e`-row parenthetical, and a new diff-tab paragraph; docs/AGENTS.md `mg
      diff` Commands entry gains the TUI note and the "TUI and mg jdi" section
      says "opens each job's four files plus a computed `diff` tab". README
      and docs/AGENTS.md stayed in sync with each other.

Commit discipline: PASS — one commit per task in `[pick] TASK-N:` format
(TASK-1 4b2eaf4, TASK-2 13521e7, TASK-3 e375090, TASK-5 2e2e56a), each
touching only its task's files (the analyst's tasks.md content rode along in
TASK-1's commit — expected, the analyst agent is read-only); implementation.md
has its own commit (8c7d694).

Scope: PASS — no changes outside the files tasks.md lists.

Non-blocking observations (noted for the record, not blockers):
- diffBaseBranch ignores project.Load's error: a malformed .manigot/manigot.json
  would silently fall back to SymbolicRefHead where `mg diff` CLI errors. Edge
  case, graceful degrade either way.
- loadDiff runs two git subprocesses synchronously on every job open and
  ctrl+r. Same cost profile as the `mg diff` CLI; acceptable per the design.
- The ui package's gitInitRepo copy now diverges from the job package's
  (pins `-b main`); deliberate and documented.

## Security

No security findings — the diff tab is host-side-only, runs the existing
internal/git read-only helpers, and touches no mounts, agents, or git
metadata. No secrets involved.

## Overall

APPROVED

The diff tab matches the brief ("use mg diff to see the changes in the TUI
before D done") and every task in tasks.md is implemented as specified, with
the optional TASK-4 correctly dropped. No blockers.
