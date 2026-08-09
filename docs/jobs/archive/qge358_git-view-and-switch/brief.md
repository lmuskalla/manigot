# Brief: git view and switch

status: done
type: feature
id: qge358
branch: feature/qge358_git-view-and-switch
date: 2026-08-09
author: Leander Muskalla

## What

We are now tracking jobs from all git branches and have introduced the option to switch to a branch inside a job.
I am wondering if it would generally be a good idea to reflect git more in the visual layout when our jobs are that deeply integrated with git.
On the startpage, I am imagining an info on which branch we are, an option to switch between branches and maybe the last few commits from git log (on the main branch).
Please check if this makes sense from your point of view. E.g. when I was working on a job branch and go back and want to do a quick launch, I'll have to manually instruct the agent to first switch to main.
However, be honest about if this is a good idea or not.

### Product decision (locked)

Ship a scoped-down version of the original ask, three additions to the **list
view** header only — no new views, no new keybindings beyond one, no general
git browser:

1. **Show the current branch in the list header.** The list already tags
   off-branch jobs with `· <branch>` (`renderJobRow` in `tui/internal/ui/app.go`),
   but never states what "current" is. Purely additive, no interaction.
2. **A single "back to main" quick-checkout action from the list.** Not a
   generic arbitrary-branch picker. The only friction actually reported is
   "I'm on a job branch and want to fire a quick session against `main`."
   Reuse the existing `git.Checkout` / `App.checkoutCmd` mechanism already
   proven out by the detail view's "c" key (see `tui/internal/ui/app.go`,
   `tui/internal/git/git.go`). `main` is already the project's hardcoded base
   branch convention (`scripts/new-job.sh` always branches from it,
   `docs/AGENTS.md` documents this) — no new "detect default branch" logic
   needed.
3. **A small, read-only "recent activity" strip in the list header** — the
   last ~5 commits **across all local branches** (not just `main`), most
   recent first, deduped by commit hash, each showing short hash + truncated
   subject + relative time + which branch it's on. This exists to solve a
   different, real problem raised in review: quick launches (`o`) and
   job-branch work leave no trace in the job list once you move on, so it's
   easy to lose track of what you actually did. A log scoped to `main` alone
   would miss almost all of that activity, since quick launches and job work
   mostly happen off `main` — hence "across all local branches," not "on
   main" as originally floated.

Rejected from the original ask: a **generic multi-branch switcher** (no
demonstrated need beyond getting back to `main`; if one shows up later, it's
its own brief with a concrete scenario) and a **scrollable/interactive git
log view** (diff drill-down, filtering, its own keybindings/state) — both
would turn the TUI into a second git client, which pulls against its actual
job: orchestrating jobs, not browsing git history.

## Why

- The job list already tells you structured progress (status, stage) but
  nothing about raw activity — and raw activity (quick launches, in-progress
  job branch commits) is exactly what's easy to lose track of once you've
  moved between a few jobs and a few ad-hoc sessions. A glanceable recent-
  activity strip closes that gap without turning the tool into a git client.
- The list already implies a "current branch" concept (the `· <branch>` tags)
  without ever stating what it is — surfacing it is a small clarity fix to a
  feature that already shipped.
- The reported friction (manually switching to `main` before a quick launch)
  is real and repeatable; a one-key fix reusing an already-proven checkout
  path is the cheapest correct answer, not a full branch picker.

## Out of scope

- **A generic "switch to any branch" picker.** Only `main` gets a dedicated
  shortcut. No new state/view for browsing/selecting arbitrary branches.
- **Interactive git log / diff viewer.** The recent-activity strip is
  read-only, fixed to ~5 entries, no drill-down, no new keybinding to open it
  — it renders inline in the list header, same treatment as the branch tags.
- **Remote branches** — local branches only, consistent with the existing
  cross-branch discovery scope (`fvrl56_keep-track-of-jobs`).
- **Detecting the default branch dynamically** (e.g. via `origin/HEAD`).
  `main` is hardcoded elsewhere in the project already (`scripts/new-job.sh`);
  match that convention rather than adding branch-detection logic.

## Notes

### Open design question for the analyst: dedup across branch tips

Commits reachable from multiple branches (e.g. a job branch that hasn't
diverged from `main` yet shares `main`'s tip) will otherwise show up more than
once if the recent-activity list is built naively per-branch-tip. Design a
dedup strategy (e.g. union of commits by hash, most-recent N after dedup)
rather than concatenating each branch's last commit.

### Pointers

- List header / rendering: `tui/internal/ui/app.go` (`renderList`,
  `renderJobRow`, `listColumns`).
- Branch primitives already exist and should be reused, not reinvented:
  `git.CurrentBranch`, `git.LocalBranches`, `git.Checkout` in
  `tui/internal/git/git.go`.
- The detail view's existing "c" checkout action (`App.checkoutCmd`,
  `App.branchGuard` in `tui/internal/ui/app.go`) is the pattern to mirror for
  the list-view "back to main" action, including how it re-runs discovery and
  surfaces git's own refusal reason (e.g. uncommitted changes) in the status
  line rather than failing silently.
- For the commit log itself, a new small function in `tui/internal/git/git.go`
  is likely the right home (e.g. something like `RecentCommits(root string, n
  int)`), following that package's existing pattern of degrading gracefully
  (`ErrNotARepo`, empty-repo case) rather than erroring.

### Watch-outs for the analyst / developer

- Keep the recent-activity strip glanceable. If ~5 entries proves noisy in
  practice (generic agent commit messages, WIP commits), the fallback is to
  shrink it further (e.g. last commit only), not to add filtering/interaction
  to compensate.
- The list view is still primarily the job list — the new header content
  (branch, quick-checkout hint, activity strip) must not visually compete
  with or push down the job rows themselves.
