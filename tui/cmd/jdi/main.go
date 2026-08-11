// Command mg-jdi ("just do it") drives a job's fixed agent sequence —
// @analyst → @developer → @reviewer — end to end without a human manually
// triggering each stage, per the "fully autonomous mode" brief
// (docs/jobs/vu33rn_fully-autonomous-mode/).
//
// It resolves the job by ID or directory name, then repeatedly asks
// tui/internal/orchestrate what to do next given the job's current
// job.Stage() and git-derived retry-round state, invokes that agent
// non-interactively (scripts/run.sh's --print flag, see
// tui/internal/resolve and scripts/entrypoint.sh), and loops — until the
// job's verdict is APPROVED or it needs a human (the retry budget is
// exhausted, an agent asked a question via the NEEDS-HUMAN-INPUT marker, or
// the same agent made no progress twice in a row).
//
// It does not auto-merge the finished branch — see the brief's completion
// criteria.
//
// Run from anywhere inside a project that has a docs/ directory:
//
//	mg-jdi --job <id-or-name>
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lmuskalla/manigot/tui/internal/config"
	"github.com/lmuskalla/manigot/tui/internal/git"
	"github.com/lmuskalla/manigot/tui/internal/job"
	"github.com/lmuskalla/manigot/tui/internal/orchestrate"
	"github.com/lmuskalla/manigot/tui/internal/resolve"
)

// version is the mg-jdi version. Overridden at build time with:
//
//	go build -ldflags "-X main.version=1.2.3"
var version = "0.1.0-dev"

