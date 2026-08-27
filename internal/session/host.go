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
	"github.com/lmuskalla/manigot/internal/fs"
	"github.com/lmuskalla/manigot/internal/home"
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

	// ── Global agents ─────────────────────────────────────────────────────
	// mg host runs the CLI directly with no image/mounts, so the host CLI
	// cannot see the manigot checkout's global agents (<home>/agents/) the way
	// the container path mounts them. Surface them into the CLI's own global
	// agents dir (~/.claude/agents/ for Claude Code,
	// ~/.config/opencode/agents/ for OpenCode) — symlinked raw for Claude Code
	// (the list form is its native schema), converted copies for OpenCode
	// (which hard-errors on the list-form tools: key — see convertAgentFile).
	// Non-destructive: a name already present in the target dir is left
	// untouched (the user's own host agent wins), and nothing happens when the
	// checkout has no agents/ dir — so a best-effort failure only warns and
	// never blocks the session.
	if n, err := installHostGlobalAgents(info.Tool); err != nil {
		fmt.Fprintf(diag, "Warning: could not make global agents available to %s on the host: %v\n", binary, err)
	} else if n > 0 {
		fmt.Fprintf(diag, "  Installed : %d global agent(s) into %s's host config\n", n, binary)
	}

	// ── Global skills ─────────────────────────────────────────────────────
	// The same delivery problem applies to skills: the host CLI cannot see the
	// checkout's global skills (<home>/skills/) without the image/mounts, so
	// they are surfaced into the CLI's own global skills dir — symlinked dirs
	// for Claude Code, copied dirs for OpenCode (see installHostGlobalSkills).
	// Non-destructive and warn-only, exactly like the agents install above.
	if n, err := installHostGlobalSkills(info.Tool); err != nil {
		fmt.Fprintf(diag, "Warning: could not make global skills available to %s on the host: %v\n", binary, err)
	} else if n > 0 {
		fmt.Fprintf(diag, "  Installed : %d global skill(s) into %s's host config\n", n, binary)
	}

	// ── Global meta prompt ──────────────────────────────────────────────────
	// The same delivery problem applies to the system-wide meta prompt
	// (<home>/meta.md): the host CLI cannot see it without the image/mounts,
	// so it is surfaced into the CLI's own global instruction file. Unlike
	// agents/skills it is a single file copied, never symlinked, so Claude's
	// /memory writes and agent edits can never reach the checkout. Non-
	// clobbering and warn-only, exactly like the agents/skills installs above.
	if n, err := installHostGlobalMeta(info.Tool); err != nil {
		fmt.Fprintf(diag, "Warning: could not install the global meta prompt for %s on the host: %v\n", binary, err)
	} else if n > 0 {
		fmt.Fprintf(diag, "  Installed : global meta prompt into %s's host config\n", binary)
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
	// Same shared printLogo as the docker builder (see docker.go) so the two
	// banners stay byte-identical — the ASCII logo above the box, then the
	// boxed details.
	printLogo(diag)
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
//
// For OpenCode sessions, TMUX/TMUX_PANE are filtered out of the child env —
// the same exception the docker path applies (see the terminalEnvVars comment
// in docker.go): when OpenCode sees TMUX set it wraps its OSC 52 clipboard
// write in tmux's DCS-passthrough escape, which default tmux configuration
// discards entirely, so the host clipboard is never touched. Stripping TMUX
// makes OpenCode emit plain OSC 52, which tmux's set-clipboard on handles.
// Claude Code's host env stays the full host environment.
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
	if info.Tool == config.ToolOpenCode {
		var filtered []string
		for _, kv := range env {
			if strings.HasPrefix(kv, "TMUX=") || strings.HasPrefix(kv, "TMUX_PANE=") {
				continue
			}
			filtered = append(filtered, kv)
		}
		env = filtered
	}
	return env
}

// hostGlobalAgentsDir resolves the host CLI's global agents directory for a
// tool: ~/.claude/agents/ for Claude Code, ~/.config/opencode/agents/ for
// OpenCode (the same locations the container path mounts the global agents
// into). The dir is derived from $HOME (os.UserHomeDir), so tests can point it
// at a temp dir.
func hostGlobalAgentsDir(tool string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}
	if tool == config.ToolOpenCode {
		return filepath.Join(homeDir, ".config", "opencode", "agents")
	}
	return filepath.Join(homeDir, ".claude", "agents")
}

