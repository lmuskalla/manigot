// Package launch spawns manigot agent sessions in a new terminal window or
// pane so the TUI can keep running while the agent works.
//
// Spawn order (macOS + Linux; no Windows v1):
//  1. Inside tmux — when the TUI is itself running inside tmux (detected via
//     the $TMUX env var, with a tmux binary on PATH), this works the same on
//     every platform. The session is opened as a split pane in the TUI's
//     current window (`tmux split-window), replacing the pane manigot opened
//     last so at most one manigot pane exists at a time (the replace policy).
//     Inside tmux this branch always wins — even when
//     config.Settings.Terminal is set — so a session launched from a TUI
//     that is itself inside tmux lands in a tmux pane, never in a separate
//     window.
//  2. config.Settings.Terminal, when set: the explicit user-chosen terminal
//     (e.g. "kitty") is invoked directly — see buildOverrideCmd. It applies
//     only when the TUI is NOT inside tmux (or tmux is not available there);
//     inside tmux the split pane above wins.
//  3. macOS Terminal.app via osascript.
//  4. A Linux terminal emulator, tried in order: gnome-terminal, ptyxis
//     (Fedora's default GNOME terminal since Fedora 41), x-terminal-emulator
//     (Debian), konsole, xterm.
//
// The command opened is always `<mg-binary> --agent <agent> --job <jobID>` run
// from the project root, matching the session launcher's invocation contract.
// <mg-binary> is the running binary's own path (os.Executable) — the one Go
// binary IS the command now — so an install under a different name still
// works.
package launch

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/lmuskalla/manigot/internal/config"
)

// Agent opens a new terminal — inside tmux, a split pane in the TUI's current
// window (see launchTmuxPane) — that runs manigot with `--profile <profile>
// --agent <agent> --job <jobID>` in projectRoot. profile is one of the
// subscription profiles in config.Profiles; an empty value defaults to
// config.ProfileClaudePro, matching the session launcher's own default. terminal is
// config.Settings.Terminal: when non-empty and the TUI is NOT inside tmux, that
// terminal is invoked directly instead of the auto-detect chain below — see
// buildOverrideCmd; inside tmux the split pane wins regardless of this setting.
// Empty reproduces today's auto-detect behavior unchanged. It returns a short
// human description of where it opened (e.g. "tmux pane", "Terminal.app",
// "gnome-terminal", or the override's own binary name) so the caller can surface
// it in a status line.
//
// The launcher runs from projectRoot (the main worktree) even though the job
// lives in its own worktree: the session launcher
// (session.ResolveRootFrom) re-derives the effective mount root itself from
// the --job it is given (branch-match + worktree lookup, hard error on a
// worktree-less job), so launching from projectRoot is not just sufficient
// but required — it locates the project from $PWD and only then resolves the
// job's worktree. The same reasoning keeps Quick/AgentQuick (no --job at all, mount
// PROJECT_ROOT directly) and Jdi (mg-jdi, which re-derives per invocation)
// on projectRoot.
//
// The launcher process is detached: its stdio is discarded so it cannot
// corrupt the TUI's alt screen, and it is reaped asynchronously. The one
// exception is the tmux path, where the split-window client is run
// synchronously (see launchTmuxPane) so its pane-id output can be captured
// for the replace policy — split-window itself returns in milliseconds, it
// does not wait for the agent session.
//
// cmd.Start() failing (the launcher binary itself couldn't be
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
// strictly worse than the bug being fixed. holdOnFailure
// already covers the failure mode the brief actually reports (the inner `sc
// --agent` command failing fast); a launcher-binary-itself failure is a
// narrower, rarer case left uncovered rather than risk reintroducing UI lag.
func Agent(agent, jobID, projectRoot, profile, terminal string) (string, error) {
	exe, err := ExeOverride()
	if err != nil {
		return "", fmt.Errorf("locate mg binary: %w", err)
	}
	inner := shellCommand(exe, agent, jobID, projectRoot, profile)
	return launchDetached(inner, terminal)
}

