// mg jdi ("just do it") drives a job's fixed agent sequence —
// @analyst → @developer → @reviewer — end to end without a human manually
// triggering each stage, per the "fully autonomous mode" brief
// (docs/jobs/vu33rn_fully-autonomous-mode/). It is the fold of the former
// cmd/jdi main into the single mg binary (runJDI).
//
// It resolves the job by ID or directory name, then repeatedly asks
// internal/orchestrate what to do next given the job's current
// job.Stage() and git-derived retry-round state, invokes that agent
// non-interactively (internal/session's --print path, see scripts/entrypoint.sh),
// and loops — until the job's verdict is APPROVED or it needs a human (the
// retry budget is exhausted, an agent asked a question via the
// NEEDS-HUMAN-INPUT marker, or the same agent made no progress twice in a
// row).
//
// It does not auto-merge the finished branch — see the brief's completion
// criteria.
//
// Run from anywhere inside a project that has a docs/ directory:
//
//	mg jdi --job <id-or-name>
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/notify"
	"github.com/lmuskalla/manigot/internal/orchestrate"
	"github.com/lmuskalla/manigot/internal/session"
)

// jdiVersion is the mg-jdi version. Overridden at build time with:
//
//	go build -ldflags "-X main.jdiVersion=1.2.3"
var jdiVersion = "0.1.0-dev"

// runJDI implements `mg jdi` (thematic alias `mg made-man`) — the fold of the
// former cmd/jdi main into the single mg binary. It parses the flags, drives
// the job's @analyst → @developer → @reviewer sequence end to end, and returns
// the process exit code: 2 for flag/argument errors, 1 when the run stops
// needing human input, 0 otherwise.
func runJDI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mg jdi", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jobArg := fs.String("job", "", "job ID or directory name to drive (required)")
	profileArg := fs.String("profile", "", "subscription profile to run agents under: claude-pro, zai, or opencode-go (default claude-pro)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "mg jdi %s\n\n", jdiVersion)
		fmt.Fprintf(stderr, "Drives a job's @analyst -> @developer -> @reviewer sequence end to end.\n\n")
		fmt.Fprintf(stderr, "Usage:\n")
		fmt.Fprintf(stderr, "  mg jdi --job <id-or-name> [--profile <profile>]\n\n")
		fmt.Fprintf(stderr, "Run from anywhere inside a project that has a docs/ directory.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2 // the flag package already printed the error + usage
	}

	if strings.TrimSpace(*jobArg) == "" {
		fmt.Fprintln(stderr, "mg jdi: --job is required")
		fs.Usage()
		return 2
	}

	// Default to claude-pro when unset, so existing callers/behavior are
	// unchanged; validated against config.Profiles() otherwise.
	profile := strings.TrimSpace(*profileArg)
	if profile == "" {
		profile = config.ProfileClaudePro
	}
	if _, ok := config.ProfileByID(profile); !ok {
		fmt.Fprintf(stderr, "mg jdi: --profile must be one of: claude-pro, zai, opencode-go (got %q)\n", profile)
		return 2
	}

	root, err := job.FindProjectRoot()
	if err != nil {
		fmt.Fprintf(stderr, "mg jdi: cannot read working directory: %v\n", err)
		return 1
	}
	if root == "" {
		fmt.Fprintln(stderr, "mg jdi: could not find a docs/ directory in this or any parent directory")
		return 1
	}

	j, err := resolveJob(root, *jobArg)
	if err != nil {
		fmt.Fprintf(stderr, "mg jdi: %v\n", err)
		return 1
	}

	// Crash notification (opt-in — see internal/notify): the previous run for
	// this job may have been killed without ever reporting a stop; detect it
	// here at the next run's start and push an attention notification. A
	// process that was SIGKILLed/OOM-killed cannot notify from inside itself,
	// so this stale-sidecar check on next start is the self-contained
	// approximation — an external watchdog is out of scope. Best-effort and
	// only when NTFY_TOPIC is set (unconfigured behavior unchanged).
	notifyCrashedRun(stderr, root, j)

	// Best-effort: see ensureSidecarIgnored's own doc for why a failure here
	// does not abort the run.
	if err := ensureSidecarIgnored(root); err != nil {
		fmt.Fprintf(stderr, "mg jdi: warning: could not exclude the status sidecar from git: %v\n", err)
	}

	runner, err := newCommandAgentRunner(root, profile)
	if err != nil {
		fmt.Fprintf(stderr, "mg jdi: %v\n", err)
		return 1
	}

	// Fan-out (Decision 7/7a/TASK-7): every invocation's formatted output
	// (see jdioutput.go's logInvocation) goes to both mg-jdi's own stdout —
	// which has an audience only for a direct CLI run; a TUI-launched run is
	// detached with no terminal attached to this leg at all — and the
	// sidecar's run.log, which the TUI log tab reads and is therefore the
	// only visibility path for a TUI-launched run. Writing to both
	// unconditionally is simplest and harmless: stdout always exists as a
	// file descriptor whether or not anything is reading it.
	logDest := io.Writer(stdout)
	wroteSection := false
	if logFile, hadContent, ferr := openRunLog(root, j.Name); ferr != nil {
		fmt.Fprintf(stderr, "mg jdi: warning: could not open run log, continuing with stdout only: %v\n", ferr)
	} else {
		defer logFile.Close()
		logDest = io.MultiWriter(stdout, logFile)
		wroteSection = hadContent
	}
	// Wrap the destination in a sectionWriter (see jdioutput.go) so every
	// "=== ... ===" state-log header is preceded by a blank line instead of
	// the events getting crumbled together — the very first run ever written
	// to a fresh log excepted, which is what wroteSection captures.
	logDest = &sectionWriter{w: logDest, wroteSection: wroteSection}

	// Status sidecar (Decision 4/4a/TASK-8): best-effort — a write failure
	// (e.g. a read-only filesystem) must not abort the loop itself, only
	// mean the TUI's list-row badge and stop-notification dedup won't see it.
	// mg-jdi's own stdout/run.log remain authoritative for a human watching
	// directly either way.
	statusFn := func(state job.JDIState, agent string) {
		if err := job.WriteJDIStatus(root, j.Name, state, agent); err != nil {
			fmt.Fprintf(stderr, "mg jdi: warning: could not write status sidecar: %v\n", err)
		}
	}

	logStarted(logDest, j.Name, profile)
	result := Run(root, j, runner, logDest, statusFn)
	fmt.Fprintf(stdout, "\nmg jdi: %s — %s\n", result.Kind, result.Reason)

	// Stop notification, CLI path (Decision 5/7a): ring the terminal bell
	// at both loop-exit points (finished, needs human) — this process is
	// attached to the human's own terminal for a direct `mg jdi` run, which
	// is the only case this leg has an audience for. A TUI-launched run has
	// no terminal to ring into at all; the TUI rings on its own next poll
	// tick instead (see pollJDIBell in internal/ui/app.go), once it observes
	// this job's status sidecar (just written above) transition into a
	// stopped state.
	fmt.Fprint(stdout, "\a")

	// Stop notification, ntfy path (opt-in — see internal/notify): both the
	// CLI path and a TUI-launched run go through this same runJDI stop path,
	// so notifying here covers both. Best-effort like the sidecar/run-log
	// warnings above — a send failure warns on stderr and never aborts the
	// exit path.
	notifyStop(stderr, j.Name, result)

	if result.Kind == orchestrate.StopNeedsHuman {
		return 1
	}
	return 0
}