// installHostGlobalAgents surfaces the manigot checkout's global agents
// (<home>/agents/) to the host CLI — the host equivalent of the container
// path's read-only mount (mg host runs the CLI directly, with no mounts).
//
// Delivery is per-tool, mirroring the container path's conversion split:
//
//   - Claude Code: each agent file is symlinked into ~/.claude/agents/ raw —
//     the list-form frontmatter (name:/description:/tools: Read, Grep, ...)
//     is Claude's native subagent schema. The symlinks point at the live
//     checkout files, so edits to agents/ are reflected without re-installing.
//   - OpenCode: OpenCode hard-errors on the list-form tools: key, so the raw
//     file cannot be symlinked — each agent is converted first (name:/tools:
//     stripped, permission: passed through — see convertAgentFile) and the
//     converted copy is written into ~/.config/opencode/agents/.
//
// It never clobbers existing host agent config: a name already present in the
// target dir is left untouched (the user's own host agent wins, mirroring the
// project-overrides-global precedence). One exception: a symlink pointing at
// the checkout's own agents/<name> is this installer's stale raw OpenCode link
// from before the conversion fix, and is replaced with a converted copy so it
// stops hard-erroring OpenCode. Nothing is created when the checkout has no
// agents/ dir or no home can be located. Returns the number of agents
// installed.
func installHostGlobalAgents(tool string) (int, error) {
	homeDir := home.Root()
	if homeDir == "" {
		return 0, nil
	}
	srcDir := filepath.Join(homeDir, "agents")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		names = append(names, e.Name())
	}
	if len(names) == 0 {
		return 0, nil
	}

	targetDir := hostGlobalAgentsDir(tool)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return 0, err
	}
	installed := 0
	for _, name := range names {
		target := filepath.Join(targetDir, name)
		if tool == config.ToolOpenCode {
			// OpenCode hard-errors on the list-form tools: key (see
			// convertAgents), so the raw checkout file cannot be symlinked:
			// the host target gets a converted copy instead. Never clobber an
			// existing host agent — except this installer's own stale raw
			// symlink (one pointing at the checkout's agents/<name>), which is
			// replaced with the converted copy.
			if fi, err := os.Lstat(target); err == nil {
				if fi.Mode()&os.ModeSymlink != 0 {
					if dest, rerr := os.Readlink(target); rerr == nil && dest == filepath.Join(srcDir, name) {
						os.Remove(target) // our stale raw link — replaced below
					} else {
						continue // a foreign symlink — the user's setup wins
					}
				} else {
					continue // a regular file — the user's own agent wins
				}
			}
			data, rerr := os.ReadFile(filepath.Join(srcDir, name))
			if rerr != nil {
				continue
			}
			if werr := os.WriteFile(target, convertAgentFile(data), 0o644); werr != nil {
				continue
			}
			installed++
			continue
		}
		// Claude Code — raw symlink (list form is Claude's native schema).
		// Never clobber an existing host agent config — the user's own file
		// (or an existing symlink, dangling or not) wins.
		if _, err := os.Lstat(target); err == nil {
			continue
		}
		if err := os.Symlink(filepath.Join(srcDir, name), target); err != nil {
			// Skip names we couldn't link but keep going for the rest.
			continue
		}
		installed++
	}
	return installed, nil
}

// hostGlobalSkillsDir resolves the host CLI's global skills directory for a
// tool: ~/.claude/skills/ for Claude Code, ~/.config/opencode/skills/ for
// OpenCode (the same locations the container path mounts the global skills
// into). The dir is derived from $HOME (os.UserHomeDir), so tests can point it
// at a temp dir.
func hostGlobalSkillsDir(tool string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}
	if tool == config.ToolOpenCode {
		return filepath.Join(homeDir, ".config", "opencode", "skills")
	}
	return filepath.Join(homeDir, ".claude", "skills")
}

