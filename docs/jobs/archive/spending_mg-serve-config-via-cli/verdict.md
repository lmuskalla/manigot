# Verdict: mg serve config via cli

id: spending
status: open
reviewer: glm-5.2 (reviewer pass)
date: 2026-08-29

## Review

TASK-1: PASS
notes: src/internal/serve/registry.go — AddRegistryEntry/RemoveRegistryEntry
load through LoadRegistry's full validation before every write, so the CLI
cannot produce a registry the daemon would refuse to start on, and a corrupt
file is never silently rewritten (test-pinned: TestAddRegistryEntryRefusesCorruptRegistry
asserts the file is byte-identical after the refused add). Duplicate name and
duplicate path (including a relative spelling of a registered path) are
refused with the file left unchanged; removing the last entry writes
{"projects": []} — never null, never a deleted file — verified by reading the
file after a live rm. 11 tests cover the store. One non-blocking observation:
ValidProjectName is exported "for the mg serve-projects CLI" but the CLI never
calls it (AddRegistryEntry validates internally) — tested but currently
caller-less API surface.

TASK-2: PASS
notes: src/cmd/mg/serveprojects.go + main.go dispatcher + serveprojects_test.go.
list (table + WarnMissingDocs on stderr), add [path] [name] with the $PWD and
basename defaults, rm <name>, --registry override with the same resolution as
mg serve (ErrNoRegistryPath shared), `help` subcommand (exit 0, before
registry resolution), flag handling identical to mg serve's conventions
(ContinueOnError, usage to stderr, exit 2 on parse errors). Verified beyond
the 9 unit tests by building the binary and running an end-to-end smoke:
empty list → add with default name → duplicate-path refusal (exit 1, clear
error) → list → indented JSON file shape → rm → empty projects list → help.
Exit codes, restart hints on every mutation, and the no-docs warning on add
all behave as implementation.md claims.

TASK-3: PASS
notes: mg serve's usage text and its no-projects startup warning now point at
`mg serve-projects`; README.md command table + listener registry paragraph,
docs/AGENTS.md (Commands + Project registry bullet), and
project-template/docs/AGENTS.md all document the command with accurate
semantics. Cosmetic only: the main.go help block's serve/serve-projects/
serve-token continuation lines sit at column 37 vs the file-wide 35 — the
serve/serve-token rows were already off-grid on main, and this is -h output
cosmetics, not behavior.

Commit discipline note: the developer's work landed as the host-side sweep
commit `[spending] chore: commit all` rather than per-task
`[spending] TASK-N:` commits. The full branch diff was reviewed instead
(git diff main...HEAD — the review surface), and `mg done` squash-merges the
branch regardless. No functional impact.

Scope: every changed file in `git diff main...HEAD` maps to TASK-1/2/3 or the
job's own files; nothing unexplained, no unrelated refactoring.

Verification (reviewer-run): go build ./..., go vet ./..., gofmt -l on all
touched files (clean), and the full go test ./... suite pass — with the real
git ahead of the session's PATH-first git shim, exactly as implementation.md
documents (the shim's git-init refusal breaks unrelated test setups, not this
change).

## Security

none — @security not run. No credentials, tokens, or .env content involved:
the change writes config/serve.json (project paths only, 0644), and the CLI
takes filesystem paths as arguments, not URL inputs; no new trust surface.

## Overall

APPROVED

No blockers. The three non-blocking observations above (caller-less
ValidProjectName export, help-text column cosmetics, sweep-commit shape) need
no change before merge.