func main() {
	jobArg := flag.String("job", "", "job ID or directory name to drive (required)")
	profileArg := flag.String("profile", "", "subscription profile to run agents under: claude-pro, zai, or opencode-go (default claude-pro)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "mg jdi %s\n\n", version)
		fmt.Fprintf(os.Stderr, "Drives a job's @analyst -> @developer -> @reviewer sequence end to end.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  mg jdi --job <id-or-name> [--profile <profile>]\n\n")
		fmt.Fprintf(os.Stderr, "Run from anywhere inside a project that has a docs/ directory.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	resolve.SeedHome()

	if strings.TrimSpace(*jobArg) == "" {
		fmt.Fprintln(os.Stderr, "mg jdi: --job is required")
		flag.Usage()
		os.Exit(2)
	}

	// Default to claude-pro when unset, so existing callers/behavior are
	// unchanged; validated against config.Profiles() otherwise rather than
	// left to scripts/run.sh's own (less specific) error.
	profile := strings.TrimSpace(*profileArg)
	if profile == "" {
		profile = config.ProfileClaudePro
	}
	if _, ok := config.ProfileByID(profile); !ok {
		fmt.Fprintf(os.Stderr, "mg jdi: --profile must be one of: claude-pro, zai, opencode-go (got %q)\n", profile)
		os.Exit(2)
	}

	root, err := job.FindProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mg jdi: cannot read working directory: %v\n", err)
		os.Exit(1)
	}
	if root == "" {
		fmt.Fprintln(os.Stderr, "mg jdi: could not find a docs/ directory in this or any parent directory")
		os.Exit(1)
	}

	j, err := resolveJob(root, *jobArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mg jdi: %v\n", err)
		os.Exit(1)
	}

	// Best-effort: see ensureSidecarIgnored's own doc for why a failure here
	// does not abort the run.
	if err := ensureSidecarIgnored(root); err != nil {
		fmt.Fprintf(os.Stderr, "mg jdi: warning: could not exclude the status sidecar from git: %v\n", err)
	}

	runner, err := newCommandAgentRunner(root, profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mg jdi: %v\n", err)
		os.Exit(1)
	}

	// Fan-out (Decision 7/7a/TASK-7): every invocation's formatted output
	// (see logInvocation) goes to both mg-jdi's own stdout — which has an
	// audience only for a direct CLI run; a TUI-launched run is detached
	// with no terminal attached to this leg at all — and the sidecar's
	// run.log, which TASK-9's TUI log tab reads and is therefore the only
	// visibility path for a TUI-launched run. Writing to both
	// unconditionally is simplest and harmless: os.Stdout always exists as
	// a file descriptor whether or not anything is reading it.
	logDest := io.Writer(os.Stdout)
	wroteSection := false
	if logFile, hadContent, ferr := openRunLog(root, j.Name); ferr != nil {
		fmt.Fprintf(os.Stderr, "mg jdi: warning: could not open run log, continuing with stdout only: %v\n", ferr)
	} else {
		defer logFile.Close()
		logDest = io.MultiWriter(os.Stdout, logFile)
		wroteSection = hadContent
	}
	// Wrap the destination in a sectionWriter (see output.go) so every
	// "=== ... ===" state-log header is preceded by a blank line instead of
	// the events getting crumbled together — the very first run ever written
	// to a fresh log excepted, which is what wroteSection captures.
	logDest = &sectionWriter{w: logDest, wroteSection: wroteSection}

	// Status sidecar (Decision 4/4a/TASK-8): best-effort — a write failure
	// (e.g. a read-only filesystem) must not abort the loop itself, only
	// mean the TUI's list-row badge and stop-notification dedup (TASK-9/11)
	// won't see it. mg-jdi's own stdout/run.log remain authoritative for a
	// human watching directly either way.
	statusFn := func(state job.JDIState, agent string) {
		if err := job.WriteJDIStatus(root, j.Name, state, agent); err != nil {
			fmt.Fprintf(os.Stderr, "mg jdi: warning: could not write status sidecar: %v\n", err)
		}
	}

	logStarted(logDest, j.Name, profile)
	result := Run(root, j, runner, logDest, statusFn)
	fmt.Printf("\nmg jdi: %s — %s\n", result.Kind, result.Reason)

	// Stop notification, CLI path (Decision 5/7a): ring the terminal bell
	// at both loop-exit points (finished, needs human) — this process is
	// attached to the human's own terminal for a direct `mg-jdi` run, which
	// is the only case this leg has an audience for. A TUI-launched run has
	// no terminal to ring into at all; tui/internal/ui/app.go rings on the
	// TUI's own next poll tick instead (see pollJDIBell there), once it
	// observes this job's status sidecar (just written above) transition
	// into a stopped state.
	fmt.Print("\a")

	if result.Kind == orchestrate.StopNeedsHuman {
		os.Exit(1)
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
// returns its captured output (scripts/entrypoint.sh's --print path — a
// `claude --output-format json` payload when available, otherwise plain
// text; see tui/internal/orchestrate.DetectSignal, which handles both).
//
// Implemented by commandAgentRunner for real invocations; TASK-14's tests
// use a fake instead of spawning real containers.
type AgentRunner interface {
	Run(agent string, j job.Job) ([]byte, error)
}

// commandAgentRunner is the real AgentRunner: it shells out to the resolved
// mg launcher (tui/internal/resolve) with TASK-2's --print flag.
type commandAgentRunner struct {
	manigotPath string
	projectRoot string
	profile     string
}

// newCommandAgentRunner resolves the mg launcher once up front, the same way
// tui/internal/hostcmd resolves its own host commands, so a missing install
// fails fast with resolve's own diagnostic rather than partway through a
// loop. profile is passed through unchanged to every Run invocation — see
// its own doc.
func newCommandAgentRunner(projectRoot, profile string) (*commandAgentRunner, error) {
	found, err := resolve.Resolve(resolve.Manigot())
	if err != nil {
		return nil, err
	}
	return &commandAgentRunner{manigotPath: found.Path, projectRoot: projectRoot, profile: profile}, nil
}

// Run invokes `mg --print --profile <profile> --agent <agent> --job <j.Name>`
// synchronously and returns its stdout. scripts/run.sh's --print path keeps
// stdout clean of its own diagnostics (redirected to fd 3/stderr — see
// run.sh), so stdout here is exactly the agent's own response. j.Name (the
// exact job directory name) is passed rather than j.ID to remove any ambiguity
// in run.sh's own --job resolution.
//
// r.profile is passed explicitly rather than left to run.sh's own default
// (main's --profile flag, validated and defaulted to config.ProfileClaudePro
// there) so a run keeps using the profile it was started with even when the
// user later changes their default profile via `mg profiles` mid-run.
//
// cmd.Dir and $PWD are both set to projectRoot, the same pattern
// tui/internal/hostcmd's NewJob/DoneCommand/DeleteCommand use, because
// run.sh resolves the project root from $PWD via its own find_project_root
// helper.
func (r *commandAgentRunner) Run(agent string, j job.Job) ([]byte, error) {
	cmd := exec.Command(r.manigotPath, "--print", "--profile", r.profile, "--agent", agent, "--job", j.Name)
	cmd.Dir = r.projectRoot
	cmd.Env = append(os.Environ(), "PWD="+r.projectRoot)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.Bytes(), fmt.Errorf("mg --print --agent %s --job %s: %w: %s", agent, j.Name, err, strings.TrimSpace(stderr.String()))
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
		logAgentInvoked(log, decision.Agent, i+1)
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
		logInvocation(log, decision.Agent, i+1, out, targetFile, targetContent)

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
