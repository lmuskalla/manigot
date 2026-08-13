# Tasks: Harden mg jdi on OpenCode profiles and enforce read-only agents under OpenCode

id: 63quv2
status: open
analyst: @analyst
date: 2026-08-13

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Add OpenCode `permission:` frontmatter to `agents/reviewer.md`, `agents/security.md`, `agents/analyst.md` and `agents/owner.md` (edit allowed only for the agent's own report file where one is written, bash allowed only for the read-only git commands the reviewer/security bodies prescribe, and task/webfetch/websearch/question denied), and make the reviewer body's "How to start" step 5 tool-neutral so it no longer prescribes a sed pipeline the new bash allowlist would block.
     files: agents/reviewer.md, agents/security.md, agents/analyst.md, agents/owner.md
     depends: none
     risk: medium — permission syntax must match OpenCode's schema exactly (last-match-wins, git `*` forms) or the agent's own report write / commit could be blocked under OpenCode

TASK-2: Confirm the bake-time strip logic keeps `permission:` intact for OpenCode — the Dockerfile's awk already passes any key other than `name:`/`tools:` through and `convertAgentFile` mirrors it, so this is verification plus tests: add a test that a `permission:` block survives `convertAgentFile` (both as a plain-key and map-form block, including one whose body contains a `tools:`-prefixed line to pin the drop-block logic).
     files: internal/session/agentconv_test.go, internal/session/agentconv.go (only if a gap shows up), Dockerfile (only if a gap shows up)
     depends: TASK-1
     risk: low — pass-through behavior, pinned by tests

TASK-3: Sync the docs that describe the tools-strip caveat and agent format: README.md's "One caveat" paragraph and the "enforced under Claude Code only" note under the agent table, docs/AGENTS.md's strip-logic and agent bullets, and project-template/docs/AGENTS.md's built-in-format note.
     files: README.md, docs/AGENTS.md, project-template/docs/AGENTS.md
     depends: TASK-1, TASK-2
     risk: low — prose only

TASK-4: Harden the OpenCode JSONL path with the real event shapes captured from a live `opencode run --format json` (opencode-ai 1.18.16, the version the Dockerfile installs): add a fixture-based test in internal/orchestrate/signal_test.go that feeds the exact captured JSONL lines (step_start with a nested step-start part, tool_use with a tool part, text with a time object, step_finish with tokens) through DetectSignal/ResultText, and add a Run-level integration test in cmd/mg/jdi_test.go where an agent's opencode-shaped output carries the NEEDS-HUMAN-INPUT marker inside a text event — proving the loop's marker detection and run.log prose extraction against the real output shape, not an approximation.
     files: internal/orchestrate/signal_test.go, cmd/mg/jdi_test.go
     depends: none
     risk: low — test-only, but the fixtures must match reality byte-for-byte (verified live, not guessed)

TASK-5: Verify live what this sandbox allows and record the docker-gated remainder as a concrete procedure: confirm `opencode run --agent reviewer --auto --format json` composes and that the TASK-1 permission block actually denies an edit and allows the reviewer's git commands under the real opencode binary (isolated temp config home, so this session's own config is untouched), then document in implementation.md the exact end-to-end procedure (jobs, commands, what to check in run.log and the status sidecars) for a human with docker + zai/opencode-go credentials to complete the brief's real-run success criterion.
     files: docs/jobs/63quv2_harden-mg-jdi-on-opencode-profiles-and-enforce-read-only-agents-under-opencode/implementation.md
     depends: TASK-1, TASK-2, TASK-4
     risk: medium — the real E2E run under zai/opencode-go needs docker + subscription credentials, neither present in this sandbox; the procedure must be precise enough that the human's run is a formality, not a new investigation
