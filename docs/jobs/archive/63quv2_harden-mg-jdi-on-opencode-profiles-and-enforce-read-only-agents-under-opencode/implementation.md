# Implementation: Harden mg jdi on OpenCode profiles and enforce read-only agents under OpenCode

id: 63quv2
status: open
developer: @developer
date: 2026-08-13

<!-- Produced by @developer after implementation. -->

## Summary

Two halves, implemented and — as far as this sandbox allows — verified against
the real `opencode` binary:

1. **Read-only agents enforced under OpenCode.** `agents/reviewer.md`,
   `agents/security.md`, `agents/analyst.md` and `agents/owner.md` now carry
   an OpenCode `permission:` frontmatter block (TASK-1). It lives in the same
   source files as the Claude-Code `tools:` list; the bake-time strip
   (Dockerfile awk) and the session launcher's `convertAgents`
   (`internal/session/agentconv.go`) drop `name:`/`tools:` for OpenCode and
   pass `permission:` through untouched, so the read-only restriction is now
   enforced under both CLIs (TASK-2, TASK-3).
2. **`mg jdi` on the OpenCode profiles hardened and proven at every layer
   this sandbox can reach.** The JSONL parser and retry-budget state machine
   were audited against real `opencode run --format json` output — captured
   live from opencode-ai 1.18.16, the version the Dockerfile installs — and
   pinned with real-shape fixtures at both the parser unit level and the
   `mg jdi` loop level (TASK-4). The docker-gated end-to-end run under
   `zai`/`opencode-go` (the brief's success criterion) cannot be executed in
   this sandbox — no `docker` binary — so TASK-5 records the exact procedure
   for a human to complete it, plus the live verification that was possible.

## Changes

TASK-1: `permission:` frontmatter added to the four read-only agents. Under
OpenCode, `edit` is limited to the agent's own report file (`tasks.md` for
`@analyst`, `verdict.md` for `@reviewer`), `bash` is denied except read-only
git commands (`@reviewer`: add/commit/diff/branch/status/log/rev-parse/show;
`@security`: the git read set minus add/commit; `@analyst`/`@owner`: none),
and `task`/`webfetch`/`websearch`/`question` are denied. Rules use OpenCode's
last-match-wins object syntax (`"*": deny` first, specific allows after), so
`--auto` auto-approval never overrides an explicit deny. The reviewer body's
"How to start" step 5 was made tool-neutral (read `baseBranch` from
`.manigot/manigot.json`, then `git diff <base>...HEAD`) — the previous sed
pipeline would have been blocked by the new git-only bash allowlist.

TASK-2: `internal/session/agentconv_test.go` gained
`TestConvertAgentFilePreservesPermissionBlock` and
`TestConvertAgentFilePermissionAfterMapFormTools`, pinning that `permission:`
(plain-key and map-form) survives the conversion and that a permission block
following a multi-line map-form `tools:` block isn't eaten by the drop-block
state machine. The Dockerfile awk needed no change — `/^(name|tools):/` never
matches `permission:`.

TASK-3: Documentation sync. README's "One caveat ... not restricted under
OpenCode" paragraph and the "enforced under Claude Code only" note are
replaced with the current behavior; `docs/AGENTS.md` (both the
`internal/session` and `Dockerfile` bullets) and the Dockerfile comment note
that a `permission:` block passes through the strip; `project-template/docs/
AGENTS.md` and `docs/CLAUDE.md` note that custom read-only agents can carry a
`permission:` block.

TASK-4: `internal/orchestrate/signal_test.go` embeds `realOpenCodeJSONL` —
verbatim stdout of a real tool-using `opencode run --format json` session —
and tests that ResultText extracts exactly the text-event prose, that the
marker inside the real-shape text event is detected, and that a marker
literal inside a tool_use event's `state.output` does not false-positive.
`cmd/mg/jdi_test.go` gained `TestRunStopsOnNeedsHumanMarkerInOpenCodeJSONL`,
driving the full loop with real-shape JSONL and asserting the stop reason
plus that run.log shows extracted prose, never the raw JSONL blob.

TASK-5: Verification + procedure, below.

## Verification (done in this sandbox)

The sandbox has no `docker` binary, so the container path (`mg jdi` itself)
could not be exercised. Everything short of that was verified live against
the real `opencode` binary (opencode-ai 1.18.16, same version the Dockerfile
installs) in an isolated temp config home:

