// Package launch spawns manigot agent sessions in a new terminal window or
// pane so the TUI can keep running while the agent works.
//
// Spawn order (per the TASK-1 scope decision — macOS + Linux, no Windows v1):
//  1. tmux new-window — when the TUI is itself running inside tmux
//     (detected via the $TMUX env var), this works the same on every platform.
//  2. macOS Terminal.app via osascript.
//  3. A Linux terminal emulator, tried in order: gnome-terminal, ptyxis
//     (Fedora's default GNOME terminal since Fedora 41), x-terminal-emulator
//     (Debian), konsole, xterm.
//
// The command opened is always `<manigot> --agent <agent> --job <jobID>` run
// from the project root, matching the invocation contract in scripts/run.sh.
// <manigot> is an absolute path located by the resolve package, not the bare
// word "manigot", so an install under a different name still works.
package launch

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/lmuskalla/manigot/tui/internal/config"
	"github.com/lmuskalla/manigot/tui/internal/resolve"
)

// Agent opens a new terminal that runs manigot with `--tool <tool> --agent
// <agent> --job <jobID>` in projectRoot. tool is one of config.ToolClaudeCode
// or config.ToolOpenCode; an empty value defaults to config.ToolClaudeCode,
// matching scripts/run.sh's own default. It returns a short human description
// of where it opened (e.g. "tmux window", "Terminal.app", "gnome-terminal")
// so the caller can surface it in a status line.
//
// The launcher process is detached: its stdio is discarded so it cannot
// corrupt the TUI's alt screen, and it is reaped asynchronously.
//
// TASK-7 review: cmd.Start() failing (the launcher binary itself couldn't be
// spawned — e.g. permission denied) is already surfaced: it's returned as an
// error here and shown by the caller (App.updateDetail via cmdErrorText). The
// remaining gap is a launcher that starts successfully but then fails on its
// own after Agent has already returned "success" to the caller — e.g.
// gnome-terminal/konsole exiting non-zero because there's no display server.
// Surfacing that would need either blocking on cmd.Wait() here or wiring the
// reaping goroutine back into a tea.Msg, and neither is safe to add: xterm
// (unlike gnome-terminal/konsole/tmux/osascript, which detach quickly) *is*
// the window process, so it doesn't return until the window closes — for it,
// Wait()-ing here would block Update() for the whole agent session, which is
// strictly worse than the bug this job is fixing. TASK-6's holdOnFailure
// already covers the failure mode the brief actually reports (the inner `sc
// --agent` command failing fast); a launcher-binary-itself failure is a
// narrower, rarer case left uncovered rather than risk reintroducing UI lag.
func Agent(agent, jobID, projectRoot, tool string) (string, error) {
	found, err := resolve.Resolve(resolve.Manigot())
	if err != nil {
		return "", err
	}
	inner := shellCommand(found.Path, agent, jobID, projectRoot, tool)
	return launchDetached(inner)
}

// Quick opens a new terminal that runs manigot with just `--tool <tool>` (no
// --agent, no --job) in projectRoot — a bare session for an ad-hoc change that
// doesn't belong to any specific job/agent workflow. tool is one of
// config.ToolClaudeCode or config.ToolOpenCode; an empty value defaults to
// config.ToolClaudeCode, matching Agent. It returns the same short human
// description of where it opened so the caller can surface it in a status line.
//
// scripts/run.sh already treats --agent and --job as optional, so a bare `sc
// --tool <tool>` simply runs claude/opencode against the current project with
// no agent flag and no job prompt — no container-side changes were needed to
// support this.
func Quick(projectRoot, tool string) (string, error) {
	found, err := resolve.Resolve(resolve.Manigot())
	if err != nil {
		return "", err
	}
	inner := quickShellCommand(found.Path, projectRoot, tool)
	return launchDetached(inner)
}

