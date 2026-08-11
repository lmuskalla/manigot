# Tasks: Commit feature branches

id: v6h2us
status: open
analyst: claude
date: 2026-08-10

<!-- Produced by @analyst from brief.md. -->

## Best option (brief's "Notes" ask)

Add a manual **`P` push** action to the TUI's job **detail view**, mirroring
the existing `b` (switch branch) shortcut: it runs `git push -u origin
<branch>` for the open job's branch, host-side, using the credentials/SSH
agent already configured in the user's normal shell environment.

Why this over the alternatives:
- **A new `mg push` CLI subcommand** (like `mg done`/`mg delete`) would just
  wrap `git push origin <branch>`, which already works fine typed directly —
  it adds a new script and dispatcher case for no real convenience over
  `git push` itself. The actual gap in the brief is a *quick* action while
  already sitting in the TUI browsing/working a job, not a missing CLI verb.
- **Auto-pushing on every commit** (e.g. from `git.CommitFile`'s "e" edit
  auto-commit, or from inside agent sessions run in the container) was
  rejected: agent sessions run *inside* the Docker container, which has no
  guaranteed network access or the host's git credentials/SSH agent — pushing
  is fundamentally a host-side, human-initiated action, same as checkout.
  Auto-pushing every small edit (including mid-work agent commits not yet
  meant to be shared) is also more than "a quick way to push" asked for —
  it removes the user's choice of *when* a branch becomes visible on other
  hosts.
- The **TUI detail view** already owns every other per-job git action
  (`b` checkout, `e` edit+auto-commit, `D` done, `x` delete) — adding `P`
  there keeps all job/branch actions in one place instead of splitting them
  across a new CLI command and the TUI.

Key design point: unlike `e`/`D`/`j`/`x` and the agent-launch keys, `P` does
**not** need the `branchGuard` check. `git push origin <branch>` pushes the
named local branch ref directly — it does not require that branch to be
checked out in the working tree — so a job can be pushed regardless of which
branch is currently checked out, same as `b` itself needs no guard.

## Task breakdown

TASK-1: Add a `git.Push(root, branch string) error` helper to
`tui/internal/git/git.go`, following the file's existing conventions
(`ErrNotARepo` classification via `notARepo`, wrapped error + stderr via
`wrapErr`, doc comment in the same style as `Checkout`/`CommitFile`).
Implementation: `git -C root push -u origin <branch>` (the `-u` sets
upstream tracking on first push; a later push against an already-tracking
branch is then a plain fast-forward push — no force push, ever, so a
diverged remote surfaces as a normal rejected-push error rather than being
silently overwritten). Must not hang the TUI waiting on an interactive
credential prompt that has no terminal to render into: run with
`GIT_TERMINAL_PROMPT=0` in the child process's environment (this needs a
small `run`-adjacent helper in `git.go` that accepts extra env, since the
existing unexported `run()` has no env parameter and is called from many
other functions that must not change behavior).
  - files: `tui/internal/git/git.go`
  - depends: none
  - risk: medium — the happy path (existing `origin` remote, valid
    credentials) is simple and mirrors `Checkout`, but the failure modes are
    new to this file: no `origin` remote configured at all, a rejected
    non-fast-forward push, and a credential/network failure with
    `GIT_TERMINAL_PROMPT=0` set. Each needs to degrade to a clear wrapped
    error rather than hang or crash — worth explicit manual verification,
    not just unit tests against a local bare-repo remote.

