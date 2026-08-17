# Verdict: playwright

id: village
status: open
reviewer: deepseek-v4-flash (2026-08-16)
date: 2026-08-16

## Review

Branch `feature/village_playwright` verified against base `main`
(`git diff main...HEAD`); all 13 job commits present with per-task
`[village] TASK-N:` format and a separate `[village] implementation:`
commit. Working tree clean. No out-of-scope files changed — every diff
entry maps to a task (Dockerfile, scripts/shot.js, internal/session/docker.go,
agents/*.md, docs/PLAYWRIGHT.md, docs/AGENTS.md, README.md,
project-template/docs/AGENTS.md, job-dir files). Code was reviewed by
inspection (this environment cannot run node/go or the image build; the
developer's execution claims were checked against the code and, for the
fixture, independently re-derived).

TASK-1: PASS
notes: Dockerfile installs registry-pinned playwright + `--with-deps
chromium-headless-shell` + Liberation/DejaVu fonts; `PLAYWRIGHT_BROWSERS_PATH`
points into /home/claude so the root-run install lands where the arbitrary-UID
runtime finds it; `chmod -R o+rwX` after the install runs as root before
`USER claude`. Two non-blocking notes: (a) `ENV PLAYWRIGHT_BROWSERS_PATH` is
set before the install RUN, so the npm postinstall also downloads full
chromium/firefox/webkit into that path — the image carries browsers never used
at runtime, contradicting the "headless-shell only (~90MB)" comment; setting
`PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1` on the npm install would fix it (build
time + image size, not correctness). (b) The analyst's explicit-apt-list
fallback was deliberately not implemented because `--with-deps` was verified
against playwright's debian13 deps map; the human `make rebuild` is the
documented gate and would surface any distro issue at the point the DoD's
first item is checked.

TASK-2: PASS
notes: scripts/shot.js implements the full interface (`<url>`, `--widths`,
`--full-page`, `--describe`, `--help`), resolves the job dir from the branch
with SHOT_OUTDIR override, and writes PNG + render-report.md/.json to
`<job dir>/screenshots/`. Measured layer matches PLAYWRIGHT.md scope item 3:
inventory with computed styles, WCAG AA/AAA with large-text rules,
overflow/clipping with leaf-cause dedup, sibling-grid alignment, spacing
scale, font status/fallbacks, viewport/DPR/overlap. PATH placement via
/usr/local/bin/shot + NODE_PATH without growing entrypoint.sh's bash. Two
non-blocking code notes: (a) findings are not deduplicated across `--widths`,
so the same defect is listed once per width in the aggregated Findings section
(cosmetic); (b) an element that is simultaneously an internal-overflow
candidate and a beyond-viewport candidate is dropped by the `isCause` dedup
(its two entries nullify each other) — an ancestor is usually flagged instead,
and the fixture does not hit this edge case.

TASK-3: PASS
notes: `--describe` matches the spec: Zhipu v4 chat-completions endpoint,
`image_url` content, `SHOT_VISION_MODEL` default `glm-4v-flash`, raw output
folded verbatim, non-OK responses name the model, and the no-key path records
`--describe unavailable: <reason>` in the report while capture + measured
report still land. The live one-shot smoke test is correctly queued behind the
human rebuild + a zai session (no ZHIPU_API_KEY in this environment). Minor:
the fetch has no timeout, so a hung API call hangs shot.

TASK-4: PASS
notes: zai-only availability documented in README.md's profile-key note and
docs/AGENTS.md's Config-files section. Verified against internal/session/
session.go: zai forwards only `ZHIPU_API_KEY` (line 189), opencode-go only
`OPENCODE_API_KEY` (line 193), claude-pro only the OAuth token + UUIDs — so
the documentation is accurate. No env-forwarding change made, per the
decision; the one-line `OpenCodeKeys` flip is recorded in probe.md.

TASK-5: PASS
notes: fixture/index.html + style.css committed with exactly the three seeded
defects (`.low-contrast` #999999 on white ≈2.85:1; `.notice-board` fixed
320px + `overflow-x: hidden` with a long unbroken word; `.card.misaligned`
margin-left 8px). Independently re-derived the finding set at 1280×900:
1 contrast ERROR, 1 internal-overflow ERROR (the card clips so no page-level
overflow; at 375px the notice-board still resolves to the same single leaf
cause), 1 alignment WARN (median grid line 20px, 8px near-miss) — exactly
"2 errors + 1 warning", nothing spurious. The PNG doubles as the probe image
per plan. Verification screenshots are not committed, which is consistent
with the hygiene rule (prune before summary commit); the human probe runs
will produce and commit them.

TASK-6: PASS
notes: quality gained the missing permission block (mirrors reviewer: git
commit allows, everything else deny) and designer gained one. Wrapper guard
implemented in scripts/shot.js (`guardAgent()`, default-allow when
`MANIGOT_AGENT_COMMITS` unset) and wired in internal/session/docker.go —
`agentCommits()` computed once and reused for both the gitdir mount mode and
the new `-e MANIGOT_AGENT_COMMITS=` flag; additive argv change cannot break
the existing contains-based docker tests. Existing OpenCode blocks verified to
deny bash for analyst/owner/security; developer's `bash: "*": allow` permits
shot; `permission:` passthrough through agentconv.go confirmed by reading
convertAgentFile (strips only name:/tools:). Two non-blocking notes:
(a) designer's block denies edit/task/webfetch/websearch/question — broader
than the task's "denying bash/shot". It makes designer fully read-only under
OpenCode (Claude Code is unchanged — blocks ignored there, designer keeps
Write/Edit), consistent with its `commit: false` and the read-only pattern,
but implementation.md's "matches its Claude Code surface" is inaccurate for
edit. (b) The guard keys off the committing marker, so reviewer and quality
(commit: true, Bash present) are NOT stopped by the guard under Claude Code —
only by the soft instruction text. This is exactly the analyst's documented
decision, but worth restating: Claude Code-side enforcement for reviewer/
quality is instruction-only.

TASK-7: PASS
notes: developer.md gains "Verifying rendered work" (shot usage + the
self-selection rule), read-only agents get consume-never-run instructions,
docs/AGENTS.md documents shot + the key-forwarding consequence,
project-template/docs/AGENTS.md mentions shot, README.md lists shot.js and
the profile-key table note. Two non-blocking doc nits: PLAYWRIGHT.md scope
item 7 still claims "read-only agents don't have Bash" — known-false
(reviewer/quality have Bash) and corrected in tasks.md/implementation.md/
docs/AGENTS.md but not in PLAYWRIGHT.md itself; and README.md's
"scripts/ ← one script only" comment is stale now that shot.js lives there.

TASK-8: PARTIAL (by design)
notes: probe.md commits the protocol (six sessions: 3 profiles ×
interactive/--print, fixture server + `shot --describe` steps, per-session
questions), the empty perception matrix, and prose-sample placeholders;
PLAYWRIGHT.md documents the fallback rule (agent self-selection) and that the
matrix decides the TASK-4 flip. The six sessions, the recorded matrix, prose
samples, and the final E2E pass are human-executed per PLAYWRIGHT.md's
human-in-the-loop (agents cannot rebuild the image — no docker socket — nor
launch other profiles — mg is host-side); this is the documented protocol,
not a defect, and the developer's own verification was correctly limited to
what a session can do (wrapper-guard refusal, no-key path, live-page capture,
external-URL smoke test). DoD items gated on the human steps remain open and
are listed below.

## Security

No security findings beyond what is already noted: shot renders arbitrary
URLs in a headless browser inside the container (bounded by the image's
`--security-opt=no-new-privileges` and no docker socket); `--describe` sends
the PNG to Zhipu using the existing forwarded `ZHIPU_API_KEY` (zai only); the
new `chmod -R o+rwX /home/claude/.cache` matches the image's existing
`o+rwX /home/claude` pattern; the wrapper guard is soft (mirrors the git
shim) and OpenCode permission blocks are the hard layer for read-only agents.
No secrets committed; `.env` untouched.

## Overall

APPROVED

Code-level review found no blockers. Every task's implementation matches its
tasks.md spec; commit discipline and scope are clean; the fixture's
objective-finding claim was independently re-derived and holds. The remaining
Definition-of-done items are the documented human-in-the-loop steps and are
not developer-fixable in a session — the human must execute them per
PLAYWRIGHT.md before this feature is operational:

1. `make rebuild` — the gate that bakes playwright + shot into the image
   (also validates the `--with-deps` claim on trixie).
2. The six perception-probe sessions (3 profiles × interactive/--print),
   results copied into the matrix + prose samples in probe.md.
3. The live `--describe` smoke test under a zai session (ZHIPU_API_KEY).
4. Final E2E pass: one session per profile confirming the full loop; commit
   the produced renders per the hygiene rule.

Non-blocking follow-up notes for a future job (none gate this merge): scope
the designer permission block to the decided bash/shot deny or accept the
full read-only pattern deliberately; correct the stale "read-only agents
don't have Bash" claim in PLAYWRIGHT.md scope item 7; set
`PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1` for the npm install so only the
headless shell lands in the image; add a fetch timeout to the vision call;
deduplicate findings across widths in the aggregated report.
