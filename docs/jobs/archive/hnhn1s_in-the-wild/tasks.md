# Tasks: in the wild

id: hnhn1s
status: open
analyst: @analyst
date: 2026-08-13

Produced by @analyst from brief.md.

## Summary

Add a `mg host` command (thematic alias proposed: `mg wild`) that runs the
profile's agent CLI (`claude` / `opencode`) directly on the host — no Docker,
no container image, no mounts. It reuses the existing session machinery
(profile resolution, credential validation, project-root + `--job` worktree
resolution, `--agent`/`--prompt` passthrough) and replaces only the last step:
instead of building a `docker run` argv, it builds a direct CLI invocation
running from the resolved root with the profile's credentials in the child
process environment. Use case: work that must happen on the host, done through
mg as the launcher.

## Decisions (confirmed by the user 2026-08-13)

1. **Command name / alias**: primary `mg host`; thematic alias **`mg wild`**
   ("in the wild" — the agent outside the container cage, tying to this job's
   title).
2. **No auto-approval flags on the host**: the container path always passes
   `--dangerously-skip-permissions` (Claude) / `--auto` (OpenCode), safe only
   because the container is isolated and ephemeral. On the host there is no
   isolation, and the brief's wording ("i need to manually run opencode /
   claude code") implies supervised interactive sessions — so `mg host`
   deliberately does NOT pass those flags and lets the CLI keep its normal
   per-tool confirmation prompts.
3. **`--print` is rejected in host mode** with a clear error: `--print` and
   `mg jdi` are the non-interactive container paths (jdi's orchestration
   parses the container output contract) and stay docker-only. Host mode is for
   manual, interactive work.
4. **OpenCode model**: the zai/opencode-go profiles default to a
   plan-specific model. On the host, mg must NOT write the user's real
   `~/.config/opencode/opencode.json` (the entrypoint only writes it inside
   the container). Instead, forward the effective model via opencode's
   `--model` flag if the installed binary supports it (verify with
   `opencode --help` during implementation); otherwise skip model injection
   and document that a host session uses the host's own opencode config.
5. **Agent availability on the host**: manigot's global agents are baked into
   the container image only, not installed on the host. `mg host --agent X`
   passes `--agent X` through and works only if the host's own CLI has that
   agent installed — the CLI errors clearly otherwise. Document this in the
   README; do not try to install agents into the user's home config.

## Task breakdown

TASK-1: Add `internal/session/host.go` — a `HostInvocation` type (Argv/Dir/Env) with `BuildHostInvocation` + `Run`, mirroring `docker.go`: direct `claude`/`opencode` argv (no docker flags/mounts/network/memory), tool-id→binary mapping ("claude-code"→"claude") with an `exec.LookPath` presence check and a clear error, working dir set to the resolved root (job worktree when `--job`), child env = `os.Environ()` + the profile's credential key=value pairs converted from `ProfileInfo.KeyEnv` (appended last so duplicates resolve to the .env-effective value), the host-pathed job prompt ("Please work on the job at <root>/docs/jobs/<job> — start by reading brief.md"), no auto-approval flags (decision 2), a `--print` rejection with a clear error (decision 3), opencode `--model` forwarding per decision 4, and a host-mode info banner on the diag writer (no docker-specific lines, no .env shadowing, no git-identity warning).
     files: internal/session/host.go (new)
     depends: none
     risk: medium — a new launch surface whose env/argv assembly must match each CLI's real conventions (and get the billing-correct model handling right); purely additive, the docker path is untouched

TASK-2: Wire the `mg host` command in `cmd/mg`: a `runHost` handler in a new `cmd/mg/host.go` mirroring `runSession` (ParseArgs → ResolveProfile → ResolveRoot → CheckAuth → BuildHostInvocation → Run), plus the dispatcher case and the thematic alias in `cmd/mg/main.go`, and the `mg host` entry in `printHelp`.
     files: cmd/mg/host.go (new), cmd/mg/main.go
     depends: TASK-1
     risk: low — a thin command mirroring the existing runSession; the only judgment calls are the help wording and the alias (decision 1)

TASK-3: Add `internal/session/host_test.go` covering BuildHostInvocation/Run: plain claude session, opencode session, credential env forwarding (KeyEnv → child env), `--job` worktree root as Dir, host-pathed job prompt, missing-CLI error, `--print` rejection, and no `--dangerously-skip-permissions`/`--auto` in argv — following docker_test.go's conventions (containsAll etc.).
     files: internal/session/host_test.go (new)
     depends: TASK-1
     risk: low — test file mirroring established docker_test.go patterns

TASK-4: Add `cmd/mg/host_test.go` for the command wiring: alias dispatch, flag/argument error exit codes, and that the help text lists the new command (following the existing command-test conventions).
     files: cmd/mg/host_test.go (new)
     depends: TASK-2
     risk: low — mirrors existing command test conventions

TASK-5: Document `mg host` (and the thematic alias): add it to the README's installed-commands table and usage section with a short "host mode" subsection (what it is for, no isolation / no auto-approval, host-side agents only, opencode model behavior per decision 4), and to the Commands section of the root `docs/AGENTS.md`; verify `project-template/docs/AGENTS.md`, `project-template/docs/CLAUDE.md` and `agents/*.md` stay accurate (the sync rule — they should need no change since host mode is additive, but confirm no claim about "always docker-isolated" is left behind).
     files: README.md, docs/AGENTS.md (project-template/docs/* and agents/* only if a sync check finds drift)
     depends: TASK-2
     risk: low — documentation only; the sync rule requires checking the template/agent docs for stale claims

TASK-6: Verify the build and the full test suite: `make mg` and `go test ./...` pass with the new code (and `gofmt` clean).
     files: none (verification only)
     depends: TASK-1, TASK-2, TASK-3, TASK-4, TASK-5
     risk: low — additive change; the risk is a stray compile/test breakage in the new files, caught here

## Out of scope

- The docker session path (bare `mg`), `mg jdi`, the TUI, and `launch.Agent`/`Quick`/`AgentQuick` all stay docker-based — no changes.
- Installing manigot's global agents onto the host.
- Writing to the user's host opencode config.
