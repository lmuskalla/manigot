# Viewing a git diff

Reference for quickly eyeballing what a branch changed, or reading a diff
carefully. Written for use by both humans and agents.

## The one thing that matters

Use **three-dot** diff notation. It diffs from the merge-base (where the
branch diverged from the base branch), so you see *only what this branch
did*, even if the base branch has moved on since it was cut.

```bash
git diff main...<branch>            # what the branch changed
git diff main..<branch>             # two-dot: tip-vs-tip, usually NOT what you want
```

In a manigot job, `<branch>` is `[<prefix>/]feature|fix|chore/<id>_<slug>`
and the base branch is the project's configured `baseBranch` (from
`.manigot/manigot.json`, default `main`). The job worktree lives at
`<project>/.manigot-worktrees/<project>/<id>_<slug>/`, so from inside it the
diff is `git diff main...HEAD`. Review the job while it's open — `mg done`
squash-merges it into the base branch, after which there's no branch left to
diff.

## Quick eyeball

```bash
git diff --stat main...<branch>       # files changed + line counts
git diff --name-only main...<branch>  # just filenames
git log --oneline main...<branch>     # the commits themselves
```

`tig` is the best TUI for skimming a branch's commits and their diffs:

```bash
sudo apt install tig
tig                # commit list; Enter on a commit shows its diff
tig main...HEAD    # only this branch's commits, browse each one
```

For one-shot colored output in the terminal, pipe through `delta`
(`git diff ... | delta`) or set it as the default pager:

```bash
sudo apt install git-delta
git config --global core.pager delta   # also themes log and blame
```

## Careful review

- `git difftool -d main...<branch>` — open the full before/after tree in a
  side-by-side GUI (`meld`, `vimdiff`, `kdiff3`).
- `git diff --word-diff=color` — word-level (not just line-level) changes.
- Delta's `--side-by-side` gives the same feel in the terminal.
- If the remote is GitHub/GitLab, a merge-request review UI adds inline
  comments and approvals; use it when a full review trail matters.

## Agent-driven review

The systematic pass is already built into manigot — humans only jump in to
verify:

- `@reviewer` — correctness review against the original task requirements.
- `@security` — security vulnerabilities and exposure risks.
- `@quality` — code quality pass (after reviewer approves).

Read-only agents: they can read the diff and the git log, but cannot modify
git metadata.