// notifyStop sends the opt-in ntfy stop notification for a finished mg-jdi
// run: a default-priority success message (tag white_check_mark) when the
// run finished (result.Kind == StopFinished), and a high-priority attention
// message (tag warning, priority 4) when it stopped needing a human. Both
// messages carry the job name and the stop reason. It is a strict no-op when
// NTFY_TOPIC is unset (nothing is sent, no error), and a send failure only
// warns on stderr — never aborting the caller.
func notifyStop(stderr io.Writer, jobName string, result LoopResult) {
	c := notify.FromConfig()
	if !c.Enabled() {
		return
	}

	msg := notify.Message{Message: result.Reason}
	switch result.Kind {
	case orchestrate.StopFinished:
		msg.Title = "mg jdi: " + jobName + " finished"
		msg.Tags = []string{"white_check_mark"}
	default: // orchestrate.StopNeedsHuman
		msg.Title = "mg jdi: " + jobName + " needs attention"
		msg.Tags = []string{"warning"}
		msg.Priority = 4
	}
	if err := c.Publish(msg); err != nil {
		fmt.Fprintf(stderr, "mg jdi: warning: could not send ntfy notification: %v\n", err)
	}
}

// notifyCrashedRun checks, at a new `mg jdi` run's start, whether the
// previous run for job j died without ever reporting a stop — a JDIRunning
// status in the sidecar stale past job's jdiRunningStaleAfter (see
// job.StaleRunningJDI) — and pushes a high-priority attention notification
// (tag warning, priority 4) about it. Once the crash has been notified, the
// stale sidecar is overwritten with JDIStoppedNeedsHuman so a later start
// does not re-notify about the same already-known crash; a failed send does
// not mark it, so the notification is retried on the next start instead.
//
// It is a strict no-op when ntfy is unconfigured (no NTFY_TOPIC): nothing is
// read-modify-written, keeping unconfigured behavior byte-for-byte identical
// (the TUI still degrades the stale sidecar away on its own). Failures warn
// on stderr and never abort the run.
func notifyCrashedRun(stderr io.Writer, root string, j job.Job) {
	c := notify.FromConfig()
	if !c.Enabled() {
		return
	}

	st, crashed := job.StaleRunningJDI(root, j.Name)
	if !crashed {
		return
	}

	msg := notify.Message{
		Title:    "mg jdi: " + j.Name + " crashed",
		Message:  fmt.Sprintf("The previous mg jdi run appears to have crashed or been killed (status stale since %s).", st.Updated.Format(time.RFC3339)),
		Tags:     []string{"warning"},
		Priority: 4,
	}
	if st.Agent != "" {
		msg.Message += fmt.Sprintf(" Last agent: %s.", st.Agent)
	}
	if err := c.Publish(msg); err != nil {
		fmt.Fprintf(stderr, "mg jdi: warning: could not send ntfy notification: %v\n", err)
		return
	}

	// Mark the crash as known so the next start doesn't re-notify about it.
	if err := job.WriteJDIStatus(root, j.Name, job.JDIStoppedNeedsHuman, st.Agent); err != nil {
		fmt.Fprintf(stderr, "mg jdi: warning: could not mark the crashed run as known: %v\n", err)
	}
}

