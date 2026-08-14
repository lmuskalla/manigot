# Verdict: jdi-status sidecar cleanup (roadmap item 1)

Reviewed: the working-tree diff on `main` (uncommitted — no formal job dir
existed for this work; the diff covers the developer's implementation of
`docs/ROADMAP.md` item 1, "jdi-status sidecar cleanup"). Full suite was run
green by the developer before review: `go build ./...`, `go vet ./...`,
`go test ./...` (with the real git on PATH — the container's git shim refuses
`git init`, an unrelated pre-existing environment artifact).

## Per task

TASK-1 (mg delete must stop leaving `.manigot/jdi-status/<job>/` behind): PASS
- `internal/job/jdistatus.go:187` — new `RemoveJDIStatus(root, jobName)`, keyed
  by the exact same `JDIStatusDir` (`<root>/.manigot/jdi-status/<name>/`) that
  mg-jdi's write side uses, so the cleanup cannot drift from the write path.
  Absent sidecar returns `false, nil` (no error, no output); present is removed
  wholesale with `os.RemoveAll` (status + run.log together).
- `internal/job/delete.go:164` (git path) and `:208` (non-git path) — called
  after the worktree/branch/dir deletion has already succeeded. A removal
  failure is a printed warning, never an abort — correct, because the deletion
  itself must not be reported as failed over a stale log dir. The
  `→ Removing mg-jdi status for <job>...` line prints only when a sidecar
  existed, so the common no-sidecar case adds zero noise
  (`TestDeleteJobNoJDISidecarPrintsNothing` pins this).
- Orphan paths `internal/job/orphan.go:180` (`RemoveOrphans`) and `:203`
  (`RemoveOrphansConfirmed`) also clean the matching sidecar by `o.Name`. This
  covers both `mg delete <name>` on an orphan — the `mg delete` route the
  roadmap's complaint actually flows through — and `mg jobs`' batch removal
  offer. Job ids are unique and never reused, so an orphan's name cannot
  collide with a live job's sidecar.

TASK-2 (mg done keep-vs-remove decision): PASS
- Decision made and documented: **remove** (`internal/job/finish.go:235`, after
  the branch delete). The reasoning is recorded in `docs/ROADMAP.md` and
  `docs/AGENTS.md`: the archive keeps the job's docs, mg-jdi never runs against
  an archived job (discovery is open-jobs only), so the sidecar would otherwise
  be dead weight forever. Sound and consistent with the roadmap's framing ("the
  tool not cleaning up after itself quietly erodes trust in the lifecycle").

TASK-3 (tests): PASS
- `TestRemoveJDIStatus` (absent / present / already-removed),
  `TestDeleteJobRemovesJDISidecar`, `TestDeleteJobNoJDISidecarPrintsNothing`,
  `TestFinishJobRemovesJDISidecar`, non-git delete extended, `TestRemoveOrphans`
  extended. All pass.

TASK-4 (docs): PASS
- `docs/AGENTS.md` — `mg done`/`mg delete`/orphan lifecycle paragraphs plus the
  `.manigot/` config bullet now state the sidecar removal. `/AGENTS.md` is a
  gitignored hardlink mirror of `docs/AGENTS.md` and stays in sync
  automatically. `agents/*.md` and `project-template/docs/AGENTS.md` do not
  reference the sidecar, so the keep-in-sync rule is unaffected.
- `README.md` — the orphan/lifecycle paragraph notes the cleanup.
- `docs/ROADMAP.md` — item 1 marked done with the decision recorded; bottom
  line updated.

## Notes (context, not findings)

- The implementation is uncommitted on `main`; it was reviewed as the
  working-tree diff. The human drives the commit/merge.
- Deleting a job *while* mg-jdi is mid-run could make the still-running process
  rewrite a sidecar for the deleted job on its next status write. This is a
  pre-existing footgun of force-removing a worktree under an active run, and is
  outside the roadmap item's scope.

## Overall

APPROVED

Must change before merge: nothing.
