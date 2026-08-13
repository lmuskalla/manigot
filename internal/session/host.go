// Host mode (mg host) is the counterpart of the docker session path in
// docker.go: instead of assembling a `docker run` argv, it assembles a direct
// invocation of the profile's agent CLI (claude / opencode) running on the
// host itself — no container, no mounts, no isolation. Everything before the
// launch (ParseArgs, ResolveProfile, ResolveRoot, CheckAuth) is shared; only
// the last step differs.
//
// Host mode is for work that must happen on the host — outside the container.
// The CLIs are launched exactly as installed on the host: no auto-approval
// flags (--dangerously-skip-permissions / --auto are only safe inside the
// isolated, ephemeral container), so the user supervises every tool call.
package session

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lmuskalla/manigot/internal/config"
)

// HostInvocation is a fully assembled direct CLI invocation for host mode.
type HostInvocation struct {
	// Argv is the complete argument vector: ["claude", ...] or
	// ["opencode", ...].
	Argv []string

	// Dir is the working directory the CLI runs from: the resolved project
	// root, or the job's worktree once --job has resolved one.
	Dir string

	// Env is the child process environment: the host environment plus the
	// profile's credential keys (see hostEnv).
	Env []string
}

// hostBinaryName maps the profile's tool id (config.ToolClaudeCode /
// config.ToolOpenCode) to the binary installed on the host. The tool ids are
// manigot's own names ("claude-code"); the binaries are "claude"/"opencode".
func hostBinaryName(tool string) string {
	if tool == config.ToolOpenCode {
		return "opencode"
	}
	return "claude"
}

// hostLookPath is exec.LookPath, split out so tests can point it at stub
// binaries (or a failure) instead of requiring claude/opencode on the test
// machine's PATH. Mirrors internal/launch's ExeOverride pattern.
var hostLookPath = exec.LookPath