// resolveJob finds the job named or ID-prefixed by arg among every job
// job.Discover can see (cross-branch — see tui/internal/job), mirroring
// scripts/run.sh's own --job resolution: an exact job-directory-name match
// first, then an exact ID match, then a directory-name prefix match (the
// same fallback scripts/run.sh's `find ... -name "${JOB}*"` uses).
func resolveJob(root, arg string) (job.Job, error) {
	jobs, err := job.Discover(root)
	if err != nil {
		return job.Job{}, fmt.Errorf("could not list jobs: %w", err)
	}

	for _, j := range jobs {
		if j.Name == arg {
			return j, nil
		}
	}
	for _, j := range jobs {
		if j.ID == arg {
			return j, nil
		}
	}

	var prefixMatches []job.Job
	for _, j := range jobs {
		if strings.HasPrefix(j.Name, arg) {
			prefixMatches = append(prefixMatches, j)
		}
	}
	switch len(prefixMatches) {
	case 0:
		return job.Job{}, fmt.Errorf("job %q not found under docs/jobs/", arg)
	case 1:
		return prefixMatches[0], nil
	default:
		return job.Job{}, fmt.Errorf("job %q is ambiguous — matches %d jobs", arg, len(prefixMatches))
	}
}

// AgentRunner runs one non-interactive agent invocation for job j and
// returns its captured output (the session launcher's --print path — a
// `claude --output-format json` payload when available, otherwise plain
// text; see tui/internal/orchestrate.DetectSignal, which handles both).
//
// Implemented by commandAgentRunner for real invocations; the tests use a
// fake instead of spawning real containers.
type AgentRunner interface {
	Run(agent string, j job.Job) ([]byte, error)
}

// commandAgentRunner is the real AgentRunner: it calls the session package's
// --print path in-process (the replacement for exec'ing the resolved run.sh),
// capturing the agent's stdout and routing diagnostics to a buffer that only
// surfaces on error — the same visibility the exec'd run.sh's fd-3
// diagnostics had.
type commandAgentRunner struct {
	projectRoot string
	profile     string
}

// newCommandAgentRunner builds the in-process runner. projectRoot is where
// --job resolution starts from (run.sh used $PWD, which the old runner set to
// the project root); profile is passed through unchanged to every Run
// invocation — see its own doc.
func newCommandAgentRunner(projectRoot, profile string) (*commandAgentRunner, error) {
	return &commandAgentRunner{projectRoot: projectRoot, profile: profile}, nil
}

// agentInvocation builds the session options for one agent run — the --print
// invocation with the runner's pinned profile, the agent, and the exact job
// directory name.
func (r *commandAgentRunner) agentInvocation(agent string, j job.Job) session.Options {
	return session.Options{Print: true, Profile: r.profile, Agent: agent, Job: j.Name}
}

