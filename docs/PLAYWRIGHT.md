# PLAYWRIGHT — Agent sight: render, measure, perceive

**Status:** brief basis for a feature job
**Type:** feature
**Runs via `mg jdi`?** No — requires human-executed sessions under multiple profiles (see "Human-in-the-loop"). The developer's tasks are autonomous; the cross-profile verification is not.

## Why

Every agent in this system judges rendered UI by *inferring* it from code through a mental model of the browser. That model is wrong exactly when it matters: overflow, contrast that computes differently than expected, font failures, breakpoints collapsing, CSS properties that don't compose the way they read. For a product whose end users are non-technical editors, the rendered page *is* the product — agents reasoning from rendered reality instead of inference is a direct quality improvement.

The gap being closed is **perception, not judgment**. The solution has three layers:

1. **Capture** — Playwright renders a URL to a PNG.
2. **Measure** — the *primary* layer, deliberately model-free: a structured render report of exact DOM facts (contrast ratios, spacing, alignment, overflow, font status). Deterministic, works identically on every profile, interactive or headless, and *more precise* for design review than a model squinting at pixels.
3. **Perceive** — the *conditional* layer: for models that cannot see images (working hypothesis: the zai and opencode-go models), a vision-model prose description (GLM-4V-Flash via the existing `ZHIPU_API_KEY`) converts the PNG into design-relevant text the model can reason from.

Either way the model question lands — can see or can't — the feature ships. The answer only tunes whether layer 3 is wired in. That is a parameter, not a gate.

## Decisions made (do not re-litigate)

