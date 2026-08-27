# Provenance

## webapp-testing

- **Upstream**: Anthropic's official Agent Skills repository —
  https://github.com/anthropics/skills — `skills/webapp-testing/`
  (fetched at commit `3b3fad96af16a10759d930941b4520ba0c40edae`, 2026-08-27)
- **License**: Apache-2.0 (see `LICENSE.txt` in this directory, verbatim from
  upstream)
- **Adapted from the original**:
  - Removed the `license:` frontmatter key (manigot frontmatter contract:
    `name:` + one-line `description:` only).
  - Added a short "Prerequisite" note to the body: the manigot container ships
    Python 3 + a Playwright browser; the Python client may need
    `pip install playwright` (network egress available). Upstream assumes
    Playwright is already installed.
  - `examples/console_logging.py` and `examples/static_html_automation.py`:
    replaced the Claude.ai-specific `/mnt/user-data/outputs/` output path with
    `/tmp/` (Linux container path). All other content verbatim.
- **Runtime note**: the bundled `scripts/with_server.py` and `examples/` are
  plain Python + bash — no CLI-specific syntax.