// Run invokes the session launcher's --print path synchronously and returns
// its stdout. The --print path keeps stdout clean of its own diagnostics
// (they go to the diag writer), so stdout here is exactly the agent's own
// response. j.Name (the exact job directory name) is passed rather than j.ID
// to remove any ambiguity in the --job resolution.
//
// r.profile is passed explicitly rather than left to the launcher's own
// default (main's --profile flag, validated and defaulted to
// config.ProfileClaudePro there) so a run keeps using the profile it was
// started with even when the user later changes their default profile via
// `mg profiles` mid-run.
func (r *commandAgentRunner) Run(agent string, j job.Job) ([]byte, error) {
	opts := r.agentInvocation(agent, j)
	info, err := session.ResolveProfile(opts)
	if err != nil {
		return nil, err
	}
	root, err := session.ResolveRootFrom(opts, r.projectRoot)
	if err != nil {
		return nil, err
	}
	if err := info.CheckAuth(); err != nil {
		return nil, err
	}

	var diag bytes.Buffer
	inv, err := session.BuildDockerInvocation(opts, info, root, false, &diag)
	if err != nil {
		return nil, err
	}

	var stdout bytes.Buffer
	if code := inv.Run(os.Stdin, &stdout, &diag); code != 0 {
		return stdout.Bytes(), fmt.Errorf("mg --print --agent %s --job %s: exit code %d: %s", agent, j.Name, code, strings.TrimSpace(diag.String()))
	}
	return stdout.Bytes(), nil
}

// LoopResult is Run's outcome: always one of orchestrate.StopFinished or
// orchestrate.StopNeedsHuman — Run never returns while orchestrate.Next
// keeps saying RunAgent.
type LoopResult struct {
	Kind   orchestrate.Kind
	Reason string
}

// maxIterations is a hard safety cap on loop iterations, independent of
// orchestrate.Next's own one-bounce retry budget — a defensive backstop
// against an unforeseen bug in the state machine looping forever rather than
// trusting Next's own termination alone.
const maxIterations = 20

// StatusFunc is called by Run at each state transition (Decision 4a): once
// before invoking an agent (job.JDIRunning, that agent's name) and once
// more at loop exit (the terminal state, and the last agent that ran, or ""
// if none ever did — e.g. a job stuck at StageDefine). May be nil, in which
// case Run simply doesn't report status — used by tests that don't care
// about the sidecar file.
type StatusFunc func(state job.JDIState, agent string)

// jdiTerminalState maps a LoopResult's orchestrate.Kind (always
// StopFinished or StopNeedsHuman by the time Run returns) to the
// corresponding job.JDIState reported to StatusFunc.
func jdiTerminalState(k orchestrate.Kind) job.JDIState {
	if k == orchestrate.StopFinished {
		return job.JDIStoppedFinished
	}
	return job.JDIStoppedNeedsHuman
}

