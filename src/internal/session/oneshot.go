package session

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/lmuskalla/manigot/internal/job"
)

// AgentRunner runs one non-interactive agent invocation for job j and
// returns its captured raw output. live receives the exact same bytes as
// they arrive, so a caller can tee them into a live log. Mirrors cmd/mg/
// jdi.go's own AgentRunner interface — that package cannot be imported here
// (cmd/mg already depends on internal/session, not the other way around),
// so the shape is duplicated rather than shared; CommandAgentRunner below is
// this package's own concrete implementation, independent of cmd/mg's
// private commandAgentRunner.
type AgentRunner interface {
	Run(agent string, j job.Job, live io.Writer) ([]byte, error)
}

// CommandAgentRunner is the real AgentRunner for callers outside mg-jdi
// (internal/serve's detached one-shot agent launch, today): a single --print
// invocation via this package's own ResolveProfile → ResolveRootFrom →
// CheckAuth → BuildDockerInvocation → DockerInvocation.Run path, ending with
// the same post-run worktree sweep (SweepJobWorktree) mg-jdi's own runner
// performs so nothing an agent (or this invocation's own session.log
// section) left uncommitted lingers in the job's worktree.
type CommandAgentRunner struct {
	// ProjectRoot is where --job resolution starts from (mirrors
	// cmd/mg/jdi.go's commandAgentRunner.projectRoot).
	ProjectRoot string
	// Profile is the subscription profile every invocation runs under.
	Profile string
}

// Run invokes the session launcher's --print path synchronously and returns
// its captured stdout. live receives the exact same bytes as they arrive —
// the io.MultiWriter tee that lets a caller grow a live log during the
// invocation. j.Name (the exact job directory name) is passed rather than
// j.ID to remove any ambiguity in the --job resolution.
func (r *CommandAgentRunner) Run(agent string, j job.Job, live io.Writer) ([]byte, error) {
	opts := Options{Print: true, Profile: r.Profile, Agent: agent, Job: j.Name}
	info, err := ResolveProfile(opts)
	if err != nil {
		return nil, err
	}
	root, err := ResolveRootFrom(opts, r.ProjectRoot)
	if err != nil {
		return nil, err
	}
	if err := info.CheckAuth(); err != nil {
		return nil, err
	}

	var diag bytes.Buffer
	inv, err := BuildDockerInvocation(opts, info, root, false, &diag)
	if err != nil {
		return nil, err
	}

	// Prune orphaned manigot containers before the run — the same fail-soft
	// self-healing cleanup as every other launch path. A prune failure never
	// aborts the invocation.
	if _, err := PruneOrphans(&diag); err != nil {
		fmt.Fprintf(&diag, "mg: warning: could not prune orphaned containers: %v\n", err)
	}

	var stdout bytes.Buffer
	// No interactive stdin: this runner is only ever used for a detached,
	// one-shot --print invocation with nobody at a terminal to type into it.
	code, ran := inv.Run(nil, io.MultiWriter(&stdout, live), &diag)
	// Same host-side sweep as every other --print invocation: commits
	// whatever the agent (and this invocation's own session.log section) left
	// uncommitted. Sweep only when the container actually ran.
	if ran {
		SweepJobWorktree(root, &diag)
	}
	if code != 0 {
		return stdout.Bytes(), fmt.Errorf("mg --print --agent %s --job %s: exit code %d: %s", agent, j.Name, code, strings.TrimSpace(diag.String()))
	}
	return stdout.Bytes(), nil
}

// RunOneShot runs one agent invocation for job j via runner, opening a
// job.OpenSessionLog section (attempt fixed at 1 — this is not part of an
// mg-jdi run, so there is no per-agent attempt counter to thread through)
// before the invocation and closing it after, tee-ing the invocation's raw
// output into it live exactly as mg-jdi's own loop does.
//
// It is meant to be started in its own goroutine by an HTTP handler that
// returns immediately instead of blocking for the invocation's full duration
// (see internal/serve's detached agent-launch endpoint) — the goroutine's
// own errors have nowhere to report back to an already-returned HTTP
// response, so RunOneShot writes a failure into the same session.log section
// rather than dropping it silently, in addition to returning it (for a
// caller — chiefly a test — that runs it synchronously and wants the error
// directly).
//
// A failure to even open the session.log is best-effort, matching mg-jdi's
// own loop: the invocation still runs (with live output simply discarded),
// and only the open failure itself is returned/reported.
//
// KNOWN LIMITATION (documented, not solved here — see the brief's own
// callout): a concurrently-running mg-jdi loop against the SAME job would
// also be writing to the same session.log file at the same time. That is
// safe against file corruption — both are append-only,
// single-writer-per-section — but the underlying git worktree itself would
// have two independent processes touching it (container invocations,
// worktree sweeps), which this function does not detect or prevent.
func RunOneShot(runner AgentRunner, agent string, j job.Job) error {
	sess, sessErr := job.OpenSessionLog(filepath.Join(j.Dir, "session.log"), agent, 1)
	var live io.Writer = io.Discard
	if sess != nil {
		live = sess
	}

	_, runErr := runner.Run(agent, j, live)

	if runErr != nil && sess != nil {
		fmt.Fprintf(sess, "\nmg serve: agent invocation failed: %v\n", runErr)
	}
	if sess != nil {
		if cerr := sess.Close(); cerr != nil && runErr == nil {
			runErr = fmt.Errorf("could not finalize session log: %w", cerr)
		}
	}
	if runErr != nil {
		return runErr
	}
	if sessErr != nil {
		return fmt.Errorf("could not open session log: %w", sessErr)
	}
	return nil
}
