package session

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/fs"
	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/home"
)

// DockerInvocation is a fully assembled `docker run` command.
type DockerInvocation struct {
	// Argv is the complete argument vector: ["docker", "run", ...].
	Argv []string

	// Cleanup removes any host-side resources the invocation created (e.g.
	// the converted-project-agents temp dir shadow-mounted over the docs
	// mount's agents/ subpath — see BuildDockerInvocation's agentMount
	// block). Run defers it so every caller — the interactive `mg` session
	// path and `mg jdi`'s --print runs alike — gets the cleanup for free
	// without having to know about it. nil when there is nothing to clean
	// up.
	Cleanup func()
}

// dockerImageName is the container image run.sh launched.
const dockerImageName = "manigot"

// terminalEnvVars lists the host terminal-identity variables forwarded into
// the container so the in-container agent TUIs see the real terminal. TUIs
// gate their copy/clipboard behavior on these: OSC 52 emission is decided
// from terminal identity/capability vars (TERM, COLORTERM, TERM_PROGRAM,
// TERM_PROGRAM_VERSION, VTE_VERSION, KITTY_WINDOW_ID, TMUX, TMUX_PANE,
// WT_SESSION — plus the WEZTERM_* family, which WezTerm sets several of).
// None of these were forwarded before, so the in-container TUIs saw an
// unrecognized terminal and copying from agent output silently failed (see
// the "Clipboard / copying from agent sessions" docs section).
//
// TMUX/TMUX_PANE are deliberately NOT forwarded for OpenCode sessions
// (terminalEnvArgs skips them when the tool is OpenCode): when OpenCode sees
// TMUX set it wraps its OSC 52 clipboard write in tmux's DCS-passthrough
// escape, which default tmux configuration discards entirely
// (allow-passthrough defaults to off), so the host clipboard is never
// touched. Stripping TMUX makes OpenCode emit plain OSC 52, which tmux's
// set-clipboard on handles correctly. Claude Code's container env stays
// byte-identical — TMUX/TMUX_PANE are still forwarded for it.
//
// Each var is forwarded only when set and non-empty on the host — never an
// empty value — so a session launched outside a terminal (CI, a script)
// keeps a byte-identical argv to a build without this feature.
var terminalEnvVars = []string{
	"TERM",
	"COLORTERM",
	"TERM_PROGRAM",
	"TERM_PROGRAM_VERSION",
	"VTE_VERSION",
	"KITTY_WINDOW_ID",
	"TMUX",
	"TMUX_PANE",
	"WT_SESSION",
}

// tmuxEnvVars are the terminal-identity vars whose forwarding is gated on
// the resolved tool: they are sent only to Claude Code, never to OpenCode
// (see the comment on terminalEnvVars for why).
var tmuxEnvVars = map[string]bool{
	"TMUX":      true,
	"TMUX_PANE": true,
}

// terminalEnvArgs returns the docker -e entries forwarding the host's
// terminal identity into the container: every terminalEnvVars entry that is
// set and non-empty on the host, plus every WEZTERM_* variable (also only
// when set to a non-empty value). TMUX/TMUX_PANE are skipped when tool is
// OpenCode (see terminalEnvVars). No var set → an empty slice, keeping the
// docker argv byte-identical to a session built without terminal forwarding.
func terminalEnvArgs(tool string) []string {
	var args []string
	for _, key := range terminalEnvVars {
		if tmuxEnvVars[key] && tool == config.ToolOpenCode {
			continue
		}
		if v := os.Getenv(key); v != "" {
			args = append(args, "-e", key+"="+v)
		}
	}
	for _, kv := range os.Environ() {
		name, value, ok := strings.Cut(kv, "=")
		if ok && strings.HasPrefix(name, "WEZTERM_") && value != "" {
			args = append(args, "-e", kv)
		}
	}
	return args
}