// Quick opens a new terminal — inside tmux, a split pane in the TUI's current
// window, exactly like Agent, including the replace policy — that runs manigot
// with just `--profile <profile>` (no --agent, no --job) in projectRoot: a
// bare session for an ad-hoc change that doesn't belong to any specific
// job/agent workflow. profile is one of the subscription profiles in
// config.Profiles; an empty value defaults to config.ProfileClaudePro,
// matching Agent. terminal is config.Settings.Terminal, threaded through to
// launchDetached exactly like Agent's own terminal parameter — see Agent's
// doc for the override semantics. It returns the same short human description
// of where it opened so the caller can surface it in a status line.
//
// The session launcher treats --agent and --job as optional, so a bare `sc
// --profile <profile>` simply runs claude/opencode against the current project
// with no agent flag and no job prompt.
func Quick(projectRoot, profile, terminal string) (string, error) {
	exe, err := ExeOverride()
	if err != nil {
		return "", fmt.Errorf("locate mg binary: %w", err)
	}
	inner := quickShellCommand(exe, projectRoot, profile)
	return launchDetached(inner, terminal)
}

// AgentQuick opens a new terminal — inside tmux, a split pane in the TUI's
// current window, exactly like Agent/Quick, including the replace policy —
// that runs manigot with `--profile <profile> --agent <agent>` (no --job) in
// projectRoot: launching a specific agent for an ad-hoc session that doesn't
// belong to any particular job. profile is one of the subscription profiles
// in config.Profiles; an empty value defaults to config.ProfileClaudePro,
// matching Agent/Quick. terminal is config.Settings.Terminal, threaded
// through to launchDetached exactly like Agent/Quick's own terminal
// parameter — see Agent's doc for the override semantics. It returns the
// same short human description of where it opened so the caller can surface
// it in a status line.
//
// The session launcher treats --job as optional (see Quick's own doc for the
// --agent/--job-less case), so `sc --profile <profile> --agent <agent>`
// simply runs that agent against the current project with no job prompt.
func AgentQuick(agent, projectRoot, profile, terminal string) (string, error) {
	exe, err := ExeOverride()
	if err != nil {
		return "", fmt.Errorf("locate mg binary: %w", err)
	}
	inner := agentQuickShellCommand(exe, agent, projectRoot, profile)
	return launchDetached(inner, terminal)
}

// ExeOverride returns the path of the binary implementing the mg session
// subcommand — the running binary itself. An exported package-level variable
// so tests (in this package and internal/ui) can point Agent/Quick/AgentQuick
// at a stub or a resolution failure instead of the real os.Executable().
var ExeOverride = func() (string, error) { return os.Executable() }

// JdiExe returns the path of the binary implementing the `mg jdi` subcommand —
// the running binary itself. An exported package-level variable, mirroring
// ExeOverride, so tests (in this package and internal/ui) can point it at a
// stub or a resolution failure.
var JdiExe = func() (string, error) { return os.Executable() }

