# Verdict: Harden mg jdi on OpenCode profiles and enforce read-only agents under OpenCode

id: 63quv2
status: open
reviewer: @reviewer
date: 2026-08-13

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: `agents/reviewer.md`, `agents/security.md`, `agents/analyst.md`, `agents/owner.md` carry `permission:` blocks that parse as valid OpenCode `PermissionConfig` (verified with a YAML parse and, authoritatively, by the real `opencode` 1.18.16 binary loading the stripped copies — opencode hard-errors on invalid agent config, so a clean load is a schema proof). Enforcement was verified live in an isolated `XDG_CONFIG_HOME`: the reviewer's `git branch --show-current`/`git diff HEAD` succeeded; `edit` and `write` of a source file were denied (the denial error echoed the exact rule set — `edit * deny`, `docs/jobs/**/verdict.md allow`); non-git bash (`ls`) was denied; the source file was byte-identical after the run. The allow-paths mg jdi depends on also work live: the reviewer wrote `docs/jobs/<id>/verdict.md` and committed it (`git add` + `git commit`, commit present in `git log`), and the analyst wrote its `tasks.md`. Reviewer body step 5 was made tool-neutral (the old sed pipeline would have been blocked by the git-only bash allowlist) — a consistent, necessary change. Matches the brief's requirement: the reviewer cannot edit `implementation.md` or code under OpenCode.

TASK-2: PASS
notes: `TestConvertAgentFilePreservesPermissionBlock` and `TestConvertAgentFilePermissionAfterMapFormTools` (internal/session/agentconv_test.go) pin that `permission:` survives `convertAgentFile` verbatim, in both plain-key and map-form variants, and that a permission block directly following a multi-line map-form `tools:` block is not eaten by the `droppingToolsBlock` state machine. The Dockerfile awk needed no change — `/^(name|tools):/` cannot match `permission:` — and the awk-stripped output was itself loaded by real opencode during verification, so the strip logic is proven end to end, not just by unit test.

TASK-3: PASS
notes: README's stale "One caveat ... not restricted under OpenCode" paragraph and "enforced under Claude Code only" note are replaced with the current two-schema enforcement; `docs/AGENTS.md` (both the `internal/session` and `Dockerfile` bullets), the Dockerfile comment, `project-template/docs/AGENTS.md` + `docs/CLAUDE.md`, and the `internal/session/agentconv.go` doc comment are all synced. A repo-wide grep for the old "unrestricted under OpenCode" caveat found no remaining live references (only archived job history, which is intentionally immutable). No prose drift between the four docs.

TASK-4: PASS
notes: `internal/orchestrate/signal_test.go` embeds `realOpenCodeJSONL` — verbatim stdout of a real tool-using `opencode run --format json` session (opencode-ai 1.18.16, the version the Dockerfile installs unpinned) — with tests proving ResultText extracts exactly the text-event prose ("DONE"), that a marker inside the real-shape text event is detected with the exact reason, and that a marker literal inside a tool_use event's `state.output` does not false-positive. I independently re-checked the fixture quoting: the `\n` sequences are literal backslash-n inside backtick raw-string literals, so the JSONL lines stay valid and the tests exercise the intended text-events-only scan path rather than the malformed-line fallback. `cmd/mg/jdi_test.go`'s `TestRunStopsOnNeedsHumanMarkerInOpenCodeJSONL` drives the full loop with real-shape JSONL and asserts the stop reason plus that run.log shows extracted prose, never the raw JSONL blob — the brief's "run.log checked for correctness" concern pinned at the loop layer.

TASK-5: PASS
notes: All in-sandbox verification was genuinely executed against the real `opencode` binary in an isolated temp config home (this session's own config untouched): real event-shape capture, `--agent` selection with the permission frontmatter loaded, enforcement (deny), and the reviewer/analyst allow-paths. The brief's docker-gated success criterion — a real `mg jdi --job <id> --profile zai|opencode-go` run with run.log and status-sidecar checks — cannot execute in this sandbox (no `docker` binary, no zai/opencode-go subscription credentials); implementation.md records the exact procedure and expected artifacts, and tasks.md explicitly scoped TASK-5 to "verify live what this sandbox allows and record the docker-gated remainder as a concrete procedure". This matches the archived 4i5tcx precedent, which accepted the same sandbox limitation as a noted non-blocker. The human E2E run is the one remaining step before the brief's success criterion is formally met.

## Security

- The change strengthens the unattended-run posture: OpenCode's `--auto` auto-approves only what is not *explicitly denied*, so the `deny` rules hold even in auto mode — verified live (the edit/write denials fired during a `--auto` run). The reviewer/security/analyst/owner can no longer modify source or `implementation.md` under OpenCode; bash is restricted to read-only git; `task`, `webfetch`, `websearch` and `question` are denied.
- No secrets committed; `.env` untouched; the only new attack surface would be a mis-typed permission rule, and the two live enforcement runs plus the TASK-2 conversion tests cover both the deny and allow paths.
- Residual risk (flagged, not a code defect): the `permission:` frontmatter key is also seen by Claude Code, since `agents/*.md` is the single source for both CLIs. Claude Code's tolerance of this unknown subagent key could not be empirically verified in this sandbox (claude requires auth; `claude doctor` does not load subagents). Claude Code's documented behavior with unknown subagent frontmatter keys is lenient, and the brief explicitly directed this single-source design, but one claude-pro session using any of the four modified agents should be smoke-tested as part of the human E2E pass.

## Overall

APPROVED

Nothing in the code must change before merge. Two human verification steps remain, both already documented in implementation.md and both impossible in this sandbox (no docker binary / no claude-pro auth):

1. Complete the docker-gated E2E per implementation.md: `mg jdi --job <id> --profile zai` and `--profile opencode-go` on real jobs, checking exit code 0, `stopped:finished` in `.manigot/jdi-status/<job>/status.json`, prose-only `run.log` ending in `mg jdi finished: stop-finished`, and no false needs-human/stall — the brief's stated success criterion.
2. As part of that pass, run one claude-pro session with `@reviewer` (or any of the four modified agents) to confirm Claude Code loads the agent with the `permission:` frontmatter key present.

Also note (non-blocking, already recorded in implementation.md's Known issues): `@mentor`/`@architect` are listed "read-only" in the README table but are not in the brief's four and remain tool-unrestricted under OpenCode — a follow-up decision, not a defect of this job.
