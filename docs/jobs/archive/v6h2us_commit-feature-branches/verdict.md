# Verdict: Commit feature branches

id: v6h2us
status: open
reviewer: claude
date: 2026-08-10

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `git.Push(root, branch string) error` in `tui/internal/git/git.go` follows
the file's existing conventions exactly (`notARepo`/`ErrNotARepo` classification,
`wrapErr`-wrapped errors, doc-comment style matching `Checkout`). The new `runEnv`
helper is additive only — `run()` and its signature/behavior are untouched, as
required. `git -C root push -u origin <branch>` with `GIT_TERMINAL_PROMPT=0` set
matches the spec; no force push. Verified against `git diff` and confirmed the
package builds (`go build ./...`) and vets cleanly.

TASK-2: PASS
notes: `pushMsg`/`pushCmd` in `tui/internal/ui/app.go` mirror
`checkoutMsg`/`checkoutCmd` exactly (plain off-goroutine git call, no
`tea.ExecProcess`). `case "P":` in `updateDetail` is not gated by
`a.branchGuard()`, as specified, and correctly no-ops with a status message when
`job.Branch == ""` (mirroring the existing `b` no-branch guard). The `pushMsg`
case in the root `Update` switch sets `a.detail.status` on success/failure via
`cmdErrorText`. Confirmed no key collision: grepped every `case "..."` in
`updateDetail` and the agent-key table in `agents.go` — no existing binding uses
`"P"`; lowercase `p` remains the Product Owner agent key, same pattern as
`D`/`d`.

TASK-3: PASS
notes: `renderFooter` in `detail.go` gets `P push to origin` in the hint string;
the key-collision comment at the top of `agents.go` now lists `P push to origin`
alongside the other reserved keys. Comment/string-only, no behavior change, as
specified.

TASK-4: PASS
notes: `tui/internal/git/push_test.go` exercises `Push` against a real local
bare-repo remote (not mocked): happy-path push + hash verification on the
remote, `-u` upstream-tracking verification, missing-origin-remote (asserts not
misclassified as `ErrNotARepo`), a genuine rejected non-fast-forward push
(diverged commit from a second clone, remote left untouched — verified by hash
comparison), and the not-a-repo case. Each asserts the specific documented
degrade, not just "no crash", per the task's risk note. `go test
./internal/git/...` passes.

TASK-5: PASS
notes: `tui/internal/ui/push_test.go` mirrors `checkout_test.go`'s pattern
(`gitInitRepo`/`gitRun`, a real bare-repo `origin`). Covers: full `P` →
`pushCmd` → `pushMsg` → status flow with a real push verified on the remote; an
injected error surfacing via `cmdErrorText`; the no-branch no-op case
(`TestPushKeyWithoutBranchIsNoop`, mirroring
`TestCheckoutKeyWithoutBranchIsNoop`); and the key regression test
`TestPushKeyIgnoresBranchGuard`, which first asserts `branchGuard()` really
would block a guarded action for the off-branch job, then confirms `P` still
dispatches and succeeds regardless. `go test ./internal/ui/...` passes.

TASK-6: PASS
notes: README's `### Keybindings` → "Detail view" table gets a `P` row next to
`b`, describing the push action and noting (accurately) that it works
regardless of which branch is checked out.

## Security

None. `Push` never force-pushes (plain `git push -u origin <branch>`, no `-f`/
`--force`), sets `GIT_TERMINAL_PROMPT=0` so a missing credential fails cleanly
instead of hanging on a prompt with no terminal, and does not shell out via a
string-interpolated command (uses `exec.Command` with argv, same as every
other function in the package). No new attack surface beyond what `Checkout`
already has.

## Scope

Matches `tasks.md` exactly: no new `mg` subcommand, no `scripts/*.sh` or
`agents/*.md` changes, no auto-push. The one item outside the strict task list
is `tasks.md`/`implementation.md` themselves being folded into the
`TASK-*`/`implementation` commits (noted explicitly in `implementation.md`'s
"Also included in this pass" — `tasks.md` was already written by @analyst but
left uncommitted when the developer's session started). This is job-workflow
bookkeeping, not a source-code scope change, and is disclosed rather than
hidden — not a blocker.

## Commit discipline

Every task has its own commit in `[ID] TASK-N: description` format
(`2b954bb`…`3aaf938`), plus a separate `a42c1e2 [v6h2us] implementation: add
summary` commit. Matches the required convention.

## Verification performed

- `go build ./...` — clean
- `go vet ./...` — clean
- `gofmt -l .` — no files need formatting
- `go test ./internal/git/... ./internal/ui/...` — all pass
- Manual read-through of every changed file via `git diff main...HEAD`
- Grep-verified no `case "P"` / agent-key collision

## Overall

APPROVED

All six tasks are fully implemented as specified in `tasks.md`, with no scope
creep, correct commit discipline, and passing tests. No changes required
before merge.
