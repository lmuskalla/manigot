# Provenance

## frontend-design

- **Upstream**: Anthropic's official Agent Skills repository —
  https://github.com/anthropics/skills — `skills/frontend-design/`
  (fetched at commit `3b3fad96af16a10759d930941b4520ba0c40edae`, 2026-08-27)
- **License**: Apache-2.0 (see `LICENSE.txt` in this directory, verbatim from
  upstream)
- **Adapted from the original**:
  - Removed the `license:` frontmatter key — manigot's skill frontmatter
    contract is `name:` + one-line `description:` only; license is recorded
    here and in `LICENSE.txt`.
  - Body otherwise verbatim. No Claude-specific syntax; relies only on the
    standard tool set (Read/Grep/Glob/Write/Edit) both CLIs provide.