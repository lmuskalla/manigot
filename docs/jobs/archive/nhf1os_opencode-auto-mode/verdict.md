# Verdict: opencode auto mode

id: nhf1os
status: open
reviewer: deepseek-v4-flash
date: 2026-08-11

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

TASK-1: PASS
notes: Investigation, no code — correctly produced no commit. Verified against
the actually-installed binary (`opencode --version` → 1.18.16, matching the
Dockerfile's unpinned `npm install -g opencode-ai`): `--auto` ("auto-approve
permissions that are not explicitly denied (dangerous!)") is present in both
`opencode --help` (interactive `opencode [project]`) and `opencode run --help`
(headless subcommand), composing with `--format json` on the run subcommand.
(a) Confirmed OpenCode has no folder-trust/onboarding dialog — the brief's
complaint is per-tool prompts, not a trust dialog; (b) the global opencode
config entrypoint.sh writes (`$schema` + `model` only, line 87-93) and all
baked agents under `~/.config/opencode/agents/` contain no `permission:` deny
rules, so `--auto`'s "not explicitly denied" semantics yield full auto. Gates
TASK-2/3 correctly.

TASK-2: PASS
notes: `scripts/entrypoint.sh` line 169: `exec opencode "$@"` →
`exec opencode --auto "$@"`, flag placed before `"$@"` as specified, with a
comment mirroring the claude-code branch. run.sh builds `"$@"` as `(--agent
<name>) (--prompt <text>) [passthrough]` for opencode (run.sh:537-539), all
flags — the live check `opencode --auto --prompt "..." --agent developer`
(exit 0, TUI session ran with the `auto` badge shown) confirms composition.
Covers every interactive launch path (plain `mg --profile zai|opencode-go`,
`mg --agent`, job-prompted, TUI, legacy `--tool opencode`) since all route
through this single exec. The claude-code branch is untouched (diff shows it
as context only).

TASK-3: PASS
notes: `scripts/entrypoint.sh` line 159: `OC_ARGS+=(--format json)` →
`OC_ARGS+=(--auto --format json)` in the `MANIGOT_PRINT` branch. Exact
translated shape `opencode run <prompt> --agent <agent> --auto --format json`
reproduced live (exit 0, JSONL `step_start`/`text`/`step_finish` events,
agent replied "FLAG-OK") — flag composition with `--format json` and `--agent`
confirmed. Comment correctly frames it as a safety net per the foycfl finding,
and the "slightly beyond the strict brief" scope is explicitly called out in
tasks.md and implementation.md as requested.

TASK-4: PASS
notes: `docs/AGENTS.md` (canonical source) entrypoint bullet rewritten: opencode
half now reads "starts OpenCode in auto mode via `--auto` (full auto, no
per-tool prompts — the direct OpenCode analog of Claude's
`--dangerously-skip-permissions`...)" and the headless description now reads
`opencode run <message> --agent <agent> --auto --format json` — both match the
actual behavior at HEAD. `README.md` "Choosing a profile" table gained a
"Permissions" row (auto-approved via `--dangerously-skip-permissions` vs
`--auto`) plus a clarifying paragraph. Scope decisions verified: `docs/CLAUDE.md`
is empty and `project-template/docs/AGENTS.md` has no entrypoint bullet, so no
change there; root `AGENTS.md` is a runtime mount of `docs/AGENTS.md` (not a
tracked file at HEAD), so editing the canonical source is right.

TASK-5: PASS (with one acknowledged gap)
notes: `bash -n scripts/entrypoint.sh` clean. Both invocation shapes reproduced
against the real 1.18.16 binary (headless and interactive, exit 0, see
TASK-2/3 notes). `git diff f4a0adc..HEAD` (pre-job merge-base) shows only the
intended changes: `scripts/entrypoint.sh` (opencode branch only), `docs/AGENTS.md`,
`README.md`, and the job's own docs — provider-key forwarding, OPENCODE_MODEL
config write, git identity, docs mount, and the claude-code branch all
untouched. The live Docker-container observation (real `mg --profile zai`
session showing zero prompts) is inherently host-side and was correctly
deferred to a human per the tasks.md charter; the mechanism is verified against
the binary's own behavior, so the remaining check is confirmatory, not
correctness-gating.

## Security

No findings. `--auto` is marked dangerous by opencode itself, but the context
is unchanged from the accepted claude-code precedent (c4ouwc): an isolated,
ephemeral `--rm` container with `--security-opt=no-new-privileges`, a 2g memory
cap, bridge networking, the project root as the only host mount, and project
`.env` files shadowed with `/dev/null`. No secrets, no new host exposure. The
narrower "not explicitly denied" semantics are deliberately kept so a future
config-level `deny` stays enforced even in auto mode — a strict improvement
over a blanket bypass.

## Commit discipline

- `[nhf1os] TASK-2`, `[nhf1os] TASK-3`, `[nhf1os] TASK-4` — one commit each,
  correct `[ID] TASK-N: description` format.
- TASK-1 (investigation) and TASK-5 (verification) produce no code and
  correctly have no commits.
- `implementation.md` has its own commit (`[nhf1os] implementation: add
  summary`).
- Minor hygiene note (non-blocking): the TASK-2 commit (`0a029d6`) also swept
  in the analyst's task-breakdown edits to `tasks.md`, which were sitting
  uncommitted in the worktree at job start. The content is legitimate job
  content that belongs on the branch regardless, so the final tree is
  unaffected; ideally it would have been its own analyst commit.

## Scope

- Changed files vs pre-job baseline `f4a0adc`: `scripts/entrypoint.sh`,
  `docs/AGENTS.md`, `README.md`, and the job's own `docs/jobs/...` files. No
  unrelated refactoring.

## Bugs / edge cases

- None found. Flag ordering before `"$@"` (interactive) and within `OC_ARGS`
  (headless) both compose correctly with the run.sh passthrough shapes; JSONL
  output shape is unchanged by `--auto` (verified live), so the jdi orchestrator's
  parsing is unaffected; boolean flags tolerate any hypothetical duplication.

## Overall: APPROVED

The implementation does exactly what the brief asked (openCode sessions no
longer prompt per tool — "the whole idea of this isolation thing is not having
to worry about it doing stupid stuff"), matches the tasks.md spec line for
line, is cleanly scoped to the opencode branch of `scripts/entrypoint.sh`
(claude-code behavior untouched), is documented accurately in both canonical
docs, and every check verifiable from this sandbox passed. Nothing blocks
merge.

Recommended pre-merge host check (the only thing TASK-5 couldn't do from inside
the sandbox): run `make rebuild` (entrypoint.sh is COPYied into the image at
build time), then one `mg --profile zai` session and one
`mg jdi --job <id> --profile zai` run, and confirm no permission prompt
appears.