// Run drives job j through the agent sequence via runner, writing each
// invocation's formatted output to log (Decision 7/TASK-7 — see
// output.go's logInvocation) and reporting state transitions to status
// (Decision 4/4a/TASK-8; may be nil), until orchestrate.Next reports
// StopFinished or StopNeedsHuman.
//
// The attempt number in each invocation's log header (logAgentInvoked /
// logInvocation) counts that agent's own invocations within this run — per
// agent, not a run-wide counter — so a bounce back to the developer logs
// that developer's second call as attempt 2, not attempt 4. The counter is
// seeded at 0 before the loop and reset at the start of every run, matching
// the loop index's old behavior.
//
// Each iteration re-derives its decision from job.Stage() and git — the
// job.Job struct's identity fields (Dir/Root/Branch/ID/Name) never change
// across iterations, but Stage() and the git.CountVerdictCommits/
// LatestCommitIsVerdict reads underneath it are re-evaluated fresh every
// time, since they read from disk/git rather than a cached snapshot — so
// Run never needs to re-fetch or mutate j itself mid-loop.
func Run(root string, j job.Job, runner AgentRunner, log io.Writer, status StatusFunc) LoopResult {
	var lastAgent string
	var lastNoChange bool
	var agentEverRan bool

	report := func(state job.JDIState, agent string) {
		if status != nil {
			status(state, agent)
		}
	}
	// finish reports the terminal status, logs the "job finished" event
	// (TASK-4), and builds the LoopResult every exit point returns.
	// includeReason is normally true; it's false only for the
	// stop-before-any-agent-ran case below, whose reason was already printed
	// by logImmediateStop a line above — see logJobFinished's own doc.
	finish := func(kind orchestrate.Kind, reason string, includeReason bool) LoopResult {
		report(jdiTerminalState(kind), lastAgent)
		logJobFinished(log, kind, reason, includeReason)
		return LoopResult{Kind: kind, Reason: reason}
	}

	// attempts counts each agent's invocations within this run, keyed by
	// agent name — per agent, not run-wide — and feeds the "attempt N" in
	// both logAgentInvoked's and logInvocation's headers below. Seeded at 0
	// before the loop, so the first call of every agent is attempt 1.
	attempts := make(map[string]int)

	for i := 0; i < maxIterations; i++ {
		stageBefore := j.Stage()
		rounds, _ := git.CountVerdictCommits(root, j.Branch, j.ID)
		tipIsVerdict, _ := git.LatestCommitIsVerdict(root, j.Branch, j.ID)
		decision := orchestrate.Next(stageBefore, rounds, tipIsVerdict)

		if decision.Kind != orchestrate.RunAgent {
			includeReason := true
			if !agentEverRan {
				// No agent has run yet this whole loop (typically the very
				// first iteration, e.g. an unwritten brief.md) — logInvocation
				// never gets a chance to run for this stop, which otherwise
				// leaves run.log at 0 bytes for a case that has just as much
				// reason to explain itself there as any agent invocation does.
				logImmediateStop(log, decision.Reason)
				includeReason = false
			}
			return finish(decision.Kind, decision.Reason, includeReason)
		}

		report(job.JDIRunning, decision.Agent)
		attempts[decision.Agent]++
		logAgentInvoked(log, decision.Agent, attempts[decision.Agent])
		headBefore, _ := git.HeadCommit(root, j.Branch)

		out, runErr := runner.Run(decision.Agent, j)
		agentEverRan = true

		// TASK-5: read the just-run agent's expected target file fresh off
		// disk, so logInvocation can skip re-printing output that's already
		// the same as what got written to the job's own markdown file. Safe
		// to read directly from j.Dir — see agentTargetFile's own doc. A
		// missing/unreadable file (targetContent left "") just means
		// logInvocation's dedup check never matches, which is the same as
		// not having this feature at all for that call.
		var targetContent string
		targetFile := agentTargetFile[decision.Agent]
		if targetFile != "" {
			if data, rerr := os.ReadFile(filepath.Join(j.Dir, targetFile)); rerr == nil {
				targetContent = string(data)
			}
		}

		// Fan-out (Decision 7/TASK-7): log gets the same formatted section
		// regardless of whether it's os.Stdout, the sidecar run.log, or (in
		// tests) an in-memory buffer — see main()'s io.MultiWriter and
		// output.go's logInvocation. DetectSignal below scans the raw bytes
		// directly (it does its own JSON extraction), independent of what
		// logInvocation writes for a human to read.
		logInvocation(log, decision.Agent, attempts[decision.Agent], out, targetFile, targetContent)

		if sig, ok := orchestrate.DetectSignal(out); ok {
			lastAgent = decision.Agent
			return finish(orchestrate.StopNeedsHuman, fmt.Sprintf("%s asked for human input: %s", decision.Agent, sig.Reason), true)
		}
		if runErr != nil {
			lastAgent = decision.Agent
			return finish(orchestrate.StopNeedsHuman, fmt.Sprintf("%s invocation failed: %v", decision.Agent, runErr), true)
		}

		// Stall backstop (Decision 1a): the same agent invoked twice in a
		// row, each time persisting nothing at all (neither Stage() nor the
		// branch HEAD moved across that invocation), stops on the second
		// such occurrence — a single no-op invocation is tolerated (it may
		// be transient), but two in a row for the same agent means it's
		// genuinely stuck, regardless of whether the NEEDS-HUMAN-INPUT
		// marker was present.
		stageAfter := j.Stage()
		headAfter, _ := git.HeadCommit(root, j.Branch)
		noChange := stageAfter == stageBefore && headAfter == headBefore

		if noChange && lastNoChange && decision.Agent == lastAgent {
			return finish(orchestrate.StopNeedsHuman, fmt.Sprintf("%s made no progress on two consecutive runs (Stage and branch HEAD unchanged both times) — stopping rather than looping indefinitely", decision.Agent), true)
		}
		lastAgent, lastNoChange = decision.Agent, noChange
	}

	return finish(orchestrate.StopNeedsHuman, fmt.Sprintf("exceeded %d iterations without finishing", maxIterations), true)
}