TASK-2: Wire the `P` key into the detail view. Add a `pushMsg{branch string;
err error}` type and a `pushCmd(branch string) tea.Cmd` to
`tui/internal/ui/app.go`, mirroring `checkoutMsg`/`checkoutCmd` exactly
(plain git call off the UI goroutine, no `tea.ExecProcess` needed — this is
non-interactive). Add a `case "P":` to `updateDetail` that calls `pushCmd`
directly — explicitly **not** gated by `a.branchGuard()` (see the "best
option" note above for why). Handle the resulting `pushMsg` in `Update`
(alongside the existing `checkoutMsg` case) by setting the detail view's
status to something like `"→ pushed <branch> to origin"` on success, or
`cmdErrorText(err)` on failure. Confirm `"P"` (capital) doesn't collide with
any existing binding — `p` (lowercase) is already used for the Product Owner
agent button, same pattern as `D` done vs. `d` developer.
  - files: `tui/internal/ui/app.go`
  - depends: TASK-1
  - risk: medium — touches the shared `updateDetail` key-dispatch switch and
    the root `Update` message switch, both of which are hit by every other
    detail-view key; needs care not to change behavior for any existing case.

TASK-3: Cosmetic follow-ups once TASK-2 lands: add `P push to origin` to the
detail view's footer hint string (`renderFooter` in `tui/internal/ui/detail.go`,
same list as `b switch branch`/`D mark done`/etc.), and extend the
key-collision comment at the top of `tui/internal/ui/agents.go` ("chosen so
they never collide with the detail view's other bindings…") to list `P` push
alongside the other reserved keys.
  - files: `tui/internal/ui/detail.go`, `tui/internal/ui/agents.go`
  - depends: TASK-2
  - risk: low — comment/string-only changes, no behavior.

TASK-4: Unit-test `git.Push` in the `git` package: a successful push to a
local bare-repo remote (setting up `origin` the same way the package's
existing `initRepo`-style test helpers set up test fixtures), the
missing-`origin`-remote case, and a rejected non-fast-forward push (diverged
local vs. remote history) — each asserting the specific degrade documented in
TASK-1, not just "no crash".
  - files: `tui/internal/git/git_test.go` (or a new `push_test.go` in the
    same package, following the existing per-function test-file split e.g.
    `recentcommits_test.go`)
  - depends: TASK-1
  - risk: low — test-only, but setting up a real local remote (bare repo) in
    the test fixture is new to this package's tests and needs to actually
    exercise a real `git push`, not a mocked one, to catch the
    `GIT_TERMINAL_PROMPT=0` / non-fast-forward behavior for real.

TASK-5: Unit-test the `P` key wiring in package `ui`, mirroring
`checkout_test.go`'s pattern (`gitInitRepo`/`gitRun` helpers, a real
`detailView` against a real temp git repo with a bare-repo `origin`
remote): pressing `P` on a job dispatches `pushCmd`, the resulting
`pushMsg` updates `a.detail.status` on success, an error surfaces via
`cmdErrorText`, and — the key regression this task exists to guard — `P`
works even when the currently checked-out branch differs from the job's
branch (no `branchGuard` block), unlike `e`/`D`/`j`/`x`.
  - files: new `tui/internal/ui/push_test.go`
  - depends: TASK-2
  - risk: low — test-only, same local-remote fixture concern as TASK-4.

TASK-6: Update the README's TUI keybindings table (`### Keybindings` →
"Detail view" table) to add a `P` row describing the push action, next to
the existing `b` row it complements.
  - files: `README.md`
  - depends: TASK-2
  - risk: low — documentation only.

## Notes for the developer

- No new bash script, no new `mg` subcommand, no changes to `scripts/*.sh` or
  `agents/*.md` — this is scoped entirely to the host-side TUI (`tui/`).
- Out of scope (flag back if any of these turn out to be needed): force
  push, deleting/pruning remote branches, auto-push on commit or on job
  creation, pushing from inside an agent's container session, an "ahead/behind
  origin" indicator in the list or detail view, and any handling for a
  project with no `origin` remote configured beyond surfacing a clear error.
- `docs/AGENTS.md` (and `README.md`'s architecture prose) do not currently
  enumerate individual TUI keybindings outside the `### Keybindings` table
  TASK-6 updates — no change needed there for this job.
