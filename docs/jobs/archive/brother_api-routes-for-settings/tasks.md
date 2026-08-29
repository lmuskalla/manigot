# Tasks: api routes for settings

id: brother
status: open
analyst: developer session (breakdown drafted together with implementation — the job ran without a separate @analyst pass)
date: 2026-08-29

<!-- Produced from brief.md. -->

## Task breakdown

<!-- TASK-1: description
     files: list of files likely affected
     depends: none
     risk: low / medium / high — reason
-->

TASK-1: Settings API routes — global default profile
files: src/internal/serve/settings.go (new), src/internal/serve/server.go
depends: none
risk: low — additive routes; the write path reuses config.UpsertEnv, the
exact primitive `mg profiles <name>` writes MANIGOT_PROFILE with, so CLI,
TUI, and API keep sharing one default. The profile id is validated against
config.ProfileByID, so an unknown id is a 400, and only a plain profile id
(ever a non-secret) is read back.

TASK-2: Settings API routes — per-project baseBranch + jobBranchPrefix
files: src/internal/serve/settings.go (new), src/internal/serve/server.go
depends: TASK-1 (same file, shared helpers)
risk: low — additive routes; writes go through project.Save (the sanctioned
writer the TUI already uses) into the target project's .manigot/manigot.json.
Ref-component validation rejects garbage before it can reach later git argv;
the write takes the project lock so it serializes with create/done, which
read these settings inside their own locked sections.

TASK-3: Tests + surface pins for the new endpoints
files: src/internal/serve/settings_test.go (new),
src/internal/serve/credentials_test.go,
src/internal/serve/security_test.go
depends: TASK-1, TASK-2
risk: low — the credentials/leak and hostile-segment suites must enumerate
the new surface exactly as they enumerate the existing one.

TASK-4: Documentation sync
files: docs/AGENTS.md
depends: TASK-1, TASK-2
risk: low — the listener section enumerates every endpoint and the
lock-taking set; both lists must include the new routes (repo hard rule:
keep docs/AGENTS.md in sync with the system).