// warnTmuxClipboard detects the tmux clipboard-interception failure mode at
// session start and warns on diag (stderr — the diagnostics stream): when the
// session is interactive (a TUI to copy from — --print/mg-jdi runs capture
// JSON output instead, so the check is skipped), the host is inside tmux
// ($TMUX set) and tmux is on PATH, `tmux show-options -g set-clipboard` is
// run; if it reports off, tmux will strip the OSC 52 clipboard writes the
// in-container TUIs emit, so copying from the agent shows "copied" but never
// reaches the host clipboard. Strictly read-only: no tmux state is mutated,
// and any tmux failure (not installed, no server, an option query error)
// silently skips the check — no launch path can break because of it.
func warnTmuxClipboard(diag io.Writer, interactive, print bool) {
	if !interactive || print {
		return
	}
	if os.Getenv("TMUX") == "" {
		return
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return
	}
	out, err := exec.Command("tmux", "show-options", "-g", "set-clipboard").Output()
	if err != nil {
		return
	}
	if strings.TrimSpace(string(out)) == "set-clipboard off" {
		fmt.Fprintln(diag, "Warning: your tmux has set-clipboard off — it will swallow OSC 52 clipboard writes from the agent, so copying inside the session won't reach your host clipboard.")
		fmt.Fprintln(diag, "         Run 'tmux set -g set-clipboard on' to fix it (see the 'Clipboard / copying from agent sessions' docs section).")
	}
}

