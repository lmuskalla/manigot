# Verdict: in the wild

id: hnhn1s
status: done
reviewer: @reviewer
date: 2026-08-13

## Review

Scope reviewed: `git diff main...HEAD` (merge-base `0e44253`) — 874 insertions
across `internal/session/host.go`, `cmd/mg/host.go`, `cmd/mg/main.go`, two new
test files, README.md, docs/AGENTS.md, and the job docs. The `agent format
consolidation` commit (yec6o6) shown in `git log main...HEAD` lives on `main`
only (verified via `git branch --contains`) and is not part of this job.

TASK-1: PASS
notes: internal/session/host.go delivers everything specified — HostInvocation
(Argv/Dir/Env) + BuildHostInvocation + Run mirroring docker.go; tool-id→binary
mapping ("claude-code"→"claude") with a LookPath presence check and a clear
"not installed on the host" error; Dir = resolved root (job worktree with
--job); child env = os.Environ() + KeyEnv credential pairs appended last (Go's
duplicate-key handling makes the .env-effective value win) with OPENCODE_MODEL
excluded; host-pathed job prompt; no auto-approval flags; --print rejected
before anything else; opencode --model forwarding (verified against the
installed opencode 1.18.16, which documents -m/--model); host-mode banner with
no docker-specific lines. Run wires stdio through and returns the CLI's exit
code. hostLookPath is a package var so tests stay hermetic.

TASK-2: PASS
notes: cmd/mg/host.go runHost mirrors runSession's step order exactly
(ParseArgs → ResolveProfile → ResolveRoot → CheckAuth → BuildHostInvocation →
Run); dispatcher case "host", "wild" added to cmd/mg/main.go before default;
help entry lists both names. Package doc comment updated to include host.

TASK-3: PASS
notes: internal/session/host_test.go covers plain claude (no docker
machinery, no auto-approval flags, credentials in child env, banner), opencode
profile (--model forwarded, OPENCODE_MODEL absent from env, no claude keys
leaked), --print rejection, missing-binary error, --job worktree Dir +
host-pathed prompt (asserts no /workspace/ container path), and a stub-binary
Run test (Dir/Env/argv/exit code). envMap helper correctly handles the
checkout helper's cleared-keys quirk. Tests are hermetic (hostLookPath stub,
t.Setenv).

TASK-4: PASS
notes: cmd/mg/host_test.go covers --print rejection, missing binary via
stripped PATH (exercises the full flow), invalid --profile, missing auth, and
help text listing mg host + mg wild. One note: "alias dispatch" is verified
by manual smoke test (./bin/mg wild --print rejects identically to mg host)
rather than a unit test — main()'s switch is not unit-testable without
refactoring (no existing test does this either; out of scope). hostCheckout
correctly clears credential keys from the process env (the sandbox env
carries real credentials, which caused a genuine test failure before the
fix — good catch during implementation).

TASK-5: PASS
notes: README.md (commands table row, usage examples, "Host mode" section
covering no-isolation/no-auto-approval/host-side agents/--model behavior/
--print rejection, plus a pointer from the Choosing-a-profile auto-approval
paragraph) and docs/AGENTS.md Commands entry both match the implemented
behavior. Sync check performed: project-template/docs/* and agents/* contain
no stale "always docker-isolated" claims — no changes needed there.

TASK-6: PASS
notes: verified independently at HEAD — go build ./... clean, go test ./...
green across all packages, gofmt -l clean, make mg builds, and smoke tests
(mg host/wild --print rejection, --profile bogus validation, help listing)
all behave as documented.

## Security

No findings. Host mode is a deliberate trust expansion (the CLI runs on the
host with no sandbox), which is the feature itself — the auto-approval flags
are correctly withheld so every tool call still asks. No secrets committed
(credentials flow via env only; the sandbox's own credentials are present in
the test env and correctly neutralized by the test helpers). No injection
vectors: Run uses exec.Command with argv elements (no shell), so prompt,
--model, and passthrough values cannot break out. The claude-pro
ANTHROPIC_API_KEY subscription-protection guard still applies via the shared
CheckAuth. One inherent note, not a defect: on the host the child inherits
the full process environment (including any provider keys beyond the
profile's own) — this is what "run the CLI as-is on the host" means and is
documented; the docker path's key isolation deliberately does not apply.

## Overall

APPROVED

The implementation matches brief.md (a way to run mg without docker — a
launcher for claude/opencode on the host) and every task in tasks.md, with
all four confirmed decisions honored (alias `mg wild`, no auto-approval flags,
--print rejected, --model forwarding without touching host config). Commit
discipline is clean: one commit per task in the [hnhn1s] TASK-N format plus
the implementation summary commit.

Non-blocking notes for the human at merge time (`mg done`):
- `main` advanced while this job was open (job yec6o6 "agent format
  consolidation" touched README.md, docs/AGENTS.md, project-template docs and
  internal/session/docker.go). This branch was cut before that landed, so the
  squash merge may hit conflicts in README.md/docs/AGENTS.md — resolve them
  then; no code interaction exists between the two jobs (agentconv vs host
  mode).
- Known follow-ups (already recorded in implementation.md): host sessions do
  not inject the manigot docs/AGENTS.md context file; the opencode --model
  forwarding assumes a recent opencode (verified 1.18.16); --print remains
  container-only by design.
