package main

import (
	"bufio"
	"fmt"
	"io"

	"github.com/lmuskalla/manigot/internal/agents"
	"github.com/lmuskalla/manigot/internal/cli"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/session"
	"github.com/lmuskalla/manigot/internal/ui"
)

// runJobs implements `mg jobs` — list every open job with its state
// (mirroring the TUI's list row: ID, status, type, date, title plus a
// plain-text mg-jdi activity badge), then let the user pick one on a TTY and
// re-exec `mg --job <id> <passthrough>` so the session launcher mounts the
// chosen job's worktree and prompts with its brief.md. The picked job's
// workflow stage selects the agent the session launches in (plan → analyst,
// implement → developer, review → reviewer — see stageAgent). Done jobs
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

	id, ok, err := pick("Select a job", rows, 0)
	if err != nil {
		fmt.Fprintf(stderr, "mg jobs: %v\n", err)
		return 1
	}
	if !ok {
		// Cancelled (esc/q) — a quiet exit 0, not the old "quit" error.
		return 0
	}

	// Auto-select the stage-appropriate agent for the picked job: look it up
	// among the discovered jobs (by ID — the picker row's ID) and derive the
	// agent from its workflow stage. Stages without a fitting agent (define —
	// brief not written yet; finished — verdict APPROVED) stay agent-less,
	// keeping the current launch behavior, and print a short heads-up instead
	// (see stageGuidance).
	var stage job.Stage
	for i := range jobs {
		if jobs[i].ID == id {
			stage = jobs[i].Stage()
			break
		}
	}
	// The launch line names the agent that will actually run: an explicit
	// --agent/-a in passthrough wins over the stage-derived default (TASK-3 —
	// the derived agent only fills the default; jobsLaunchArgs applies the
	// same precedence when building the re-exec args).
	agent := session.ParseArgs(passthrough).Agent
	if agent == "" {
		agent = stageAgent(stage)
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, jobsLaunchLine(id, agent))
	if guidance := stageGuidance(stage); guidance != "" {
		fmt.Fprintf(stdout, "  %s\n", guidance)
	}
	fmt.Fprintln(stdout, "")

	// Re-exec the same binary's session path — the launch is in-process, so
	// this just re-runs `mg --job <id> --agent <name> <passthrough>` (when
	// the stage has a fitting agent), mounting the job's worktree via the
	// session launcher.
	return reexec(jobsLaunchArgs(id, stage, passthrough), stderr)
}

// stageAgent maps a job's workflow stage to the agent `mg jobs` should launch
// the picked job in: plan → analyst, implement → developer, review →
// reviewer. Stages with no fitting agent (define — the brief isn't written
// yet; finished — the verdict is APPROVED) return "", meaning launch
// agent-less exactly as before. The mapping lives here as a launch default,
// not in the job package, per internal/job/stage.go's "do not reintroduce an
// Agents() method as a gate" comment — nothing is gated by it.
func stageAgent(stage job.Stage) string {
	switch stage {
	case job.StagePlan:
		return agents.Analyst
	case job.StageImplement:
		return agents.Developer
	case job.StageReview:
		return agents.Reviewer
	default:
		return ""
	}
}

// stageGuidance returns a short heads-up line for the edge stages that have
// no fitting agent to launch in — define (the brief isn't written yet; a
// human task, not any agent's) and finished (the verdict is APPROVED; the job
// is ready for mg done). It accompanies the unchanged agent-less launch for
// those stages; the mapped stages return "" and need no guidance.
func stageGuidance(stage job.Stage) string {
	switch stage {
	case job.StageDefine:
		return "brief.md is not written yet — write it first"
	case job.StageFinished:
		return "verdict is APPROVED — run mg done to merge"
	default:
		return ""
	}
}

// jobsLaunchLine renders the "→ Starting a session ..." line printed before
// the launch: it names the derived agent when there is one ("→ Starting a
// session in @analyst for aaa01..."), and stays agent-less otherwise.
func jobsLaunchLine(id, agent string) string {
	if agent != "" {
		return fmt.Sprintf("→ Starting a session in @%s for %s...", agent, id)
	}
	return fmt.Sprintf("→ Starting a session in %s...", id)
}

// jobsLaunchArgs builds the re-exec args for the picked job: `mg --job <id>
// --agent <name> <passthrough>` when the stage has a fitting agent and the
// caller hasn't passed an explicit --agent/-a of their own in passthrough.
// The user's explicit choice wins over the derived default — session.ParseArgs
// is last-wins, so appending the derived flag after an explicit one would
// silently override it; skipping the derived flag entirely keeps the user's
// flag intact (and in its original position).
func jobsLaunchArgs(id string, stage job.Stage, passthrough []string) []string {
	args := []string{"--job", id}
	if agent := stageAgent(stage); agent != "" && !hasExplicitAgent(passthrough) {
		args = append(args, "--agent", agent)
	}
	return append(args, passthrough...)
}

// hasExplicitAgent reports whether args already carry an explicit --agent/-a
// selection — a token match on the flag names, the same names
// session.ParseArgs's sessionValueFlags recognises. A value-less trailing
// --agent counts too: the caller clearly meant to choose one, and skipping
// the derived flag is the safe direction.
func hasExplicitAgent(args []string) bool {
	for _, a := range args {
		if a == "--agent" || a == "-a" {
			return true
		}
	}
	return false
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