// installHostGlobalSkills surfaces the manigot checkout's global skills
// (<home>/skills/) to the host CLI — the host equivalent of the container
// path's read-only mount (mg host runs the CLI directly, with no mounts).
//
// A skill is a directory (SKILL.md plus optional support files), so delivery
// is per-directory, mirroring installHostGlobalAgents' per-tool split:
//
//   - Claude Code: each skill dir is symlinked into ~/.claude/skills/ — the
//     symlinks point at the live checkout dirs, so edits to skills/ are
//     reflected without re-installing.
//   - OpenCode: each skill dir is copied into ~/.config/opencode/skills/ — a
//     self-contained snapshot rather than a symlink, so the CLI's skills dir
//     never points into (or writes into) the manigot checkout.
//
// It never clobbers existing host skill config: a name already present in the
// target dir is left untouched (the user's own host skill wins, mirroring the
// project-overrides-global precedence). Nothing is created when the checkout
// has no skills/ dir or no home can be located. Returns the number of skills
// installed.
func installHostGlobalSkills(tool string) (int, error) {
	homeDir := home.Root()
	if homeDir == "" {
		return 0, nil
	}
	srcDir := filepath.Join(homeDir, "skills")
	skills, err := listSkills(srcDir)
	if err != nil {
		return 0, err
	}
	if len(skills) == 0 {
		return 0, nil
	}

	targetDir := hostGlobalSkillsDir(tool)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return 0, err
	}
	installed := 0
	for _, s := range skills {
		target := filepath.Join(targetDir, s.Name)
		// Never clobber an existing host skill — the user's own dir (or an
		// existing symlink, dangling or not) wins.
		if _, err := os.Lstat(target); err == nil {
			continue
		}
		if tool == config.ToolOpenCode {
			// OpenCode gets a copied snapshot (see the doc comment above).
			if err := copyDir(target, s.Dir); err != nil {
				continue // skip names we couldn't copy, keep going for the rest
			}
		} else {
			// Claude Code — raw symlink to the live checkout dir.
			if err := os.Symlink(s.Dir, target); err != nil {
				continue
			}
		}
		installed++
	}
	return installed, nil
}

// hostGlobalMetaFile resolves the host CLI's global instruction file for a
// tool — the single file the CLI loads in every session at its *global
// instruction* location: ~/.claude/CLAUDE.md for Claude Code (the user-global
// memory file) and ~/.config/opencode/AGENTS.md for OpenCode (the global
// rules file) — the same locations the container path mounts <home>/meta.md
// into. The path is derived from $HOME (os.UserHomeDir), so tests can point it
// at a temp dir.
func hostGlobalMetaFile(tool string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}
	if tool == config.ToolOpenCode {
		return filepath.Join(homeDir, ".config", "opencode", "AGENTS.md")
	}
	return filepath.Join(homeDir, ".claude", "CLAUDE.md")
}

// installHostGlobalMeta surfaces the manigot checkout's system-wide meta
// prompt (<home>/meta.md) to the host CLI — the host equivalent of the
// container path's read-only mount (mg host runs the CLI directly, with no
// mounts).
//
// It delivers a single file by COPY, never a symlink: ~/.claude/CLAUDE.md is
// Claude Code's user-writable memory file, so a symlink would let Claude's
// /memory writes and agent edits land back in the manigot checkout. A copied
// file keeps the host CLI's own global instruction file fully self-contained.
//
// It never clobbers an existing host file: a target that already exists (the
// user's own CLAUDE.md / AGENTS.md) wins and nothing is written. Nothing is
// created when the checkout has no meta.md or no home can be located. Returns
// 1 when the file was installed, 0 otherwise.
func installHostGlobalMeta(tool string) (int, error) {
	homeDir := home.Root()
	if homeDir == "" {
		return 0, nil
	}
	src := filepath.Join(homeDir, "meta.md")
	if !fs.IsFile(src) {
		return 0, nil
	}

	target := hostGlobalMetaFile(tool)
	// Never clobber an existing host instruction file — the user's own file
	// wins.
	if _, err := os.Lstat(target); err == nil {
		return 0, nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return 0, err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return 0, err
	}
	return 1, nil
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
