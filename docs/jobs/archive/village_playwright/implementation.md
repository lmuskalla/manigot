# Implementation: playwright

id: village
status: open
developer: deepseek-v4-flash (2026-08-15)
date: 2026-08-15

## Summary

Introduced `shot` — Playwright-based agent sight — into the tool kit per
`docs/PLAYWRIGHT.md`. The image now builds playwright (version resolved from
the npm registry at build time) with `chromium-headless-shell` installed via
`--with-deps` plus Liberation/DejaVu fonts. `scripts/shot.js` (baked into the
image as `/usr/local/bin/shot`) renders a URL to PNGs and produces a
model-free render report (element inventory, WCAG AA/AAA contrast, overflow/
clipping, alignment/spacing, font status, viewport/DPR/z-index), with an
optional `--describe` vision-prose layer via the Zhipu GLM endpoint.
Gating follows the proven dual pattern: OpenCode `permission:` blocks
(added the missing blocks for designer and quality), a wrapper guard in
`shot` itself for the Claude Code side (`MANIGOT_AGENT_COMMITS` env), and
soft instruction text in the agent files. A committed fixture page with
three seeded defects verifies `shot` flags exactly those and nothing
spurious; the external-URL smoke test passes; the perception-probe protocol
and matrix template are committed for the human to execute.

## Changes

TASK-1: `Dockerfile` — playwright installed per the analyst's decision: the
  version is read from the registry at build time (`npm view playwright
  version`) and pinned for the build, then `npx playwright install
  --with-deps chromium-headless-shell`. **`--with-deps` was verified to work
  on Debian 13 trixie** (playwright 1.62's deps map has an official `debian13`
  entry — confirmed by inspecting the installed package), so the analyst's
  fallback explicit-apt-list was NOT needed. Fonts (`fonts-liberation`,
  `fonts-dejavu-core`) added to the system-deps layer. `PLAYWRIGHT_BROWSERS_PATH`
  points at `/home/claude/.cache/ms-playwright` so the root-run install lands
  where the runtime (arbitrary host UID, HOME=/home/claude) finds it; the
  install runs after the `usermod` step and chmods the cache dir so every
  session UID can read/execute the browser.

TASK-2: `scripts/shot.js` (the `shot` CLI) — capture + render-report core.
  Interface: `shot <url>` (1280×900), `--widths 375,768,1280`, `--full-page`,
  `--help`; PNGs + `render-report.md`/`.json` written to
  `<job dir>/screenshots/` (job dir resolved from the git branch, override
  with `SHOT_OUTDIR`). Measured layer runs fully in-page: element inventory
  with computed styles, WCAG contrast (AA/AAA, large-text rules), overflow
  deduplication (page-level scroll + leaf cause, skipping ancestors of a
  flagged descendant), sibling-based shared-grid-line alignment with
  near-miss detection, spacing-scale extraction, font status/fallbacks,
  viewport/DPR/overlap anomalies. Lands on PATH as `/usr/local/bin/shot`
  (node shebang + `NODE_PATH=/usr/local/lib/node_modules` in the Dockerfile)
  without growing entrypoint.sh's bash. Verified locally against a live page
  (headless-shell with system deps assembled in a scratch prefix).

TASK-3: `--describe` vision-prose layer in `shot` — base64 PNG → Zhipu v4
  chat-completions (`https://open.bigmodel.cn/api/paas/v4/chat/completions`,
  `image_url` content), model from `SHOT_VISION_MODEL` defaulting to
  `glm-4v-flash`; design-review prompt (layout, hierarchy, typography, color/
  contrast, spacing, "what looks off"); raw model output folded into the
  report verbatim. Errors are non-fatal and documented: no `ZHIPU_API_KEY`
  records `--describe unavailable: <reason>` in the report instead of aborting
  (capture + measured report still land); a non-OK API response names the
  model so a stale name is a one-line `SHOT_VISION_MODEL` fix. No-key path
  verified locally; the live one-shot smoke test is queued behind the human
  `make rebuild` + a zai session (no ZHIPU_API_KEY in this environment).

TASK-4: `--describe` availability resolved as **zai-only, documented** — no
  env forwarding change. `internal/session` already forwards `ZHIPU_API_KEY`
  only for zai; forwarding it into opencode-go (and claude-pro) is
  test-pinned and security-sensitive, and the perception matrix that would
  justify it is human-gated. Documented in README.md (profile table note) and
  docs/AGENTS.md (config-files section): zai has `--describe` today, other
  profiles get the documented "no key" error and rely on the model-free
  render report. The flip path is a one-line `OpenCodeKeys` change + tests,
  recorded in probe.md.

