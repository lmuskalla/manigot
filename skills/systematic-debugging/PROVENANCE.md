# Provenance

## systematic-debugging

- **Upstream**: obra/superpowers — https://github.com/obra/superpowers —
  `skills/systematic-debugging/` (fetched at commit
  `b36e0829c6d0140e93cfef2ca599b1b07d4a7797`, 2026-08-27)
- **License**: MIT (repo-wide LICENSE, Copyright (c) 2025 Jesse Vincent). No
  separate per-skill license file upstream.
- **Adapted from the original**:
  - `SKILL.md`: de-referenced two sibling-skill pointers that manigot does not
    ship — `superpowers:test-driven-development` → `test-driven-development`
    (shipped) and `superpowers:verification-before-completion` → a plain
    "verify the fix before claiming success" instruction.
  - `root-cause-tracing.md`: changed the `find-polluter.sh` invocation from
    `./find-polluter.sh` to `bash find-polluter.sh`, because staged skill
    copies are written with 0644 (the +x bit is stripped), so a direct
    `./` invocation would fail in the container; `bash <script>` does not
    need the executable bit.
  - Kept verbatim (byte-for-byte identical to the upstream commit): the
    remaining in-directory support files — `defense-in-depth.md`,
    `condition-based-waiting.md`, `condition-based-waiting-example.ts`, and
    `find-polluter.sh`. Note that `find-polluter.sh`'s `${TEST_PATTERN#./}`
    strip and the dual `-path` with the `**/`-collapsed fallback (lines
    21-27, including the developer-style commentary) are present in the
    upstream file at the pinned commit and are reproduced unchanged — they
    are upstream-verbatim, not manigot modifications.
  - Dropped upstream's internal development artifacts (`CREATION-LOG.md`,
    `test-academic.md`, `test-pressure-*.md`) — not referenced by the skill
    body and not part of the skill's runtime.
  - No CLI-specific syntax anywhere; TypeScript examples are illustrative.