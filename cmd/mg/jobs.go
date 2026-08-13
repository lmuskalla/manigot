package main

import (
	"bufio"
	"fmt"
	"io"

	"github.com/lmuskalla/manigot/internal/cli"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/ui"
)

// runJobs implements `mg jobs` — list every open job with its state
// (mirroring the TUI's list row: ID, status, type, date, title plus a
// plain-text mg-jdi activity badge), then let the user pick one on a TTY and
// re-exec `mg --job <id> <passthrough>` so the session launcher mounts the
// chosen job's worktree and prompts with its brief.md. Done jobs
// (docs/jobs/archive/) are excluded by job.Discover, same as the TUI.
//
// On a TTY the selection is an interactive picker (ui.Picker) instead of the
// old numbered prompt — pick runs the injected seam so tests never start a
// real Bubble Tea program. The picker is a TTY-only enhancement: the plain
// listing and the "needs an interactive terminal" refusal are byte-identical
// to before off a TTY, and cancelling the picker (esc/q) exits 0 quietly.
//
// Orphaned worktrees — leftover directories under .manigot-worktrees/ whose
// git registration is gone (see job.DiscoverOrphans) — are surfaced after the
// job list, and on a TTY the user is offered to remove them with mg delete's
// confirmation discipline. They have no branch and no worktree registration,
// so they are not jobs and never appear in the picker; the removal
// offer is the tool's only path to clean them up.
//
// passthrough holds the args after the subcommand (e.g. --agent/--profile),
// which the session launch re-execs the binary with — exactly like runAgents.
func runJobs(passthrough []string, r io.Reader, stdout, stderr io.Writer, tty bool, pick pickerRunFunc) int {
	root, err := job.FindProjectRoot()
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	if root == "" {
		fmt.Fprintln(stderr, "Error: could not find project root (no docs/ directory found).")
		return 1
	}

	jobs, err := job.Discover(root)
	if err != nil {
		cliError(stderr, err)
		return 1
	}
	orphans, oerr := job.DiscoverOrphans(root)
	if oerr != nil {
		cliError(stderr, oerr)
		return 1
	}

	if len(jobs) == 0 {
		fmt.Fprintln(stdout, `No jobs yet — run 'mg job "<title>"' to create one.`)
	} else {
		fmt.Fprintln(stdout, "Jobs:")
		fmt.Fprintln(stdout, "")
		for i, j := range jobs {
			row := fmt.Sprintf("  %2d) %-8s %-6s %-8s %-12s %s", i+1, j.ID, j.Status, j.Type, j.Date, j.Title)
			if badge := jobsBadge(root, j); badge != "" {
				row += "  " + badge
			}
			fmt.Fprintln(stdout, row)
		}
		fmt.Fprintln(stdout, "")
	}

	// ── Orphaned-worktree surfacing + removal offer ─────────────────────────
	// The bufio.Reader exists only for the removal confirm: the picker (on a
	// TTY) reads stdin itself, so no shared reader should sit on stdin before
	// it — the only prompt in this command is the orphan one.
	if len(orphans) > 0 {
		fmt.Fprintln(stdout, "Orphaned worktrees (no branch or worktree registration):")
		for i, o := range orphans {
			fmt.Fprintf(stdout, "  %2d) %s\n", i+1, o.Name)
		}
		fmt.Fprintln(stdout, "")
		if !tty {
			fmt.Fprintln(stdout, "  Remove them with: mg delete <name>")
		} else {
			confirm := func(prompt string) (bool, error) {
				return cli.Confirm(prompt, bufio.NewReader(r), stdout)
			}
			fmt.Fprintln(stdout, "This cannot be undone.")
			if err := askOrphanRemoval(root, orphans, confirm, stdout); err != nil {
				cliError(stderr, err)
				return 1
			}
			fmt.Fprintln(stdout, "")
		}
	}

	if len(jobs) == 0 {
		return 0
	}
	if !tty {
		fmt.Fprintln(stderr, "Error: mg jobs needs an interactive terminal to select a job.")
		return 1
	}

	rows := make([]ui.PickerRow, 0, len(jobs))
	for _, j := range jobs {
		label := fmt.Sprintf("%-8s %-6s %-8s %-12s %s", j.ID, j.Status, j.Type, j.Date, j.Title)
		if badge := jobsBadge(root, j); badge != "" {
			label += "  " + badge
		}
		rows = append(rows, ui.PickerRow{
			ID:        j.ID,
			SearchKey: j.ID + " " + j.Title,
			Label:     label,
		})
	}

	id, ok, err := pick("Select a job", rows)
	if err != nil {
		fmt.Fprintf(stderr, "mg jobs: %v\n", err)
		return 1
	}
	if !ok {
		// Cancelled (esc/q) — a quiet exit 0, not the old "quit" error.
		return 0
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "→ Starting a session in %s...\n", id)
	fmt.Fprintln(stdout, "")

	// Re-exec the same binary's session path — the launch is in-process, so
	// this just re-runs `mg --job <id> <passthrough>`, mounting the job's
	// worktree via the session launcher.
	launchArgs := append([]string{"--job", id}, passthrough...)
	return reexec(launchArgs, stderr)
}

// askOrphanRemoval confirms and performs the removal of the given orphans —
// the "offer to remove" half of the orphan surfacing. A declined confirmation
// is not an error (the script-style default-no), so the caller continues to
// job selection.
func askOrphanRemoval(root string, orphans []job.Orphan, confirm job.ConfirmFunc, stdout io.Writer) error {
	ok, err := confirm("  Remove orphaned worktrees? [y/N] ")
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(stdout, "  Skipped.")
		return nil
	}
	return job.RemoveOrphansConfirmed(root, orphans, stdout)
}

// jobsBadge renders the plain-text mg-jdi activity badge for a job's list
// row: [running @<agent>] / [finished] / [needs human], mirroring the TUI's
// jdiStatusBadge without its styling or spinner frame. Returns "" when there
// is nothing live to report — job.ReadJDIStatus itself gates that (missing,
// unparseable, or stale statuses all degrade to ok=false).
func jobsBadge(root string, j job.Job) string {
	st, ok := job.ReadJDIStatus(root, j.Name)
	if !ok {
		return ""
	}
	switch st.State {
	case job.JDIRunning:
		label := "running"
		if st.Agent != "" {
			label += " @" + st.Agent
		}
		return "[" + label + "]"
	case job.JDIStoppedFinished:
		return "[finished]"
	case job.JDIStoppedNeedsHuman:
		return "[needs human]"
	default:
		return ""
	}
}
