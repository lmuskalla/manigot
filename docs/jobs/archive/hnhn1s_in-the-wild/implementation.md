# Implementation: in the wild

id: hnhn1s
status: done
developer: @developer
date: 2026-08-13

## Summary

Added `mg host` (thematic alias `mg wild`): a session mode that runs the
profile's agent CLI (`claude` / `opencode`) directly on the host — no Docker,
no container image, no mounts — for work that must touch the host itself. It
reuses the existing session machinery (flag parsing, profile resolution,
credential validation, project-root + `--job` worktree resolution) and
replaces only the launch step: instead of a `docker run` argv, it assembles a
direct CLI invocation running from the resolved root with the profile's
credentials in the child process environment. The docker path, `mg jdi`, and
the TUI are untouched.

Per the confirmed decisions: host sessions pass **no auto-approval flags**
(`--dangerously-skip-permissions` / `--auto` — safe only inside the isolated
container), `--print` is rejected with a clear error (non-interactive paths
stay container-only), the zai/opencode-go plan model is forwarded via
opencode's `--model` flag (verified supported by opencode 1.18.16; mg never
writes the user's host opencode config), and `--agent` works only if the
host's own CLI has that agent installed.

## Changes

TASK-1: Added `internal/session/host.go` — `HostInvocation` (Argv/Dir/Env)
with `BuildHostInvocation` + `Run`, mirroring `docker.go`: tool-id→binary
mapping (`claude-code`→`claude`), an `exec.LookPath` presence check with a
clear "not installed on the host" error, working dir set to the resolved root
(the job worktree with `--job`), child env = `os.Environ()` + the profile's
credential pairs from `ProfileInfo.KeyEnv` (appended last so duplicates
resolve to the .env-effective value, excluding `OPENCODE_MODEL` which goes via
`--model`), the host-pathed job prompt, a `--print` rejection, and a host-mode
info banner on the diag writer. `hostLookPath` is a package var so tests can
stub it, mirroring `internal/launch`'s `ExeOverride` pattern.

TASK-2: Added `cmd/mg/host.go` — `runHost`, a thin mirror of `runSession`
(ParseArgs → ResolveProfile → ResolveRoot → CheckAuth → BuildHostInvocation →
Run); added the `host`/`wild` dispatcher case, the package doc, and the
`mg host` help entry (with the `mg wild` alias) in `cmd/mg/main.go`.

TASK-3: Added `internal/session/host_test.go` — plain claude session (no
docker machinery, no auto-approval flags, credentials in the child env),
opencode profile (`--model` forwarded, `OPENCODE_MODEL` not in env, no
claude keys leaked), `--print` rejection, missing-binary error,
`--job` worktree Dir + host-pathed prompt, and a stub-binary `Run` test. Added
an `envMap` helper because the session tests' `checkout` helper clears
credential keys to empty in the process env, which would false-positive
presence checks.

TASK-4: Added `cmd/mg/host_test.go` — `runHost` error paths (`--print`
rejection, missing binary via stripped `PATH`, invalid `--profile`, missing
auth) and a help-text test asserting `mg host` + `mg wild` are listed. The
`hostCheckout` helper clears credential keys from the process env (the
session flow falls back to the process env when the checkout `.env` lacks a
key, and the test machine's own env can carry real credentials).

TASK-5: Documented `mg host` in `README.md` (installed-commands table row,
usage examples, a "Host mode" section covering no-isolation/no-auto-approval,
host-side agents, the `--model` behavior and the `--print` rejection, plus a
pointer from the Choosing-a-profile auto-approval paragraph) and in the root
`docs/AGENTS.md` Commands section. Sync check: `project-template/docs/*` and
`agents/*` needed no changes — they carry no "always docker-isolated" claims.

TASK-6: Verified `make mg` builds `bin/mg`, `gofmt -l` is clean, and
`go test ./...` passes across all packages. Smoke-tested the built binary:
`mg host --print` and `mg wild --print` both reject with the clear error,
`mg host --profile bogus` validates the profile.

## Known issues / follow-ups

- **Project context on the host**: the docker path mounts `docs/AGENTS.md`
  read-only into the container as the CLI's context file. `mg host` does not
  inject it — the host CLI reads whatever context it finds in the host
  project's cwd (`./AGENTS.md` etc.), which is normally the user's own file,
  not `docs/AGENTS.md`. A follow-up could pass the context explicitly (e.g.
  via claude's `--append-system-prompt`) if host sessions should see the
  manigot project context.
- **opencode `--model` assumption**: the code forwards `--model` whenever the
  profile's `OpenCodeModel` is set, assuming a recent opencode (verified
  against 1.18.16 on the development host). An older opencode without the
  flag would fail at launch; no runtime capability probe was added (the
  `--model` support check during implementation confirmed the installed
  version accepts it).
- **No `--print` in host mode**: deliberate per the confirmed decision —
  `--print` and `mg jdi` remain docker-container paths. If unattended host
  runs are ever needed, the entrypoint's flag translation (`opencode run
  ...` / `claude --print`) would have to be re-implemented host-side.
