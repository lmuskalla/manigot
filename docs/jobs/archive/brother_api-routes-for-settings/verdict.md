# Verdict: api routes for settings

id: brother
status: reviewed
reviewer: @reviewer
date: 2026-08-29

## Review

TASK-1: PASS
notes: `src/internal/serve/settings.go`'s GET/PUT `/settings` implement the
global default profile exactly as scoped. I verified the shared-write claim
directly in the code: `handlePutSettings` persists via `config.UpsertEnv`
— the identical write `profilesSet` in `cmd/mg/profiles.go` makes for `mg
profiles <name>` — and validates against `config.ProfileByID` (built-in +
user-defined store), so an unknown id is a 400. `activeProfile()` returns
`config.EnvValue("MANIGOT_PROFILE")` falling back to `config.ProfileClaudePro`,
the same "Active default" chain `profilesList` renders. Empty profile is a
400 (never a silent reset). Tests confirm the default chain, the .env
shared-write (pre-existing `ZHIPU_API_KEY=keep-me` line survives), and the
validation 400s.

TASK-2: PASS
notes: GET/PUT `/projects/{project}/settings` cover the brief's baseBranch +
prefix. I verified the storage/writer primitives: `project.Load` /
`project.Save` (the TUI settings screen's own store into
`.manigot/manigot.json`, `BaseBranchValue()` defaulting to `main`), the
wholesale-replacement semantics (absent field clears to default), the
ref-component validation before write, and the project lock around the write
(`s.locks.Lock(root)`). The "settings written actually drive job creation"
test (`TestPutProjectSettingsDrivesJobCreation`) exercises the real path and
passes — a `jobBranchPrefix` of `mg` makes the next API-created job branch
under `mg/`. Validation 400s (19 bad shapes, including spaces, `..`, leading
`-`, `@{`, `.lock` suffix, control chars, and the length cap) assert nothing
is written on rejection.

TASK-3: PASS
notes: `settings_test.go` covers the defaults, roundtrip, drives-job-creation,
and ref-shape unit checks; `credentials_test.go` now enumerates the four
settings routes (including the unknown-profile and bad-ref 400 paths) in the
credentials-never-returned surface; `security_test.go`'s hostile-segment
suite covers the project segment on both GET and PUT project-settings routes
(confirmed by reading the diffs against main). All pass under
`go test ./internal/serve/`.

TASK-4: PASS
notes: `docs/AGENTS.md`'s listener/control-plane section documents all four
routes, their validation, and the lock-taking set (project settings PUT takes
the project lock; the global settings write does not). One cosmetic note: the
change's diff also nudged the orphan-delete bullet from a 2-space to a
3-space indent — harmless markdown, but a formatting regression in the
canonical doc.

## Security

None blocking. The settings surface carries only a plain profile id and two
ref names — never credential material (readiness stays in `/health` as
booleans). Both project-settings routes go through the `resolveProject`
choke point (the zero-path-inputs invariant), and the hostile-segment suite
now pins them. The PUTs validate their inputs before any write, and the
credentials/leak suite was extended to the new surface including its 400
paths. Lock discipline matches the documented boundary: the project settings
write takes the project lock (create/done read those settings inside their
own locked sections); the global settings write is deliberately unlocked (no
project scope).

## Overall

APPROVED

The brief's three settings — the default profile (global) and the per-project
baseBranch + jobBranchPrefix — are implemented, tested, documented, and
verified: `go build ./...` and `go vet ./...` are clean, the touched packages
(`internal/serve`, `internal/config`, `internal/project`) all pass, and the
full-suite failures are confined to the git-shim-dependent packages
(`cmd/mg`, `internal/git`, `internal/job`, `internal/session`, `internal/ui`)
whose tests shell out to `git init`/`git worktree` — an environmental
limitation of this container, none of which this change touches. Two minor,
non-blocking follow-ups if this branch sees another round: (1)
`refComponentProblem` rejects a value *ending* in `.lock` but not a component
*ending* in `.lock` mid-chain (`foo.lock/bar` passes the API yet git
check-ref-format rejects it) — pathological input, low risk, but the
validation's stated purpose is exactly to keep such values out of later git
argv; (2) the cosmetic 3-space indent on the orphan-delete bullet in
`docs/AGENTS.md`.