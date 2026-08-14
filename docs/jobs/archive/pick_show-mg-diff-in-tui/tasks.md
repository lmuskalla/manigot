# Tasks: show mg diff in tui

id: pick
status: open
analyst: @analyst
date: 2026-08-14

<!-- Produced by @analyst from brief.md. -->

## Scope assumptions (confirm before implementing)

The brief ("We had previously built mg diff. However, it has no representation
in the TUI. I'd expect that if a job is finished, before I do D done, I can use
mg diff to see the changes in the TUI.") is sparse. `mg diff <id>` exists as a
host-side CLI command (cmd/mg/diff.go) over the `internal/git` three-dot
helpers (Diff / DiffStat / DiffNameOnly / LogOneline, see docs/GIT_DIFF.md),
but the TUI has no surface for it. The user wants to eyeball a job's changes
in the TUI before archiving it with `D`.

1. **Deliverable is a diff tab in the job detail view**, not a new overlay or
   a keybinding that shells out. The detail view already has a non-file tab
   precedent — the `log` tab (`isLog`, content from the mg-jdi sidecar, never
   editable) — so a computed `diff` tab appended after it (index 5, keeping
   `log` at 4 and the `1`–`5` bindings untouched) is the natural fit and the
   smallest change. Content mirrors `mg diff`'s default "quick eyeball":
   `git log --oneline <base>...<branch>` + `git diff --stat <base>...<branch>`
   via the existing internal/git helpers.
2. **Base branch resolves exactly as `mg diff` does** (cmd/mg/diff.go):
   `project.Load(root).BaseBranch`, falling back to `git.SymbolicRefHead(root)`
   when unset — NOT `Settings.BaseBranchValue()` (which would default to
   "main" and skip the origin/HEAD fallback). `confirm.go`'s `doneConfirmLines`
   already duplicates this same chain, so mirroring it inline (or extracting a
   tiny shared helper — developer's call, flag if extracted) keeps the diff tab
   consistent with the done-confirmation's "Branch : <job> → <base>" line.
3. **Degrade, never crash**: a job with no branch (working-tree fallback /
   non-repo project) shows a placeholder like the log tab's; an undiverged
   branch ("No changes on <branch> relative to <base>.") and a git error (e.g.
   branch deleted out from under the TUI) each get a plain-text placeholder.
4. **The full patch (`mg diff --full`) is a toggle, not the default** — see
   TASK-4, which is optional and can be deferred or dropped. The brief only
   asks to "see the changes"; the quick eyeball satisfies that minimally.
5. **Open jobs only, like `mg diff`.** The TUI lists open jobs only
   (job.Discover excludes archive/), so no archived-job handling is needed.
6. **No agent / Dockerfile / git-shim changes.** The diff is computed
   host-side by the TUI from the existing internal/git helpers.

## Task breakdown

TASK-1: Add a computed "diff" tab to the detail view — a 6th tab (index 5,
after `log`), flagged `isDiff`, non-editable, content built in loadTab from
git.LogOneline + git.DiffStat over `<base>...<branch>` (base resolved via the
project.Load → git.SymbolicRefHead chain, mirroring cmd/mg/diff.go), with
placeholders for no-branch / no-changes / git-error; update the existing
tab-structure test that pins 5 tabs (TestDetailViewHasFiveTabsIncludingLog →
6, tabs[5].label == "diff", not editable).
     files: internal/ui/detail.go, internal/ui/detail_test.go
     depends: none
     risk: medium — extends the tab model (new non-file tab type, load path,
           rendering) and changes the tab count tests, but the `log` tab is a
           close precedent to follow.

TASK-2: Wire navigation/chrome for the 6th tab: `6` key → cur 5 in
detailView.update, footer hint "tab/1-5 files" → "tab/1-6", sync the
stale-comment mentions of tab counts (agents.go's "1-5 file/log select"
comment), and add a key-binding test (press "6" switches to the diff tab).
     files: internal/ui/detail.go, internal/ui/detail_test.go,
            internal/ui/agents.go
     depends: TASK-1
     risk: low — mechanical keybinding/hint/comment updates; "6" collides with
           no agent key (agents are a/o/d/r/s) and no other detail binding.

TASK-3: Content tests for the diff tab on real scratch repos (existing
gitInitRepo / addJobWorktree helpers in detail_test.go): log + stat rendered
for a diverged branch, "no changes" placeholder for an undiverged branch,
no-branch placeholder for the working-tree fallback, base branch respected
when .manigot/manigot.json sets a non-default base, and ctrl+r refresh
picking up newly committed changes.
     files: internal/ui/detail_test.go
     depends: TASK-1, TASK-2
     risk: low — test-only, same real-git scratch-repo style the package
           already uses; the only subtlety is asserting stat/log text that git
           itself produces.

TASK-4 (optional — confirm before implementing): Add a toggle inside the diff
tab (e.g. "f") switching between the quick eyeball and the complete patch
(git.Diff), rendered safely (code-fenced) so glamour doesn't mangle
+/-/@@-prefixed lines; includes its own content tests.
     files: internal/ui/detail.go, internal/ui/detail_test.go
     depends: TASK-1
     risk: medium — adds per-tab interactive state and a rendering nuance for
           raw patch text; the brief doesn't strictly require the full patch,
           so it can be deferred or dropped.

TASK-5: Documentation sync: README keybindings table (`tab` / `1`-`6`, tab
list gains `diff`) and a short paragraph describing the diff tab; docs/AGENTS.md
Commands entry for `mg diff` (note the TUI representation) and the "TUI and
mg jdi" section ("opens each job's four files" → mention the diff tab).
     files: README.md, docs/AGENTS.md
     depends: TASK-2 (final keybinding/label wording)
     risk: low — doc-only, but README and docs/AGENTS.md must stay in sync per
           the hard rule.