// Jdi starts `mg jdi --job <jobID> --profile <profile>` detached in the
// background — no spawned terminal window at all, unlike Agent/Quick.
// mg-jdi drives a fixed,
// non-interactive agent sequence with no TTY at any point in its own
// container invocations (the session launcher's --print flag) and needs no
// terminal for a human or a subprocess to attach to — spawning one anyway
// would be pure overhead and would reintroduce exactly the per-agent-window
// cost the backlog's "in-TUI agent terminal" idea is about removing, not
// adding to. Visibility into a TUI-launched run is the list-row status
// badge and the detail-view log tab, not a window; see those for how a
// human watches it run.
//
// profile is one of the subscription profiles in config.Profiles, matching
// Agent/Quick — an empty value defaults to config.ProfileClaudePro, mg-jdi's
// own default (see cmd/mg/jdi.go).
//
// cmd.Dir stays on projectRoot (the main worktree) even though the job lives
// in its own worktree: mg-jdi — like every other
// `--job` invocation — re-derives the effective worktree root itself from the
// job id it is given, per invocation, so it must be launched from the project
// root that find_project_root expects, never from a job worktree directly.
// See Agent's doc for the full reasoning, shared by all four launch paths.
//
// Unlike Agent/Quick there is no "where it opened" description to return —
// there is no terminal/pane to describe — so this returns only an error (nil
// on a successful start). Like launchDetached, the started process's stdio is
// discarded and it is reaped asynchronously so it cannot corrupt the TUI's
// alt screen or zombie; mg-jdi may run for a long time (several full agent
// sessions), so this must not block waiting for it.
func Jdi(jobID, projectRoot, profile string) error {
	if profile == "" {
		profile = config.ProfileClaudePro
	}
	exe, err := JdiExe()
	if err != nil {
		return fmt.Errorf("locate mg binary: %w", err)
	}
	cmd := exec.Command(exe, "jdi", "--job", jobID, "--profile", profile)
	cmd.Dir = projectRoot
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

// tmuxPaneTag is the pane title manigot tags the split panes it opens with
// (`tmux select-pane -t <id> -T`, run by launchTmuxPane after the split —
// split-window itself has no -T on any released tmux). It is how a later
// launch finds the pane manigot opened last so it can be replaced — see
// killPreviousTmuxPane. It lives in the pane title (in tmux, not in memory)
// so it survives a TUI restart.
const tmuxPaneTag = "manigot"

// tmuxMu serializes the find-kill-split sequence in launchTmuxPane so two
// concurrent launches (Agent/Quick can be invoked from Bubble Tea command
// goroutines) can never interleave: without it, both could list the same
// previous pane, both split, and the "at most one manigot pane" invariant
// would break. Also guards tmuxLastPaneID.
var tmuxMu sync.Mutex

// tmuxLastPaneID is the pane_id (e.g. "%12") of the most recent pane manigot
// opened via launchTmuxPane, or "" if none is known yet (e.g. the TUI was
// restarted after the pane was opened, or no launch has happened). Guarded by
// tmuxMu.
var tmuxLastPaneID string

// launchTmuxPane opens the manigot session as a split pane in the TUI's
// current tmux window and implements the "replace" reuse policy:
// before splitting, the pane manigot opened last is killed so at most one
// manigot pane exists at a time. It is the tmux-specific launch path — unlike
// the generic launchDetached tail it runs the split-window client
// synchronously because it needs its stdout (the new pane's id, via
// -P -F '#{pane_id}') to tag the pane with tmuxPaneTag (select-pane -T, since
// split-window has no -T on released tmux) and to record it for the next
// replace; split-window itself returns as soon as the pane exists, the inner
// command runs inside the pane, not in this client, so blocking on it is
// milliseconds and Output() captures stdout+stderr so nothing leaks onto the
// TUI's screen.
func launchTmuxPane(inner string) (string, error) {
	tmuxMu.Lock()
	defer tmuxMu.Unlock()

	// Replace, then split. The replace is best-effort: a failure to find the
	// previous pane must not prevent the new launch — if tmux is in a state
	// where list-panes fails, the split-window below will fail too and its
	// error is the accurate one to surface. The user having already closed
	// the previous pane is not an error at all (tolerated inside).
	_ = killPreviousTmuxPane()

	cmd, desc, err := buildCmd(inner, "")
	if err != nil {
		return "", err
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("launch %s: %w", desc, err)
	}
	paneID := strings.TrimSpace(string(out))
	if paneID == "" {
		return "", fmt.Errorf("launch %s: tmux split-window printed no pane id", desc)
	}

	// Tag the new pane so a later launch can find it even after a TUI restart
	// (which loses tmuxLastPaneID). select-pane -T is supported by every
	// released tmux, unlike split-window -T. Best-effort: the pane is already
	// live and the in-memory id covers replacement within this TUI process, so
	// a tagging failure only narrows the restart-surviving lookup — it must
	// not abort a launch that already succeeded.
	_ = exec.Command("tmux", "select-pane", "-t", paneID, "-T", tmuxPaneTag).Run()

	tmuxLastPaneID = paneID
	return desc, nil
}

// killPreviousTmuxPane implements the "replace" reuse policy: it kills
// the split pane manigot opened last so that at most one manigot pane exists
// at a time. The pane is identified two ways, both safe against ever killing
// a pane manigot did not itself open:
//
//   - tmuxLastPaneID — the pane_id learned from the previous split-window's
//     -P output; reliable while this TUI process lives.
//   - any pane in the current session whose title is tmuxPaneTag — set via
//     select-pane -T after the split (see launchTmuxPane); survives TUI
//     restarts because it lives in tmux, not in memory (a restart loses
//     tmuxLastPaneID).
//
// list-panes is scoped to the current session (-s): the manigot pane is split
// from the TUI's own pane, so it lives in the TUI's session (a tmux client
// run as a subprocess of the TUI resolves "current session" from $TMUX_PANE),
// and -s will also find it if the user moved it to another window of that
// session — without reaching into a different session where a different
// manigot TUI might legitimately have its own pane. A pane the user already
// closed in the meantime is not an error: that kill-pane failure is ignored.
// The kill is unconditional with respect to the pane's state — a pane mid-
// hold-on-failure is replaced exactly like a live session.
// Called with tmuxMu held.
func killPreviousTmuxPane() error {
	candidates := map[string]struct{}{}
	if tmuxLastPaneID != "" {
		candidates[tmuxLastPaneID] = struct{}{}
	}
	out, err := exec.Command("tmux", "list-panes", "-s", "-F", "#{pane_id} #{pane_title}").Output()
	if err != nil {
		return fmt.Errorf("tmux list-panes: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.SplitN(line, " ", 2)
		if len(fields) == 2 && fields[1] == tmuxPaneTag {
			candidates[fields[0]] = struct{}{}
		}
	}
	for id := range candidates {
		// Already closed by the user (or its window closed with it) → the
		// replace is a no-op, which is the tolerated case.
		_ = exec.Command("tmux", "kill-pane", "-t", id).Run()
	}
	return nil
}

// launchDetached is the shared spawn/reap tail used by both launch paths: it
// builds the *exec.Cmd for inner, discards its stdio so the launcher cannot
// corrupt the TUI's alt screen, starts it, and reaps the launcher
// asynchronously so it doesn't zombie (the actual terminal/pane outlives it).
// It returns the short "where it opened" description buildCmd produces.
//
// terminal is config.Settings.Terminal: when non-empty it is used in place of
// the macOS/Linux auto-detect chain (branch 3/4 below) — but only when the TUI
// is NOT inside tmux. Inside tmux (with a tmux binary on PATH) the launch is
// handled by launchTmuxPane instead, regardless of the override: a user who
// runs the TUI inside tmux always gets the split-pane behavior, and the
// override applies only outside tmux.
//
// Inside tmux, the launch is handled by launchTmuxPane: it implements the
// replace policy (killing the previously manigot-opened pane before splitting
// a new one) and needs the split-window client's stdout (the new pane's id),
// so it runs synchronously rather than through the detached tail below. The
// same tmux detection ($TMUX set and a tmux binary on PATH) is used here and
// in buildCmd.
//
// See Agent's doc comment for why a launcher that starts successfully but then
// fails on its own (e.g. no display server) is not surfaced here — holdOnFailure
// covers the inner command's own fast failures instead.
func launchDetached(inner, terminal string) (string, error) {
	// Inside tmux with a tmux binary on PATH, the tmux split pane always wins —
	// even when an override is set. The override applies only outside tmux.
	if os.Getenv("TMUX") != "" {
		if _, err := exec.LookPath("tmux"); err == nil {
			return launchTmuxPane(inner)
		}
	}

	cmd, desc, err := buildCmd(inner, terminal)
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
//	cd '<projectRoot>' && '<manigot>' --profile '<profile>' --agent '<agent>' --job '<jobID>'; ec=$?; ...
//
// cd-ing first matters because manigot finds the project root from $PWD (the
// same walk-up job.FindProjectRoot uses). manigotPath is the absolute path from the
// resolve package; it is quoted like every other value, so a checkout in a
// directory with spaces survives both osascript and `bash -lc`. Arguments are
// single-quoted and embedded single quotes are escaped, so no value can break
// out of its quotes. An empty profile defaults to config.ProfileClaudePro so the
// flag is always passed explicitly, regardless of what the session
// launcher's own default happens to be.
//
// None of buildCmd's five spawn paths keep the
// window/pane open once the inner command exits, success or failure — tmux
// destroys a new window as soon as its command exits (no remain-on-exit),
// Terminal.app's `do script` types the command but the window otherwise
// behaves normally, and gnome-terminal/x-terminal-emulator/konsole/xterm all
// close on exit by default. A fast failure (docker not running, resolve
// failure inside the container, an auth error) is therefore invisible: the
// window flashes and disappears before its output can be read — this is the
// brief's "a window appears and it immediately closes again". The result is
// wrapped in holdOnFailure so a non-zero exit pauses instead.
func shellCommand(manigotPath, agent, jobID, projectRoot, profile string) string {
	if profile == "" {
		profile = config.ProfileClaudePro
	}
	inner := fmt.Sprintf("cd %s && %s --profile %s --agent %s --job %s",
		shellQuote(projectRoot), shellQuote(manigotPath), shellQuote(profile), shellQuote(agent), shellQuote(jobID))
	return holdOnFailure(inner)
}

// quickShellCommand builds the shell string for a bare manigot session (no
// --agent, no --job), executed inside the new terminal:
//
//	cd '<projectRoot>' && '<manigot>' --profile '<profile>'; ec=$?; ...
//
// It is the --agent/--job-less counterpart to shellCommand: same cd-first,
// shellQuote-everything, holdOnFailure-wrap behavior (so a fast failure of the
// inner command still holds the window open), just without the
// agent/job flags. An empty profile defaults to config.ProfileClaudePro for the
// same reason shellCommand does. It is deliberately a separate function rather
// than a generalization of shellCommand so the latter's exact-format tests stay
// unchanged.
func quickShellCommand(manigotPath, projectRoot, profile string) string {
	if profile == "" {
		profile = config.ProfileClaudePro
	}
	inner := fmt.Sprintf("cd %s && %s --profile %s",
		shellQuote(projectRoot), shellQuote(manigotPath), shellQuote(profile))
	return holdOnFailure(inner)
}

// agentQuickShellCommand builds the shell string for AgentQuick's jobless
// agent session (--agent, no --job), executed inside the new terminal:
//
//	cd '<projectRoot>' && '<manigot>' --profile '<profile>' --agent '<agent>'; ec=$?; ...
//
// It sits between shellCommand (agent + job) and quickShellCommand (neither):
// same cd-first, shellQuote-everything, holdOnFailure-wrap behavior, just
// with --agent and no --job. An empty profile defaults to
// config.ProfileClaudePro for the same reason the other two do. Deliberately
// its own function rather than a generalization of either existing one, per
// the file's "deliberately a separate function" convention — keeps
// shellCommand/quickShellCommand's exact-format tests stable.
func agentQuickShellCommand(manigotPath, agent, projectRoot, profile string) string {
	if profile == "" {
		profile = config.ProfileClaudePro
	}
	inner := fmt.Sprintf("cd %s && %s --profile %s --agent %s",
		shellQuote(projectRoot), shellQuote(manigotPath), shellQuote(profile), shellQuote(agent))
	return holdOnFailure(inner)
}

// holdOnFailure wraps a shell command so that, if it exits non-zero, the
// window/pane it is running in stays open with the failure message visible
// instead of closing immediately. A clean exit (status 0) is left
// alone — the launched agent session usually runs interactively until the
// user ends it themselves, so closing right away once that happens is the
// expected behavior; only a fast, invisible failure needs holding open.
//
// Wrapping the shared inner command string (rather than reaching for a
// per-terminal mechanism like tmux's remain-on-exit or `xterm -hold`) keeps
// the fix uniform across all five spawn paths in buildCmd and lets it be
// exercised by ordinary string-based tests.
//
// The wrap behaves identically inside a tmux split pane as in a
// window — tmux runs the pane's shell-command in a pty and destroys the pane
// when the command exits, so the `read -r _ignored` blocking on a non-zero
// exit keeps the pane open with the failure message visible exactly as it
// held a new window open (the message may wrap in the narrower 35% pane, but
// stays fully visible). And because the replace policy treats every
// manigot pane alike, a pane mid-hold-on-failure is killed by a subsequent
// launch just like a live session is — deliberate and unconditional (the
// "skip or warn?" alternative was rejected: skipping would break
// the at-most-one-pane invariant and needs unfeasible mid-hold detection from
// outside the pane, and warning needs new TUI plumbing that is out of scope
// for this job). A user who launches something new has moved on from the
// failure they were present for when they launched it.
func holdOnFailure(cmd string) string {
	return cmd + `; ec=$?; if [ "$ec" -ne 0 ]; then echo; echo "--- manigot: exited with status $ec ---"; printf 'press enter to close... '; read -r _ignored; fi; exit "$ec"`
}

// shellQuote wraps s in single quotes, escaping any embedded single quote via
// the standard '\” idiom.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// terminalCandidates is the Linux terminal-emulator candidate table used both
// by buildCmd's auto-detect chain (branch 4) and, by name lookup, by
// buildOverrideCmd's known-name-vs-"-e"-fallback flag-convention choice:
// an override whose binary matches one of these names
// (case-insensitively) reuses that name's own flag convention instead of the
// "-e" default.
var terminalCandidates = []struct {
	name string
	pre  []string // args between the emulator name and the shell command
}{
	{"gnome-terminal", []string{"--"}},
	{"ptyxis", []string{"--"}},
	{"x-terminal-emulator", []string{"-e"}},
	{"konsole", []string{"-e"}},
	{"xterm", []string{"-e"}},
}

// buildOverrideCmd constructs the *exec.Cmd for an explicit user-chosen
// terminal override (config.Settings.Terminal): it takes over the auto-detect
// spawn order — the tmux branch is only reachable inside tmux, where
// launchDetached/buildCmd prefer the tmux pane, so an override is invoked when
// the TUI is NOT inside tmux (or tmux is unavailable there), replacing the
// macOS/Linux auto-detect chain (see launchDetached).
//
// terminal's first whitespace-separated token is the binary to run; any
// remaining tokens are passed as leading arguments before the flag that hands
// off the inner shell command — mirroring how config.Settings.Editor (and
// $VISUAL/$EDITOR) already allow trailing args, e.g. "code --wait". The
// binary is looked up via exec.LookPath exactly like the auto-detect
// candidates in terminalCandidates, so a typo'd or missing binary surfaces a
// clear "launch <name>: exec: ... file not found" error here instead of
// silently falling back to auto-detect.
//
// The flag used to hand off the inner command is chosen the same way as the
// existing candidates: if the binary case-insensitively matches one of
// terminalCandidates' own names, that name's convention ("--" or "-e") is
// reused; otherwise "-e" is used, since it is the more common convention
// among terminals not already in that list (kitty, alacritty, wezterm, rxvt,
// terminator all accept it). This is a best-effort convention, not a
// guarantee for every possible terminal emulator — a known limitation, not
// solved generally, since a fully general solution would need a per-terminal
// flag field the brief did not ask for.
func buildOverrideCmd(inner, terminal string) (*exec.Cmd, string, error) {
	fields := strings.Fields(terminal)
	if len(fields) == 0 {
		return nil, "", fmt.Errorf("terminal override is blank")
	}
	name := fields[0]
	leadingArgs := fields[1:]

	if _, err := exec.LookPath(name); err != nil {
		return nil, "", fmt.Errorf("launch %s: %w", name, err)
	}

	flag := "-e"
	for _, c := range terminalCandidates {
		if strings.EqualFold(c.name, name) {
			flag = c.pre[0]
			break
		}
	}

	args := append(append([]string{}, leadingArgs...), flag, "bash", "-lc", inner)
	return exec.Command(name, args...), name, nil
}

// buildCmd constructs the *exec.Cmd for the detected environment, or — when
// terminal (config.Settings.Terminal) is non-empty and the TUI is not inside
// tmux — for the explicit override instead (see buildOverrideCmd). The tmux
// branch (1) is checked first and always wins when the TUI is inside tmux and
// a tmux binary is on PATH, even over an override; branches 2–4 (override,
// macOS Terminal.app, Linux emulators) run detached via launchDetached's
// spawn/reap tail. The tmux branch is also the command launchTmuxPane runs
// synchronously.
func buildCmd(inner, terminal string) (*exec.Cmd, string, error) {
	// 1. Inside tmux (works on every OS that has tmux): split a pane off the
	// TUI's current window, replacing the previously manigot-opened
	// pane. Checked before the override so a session launched from a TUI
	// inside tmux always lands in a tmux pane, regardless of
	// config.Settings.Terminal.
	if os.Getenv("TMUX") != "" {
		if _, err := exec.LookPath("tmux"); err == nil {
			// -h splits side by side (vertical divider); -l 35% sizes the new
			// pane to 35% of the window width so the TUI's own pane (run
			// with no explicit -t, so this targets the TUI's own currently
			// active pane) keeps the majority share.
			// -l (not the older -p) is required: tmux deprecated -p's
			// percentage sizing in 3.1 in favor of -l with a "%" suffix, and
			// tmux 3.4 rejects -p outright with "size missing" (surfaced to
			// the user as the split-window client exiting 1) instead of
			// splitting.
			// -P -F '#{pane_id}' makes the split-window client print the new
			// pane's id to its stdout so launchTmuxPane can tag it with
			// tmuxPaneTag (via select-pane -T — split-window has no -T on any
			// released tmux) and record it for the replace policy.
			//
			// The trailing "bash", "-lc", inner (three separate argv elements,
			// not one combined string) matters: given a single shell-command
			// argument, tmux runs it via the pane's default-shell in -c mode,
			// which is non-login and non-interactive — .zshrc/.bash_profile are
			// never sourced, so any PATH customization living there (docker,
			// a manigot install dir, nvm, homebrew, ...) is invisible to it and
			// the inner command fails fast (caught by holdOnFailure, but the
			// session never actually starts). Passing multiple argv elements
			// instead makes tmux exec them directly with no shell in between,
			// so this becomes an ordinary `bash -lc inner` invocation — the
			// exact login-shell wrapping every other spawn path below already
			// uses (see shellArgs), just constructed inline since tmux's
			// argv-vs-single-string parsing is what selects shell-vs-direct-exec
			// here, unlike a plain exec.Command call.
			return exec.Command("tmux", "split-window", "-h", "-l", "35%",
				"-P", "-F", "#{pane_id}", "bash", "-lc", inner), "tmux pane", nil
		}
	}

	// 2. The explicit terminal override (config.Settings.Terminal), when the
	// TUI is not inside tmux (or tmux is unavailable there).
	if terminal != "" {
		return buildOverrideCmd(inner, terminal)
	}

	// 3. macOS Terminal.app.
	if runtime.GOOS == "darwin" {
		script := fmt.Sprintf(`tell application "Terminal" to do script %q`, inner)
		return exec.Command("osascript", "-e", script), "Terminal.app", nil
	}

	// 4. Linux terminal emulators, in order of preference.
	shellArgs := []string{"bash", "-lc", inner}
	for _, c := range terminalCandidates {
		if _, err := exec.LookPath(c.name); err == nil {
			args := append(append([]string{}, c.pre...), shellArgs...)
			return exec.Command(c.name, args...), c.name, nil
		}
	}

	return nil, "", fmt.Errorf("no supported terminal launcher found (looked for: tmux, Terminal.app, gnome-terminal, ptyxis, x-terminal-emulator, konsole, xterm)")
}