// BuildDockerInvocation assembles the docker run argv, mirroring run.sh's
// launch construction — every flag, mount, and env
// var is load-bearing, so the argument order and exact strings are pinned by
// the tests. Diagnostics (banner, warnings, shadowing lines) are written to
// diag — always stderr, per the brief's note 1 (the script's fd-3 juggling
// existed only to keep --print stdout clean; Go separates the streams
// natively). interactive controls the -it flag (stdin is a terminal).
func BuildDockerInvocation(opts Options, info ProfileInfo, root Root, interactive bool, diag io.Writer) (DockerInvocation, error) {
	// The .env warning run.sh printed when the file was missing.
	if envFile := config.EnvFile(); envFile != "" {
		if _, err := os.Stat(envFile); err != nil {
			fmt.Fprintf(diag, "Warning: no .env found at %s\n", envFile)
		}
	}

	// The tmux OSC 52 clipboard-interception warning (TASK-2 of the clipboard
	// job): strictly read-only, failures silently skipped.
	warnTmuxClipboard(diag, interactive, opts.Print)

	// ── Git identity ─────────────────────────────────────────────────────────
	// Host GIT_AUTHOR_NAME/EMAIL env vars win; otherwise fall back to the
	// project's git config (local overrides global, matching git behavior).
	gitName := os.Getenv("GIT_AUTHOR_NAME")
	if gitName == "" {
		gitName = git.ConfigUserName(root.ProjectRoot)
	}
	gitEmail := os.Getenv("GIT_AUTHOR_EMAIL")
	if gitEmail == "" {
		gitEmail = git.ConfigEmail(root.ProjectRoot)
	}
	if gitName == "" || gitEmail == "" {
		fmt.Fprintln(diag, "Warning: no git user.name/user.email configured for this project — container commits will fall back to a generic 'manigot' identity.")
	}

	// ── Docs mount target + context file ────────────────────────────────────
	docsMountTarget := "/workspace/.claude"
	if info.Tool == config.ToolOpenCode {
		docsMountTarget = "/workspace/.opencode"
	}

	contextFile := ""
	if fs.IsFile(filepath.Join(root.DocsDir, "AGENTS.md")) {
		contextFile = filepath.Join(root.DocsDir, "AGENTS.md")
	} else if fs.IsFile(filepath.Join(root.DocsDir, "CLAUDE.md")) {
		contextFile = filepath.Join(root.DocsDir, "CLAUDE.md")
	}

	var contextMount []string
	contextTarget := ""
	if contextFile != "" {
		if info.Tool == config.ToolOpenCode {
			contextTarget = "/workspace/AGENTS.md"
		} else {
			contextTarget = "/workspace/.claude/CLAUDE.md"
		}
		contextMount = []string{"-v", contextFile + ":" + contextTarget + ":ro"}
	} else if root.DocsInitialized {
		fmt.Fprintf(diag, "Warning: no docs/AGENTS.md found — the agent will start without project context.\n")
	} else {
		fmt.Fprintln(diag, "Note: no docs/ directory — starting a plain session with no project context or job workflow.")
		fmt.Fprintln(diag, "      See 'Per-project setup' in the manigot README to enable them.")
	}

	var docsMount []string
	if fs.IsDir(root.DocsDir) {
		docsMount = []string{"-v", root.DocsDir + ":" + docsMountTarget + ":z"}
	}

	// ── Project agents ─────────────────────────────────────────────────────
	// docs/agents/ rides along inside the docs mount above — fine for Claude
	// Code, whose subagent schema the list-form frontmatter already is, but
	// OpenCode hard-errors on the list-form tools: key (see convertAgents),
	// so custom project agents must be converted before the container sees
	// them. The converted copies land in a temp dir shadow-mounted over the
	// docs mount's agents/ subpath; the host's docs/agents/ source tree is
	// never modified. The temp dir is removed via the invocation's Cleanup
	// hook (see DockerInvocation.Run).
	var agentMount []string
	var agentCleanup func()
	if tmp, hasAgents, err := convertAgents(filepath.Join(root.DocsDir, "agents"), info.Tool); err != nil {
		return DockerInvocation{}, fmt.Errorf("convert project agents: %w", err)
	} else if hasAgents {
		agentMount = []string{"-v", tmp + ":/workspace/.opencode/agents:z"}
		agentCleanup = func() { os.RemoveAll(tmp) }
	}

	// ── Job prompt ──────────────────────────────────────────────────────────
	initialPrompt := ""
	if root.Job != "" {
		containerJobDir := "/workspace/docs/jobs/" + root.Job
		initialPrompt = "Please work on the job at " + containerJobDir + " — start by reading brief.md"
		if opts.Print {
			// --print sessions are non-interactive and unattended, so they get
			// the NEEDS-HUMAN-INPUT marker definition (interactive sessions
			// never see this — a human is there to answer questions).
			initialPrompt += "\n\nYou are running non-interactively and unattended — no human is watching this session or able to answer questions. If you cannot proceed without a human decision, stop immediately and print a line starting with exactly `NEEDS-HUMAN-INPUT:` followed by a one-sentence reason, and make no further changes."
		}
	} else if opts.Prompt != "" {
		initialPrompt = opts.Prompt
	}

	var agentFlag []string
	if opts.Agent != "" {
		agentFlag = []string{"--agent", opts.Agent}
	}

	// Claude takes the prompt positionally; OpenCode as --prompt.
	var promptArgs []string
	if initialPrompt != "" {
		if info.Tool == config.ToolOpenCode {
			promptArgs = []string{"--prompt", initialPrompt}
		} else {
			promptArgs = []string{initialPrompt}
		}
	}

	// ── .env shadow mounts ──────────────────────────────────────────────────
	var envMounts []string
	for _, envFile := range findEnvFiles(root.ProjectRoot) {
		rel := strings.TrimPrefix(envFile, root.ProjectRoot)
		containerPath := "/workspace" + rel
		fmt.Fprintf(diag, "  Shadowing: %s → /dev/null inside container\n", envFile)
		envMounts = append(envMounts, "--mount", "type=bind,source=/dev/null,target="+containerPath+",readonly")
	}
	if len(envMounts) == 0 {
		fmt.Fprintln(diag, "  Shadowed : none (no .env files found)")
	}

	// ── Flavor quote ────────────────────────────────────────────────────────
	// Picked once per session from assets/quotes.json; skipped in --print mode
	// (the entrypoint wouldn't print it anyway). Missing/empty is not an error.
	quote := ""
	if !opts.Print {
		quote = pickQuote()
	}

	// ── Info banner ─────────────────────────────────────────────────────────
	fmt.Fprintln(diag, "╔══════════════════════════════════════╗")
	fmt.Fprintln(diag, "║           manigot                   ║")
	fmt.Fprintln(diag, "╠══════════════════════════════════════╣")
	fmt.Fprintln(diag, "║  Entering safehouse (isolated session)...")
	fmt.Fprintf(diag, "║  Project : %s\n", filepath.Base(root.InvocationRoot))
	fmt.Fprintf(diag, "║  Root    : %s\n", root.ProjectRoot)
	if root.DocsInitialized {
		fmt.Fprintf(diag, "║  Docs    : %s\n", root.DocsDir)
	} else {
		fmt.Fprintln(diag, "║  Docs    : (none — job workflow unavailable, running off the books)")
	}
	if contextFile != "" {
		fmt.Fprintf(diag, "║  Context : %s → %s\n", contextFile, contextTarget)
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
	if opts.Print {
		fmt.Fprintln(diag, "║  Print   : yes (non-interactive)")
	}
	fmt.Fprintln(diag, "╚══════════════════════════════════════╝")
	fmt.Fprintln(diag, "")

	// ── Assemble the argv ───────────────────────────────────────────────────
	var ttyFlags []string
	if interactive && !opts.Print {
		ttyFlags = []string{"-it"}
	}

	// ── Git common-dir mount ────────────────────────────────────────────────
	// Committing agents need the gitdir writable (they commit their work);
	// non-committing agents get it read-only so they physically cannot touch
	// git metadata — the hard filesystem boundary behind the soft git shim —
	// with GIT_OPTIONAL_LOCKS=0 so read commands that would refresh the index
	// (git status, git diff) don't fail on the ro mount.
	var gitDirMount []string
	var gitDirEnv []string
	commits := agentCommits(opts, root)
	if root.GitCommonDir != "" {
		mode := "z"
		if !commits {
			mode = "ro"
			gitDirEnv = []string{"-e", "GIT_OPTIONAL_LOCKS=0"}
		}
		gitDirMount = []string{"-v", root.GitCommonDir + ":" + root.GitCommonDir + ":" + mode}
	}

	// ── Gitdir overlay mounts (job-worktree sessions) ───────────────────────
	// The gitdir mount exposes the whole repository's git metadata; read-only
	// overlay mounts shadow the sensitive subpaths so an agent cannot corrupt
	// them. Each overlay skips a missing source — docker would otherwise
	// create an empty, root-owned directory at the target path.
	var gitOverlayMounts []string
	if root.GitCommonDir != "" {
		// hooks/: an agent must not plant a hook that would later execute on
		// host-side git operations (mg done, mg delete) with host privileges.
		if hooksDir := filepath.Join(root.GitCommonDir, "hooks"); fs.IsDir(hooksDir) {
			gitOverlayMounts = append(gitOverlayMounts, "-v", hooksDir+":"+hooksDir+":ro")
		}
		// worktrees/: every OTHER job's worktree gitdir, so a misbehaving
		// agent cannot delete or corrupt another job's worktree registration.
		// The current job's own worktree gitdir is excluded by the helper —
		// it must stay writable for commits.
		if wtDirs, err := git.WorktreeGitDirs(root.ProjectRoot, root.ProjectRoot); err == nil {
			for _, wtDir := range wtDirs {
				if !fs.IsDir(wtDir) {
					continue
				}
				gitOverlayMounts = append(gitOverlayMounts, "-v", wtDir+":"+wtDir+":ro")
			}
		}
	}

	argv := []string{"run"}
	argv = append(argv, ttyFlags...)
	argv = append(argv, "--rm")
	argv = append(argv, "--name", fmt.Sprintf("manigot-%s-%d", filepath.Base(root.ProjectRoot), os.Getpid()))
	argv = append(argv, "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()))
	argv = append(argv, "-v", root.ProjectRoot+":/workspace:z")
	argv = append(argv, gitDirMount...)
	argv = append(argv, gitOverlayMounts...)
	argv = append(argv, docsMount...)
	argv = append(argv, agentMount...)
	argv = append(argv, contextMount...)
	argv = append(argv, envMounts...)
	argv = append(argv, gitDirEnv...)
	argv = append(argv, terminalEnvArgs(info.Tool)...)
	argv = append(argv, info.KeyEnv...)
	argv = append(argv, "-e", "GIT_AUTHOR_NAME_CFG="+gitName)
	argv = append(argv, "-e", "GIT_AUTHOR_EMAIL_CFG="+gitEmail)
	argv = append(argv, "-e", "MANIGOT_TOOL="+info.Tool)
	argv = append(argv, "-e", "MANIGOT_PRINT="+strconv.FormatBool(opts.Print))
	argv = append(argv, "-e", "MANIGOT_QUOTE="+quote)
	// MANIGOT_AGENT_COMMITS tells container-side tooling which agent is
	// running: the `shot` render tool's wrapper guard refuses to run for
	// non-committing agents (the Claude Code gating layer — OpenCode enforces
	// the same via permission: blocks), mirroring the git shim's soft layer.
	// Defaults to true when unset, matching agentCommits' own default so a
	// missing marker never breaks a committing agent.
	argv = append(argv, "-e", "MANIGOT_AGENT_COMMITS="+strconv.FormatBool(commits))
	argv = append(argv, "--network=bridge")
	argv = append(argv, "--memory=2g")
	argv = append(argv, "--security-opt=no-new-privileges")
	argv = append(argv, dockerImageName)
	argv = append(argv, agentFlag...)
	argv = append(argv, promptArgs...)
	argv = append(argv, opts.Pass...)

	return DockerInvocation{Argv: append([]string{"docker"}, argv...), Cleanup: agentCleanup}, nil
}

// Run executes the assembled docker invocation, wiring stdin/stdout/stderr
// through so Ctrl+C reaches the container, and returns docker's exit code.
// The second return value reports whether docker was actually exec'd — false
// only when the launch itself failed before docker ran (docker missing on
// PATH, permission denied, ...). It is the "did the container session
// happen" signal the job-worktree sweep keys off (see SweepJobWorktree): an
// agent that never ran must not trigger a sweep commit. An ExitError — docker
// ran and exited non-zero — still counts as ran=true: the session happened,
// even when the agent's exit code was non-zero.
// The invocation's Cleanup hook (if any) runs after the container exits.
func (d DockerInvocation) Run(stdin *os.File, stdout, stderr io.Writer) (int, bool) {
	if d.Cleanup != nil {
		defer d.Cleanup()
	}
	cmd := exec.Command(d.Argv[0], d.Argv[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), true
		}
		fmt.Fprintf(stderr, "mg: %v\n", err)
		return 1, false
	}
	return 0, true
}

// findEnvFiles lists every .env / .env.* file under root (recursively),
// excluding *.example and *.sample — the shadow-mount targets of run.sh.
func findEnvFiles(root string) []string {
	var files []string
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if name != ".env" && !strings.HasPrefix(name, ".env.") {
			return nil
		}
		if strings.HasSuffix(name, ".example") || strings.HasSuffix(name, ".sample") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files
}

// pickQuote parses assets/quotes.json (a JSON array of strings) and returns a
// random entry, or "" when the file is missing or unparseable — missing/empty
// just means no quote this session, not an error.
func pickQuote() string {
	home := home.Root()
	if home == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, "assets", "quotes.json"))
	if err != nil {
		return ""
	}
	var quotes []string
	if err := json.Unmarshal(data, &quotes); err != nil {
		return ""
	}
	if len(quotes) == 0 {
		return ""
	}
	return quotes[rand.IntN(len(quotes))]
}
