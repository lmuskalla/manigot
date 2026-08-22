# Tasks: ui indicators need to leave again

id: across
status: open
analyst: analyst
date: 2026-08-22

<!-- Produced by @analyst from brief.md. -->

## Summary

The TUI shows a one-line "feedback" status in two places that today persist
forever until the user's very next keypress clears them (or forever if the
user is idle):

- the **list footer** (`App.status`, rendered by `listFooter`/`App.footer` and
  `listView.render`) — e.g. "Refreshed", "refreshed · N job(s)", "settings
  saved", "created ...", agent/jdi launch confirmations, and error text.
- the **detail footer** (`detailView.status`, set via `setStatus`, rendered by
  `renderFooter`) — e.g. "refreshed", "edited brief.md", "→ pushed ... to
  origin", "→ committed all changes", "→ merged ...", "→ <agent> in <desc>",
  and error text.

The brief wants these to "flash 3 times or so and then remove it" instead of
staying forever.

### Chosen behaviour (decisions to implement)

1. **Blink then expire.** When a transient status is set, it is shown solid for
   a short hold, then toggles visible/hidden a few times (the "flash"), then is
   cleared entirely — all without any keypress. Constants: `statusLifetime`
   (~3s total before removal), `statusBlinkInterval` (~200ms, the tick cadence
   and one blink toggles on/off), and the blink window begins ~0.6s before
   expiry (≈3 toggles). Fine-tune to taste; the mechanism, not the exact beat,
   is the deliverable.
2. **Applies to all action feedback uniformly** — success *and* error text
   (`cmdErrorText`), on both the list footer and the detail footer. This
   matches the brief's literal ask ("whenever I execute an action that gives me
   feedback") and mirrors the existing clear-on-next-keypress behaviour; the
   lifetime is long enough that error lines stay readable. Flag for review if
   errors should persist instead.
3. **Second timer-driven message.** The app already has one timer-driven
   message (`spinnerTickMsg`, the JDI activity indicator — explicitly
   documented as "the one narrow exception" to no timer-driven redraw). This
   feature adds a second, `statusExpireMsg`, gated behind a guard so it never
   ticks when idle and self-terminates once no status remains. Update the
   comments/README that say "one timer exception".
4. **Out of scope:** the settings-form and new-job-form footers
   (`settingsView.status`, `newJobView.status`). Those are persistent
   validation/save errors tied to a form that stays open for the user to fix —
   not the transient action feedback the brief targets. Do not blink/expire
   them.
5. **Layout stability during blink.** The blink toggles only *rendered*
   visibility; the underlying status text stays set during the blink so
   `footerLines()`/`syncViewerSize()`/`bodyHeight()` layout budgets don't
   jitter. Only at final expiry is the text actually cleared (and the footer
   shrinks back, as it does today when status clears).

## Task breakdown

TASK-1: Add the `statusExpireMsg` timer-driven state machine to `App` — a
second timer chain (after `spinnerTickMsg`) with a guard, per-surface expiry
deadlines and blink toggles, a testable clock seam, and a self-terminating
`Update` handler.
     files: `internal/ui/app.go` (fields, `statusExpireMsg`/`statusExpireCmd`/
     `armStatusExpiry`/handler, `Init()`), `internal/ui/activity.go` (or a new
     constants location) for `statusLifetime`/`statusBlinkInterval`/blink window
     depends: none
     risk: medium — the app's second timer-driven redraw; must never tick when
     idle and must self-terminate once no status remains (mirror the existing
     spinner guard pattern, including the no-duplicate-chains property).

TASK-2: Make the list footer blink-aware and route every list `a.status = ...`
assignment through the expiry chain (arm on non-empty set).
     files: `internal/ui/app.go` (list `a.status =` sites: lines ~181/199
     NewApp, 341/343 doneMsg, 354/356 deleteMsg, 701 ctrl+r, 726/730/738 j,
     758/760 o, 773/775 a, 823 settings, 856 newjob, 881/883 agentspicker;
     the `a.status = ""` clear at line 689 stays a plain clear), `internal/ui/
     list.go` (`listFooter`), `internal/ui/app.go` (`App.footer`)
     depends: TASK-1
     risk: medium — ~15 call sites, several already return a spinner/other cmd
     and need `tea.Batch`-combining; must not clear status on navigation and
     must keep immediate `a.status` values unchanged so existing tests
     (`== "refreshed"`) stay green.

TASK-3: Make the detail footer blink-aware and route every
`a.detail.setStatus(...)` call through the expiry chain.
     files: `internal/ui/detail.go` (`detailView` fields `statusUntil`/
     `statusBlinkOn`, `setStatus`, `renderFooter`), `internal/ui/app.go`
     (detail `setStatus` call sites, all in App.Update/updateDetail — see
     lines ~302/305/321/323/365/367/375/386/388/401/403/938/947/978/982/992/
     1018/1031/1035/1044/1046/1056/1070/1083/1085/1089)
     depends: TASK-1
     risk: medium — ~23 call sites; `footerLines()`/`syncViewerSize()` must
     keep keying off `d.status` (not the blink toggle) so layout stays stable
     during blink; keep the multi-line `cmdErrorText` case and
     `TestDetailBodyHeightShrinksForMultiLineStatus` green.

TASK-4: Tests for the blink-and-expire state machine.
     files: new `internal/ui/status_test.go` (and any needed tweaks to
     existing footer tests)
     depends: TASK-2, TASK-3
     risk: low — time must be controlled via a clock seam (a package var
     like the existing `jdiNow`) rather than sleeps; assert: setting a status
     arms a tick cmd, feeding `statusExpireMsg` toggles blink visibility, after
     the deadline the status clears and the chain returns nil + guard cleared,
     a re-set status refreshes the deadline, no tick when idle/no status, and
     that both list and detail surfaces blink then clear.

TASK-5: Update the "one narrow timer exception" documentation to reflect the
second timer-driven message.
     files: `internal/ui/app.go` (spinnerTickMsg doc comment ~lines 135-141 and
     pollJDIBell's ~546-550 comment), `README.md` (~line 887-889), and any
     matching statement in `docs/AGENTS.md` / `project-template/docs/AGENTS.md`
     / `agents/*.md` (keep them in sync per the hard rules)
     depends: TASK-1
     risk: low — doc-only, but must stay consistent with the "no new
     event-streaming subsystem" framing: this is still a narrow, guarded,
     self-terminating redraw, now two of them.