// BuildHostInvocation assembles the direct CLI argv for mg host, mirroring
// BuildDockerInvocation's structure minus everything docker-specific: no
// mounts, no .env shadowing, no git-identity forwarding, no quote, no
// auto-approval flags. Diagnostics (banner, warnings) go to diag — always
// stderr, per the same convention as the docker builder.
func BuildHostInvocation(opts Options, info ProfileInfo, root Root, diag io.Writer) (HostInvocation, error) {
	// --print is the non-interactive container path (bare `mg --print` and
	// mg jdi, whose orchestration parses the container output contract). Host
	// mode is for manual, interactive work — reject it outright rather than
	// silently starting an interactive CLI under an unattended caller.
	if opts.Print {
		return HostInvocation{}, fmt.Errorf("--print is not supported with mg host — host sessions run the CLI directly for interactive work.\nUse the docker session (bare `mg --print`) or 'mg jdi' for non-interactive runs.")
	}

	// The .env warning run.sh printed when the file was missing — host mode
	// reads the same credentials (config.EnvValue prefers the checkout's .env).
	if envFile := config.EnvFile(); envFile != "" {
		if _, err := os.Stat(envFile); err != nil {
			fmt.Fprintf(diag, "Warning: no .env found at %s\n", envFile)
		}
	}

	// mg host is just a launcher — the CLI must exist on the host. The docker
	// path needs no such check: both CLIs are baked into the image.
	binary := hostBinaryName(info.Tool)
	if _, err := hostLookPath(binary); err != nil {
		return HostInvocation{}, fmt.Errorf("%s is not installed on the host — mg host runs the CLI directly, without a container.\nInstall it, or use the docker session (bare mg) instead.", binary)
	}

	// ── Job prompt ──────────────────────────────────────────────────────────
	// Host-pathed variant of docker.go's container job prompt: the CLI runs on
	// the host, so the job path it is told to work on must be a host path.
	initialPrompt := ""
	if root.Job != "" {
		hostJobDir := filepath.Join(root.ProjectRoot, "docs", "jobs", root.Job)
		initialPrompt = "Please work on the job at " + hostJobDir + " — start by reading brief.md"
	} else if opts.Prompt != "" {
		initialPrompt = opts.Prompt
	}

	var agentFlag []string
	if opts.Agent != "" {
		agentFlag = []string{"--agent", opts.Agent}
	}

	// Claude takes the prompt positionally; OpenCode as --prompt — the same
	// per-tool routing the docker path uses.
	var promptArgs []string
	if initialPrompt != "" {
		if info.Tool == config.ToolOpenCode {
			promptArgs = []string{"--prompt", initialPrompt}
		} else {
			promptArgs = []string{initialPrompt}
		}
	}

	// ── OpenCode model ──────────────────────────────────────────────────────
	// The docker path forwards OPENCODE_MODEL and scripts/entrypoint.sh writes
	// it into an opencode config inside the container. On the host mg must not
	// write the user's real ~/.config/opencode/opencode.json, so the effective
	// model is forwarded via opencode's own --model flag instead (verified
	// supported: `opencode --help` shows -m/--model). OPENCODE_MODEL itself is
	// deliberately not put into the child env — opencode does not read it.
	var modelArgs []string
	if info.Tool == config.ToolOpenCode && info.OpenCodeModel != "" {
		modelArgs = []string{"--model", info.OpenCodeModel}
	}

	// ── Info banner ─────────────────────────────────────────────────────────
	fmt.Fprintln(diag, "╔══════════════════════════════════════╗")
	fmt.Fprintln(diag, "║           manigot                   ║")
	fmt.Fprintln(diag, "╠══════════════════════════════════════╣")
	fmt.Fprintln(diag, "║  Entering host mode (no docker container)...")
	fmt.Fprintf(diag, "║  Project : %s\n", filepath.Base(root.InvocationRoot))
	fmt.Fprintf(diag, "║  Root    : %s\n", root.ProjectRoot)
	if root.DocsInitialized {
		fmt.Fprintf(diag, "║  Docs    : %s\n", root.DocsDir)
	} else {
		fmt.Fprintln(diag, "║  Docs    : (none — job workflow unavailable, running off the books)")
	}
	if info.Profile != "" {
		fmt.Fprintf(diag, "║  Profile : %s\n", info.Profile)
	}
	if info.Tool != "" {
		fmt.Fprintf(diag, "║  Tool    : %s\n", info.Tool)
	}
	if opts.Agent != "" {
		fmt.Fprintf(diag, "║  Agent   : %s\n", opts.Agent)
	}
	if root.Job != "" {
		fmt.Fprintf(diag, "║  Job     : %s\n", root.Job)
	}
	fmt.Fprintln(diag, "╚══════════════════════════════════════╝")
	fmt.Fprintln(diag, "")

	// ── Assemble the argv ───────────────────────────────────────────────────
	// Deliberately no --dangerously-skip-permissions (Claude) / --auto
	// (OpenCode): safe only inside the isolated, ephemeral container. On the
	// host the CLI keeps its normal per-tool confirmation prompts.
	argv := []string{binary}
	argv = append(argv, agentFlag...)
	argv = append(argv, modelArgs...)
	argv = append(argv, promptArgs...)
	argv = append(argv, opts.Pass...)

	return HostInvocation{
		Argv: argv,
		Dir:  root.ProjectRoot,
		Env:  hostEnv(info),
	}, nil
}

// hostEnv builds the child process environment for a host invocation: the
// host's own environment plus the profile's credential keys (the KEY=VALUE
// strings behind ProfileInfo.KeyEnv's docker -e pairs). Appended last, so
// exec's duplicate-key handling (the last value wins) resolves every key to
// the .env-effective value the session flow already validated — the same
// credentials the container path forwards. OPENCODE_MODEL is excluded: it is
// forwarded as --model instead (see BuildHostInvocation), and opencode does
// not read it from the environment.
func hostEnv(info ProfileInfo) []string {
	env := os.Environ()
	for i := 0; i+1 < len(info.KeyEnv); i += 2 {
		if info.KeyEnv[i] != "-e" {
			continue
		}
		pair := info.KeyEnv[i+1]
		if strings.HasPrefix(pair, "OPENCODE_MODEL=") {
			continue
		}
		env = append(env, pair)
	}
	return env
}

// Run executes the assembled CLI invocation, wiring stdin/stdout/stderr
// through so Ctrl+C reaches the CLI directly (there is no container in
// between), and returns the CLI's exit code.
func (h HostInvocation) Run(stdin *os.File, stdout, stderr io.Writer) int {
	cmd := exec.Command(h.Argv[0], h.Argv[1:]...)
	cmd.Dir = h.Dir
	cmd.Env = h.Env
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "mg: %v\n", err)
		return 1
	}
	return 0
}
