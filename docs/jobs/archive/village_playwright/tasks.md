# Tasks: playwright

id: village
status: open
analyst: deepseek-v4-flash (2026-08-15)
date: 2026-08-15

<!-- Produced by @analyst from brief.md + docs/PLAYWRIGHT.md. -->

## Scope source

`docs/PLAYWRIGHT.md` is the authoritative feature brief (read it first). Its
decisions are **not to be re-litigated** (renderer = `chromium-headless-shell`
only; `shot` is agent-neutral tooling; artifacts land in the job dir; Node
script not bash; fonts required; external-URL rendering only; no new agent; no
`mg jdi`/roster changes). The tasks below implement it and resolve its open
questions with the decisions recorded here.

## Decisions made by the analyst (2026-08-15)

- **Playwright version pin is NOT hardcoded in this plan.** TASK-1 reads the
  current version from the registry (`npm view playwright version`) inside the
  build and pins that. No opinion needed; the registry is the authority.
- **`--with-deps` vs explicit apt list:** try `npx playwright install
  --with-deps chromium-headless-shell` first; Playwright's dep scripts may not
  recognize Debian 13 (trixie) yet — if so, fall back to an explicit apt list
  (libnss3, libnspr4, libatk1.0-0, libcups2, libdrm2, libxkbcommon0,
  libxcomposite1, libxdamage1, libxfixes3, libxrandr2, libgbm1, libasound2,
  fonts-liberation, fonts-dejavu-core). Record which worked in
  implementation.md.
- **Vision model name is env-configurable, not hardcoded.** `shot --describe`
  calls the OpenAI-compatible Zhipu v4 chat-completions endpoint
  (`https://open.bigmodel.cn/api/paas/v4/chat/completions`, `image_url`
  content) with the model read from `SHOT_VISION_MODEL`, defaulting to
  `glm-4v-flash`. A one-shot live smoke test during TASK-3 answers whether the
  default is current; a 404 must name the model in its error so the fix is a
  one-line env change. Record the working model in implementation.md.
- **`--describe` key availability (from `internal/session/session.go`
  `CheckAuth`):** today only `zai` forwards `ZHIPU_API_KEY` into the container;
  `opencode-go` forwards only `OPENCODE_API_KEY` and `claude-pro` forwards only
  the OAuth token + UUIDs. So `--describe` only works under `zai` today.
  TASK-4 decides whether to forward `ZHIPU_API_KEY` into the other profiles'
  sessions or document zai-only — informed by the TASK-8 perception matrix.