// Jdi starts `mg-jdi --job <jobID>` detached in the background — no spawned
// terminal window at all, unlike Agent/Quick (Decision 7a in the "fully
// autonomous mode" brief). mg-jdi drives a fixed, non-interactive agent
// sequence with no TTY at any point in its own container invocations (see
// scripts/run.sh's --print flag) and needs no terminal for a human or a
// subprocess to attach to — spawning one anyway would be pure overhead and
// would reintroduce exactly the per-agent-window cost the backlog's "in-TUI
// agent terminal" idea is about removing, not adding to. Visibility into a
// TUI-launched run is TASK-8's list-row status badge and TASK-9's detail-view
// log tab, not a window; see those for how a human watches it run.
//
// Unlike Agent/Quick there is no "where it opened" description to return —
// there is no terminal/pane to describe — so this returns only an error (nil
// on a successful start). Like launchDetached, the started process's stdio is
// discarded and it is reaped asynchronously so it cannot corrupt the TUI's
// alt screen or zombie; mg-jdi may run for a long time (several full agent
// sessions), so this must not block waiting for it.
func Jdi(jobID, projectRoot string) error {
	found, err := resolve.Resolve(resolve.Jdi())
	if err != nil {
		return err
	}
	cmd := exec.Command(found.Path, "--job", jobID)
	cmd.Dir = projectRoot
	// exec.Cmd sets cwd via chdir but does not update the PWD env var; mg-jdi
	// (like the mg it shells out to) resolves the project root from $PWD via
	// job.FindProjectRoot, so set it explicitly to match — same pattern
	// tui/internal/hostcmd's NewJob/DoneCommand/DeleteCommand use.
	cmd.Env = append(os.Environ(), "PWD="+projectRoot)
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start mg jdi: %w", err)
	}
	if cmd.Process != nil {
		go func() { _ = cmd.Wait() }()
	}
	return nil
}

// launchDetached is the shared spawn/reap tail used by both launch paths: it
// builds the *exec.Cmd for inner, discards its stdio so the launcher cannot
// corrupt the TUI's alt screen, starts it, and reaps the launcher
// asynchronously so it doesn't zombie (the actual terminal/pane outlives it).
// It returns the short "where it opened" description buildCmd produces.
//
// See Agent's doc comment for why a launcher that starts successfully but then
// fails on its own (e.g. no display server) is not surfaced here — holdOnFailure
// covers the inner command's own fast failures instead.
func launchDetached(inner string) (string, error) {
	cmd, desc, err := buildCmd(inner)
	if err != nil {
		return "", err
	}
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("launch %s: %w", desc, err)
	}
	// Reap the launcher to avoid a zombie; the actual terminal/pane outlives it.
	if cmd.Process != nil {
		go func() { _ = cmd.Wait() }()
	}
	return desc, nil
}

// shellCommand builds the shell string executed inside the new terminal:
//
//	cd '<projectRoot>' && '<manigot>' --tool '<tool>' --agent '<agent>' --job '<jobID>'; ec=$?; ...
//
// cd-ing first matters because manigot finds the project root from $PWD (see
// scripts/run.sh find_project_root). manigotPath is the absolute path from the
// resolve package; it is quoted like every other value, so a checkout in a
// directory with spaces survives both osascript and `bash -lc`. Arguments are
// single-quoted and embedded single quotes are escaped, so no value can break
// out of its quotes. An empty tool defaults to config.ToolClaudeCode so the
// flag is always passed explicitly, regardless of what scripts/run.sh's own
// default happens to be.
//
// TASK-5 investigation: none of buildCmd's five spawn paths keep the
// window/pane open once the inner command exits, success or failure — tmux
// destroys a new window as soon as its command exits (no remain-on-exit),
// Terminal.app's `do script` types the command but the window otherwise
// behaves normally, and gnome-terminal/x-terminal-emulator/konsole/xterm all
// close on exit by default. A fast failure (docker not running, resolve
// failure inside the container, an auth error) is therefore invisible: the
// window flashes and disappears before its output can be read — this is the
// brief's "a window appears and it immediately closes again". The result is
// wrapped in holdOnFailure (TASK-6) so a non-zero exit pauses instead.
func shellCommand(manigotPath, agent, jobID, projectRoot, tool string) string {
	if tool == "" {
		tool = config.ToolClaudeCode
	}
	inner := fmt.Sprintf("cd %s && %s --tool %s --agent %s --job %s",
		shellQuote(projectRoot), shellQuote(manigotPath), shellQuote(tool), shellQuote(agent), shellQuote(jobID))
	return holdOnFailure(inner)
}