- **Renderer:** Playwright with `chromium-headless-shell`, not full Chromium (~90MB vs ~300MB). One browser, no cross-browser matrix.
- **The `shot` tool** (provisional name) is **agent-neutral tooling**, not a designer-only feature. Consumers: developer (verify own work — the roadmap #3 framing), reviewer (judge the result), designer (direct). Read-only agents **consume artifacts; they do not run `shot`**.
- **Artifacts land in the job dir** — PNG + render report committed to the job, human-visible in the TUI, readable by every agent and by the human reviewing a verdict.
- **`shot` is a Node script in this repo** (baked into the image at build), not bash — the one-bash rule (`scripts/entrypoint.sh`) stays intact.
- **Fonts are required, not optional.** A slim Debian ships none; typography review against tofu is worse than no review. Liberation + DejaVu sets.
- **External-URL rendering is this job's target.** `localhost` dev-server rendering rides on ROADMAP #3 (toolchains) — different dependency, not this job.
- **No new agent. `mg jdi`'s three-agent sequence is unchanged.** The tool happens to work headless (`--print` mode) because the render report is text; that's a property, not a feature to build.
- **Gating uses the proven dual pattern:** git-shim-style PATH placement for the tool under Claude Code, OpenCode `permission:` blocks per agent. Developer may run `shot`; read-only agents may not.

## Scope

1. **Image changes** (`Dockerfile`): playwright package (pinned), `chromium-headless-shell` install with its system deps, font layer. Verify `make build` / `make rebuild` succeeds and the image stays within a reasonable size delta.
2. **The `shot` tool** (`scripts/shot.js` + tiny wrapper), interface:
   - `shot <url>` — default viewport 1280×900; PNG + render report to `screenshots/` in the job dir
   - `shot <url> --widths 375,768,1280` — responsive review
   - `shot <url> --full-page` — full-height capture
   - `shot <url> --describe` — adds the vision-prose call (requires a vision key in env; clear error otherwise)
   - `shot --help`
3. **The render report** (the measured layer; markdown + JSON, written beside the PNG):
   - Element inventory: visible interactive/text elements, bounding rects, computed styles (font-size, weight, line-height, color, background, border)
   - WCAG contrast ratios for text elements, AA/AAA pass/fail
   - Overflow and clipping detection (`scrollWidth > clientWidth`, elements beyond viewport)
   - Alignment / shared-grid-line detection; spacing-scale extraction
   - Font loading status (`document.fonts`), fallback detection
   - Viewport, DPR, z-index/overlap anomalies
4. **The vision-prose layer** (`--describe`): base64 PNG → GLM vision model via `ZHIPU_API_KEY`; prompt engineered for design review (layout, hierarchy, typography, color/contrast, spacing, "what looks off"). Returns prose the text-only model can reason from.
5. **Perception probe** (early task, not a gate): one **committed fixture page with seeded, known defects** — a deliberate low-contrast pair (known failing ratio), a known overflow, a misalignment, specific colors and text — served via `python3 -m http.server` for deterministic, offline verification. `shot` against the fixture must flag exactly the seeded defects (objective pass/fail). The resulting PNG is also the perception-probe image: one artifact, two purposes.
6. **Per-profile wiring:** run the probe per profile × mode (interactive, `--print`), record the matrix, and document the fallback rule. The wiring mechanism is **agent self-selection, not profile-gated plumbing**: the agent files instruct "if you cannot see the PNG, run `shot --describe` and reason from the prose" — correct in both cases, self-adapting.
7. **Gating:** OpenCode `permission:` blocks — `shot` allowed for Bash-capable agents (developer), denied/absent for read-only agents. Claude Code side needs no new machinery (developer already has Bash; read-only agents don't).
8. **Docs sync** (per the hard rule): update `agents/*.md` (developer: verify own rendered work with `shot`; reviewer/designer: read the render report, view the PNG if the model supports images), `docs/AGENTS.md`, `project-template/docs/AGENTS.md`.

## Out of scope

- Interactive browsing, click/scroll loops, browser-use agents
- `localhost` dev-server rendering (depends on roadmap #3 toolchains)
- Full Chromium, video recording, tracing, screenshots of media
- Automated accessibility auditing beyond the measured layer (axe-core is a *candidate follow-up*, not now)
- Cross-browser rendering (Chromium only)
- TUI changes beyond what falls out of artifacts existing in the job dir (a TUI PNG viewer is a separate decision)
- Any change to the `mg jdi` sequence or the agent roster

## Open questions for the analyst (design decisions, not re-decisions)

- Exact playwright + `chromium-headless-shell` versions and install flags that work in `node:22-trixie-slim` (`--with-deps` vs explicit apt list — the deps must be enumerated either way)
- Where `shot` lives in the image and how it lands on PATH without growing `entrypoint.sh`'s bash
- Which GLM vision model/endpoint is current (`glm-4v-flash` vs newer), exact request shape, and whether the container-env `ZHIPU_API_KEY` alone is sufficient (key scoping)
- **Key availability across profiles**: which provider keys exist in which profile's session env — this determines when `--describe` is available, not just which profile
- OpenCode `permission:` syntax for `shot` per agent, consistent with the read-only agents' existing blocks
- Whether `--describe` should also return the raw model output verbatim for the human (artifact transparency) — recommend yes
- Committed-screenshot hygiene: developer prunes unhelpful renders before the final summary commit

## Human-in-the-loop

Agents cannot rebuild the image (no docker socket in sessions) and cannot launch sessions under other profiles (the `mg` binary is host-side). Required human steps, documented as a protocol the developer writes:

1. `make rebuild` after the Dockerfile change
2. The perception probe sessions under `zai` and `opencode-go` (three profiles × two modes ≈ six short sessions; copy-paste results back into the job)
3. Final E2E pass: one session per profile confirming the full loop

This is the established human-E2E pattern from prior jobs, not a new mechanism. The developer's tasks must sequence around the rebuild (code + fixture first, then verify in a fresh session on the rebuilt image).

## Definition of done

- Image builds with headless-shell + fonts; build succeeds cleanly
- `shot` on the fixture page produces PNG + report; the report **objectively flags every seeded defect** (known contrast ratio, known overflow, known misalignment) and nothing spurious
- `shot` on one external URL works (smoke test)
- Perception matrix recorded for all three profiles × interactive/`--print`
- `--describe` returns usable design prose where a key is available; graceful, documented error where not
- OpenCode permission blocks enforce "read-only agents do not run `shot`"; Claude Code side verified
- Agent files and docs synced per the sync rule
- Fixture + probe + matrix + prose samples committed to the job dir
- Reviewer's verdict APPROVED

## Follow-ups (recorded, not this job)

- `localhost` rendering once roadmap #3 (toolchains) lands
- axe-core integration as an extension of the measured layer
- TUI PNG viewing decision
- Possibly graduating the fixture page into repo-level `testdata/`
