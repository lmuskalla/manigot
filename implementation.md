## Summary

Removed every display of the ASCII logo. The user wanted the logo itself (the
`assets/manigot.txt` asset, produced by the earlier "mg logo in tui" job) but
did not want it shown anywhere — not in the TUI, not in the CLI session
banner, not inside the container session. All render sites were reverted;
the asset file is kept as-is.

## Changes

- `internal/session/docker.go`, `internal/session/host.go` — removed the
  `printLogo` call (and the function) from the session info banner, so
  neither the docker nor the host launch banner prints the ASCII logo above
  the boxed details anymore.
- `internal/brand/` — deleted the whole package (`logo.go`, `logo_test.go`):
  it existed solely to serve the logo string to the two render sites; with no
  render sites left it was dead code.
- `internal/ui/list.go`, `internal/ui/styles.go` — removed the TUI job-list
  header logo: the `logo`/`logoWidth` fields, `loadLogo`, `logoShown`, the
  render block above the title, and `logoStyle`. `recentActivityShown` lost
  its now-unused `width` parameter and the logo-height folding; its vertical
  budget math (`dashboardFixedChrome`) is unchanged.
- `scripts/entrypoint.sh`, `Dockerfile` — removed the in-container logo print
  (the `cat` of the baked-in asset above the flavor quote) and the
  `COPY assets/manigot.txt` into the image.
- Tests: dropped the logo-presence assertions from
  `internal/session/docker_test.go` / `host_test.go`, the logo asset stub
  from `session_test.go`'s `checkout` helper, and the four logo tests in
  `internal/ui/list_test.go` (the two remaining `recentActivityShown` call
  sites were updated for the dropped `width` arg).
- Docs: `README.md` no longer shows the ASCII logo in a fenced block (the
  pre-existing `assets/manigot.png` header image is untouched); the
  `assets/` line in the repo map and `docs/AGENTS.md`'s `internal/home` /
  `Dockerfile` bullets no longer mention the logo; `docs/NAMING.md`'s
  `assets/manigot.txt` bullet was removed. `docs/AGENTS.md` is the canonical
  source — the root `AGENTS.md` overlay regenerates on the next session
  launch.

## Known issues / follow-ups

- `assets/manigot.txt` remains in the repo (per the user's request to keep
  the logo itself) but is now referenced by no code or docs.
- The root `AGENTS.md` context-mount copy still carries the old logo wording
  until it is regenerated from `docs/AGENTS.md` (read-only overlay — not
  editable from inside a session).
- `internal/git/commitall_test.go` and `internal/ui/tig_test.go` were already
  not gofmt-clean before this change; left untouched as out of scope.