// quickShellCommand builds the shell string for a bare manigot session (no
// --agent, no --job), executed inside the new terminal:
//
//	cd '<projectRoot>' && '<manigot>' --tool '<tool>'; ec=$?; ...
//
// It is the --agent/--job-less counterpart to shellCommand: same cd-first,
// shellQuote-everything, holdOnFailure-wrap behavior (so a fast failure of the
// inner command still holds the window open — TASK-6), just without the
// agent/job flags. An empty tool defaults to config.ToolClaudeCode for the same
// reason shellCommand does. It is deliberately a separate function rather than
// a generalization of shellCommand so the latter's exact-format tests stay
// unchanged.
func quickShellCommand(manigotPath, projectRoot, tool string) string {
	if tool == "" {
		tool = config.ToolClaudeCode
	}
	inner := fmt.Sprintf("cd %s && %s --tool %s",
		shellQuote(projectRoot), shellQuote(manigotPath), shellQuote(tool))
	return holdOnFailure(inner)
}

// holdOnFailure wraps a shell command so that, if it exits non-zero, the
// window/pane it is running in stays open with the failure message visible
// instead of closing immediately (TASK-6). A clean exit (status 0) is left
// alone — the launched agent session usually runs interactively until the
// user ends it themselves, so closing right away once that happens is the
// expected behavior; only a fast, invisible failure needs holding open.
//
// Wrapping the shared inner command string (rather than reaching for a
// per-terminal mechanism like tmux's remain-on-exit or `xterm -hold`) keeps
// the fix uniform across all five spawn paths in buildCmd and lets it be
// exercised by ordinary string-based tests.
func holdOnFailure(cmd string) string {
	return cmd + `; ec=$?; if [ "$ec" -ne 0 ]; then echo; echo "--- manigot: exited with status $ec ---"; printf 'press enter to close... '; read -r _ignored; fi; exit "$ec"`
}

// shellQuote wraps s in single quotes, escaping any embedded single quote via
// the standard '\” idiom.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// buildCmd constructs the *exec.Cmd for the detected environment.
func buildCmd(inner string) (*exec.Cmd, string, error) {
	// 1. Inside tmux (works on every OS that has tmux).
	if os.Getenv("TMUX") != "" {
		if _, err := exec.LookPath("tmux"); err == nil {
			return exec.Command("tmux", "new-window", "-n", "manigot", inner), "tmux window", nil
		}
	}

	// 2. macOS Terminal.app.
	if runtime.GOOS == "darwin" {
		script := fmt.Sprintf(`tell application "Terminal" to do script %q`, inner)
		return exec.Command("osascript", "-e", script), "Terminal.app", nil
	}

	// 3. Linux terminal emulators, in order of preference.
	shellArgs := []string{"bash", "-lc", inner}
	candidates := []struct {
		name string
		pre  []string // args between the emulator name and the shell command
	}{
		{"gnome-terminal", []string{"--"}},
		{"ptyxis", []string{"--"}},
		{"x-terminal-emulator", []string{"-e"}},
		{"konsole", []string{"-e"}},
		{"xterm", []string{"-e"}},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c.name); err == nil {
			args := append(append([]string{}, c.pre...), shellArgs...)
			return exec.Command(c.name, args...), c.name, nil
		}
	}

	return nil, "", fmt.Errorf("no supported terminal launcher found (looked for: tmux, Terminal.app, gnome-terminal, ptyxis, x-terminal-emulator, konsole, xterm)")
}
