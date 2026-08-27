# Verdict: add best practice skills

id: soil
status: open
reviewer:
date: 2026-08-27

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

Re-review of the branch after the previous NEEDS WORK verdict and the
developer's fix commit `928c751` ("TASK-3 fix: record find-polluter.sh
provenance accurately in PROVENANCE.md").

TASK-1: PASS
notes: Selection rationale in implementation.md applies the brief's six
criteria to four adoptions from reputable, actively-maintained sources
(anthropics/skills — the official Anthropic repo — and obra/superpowers),
each covering a craft area the brief names (design/UI-UX, web app testing,
TDD, debugging). The author-named `ui-ux-pro-max-skill` is explicitly
evaluated and rejected with four concrete, criteria-based reasons
(Claude-plugin `${CLAUDE_PLUGIN_ROOT}` dependency, not self-contained in
manigot's sense, multi-line description violating the frontmatter contract,
role covered by the lighter official `frontend-design`) — the
"include it if it passes, otherwise document why it was rejected"
requirement is met. The conservative reading (4 skills) is chosen and
stated. Upstream commit SHAs and the ~121k-star claim could not be
independently verified in this environment (no network), but the records
are internally consistent and the rejection list shows genuine research
(specific repos, a 404 note on software-architecture).

TASK-2: PASS
notes: All four skills vendored as `skills/<name>/` directories with
SKILL.md at the top level (the `listSkills` loader contract in
`src/internal/session/skillconv.go` — immediate subdir containing SKILL.md;
re-verified against the loader source). Frontmatter contract holds for all
four (name == directory name; one-line description). Content verified
tool-agnostic: no `superpowers:` cross-skill pointers remain in any body
(grep: matches only in PROVENANCE.md), no `/mnt/user-data` paths left
(grep: only in PROVENANCE.md's adaptation notes), no `license:` frontmatter
key, no Claude-only slash commands or `@agent` mentions. Self-contained:
every file each SKILL.md body references ships in the same directory
(webapp-testing: scripts/with_server.py + 3 examples; tdd:
writing-good-tests.md; systematic-debugging: root-cause-tracing.md,
defense-in-depth.md, condition-based-waiting.md, find-polluter.sh,
condition-based-waiting-example.ts). The `pip install playwright`
prerequisite for webapp-testing is documented in the body. Non-blocking
observations (all carried over from the prior review, cosmetic or inherent):
(a) superpowers "ask your human partner" framing retained verbatim —
acceptable for interactive sessions, safe default under `mg jdi`; (b)
relative script paths (`python scripts/with_server.py`) assume the skill dir
is the CWD — inherent to upstream, an agent can locate the scripts; (c)
heading typo "## your human partner's Signals You're Doing It Wrong" at
systematic-debugging/SKILL.md:233 (cosmetic); (d) "abslutely" typo at
webapp-testing/SKILL.md:19 (upstream-verbatim).

TASK-3: PASS
notes: The previous blocker is closed. `skills/systematic-debugging/
PROVENANCE.md` now explicitly states that the `${TEST_PATTERN#./}` strip and
the dual `-path` with the `**/`-collapsed fallback (lines 21-27 of
find-polluter.sh) are upstream-verbatim at the pinned commit
`b36e0829c6d0140e93cfef2ca599b1b07d4a7797`, and records the one real
adaptation in that directory that was previously undocumented:
`root-cause-tracing.md` invokes `bash find-polluter.sh` rather than
`./find-polluter.sh` (verified present at root-cause-tracing.md:104; correct,
since staged copies are written 0644 with the +x bit stripped). The previous
review's requirement was "record the actual adaptations OR explicitly state
that those lines are upstream-verbatim" — the latter is now done with
specificity (line range + content described), so the deliverable "exactly
what was adapted from the original" is met either way. Caveat, unchanged
from the prior review: the upstream-verbatim claim could not be
independently verified offline (no network in either review session); the
record is now explicit and specific, and the developer states they re-checked
byte-for-byte. All four PROVENANCE.md files record upstream URL, pinned
commit SHA, license, and adaptations; licenses are permissive (Apache-2.0
×2 with vendored LICENSE.txt, MIT ×2 recorded with copyright holder). The
two Apache-2.0 LICENSE.txt texts differ in length (one omits the
informational appendix) — both valid Apache-2.0 texts, provenance-accuracy
nit only.

TASK-4: PASS
notes: The `./find-polluter.sh` → `bash find-polluter.sh` fix in
root-cause-tracing.md is the TASK-4 commit's only content change (verified
in the diff) and is correct given the 0644 staging behavior. No `__pycache__`
or other build artifacts remain (glob-verified); dropped upstream dev
artifacts (CREATION-LOG.md, test-*.md) are absent. The Python files and the
bash helper are syntactically sound on inspection. `go test ./...` could not
be re-run in this review session (reviewer bash is restricted to git
commands — same limitation as the prior review), but the branch contains
zero Go changes (diff is confined to skills/, README.md, docs/AGENTS.md and
job docs), and no test references the real shipped skills by name (all
skillconv/host/docker tests use synthetic fake-home skill dirs), so the
green-suite claim is low-risk. The optional unit test was skipped with a
stated reason — allowed ("optionally"). The no-live-container smoke-test
limitation is flagged exactly as the tasks requested, mirroring the
predecessor job.

TASK-5: PASS
notes: README.md "## Skills" section lists the shipped set with one-line
roles; docs/AGENTS.md Stack bullet and hard-rule phrasing updated (diff
verified, and grep confirms "ships no skills"/"ships one example skill" no
longer appears in any live doc — remaining matches are in the archived
get_skills job, correctly left as history). project-template/docs/AGENTS.md
describes only the mechanism with no count/name claims (grep-verified) and
agents/*.md have no skills-mechanism references (matches are generic English
usage), so the "no changes needed" verification is correct.

## Security

None. Reviewed every shipped skill file for prompt-injection or harmful
instructions: all four are standard best-practice craft guidance from
reputable sources; the "calibration", "red flags" and "common
rationalizations" sections are instructional, not manipulative; no hidden
instructions, no exfiltration patterns, no external data sources referenced.
Licenses re-checked: Apache-2.0 (anthropics/skills, LICENSE.txt vendored)
and MIT (obra/superpowers, recorded in PROVENANCE.md) — both permissive and
compatible with shipping in the checkout.

## Overall

APPROVED

The four adopted skills are defensible against every brief criterion, the
vendoring is clean and tool-agnostic, provenance/license records are
complete, docs are synced, and the diff is strictly in scope with no
out-of-task changes and no Go code touched. The single blocker from the
previous review — TASK-3's incomplete find-polluter.sh provenance record —
is resolved: PROVENANCE.md now explicitly documents the script's provenance
either way (upstream-verbatim for lines 21-27, adaptation recorded for the
`bash` invocation) and implementation.md carries the fix note. The only
remaining caveats are environmental (no live-container smoke test; the
upstream-verbatim claim unverifiable offline) and are documented in
implementation.md exactly as the tasks require.

Nothing must change before merge.