TASK-5: `docs/jobs/village_playwright/fixture/` (`index.html` + `style.css`) —
  committed fixture with exactly three seeded defects: `#999999` on white
  (≈2.85:1, fails AA), a fixed-width notice box whose long word is clipped
  (`overflow-x: hidden`, exactly one internal-overflow finding — the box clips
  so the page doesn't scroll), and a card pushed 8px off the sibling grid
  line. Verified locally: `shot` on the fixture flags exactly those three
  (2 errors + 1 warning) and nothing spurious. The PNG doubles as the
  perception-probe image.

TASK-6: gating. (a) OpenCode `permission:` blocks: verified the existing
  developer `bash: "*": allow` permits `shot`; analyst/owner/reviewer/security
  already denied it; added the missing blocks for **quality** (mirrors
  reviewer: git commit allows, everything else denied) and **designer**
  (`edit: deny`, `bash: deny`, ... — matches its Claude Code surface with no
  Bash tool). `permission:` passthrough through `agentconv.go` confirmed by
  the existing test. (b) Claude Code side: `internal/session/docker.go` now
  passes `MANIGOT_AGENT_COMMITS` (reusing the already-computed
  `agentCommits()`); `shot`'s `guardAgent()` refuses to run when it's
  `false`, defaulting to allow when unset — the soft-but-present layer
  mirroring the git shim. (c) Soft instruction text ("consume the render
  report and PNG; never run `shot`") added to analyst/owner/security/reviewer/
  quality/designer. All `internal/...` Go tests pass.

TASK-7: docs sync per the hard rule — `agents/developer.md` gains a
  "Verifying rendered work" section (`shot` usage + the self-selection rule
  "if you cannot see the PNG, run `shot --describe`"), read-only agents get
  the consume-never-run instruction (TASK-6), `docs/AGENTS.md` gains a `shot`
  bullet in Stack + the key-forwarding consequence in Config files,
  `project-template/docs/AGENTS.md` mentions `shot` in its comment block, and
  README.md lists `scripts/shot.js` in the repo layout.

TASK-8: perception probe — `docs/jobs/village_playwright/probe.md` commits the
  protocol (six sessions: 3 profiles × interactive/`--print`, fixture server
  + `shot --describe` steps, per-session questions), the empty matrix
  template, and prose-sample placeholders; `docs/PLAYWRIGHT.md` documents the
  fallback rule and that the matrix decides whether TASK-4's zai-only
  key-forwarding should be widened. **The six sessions themselves are
  human-executed** (agents cannot rebuild the image — no docker socket — nor
  launch other profiles — `mg` is host-side); this is the documented
  human-in-the-loop step from PLAYWRIGHT.md. External-URL smoke test passed
  (`shot https://example.com` renders + reports cleanly).

## Verification status

Done autonomously (this session): shot captures + measured report on a live
page; fixture flags exactly the seeded defects; external-URL smoke test;
wrapper guard refusal; `--describe` no-key graceful error; all Go tests
green; agentconv passthrough confirmed.

Human-gated (per PLAYWRIGHT.md, cannot be done by an agent): `make rebuild`
to bake playwright + shot into the image; the six perception-probe sessions
+ matrix; the final E2E pass (one session per profile confirming the full
loop); the live `--describe` smoke test under a zai session (needs
`ZHIPU_API_KEY` in the session env).

## Known issues / follow-ups

- The live Zhipu vision call is unverified in this environment (no
  `ZHIPU_API_KEY`); the model default `glm-4v-flash` may need a
  `SHOT_VISION_MODEL` tweak if the registry has moved — the error message
  names the model so the fix is one env change.
- `--describe` availability is zai-only today (TASK-4 decision, documented);
  the perception matrix may flip it to also forward `ZHIPU_API_KEY` into
  opencode-go sessions (one-line `OpenCodeKeys` change + test update).
- The fixture page is committed in the job dir per the tasks; PLAYWRIGHT.md
  records "possibly graduating the fixture into repo-level `testdata/`" as a
  follow-up — not done here (out of scope).
- axe-core integration and TUI PNG viewing remain recorded follow-ups in
  PLAYWRIGHT.md, untouched.