- **Real event shapes.** `opencode run --format json` emits one JSON object
  per line: `step_start` (part.type `step-start`), `tool_use` (part.type
  `tool`, tool output in `part.state.output`, no `part.text`), `text`
  (part.type `text`, the agent's prose in `part.text`), `step_finish`
  (part.type `step-finish` with tokens/cost). Every line parses as JSON with
  a non-empty `type`, so `opencodeResultText`'s all-lines-must-fit rule holds
  on real output. The captured bytes are the TASK-4 fixtures.
- **`--agent` + permission frontmatter load.** `opencode run --agent reviewer
  --auto --format json` selects the stripped reviewer agent (permission block
  intact) and runs cleanly.
- **Read-only enforcement (the point of TASK-1), live:** with the reviewer
  agent, `git branch --show-current` and `git diff HEAD` succeeded; `edit`
  and `write` of `src.txt` were **denied** (the error echoed the exact rule
  set: `edit * deny`, `docs/jobs/**/verdict.md allow`); non-git bash (`ls`)
  was denied; `src.txt` was byte-for-byte unchanged. The allow-path still
  works: the reviewer wrote `docs/jobs/<id>/verdict.md` and committed it
  (`git add` + `git commit`, commit present in `git log`), and the analyst
  wrote its `docs/jobs/<id>/tasks.md` — the two writes every `mg jdi` run
  depends on.

## Verification (human, docker-gated — the brief's success criterion)

Requires a host with `docker` and the `zai`/`opencode-go` subscription keys
in `manigot/.env`. After this branch is merged, rebuild the image so the
`permission:` frontmatter is baked into the OpenCode agent copies:
`make rebuild` (or `make build` on a clean machine).

1. `make mg && make install` (or `make mg` + symlink `bin/mg` onto PATH).
2. Pick a real, non-trivial piece of work. Create a job, fill `brief.md` with
   substantive content (a terse-but-real brief — one filled section — is
   enough per the 4i5tcx fix, but a real feature brief is the honest test),
   and commit:
   ```
   mg job "add a small but real feature"
   # edit docs/jobs/<id>_<slug>/brief.md, commit it
   ```
3. Drive it end to end under `zai`:
   ```
   mg jdi --job <id> --profile zai
   ```
   Expected: `mg jdi: stop-finished — verdict.md's Overall verdict is
   APPROVED`, exit code 0, and on disk:
   - `.manigot/jdi-status/<job>/status.json` reads
     `{"state":"stopped:finished",...}` (the sidecar is excluded from the
     project's git via `.git/info/exclude`);
   - `.manigot/jdi-status/<job>/run.log` shows, in order, `mg jdi started`,
     per-agent `invoked (attempt 1)` / `finished (attempt 1)` pairs with the
     agent's extracted prose (never raw JSONL), and `mg jdi finished:
     stop-finished`;
   - `tasks.md`, `implementation.md` and `verdict.md` are written and the
     verdict's `## Overall` is APPROVED.
   Watch for the failure modes this job exists to kill: a false `stop-needs-
   human` with no progress, a stall ("made no progress on two consecutive
   runs"), or raw JSON in run.log.
4. Repeat with a second job under `opencode-go`:
   ```
   mg jdi --job <id2> --profile opencode-go
   ```
   same checks.
5. Reviewer read-only spot-check under an opencode profile: in the reviewer
   invocation's captured output (run.log's reviewer section), the review must
   not have modified any file other than `verdict.md`. (The enforcement was
   proven live in this sandbox; this is the belt-and-braces check on the
   container path.)

## Known issues / follow-ups

- The README's agent table lists `@mentor` and `@architect` as "read-only"
  too, but their `tools:` lists include Write/Edit and the brief scoped the
  OpenCode enforcement to `@reviewer`/`@security`/`@analyst`/`@owner`. If
  mentor/architect are meant to be truly read-only, a follow-up should either
  strip their Write/Edit under Claude Code and add `permission:` blocks, or
  fix the table.
- `@quality` describes itself as "Read-only" but is not in the brief's four
  and keeps its write tooling — same question as above, out of scope here.
- The reviewer's git-only bash allowlist means its old step-5 sed pipeline
  won't run under OpenCode; the body now prescribes the tool-neutral form,
  and the reviewer agent adapts either way (proven live — it read the files
  and ran `git diff` fine).
- The container E2E run under `zai`/`opencode-go` (the brief's success
  criterion) remains to be done by a human with docker + credentials, per the
  procedure above.
