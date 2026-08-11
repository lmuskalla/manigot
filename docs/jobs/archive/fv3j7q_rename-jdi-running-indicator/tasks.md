# Tasks: Rename jdi running indicator

id: fv3j7q
status: open
analyst: claude
date: 2026-08-11

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

TASK-1: Drop the `mg jdi: ` prefix from the `jdiStatusBadge` helper's three
label variants (`"mg jdi: running"` → `"running"`, `"[mg jdi: finished]"` →
`"[finished]"`, `"[mg jdi: needs human]"` → `"[needs human]"`), and update
the function's doc comment (which quotes the old `"[mg jdi: ...]"` format)
to match. This one shared helper backs both the job-list row badge and the
detail-view action-bar badge, so no separate change is needed for each.
     files: tui/internal/ui/app.go
     depends: none
     risk: low — pure string-literal change to one small, well-isolated
     formatting function; behavior (when the badge shows/hides) is untouched.

TASK-2: Update the existing badge-text assertions in `list_test.go` and
`detail_test.go` to match the new, prefix-free wording (e.g.
`"mg jdi: running @developer"` → `"running @developer"`,
`"mg jdi: finished"` → `"finished"`, `"mg jdi: needs human"` →
`"needs human"`). The two "no badge when there's no sidecar status" tests
(`TestRenderListOmitsJDIBadgeWhenNoStatus`,
`TestDetailActionBarOmitsJDIBadgeWhenNoStatus`) currently assert the absence
of the substring `"mg jdi:"`; since that substring won't appear in the
badge text at all anymore, this assertion needs to be rewritten against a
substring that would only appear as part of the badge (e.g. `"running"` /
`"finished"` / `"needs human"`, or the badge's own bracket + accent-style
wrapping) so the tests still meaningfully guard against a badge rendering
when it shouldn't.
     files: tui/internal/ui/list_test.go, tui/internal/ui/detail_test.go
     depends: TASK-1
     risk: low — test-only changes, but the two "omits badge" tests need
     careful rewording (not just a find/replace) to stay meaningful once the
     "mg jdi:" substring is gone from the badge entirely.

TASK-3: Update the README's "List-row badge" bullet (under "mg jdi status &
log") describing the badge text (`[mg jdi: running @<agent>]` /
`[mg jdi: finished]` / `[mg jdi: needs human]`) to the new prefix-free
wording.
     files: README.md
     depends: TASK-1
     risk: low — documentation-only change.

## Out of scope (per brief)

- The `mg jdi:`-prefixed *error/status messages* printed by the `mg jdi`
  CLI itself (`tui/cmd/jdi/main.go`'s `fmt.Fprintf(os.Stderr, "mg jdi: ...")`
  lines, and `launch.go`'s `"start mg jdi: %w"` wrapped error) are a
  different thing from the TUI badge the brief describes ("the indicator
  right next to 'just do it'") — they're a conventional CLI-tool error
  prefix on stderr/wrapped-error text, not a UI label. Left untouched.
- Archived job docs under `docs/jobs/archive/**` that mention the old
  `[mg jdi: ...]` badge text are historical records of past jobs and are
  not touched.