- **Gating, Claude Code side:** PLAYWRIGHT.md's claim "read-only agents don't
  have Bash" is FALSE for `reviewer` and `quality` (both have `Bash` in their
  Claude Code tools, needed to commit verdict.md/quality.md). Enforcement =
  a guard inside the `shot` wrapper itself that refuses to run for
  non-committing agent contexts (mirroring the git shim's soft-but-present
  layer), PLUS soft instruction text in the agent files ("consume the render
  report and PNG; never run `shot`").
- **Gating, designer:** `agents/designer.md` has NO `permission:` block —
  under OpenCode `--auto` an agent with no block gets implicitly-approved bash,
  so designer could run `shot`. TASK-6 adds a `permission:` block to designer
  denying bash/`shot`, matching its Claude Code surface (no Bash tool).
- **Gating, OpenCode side:** `permission:` blocks pass through the agent
  conversion untouched (`internal/session/agentconv.go`). Developer's existing
  `bash: "*": allow` permits `shot`; read-only agents' blocks deny it. Verify,
  don't rebuild.
- **Screenshot hygiene:** developer prunes unhelpful renders before the final
  summary commit.

## Human-in-the-loop (from PLAYWRIGHT.md — required, not optional)

Agents cannot rebuild the image (no docker socket) and cannot launch sessions
under other profiles (`mg` is host-side). The developer writes the protocol;
the human executes:

1. `make rebuild` after TASK-1's Dockerfile change — the gate between the
   code-first chunk and all verification.
2. The six perception-probe sessions (3 profiles × interactive/`--print`),
   results copied back into the job (TASK-8).
3. Final E2E pass: one session per profile confirming the full loop.

## Task breakdown

TASK-1: Dockerfile — install pinned Playwright + `chromium-headless-shell` + fonts, image build green
     files: Dockerfile
     depends: none
     risk: medium — headless-shell system deps on Debian 13 trixie can be
     finicky (`--with-deps` may not recognize the distro); resolved at build
     time by trying `--with-deps` then falling back to the explicit apt list.

TASK-2: Write `scripts/shot.js` (the `shot` CLI) — capture + render report core
     files: scripts/shot.js, Dockerfile (COPY/PATH placement)
     depends: TASK-1 (to run/verify; may be written first)
     risk: high — the measured layer (element inventory, WCAG AA/AAA contrast,
     overflow/clipping, alignment/spacing, font-load status, viewport/DPR/
     z-index) is the largest novel code chunk; DoD requires it flags exactly
     the seeded defects and nothing spurious.
     Interface (from PLAYWRIGHT.md, scope item 2): `shot <url>` (1280×900),
     `--widths 375,768,1280`, `--full-page`, `--describe`, `--help`; PNG +
     markdown/JSON render report written to `screenshots/` in the job dir.
     PATH placement: decide at implementation (image path + `#!/usr/bin/env
     node` shebang, or tiny wrapper) — must land on PATH without growing
     `scripts/entrypoint.sh`'s bash.

TASK-3: Add the `--describe` vision-prose layer to `shot`
     files: scripts/shot.js, docs/PLAYWRIGHT.md (error wording)
     depends: TASK-2
     risk: medium — depends on a live Zhipu call; model/endpoint currentness is
     answered by the one-shot smoke test, model name is env-configurable
     (`SHOT_VISION_MODEL`, default `glm-4v-flash`). Design-review prompt:
     layout, hierarchy, typography, color/contrast, spacing, "what looks off".
     Return raw model output verbatim for the human (recommended yes — fold
     into the report's prose section). Clear, documented error when no key.

TASK-4: Resolve `--describe` availability per profile (env forwarding decision)
     files: internal/session/session.go, internal/session/*_test.go,
     README.md profile table, docs/AGENTS.md
     depends: TASK-3, TASK-8 probe results
     risk: medium — session env forwarding is test-pinned and security-sensitive
     (which keys ride into which profile's container). Decide: forward
     `ZHIPU_API_KEY` into opencode-go (and claude-pro?) sessions via
     `CheckAuth()`, or document zai-only and word the agent self-selection
     instruction to match. Probe results (which models can see images) decide.

TASK-5: Build the committed fixture page with seeded defects; verify `shot`
     flags exactly those and nothing spurious
     files: fixture HTML/CSS committed in docs/jobs/village_playwright/
     depends: TASK-2 (to test against); verification runs post-rebuild (TASK-8)
     risk: low — deterministic and offline (served via `python3 -m http.server`).
     Seeded defects: known-failing contrast pair, known overflow, misalignment,
     specific colors/text. The produced PNG doubles as the perception-probe
     image (one artifact, two purposes).

TASK-6: Gating — OpenCode `permission:` blocks + Claude Code wrapper guard +
     designer block
     files: agents/developer.md, agents/reviewer.md, agents/quality.md,
     agents/designer.md, agents/analyst.md, agents/owner.md,
     agents/security.md, scripts/shot.js (wrapper refusal)
     depends: TASK-1 (tool on PATH)
     risk: medium — two gaps in PLAYWRIGHT.md's gating model: reviewer/quality
     have Bash under Claude Code (wrapper guard + soft instruction), and
     designer has no `permission:` block (add one denying bash/`shot`). Verify
     the existing OpenCode blocks pass through `agentconv.go` untouched.

TASK-7: Docs sync per the hard rule
     files: agents/*.md, docs/AGENTS.md, project-template/docs/AGENTS.md,
     README.md
     depends: TASK-3, TASK-4 (final wording), TASK-6
     risk: low — mechanical once the wiring is settled.
     Content: developer verifies own rendered work with `shot`; reviewer/
     designer read the render report and view the PNG if the model supports
     images; self-selection rule "if you cannot see the PNG, run
     `shot --describe` and reason from the prose". README profile/key table
     only if TASK-4 changes env forwarding.

TASK-8: Perception probe — protocol + matrix template written, then
     human-in-the-loop sessions run and matrix recorded
     files: docs/jobs/village_playwright/ (matrix, probe results, prose
     samples), docs/PLAYWRIGHT.md (fallback rule)
     depends: TASK-1 (rebuild gate), TASK-5, TASK-3; HUMAN EXECUTION REQUIRED
     risk: medium — gated on the human performing `make rebuild` + six sessions;
     developer's own verification only happens in a fresh session on the
     rebuilt image.
     Deliverables per DoD: fixture + probe + matrix + prose samples committed
     to the job dir; perception matrix recorded for all 3 profiles ×
     interactive/`--print`; fallback rule documented; external-URL smoke test.

## Sequencing (from PLAYWRIGHT.md)

1. Code-first chunk (autonomous): TASK-1, TASK-2, TASK-5 — write the code and
   fixture before the rebuild.
2. HUMAN: `make rebuild` (TASK-8 step 1). This gates ALL verification — the
   developer cannot test `shot` until the image is rebuilt.
3. Post-rebuild: verify `shot` on the fixture (TASK-5 verification), live
   `--describe` smoke test (TASK-3), gating verification (TASK-6), docs sync
   final wording (TASK-7).
4. HUMAN: six probe sessions + final E2E (TASK-8 steps 2–3); matrix recorded.
5. Screenshot hygiene: prune unhelpful renders before the summary commit.

## Definition of done (from PLAYWRIGHT.md)

- Image builds with headless-shell + fonts; build succeeds cleanly
- `shot` on the fixture page produces PNG + report; the report objectively
  flags every seeded defect and nothing spurious
- `shot` on one external URL works (smoke test)
- Perception matrix recorded for all three profiles × interactive/`--print`
- `--describe` returns usable design prose where a key is available; graceful,
  documented error where not
- OpenCode permission blocks enforce "read-only agents do not run `shot`";
  Claude Code side verified (wrapper guard + agent instructions)
- Agent files and docs synced per the sync rule
- Fixture + probe + matrix + prose samples committed to the job dir
- Reviewer's verdict APPROVED
