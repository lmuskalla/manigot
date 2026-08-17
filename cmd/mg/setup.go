package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lmuskalla/manigot/internal/cli"
	"github.com/lmuskalla/manigot/internal/config"
)

// sepLine is the 68-char box-drawing separator setup.sh printed above each
// profile's wizard block.
const sepLine = "────────────────────────────────────────────────────────────────────"

const setupHelp = `mg setup [profile] [--check]

Configures credentials for manigot's subscription profiles into manigot/.env:
  claude-pro    Claude Code, billed to your Claude Pro/Max subscription
  zai           OpenCode, billed to your Z.AI Coding Plan
  opencode-go   OpenCode, billed to the OpenCode Go subscription
  opencode-zen  OpenCode, billed to OpenCode Zen (DeepSeek V4 Flash Free)

With no profile, the wizard walks through all four, plus the optional ntfy
push-notification settings (NTFY_URL/NTFY_TOPIC/NTFY_TOKEN) for mg jdi.
--check reports which profiles are ready without prompting. Values are
written to the same .env mg reads; nothing is sent anywhere.
`

// runSetup implements `mg setup` — the port of scripts/setup.sh with identical
// output wording. r is the interactive input (used only when tty).
func runSetup(args []string, r io.Reader, stdout, stderr io.Writer, tty bool) int {
	// --check is a real flag; a profile name (or "help") is a positional.
	// The flags are extracted first (splitFlags) because the tests pin
	// "zai --check" — the profile before the flag — which Go's flag package
	// would otherwise stop at.
	flagArgs, rest := splitFlags(args, nil, map[string]bool{"--check": true, "-h": true, "--help": true})

	fs := flag.NewFlagSet("mg setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() { fmt.Fprint(stdout, setupHelp) }
	check := fs.Bool("check", false, "")
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0 // usage already printed to stdout
		}
		// An unknown flag (e.g. "--bogus"): the script's loop reported it the
		// same way as any other unknown argument.
		fmt.Fprintln(stderr, flagParseError(err))
		fmt.Fprintln(stderr, "Usage: mg setup [claude-pro|zai|opencode-go|opencode-zen] [--check]")
		return 1
	}

	// The positionals: profile names, or the bare-word "help" alias for -h.
	target := ""
	profileArgs := rest
	if len(profileArgs) == 1 && profileArgs[0] == "help" {
		fmt.Fprint(stdout, setupHelp)
		return 0
	}
	for _, arg := range profileArgs {
		if arg == config.ProfileClaudePro || arg == config.ProfileZAI || arg == config.ProfileOpenCodeGo || arg == config.ProfileOpenCodeZen {
			if target != "" {
				fmt.Fprintln(stderr, "Error: give a single profile, not several.")
				return 1
			}
			target = arg
		} else {
			fmt.Fprintf(stderr, "Error: unknown argument '%s'.\n", arg)
			fmt.Fprintln(stderr, "Usage: mg setup [claude-pro|zai|opencode-go|opencode-zen] [--check]")
			return 1
		}
	}

	if *check {
		if target != "" {
			checkProfile(target, stdout)
		} else {
			checkProfile(config.ProfileClaudePro, stdout)
			checkProfile(config.ProfileZAI, stdout)
			checkProfile(config.ProfileOpenCodeGo, stdout)
			checkProfile(config.ProfileOpenCodeZen, stdout)
		}
		return 0
	}

	if !tty {
		fmt.Fprintln(stderr, "mg setup: interactive setup needs a terminal.")
		fmt.Fprintln(stderr, "Use 'mg setup --check' for a non-interactive status report.")
		return 1
	}

	// One buffered reader for the whole wizard: each prompt reads a line, and a
	// fresh bufio.Reader per prompt would lose whatever the previous one
	// buffered past its newline.
	br := bufio.NewReader(r)

	if target != "" {
		switch target {
		case config.ProfileClaudePro:
			setupClaudePro(br, stdout)
		case config.ProfileZAI:
			setupZAI(br, stdout)
		case config.ProfileOpenCodeGo:
			setupOpenCodeGo(br, stdout)
		case config.ProfileOpenCodeZen:
			setupOpenCodeZen(br, stdout)
		}
	} else {
		setupClaudePro(br, stdout)
		setupZAI(br, stdout)
		setupOpenCodeGo(br, stdout)
		setupOpenCodeZen(br, stdout)
		setupNtfy(br, stdout)
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "  Done. Switch the default with: mg profiles <name>")
	fmt.Fprintln(stdout, "  or start a one-off session with:  mg --profile <name>")
	return 0
}

// checkProfile prints the `--check` status line for one profile: "✓ ready" or
// "✗ missing <keys>" with the same formatting setup.sh used.
func checkProfile(p string, w io.Writer) {
	keys := profileAuthKeys[p]
	var missing []string
	for _, k := range keys {
		if config.EnvValue(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) == 0 {
		fmt.Fprintf(w, "  \u2713 %-12s ready\n", p)
	} else {
		fmt.Fprintf(w, "  \u2717 %-12s missing: %s   (fix with: mg setup %s)\n", p, strings.Join(missing, " "), p)
	}
}

// promptSecret asks for a secret and upserts it into .env when the user types
// a value; Enter keeps the current value, and a bare prompt's empty answer
// prints the same "(skipped — NAME not set)" note setup.sh did.
func promptSecret(label, name string, r io.Reader, w io.Writer) {
	current := config.EnvValue(name)
	if current != "" {
		val, err := cli.PromptSecret(label, current, r, w)
		if err != nil {
			return
		}
		if val != "" {
			_ = config.UpsertEnv(name, val)
		}
		return
	}
	val, err := cli.PromptSecret(label, "", r, w)
	if err != nil {
		return
	}
	if val != "" {
		_ = config.UpsertEnv(name, val)
	} else {
		fmt.Fprintf(w, "  (skipped — %s not set)\n", name)
	}
}

// promptValue asks for a non-secret value and upserts the effective result
// (typed > current > default) into .env — matching setup.sh's prompt_value.
func promptValue(label, name, def string, r io.Reader, w io.Writer) {
	val, err := cli.PromptValue(label, config.EnvValue(name), def, r, w)
	if err != nil {
		return
	}
	if val != "" {
		_ = config.UpsertEnv(name, val)
	}
}

// claudeAccountFromJSON reads accountUuid/emailAddress/organizationUuid from
// ~/.claude.json (the host's Claude Code config), the Go replacement for
// setup.sh's python3 heredoc. ok is false when the file is missing, unreadable,
// or lacks a complete oauthAccount block.
func claudeAccountFromJSON() (uuid, email, org string, ok bool) {
	cfg := filepath.Join(os.Getenv("HOME"), ".claude.json")
	data, err := os.ReadFile(cfg)
	if err != nil {
		return "", "", "", false
	}
	var doc struct {
		OAuthAccount *struct {
			AccountUUID      string `json:"accountUuid"`
			EmailAddress     string `json:"emailAddress"`
			OrganizationUUID string `json:"organizationUuid"`
		} `json:"oauthAccount"`
	}
	if err := json.Unmarshal(data, &doc); err != nil || doc.OAuthAccount == nil {
		return "", "", "", false
	}
	a := doc.OAuthAccount
	return a.AccountUUID, a.EmailAddress, a.OrganizationUUID,
		a.AccountUUID != "" && a.EmailAddress != "" && a.OrganizationUUID != ""
}

func setupClaudePro(r io.Reader, w io.Writer) {
	fmt.Fprintln(w, sepLine)
	fmt.Fprintln(w, "  claude-pro — Claude Code, billed to your Claude Pro/Max subscription")
	fmt.Fprintln(w, sepLine)
	if have("CLAUDE_CODE_OAUTH_TOKEN") && have("CLAUDE_ACCOUNT_UUID") && have("CLAUDE_EMAIL") && have("CLAUDE_ORG_UUID") {
		fmt.Fprintf(w, "  ✓ Already configured (token %s, %s).\n", cli.Mask(config.EnvValue("CLAUDE_CODE_OAUTH_TOKEN")), config.EnvValue("CLAUDE_EMAIL"))
		return
	}
	fmt.Fprintln(w, "  Claude Code runs with your subscription's OAuth credentials. You need")
	fmt.Fprintln(w, "  a token plus three account details.")

	if !have("CLAUDE_CODE_OAUTH_TOKEN") {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  Step 1 — the OAuth token. On your HOST, with Claude Code installed")
		fmt.Fprintln(w, "  locally, run:")
		fmt.Fprintln(w, "      claude setup-token")
		fmt.Fprintln(w, "  Paste the 'sk-ant-oat01-…' token it prints below.")
		promptSecret("  CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN", r, w)
	} else {
		fmt.Fprintf(w, "  ✓ CLAUDE_CODE_OAUTH_TOKEN already set (%s).\n", cli.Mask(config.EnvValue("CLAUDE_CODE_OAUTH_TOKEN")))
	}

	if !(have("CLAUDE_ACCOUNT_UUID") && have("CLAUDE_EMAIL") && have("CLAUDE_ORG_UUID")) {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  Step 2 — account details (CLAUDE_ACCOUNT_UUID, CLAUDE_EMAIL,")
		fmt.Fprintln(w, "  CLAUDE_ORG_UUID).")
		if uuid, email, org, ok := claudeAccountFromJSON(); ok {
			fmt.Fprintln(w, "  Found them in ~/.claude.json on this host — applying automatically.")
			_ = config.UpsertEnv("CLAUDE_ACCOUNT_UUID", uuid)
			_ = config.UpsertEnv("CLAUDE_EMAIL", email)
			_ = config.UpsertEnv("CLAUDE_ORG_UUID", org)
			return
		}
		fmt.Fprintln(w, "  Could not read them from ~/.claude.json here. On the host where")
		fmt.Fprintln(w, "  Claude Code is logged in, extract them with:")
		fmt.Fprintln(w, `      cat ~/.claude.json | python3 -c "import json,sys; d=json.load(sys.stdin); print(json.dumps(d.get('oauthAccount'), indent=2))"`)
		fmt.Fprintln(w, "")
		promptValue("  CLAUDE_ACCOUNT_UUID", "CLAUDE_ACCOUNT_UUID", "", r, w)
		promptValue("  CLAUDE_EMAIL", "CLAUDE_EMAIL", "", r, w)
		promptValue("  CLAUDE_ORG_UUID", "CLAUDE_ORG_UUID", "", r, w)
	}
}

func setupZAI(r io.Reader, w io.Writer) {
	fmt.Fprintln(w, sepLine)
	fmt.Fprintln(w, "  zai — OpenCode, billed to your Z.AI Coding Plan")
	fmt.Fprintln(w, sepLine)
	if have("ZHIPU_API_KEY") {
		fmt.Fprintf(w, "  ✓ Already configured (ZHIPU_API_KEY %s).\n", cli.Mask(config.EnvValue("ZHIPU_API_KEY")))
	} else {
		fmt.Fprintln(w, "  OpenCode authenticates to Z.AI with an API key from your Z.AI")
		fmt.Fprintln(w, "  Coding Plan. Get one from the Z.AI / BigModel console")
		fmt.Fprintln(w, "  (https://www.bigmodel.cn), then paste it below.")
		promptSecret("  ZHIPU_API_KEY", "ZHIPU_API_KEY", r, w)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  Optional — the model this profile defaults to, as provider/model.")
	promptValue("  OPENCODE_ZAI_MODEL", "OPENCODE_ZAI_MODEL", "zai-coding-plan/glm-5.2", r, w)
}

func setupOpenCodeGo(r io.Reader, w io.Writer) {
	fmt.Fprintln(w, sepLine)
	fmt.Fprintln(w, "  opencode-go — OpenCode, billed to the OpenCode Go subscription")
	fmt.Fprintln(w, sepLine)
	if have("OPENCODE_API_KEY") {
		fmt.Fprintf(w, "  ✓ Already configured (OPENCODE_API_KEY %s).\n", cli.Mask(config.EnvValue("OPENCODE_API_KEY")))
	} else {
		fmt.Fprintln(w, "  OpenCode Go uses your OpenCode API key — the same key you get at")
		fmt.Fprintln(w, "  https://opencode.ai/auth, billed against your Go subscription.")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  1. Open https://opencode.ai/auth and sign in")
		fmt.Fprintln(w, "  2. Subscribe to OpenCode Go (if you haven't already)")
		fmt.Fprintln(w, "  3. Copy your API key and paste it below")
		promptSecret("  OPENCODE_API_KEY", "OPENCODE_API_KEY", r, w)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  Optional — the model this profile defaults to, as provider/model.")
	fmt.Fprintln(w, "  Go model ids use the opencode-go/ prefix (e.g. opencode-go/deepseek-v4-flash).")
	promptValue("  OPENCODE_GO_MODEL", "OPENCODE_GO_MODEL", "opencode-go/glm-5.2", r, w)
}

func setupOpenCodeZen(r io.Reader, w io.Writer) {
	fmt.Fprintln(w, sepLine)
	fmt.Fprintln(w, "  opencode-zen — OpenCode, billed to OpenCode Zen (DeepSeek V4 Flash Free)")
	fmt.Fprintln(w, sepLine)
	if have("OPENCODE_API_KEY") {
		fmt.Fprintf(w, "  ✓ Already configured (OPENCODE_API_KEY %s).\n", cli.Mask(config.EnvValue("OPENCODE_API_KEY")))
	} else {
		fmt.Fprintln(w, "  OpenCode Zen uses your OpenCode API key — the same key you get at")
		fmt.Fprintln(w, "  https://opencode.ai/auth. Zen's DeepSeek V4 Flash Free model costs")
		fmt.Fprintln(w, "  no credits, so this profile works with a key alone.")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  1. Open https://opencode.ai/auth and sign in")
		fmt.Fprintln(w, "  2. Copy your API key and paste it below")
		promptSecret("  OPENCODE_API_KEY", "OPENCODE_API_KEY", r, w)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  Optional — the model this profile defaults to, as provider/model.")
	fmt.Fprintln(w, "  Zen's free DeepSeek V4 Flash model is opencode/deepseek-v4-flash-free.")
	promptValue("  OPENCODE_ZEN_MODEL", "OPENCODE_ZEN_MODEL", "opencode/deepseek-v4-flash-free", r, w)
}

// setupNtfy is the optional ntfy block of the mg setup wizard: it prompts
// the three NTFY_* keys into manigot/.env so push notifications for mg jdi
// are discoverable without hand-editing the file. Unlike the profile blocks
// it is not a subscription profile, so --check does not cover it (its check
// shape is profile-shaped). Opt-in: leaving NTFY_TOPIC empty keeps
// notifications off, which is the default.
func setupNtfy(r io.Reader, w io.Writer) {
	fmt.Fprintln(w, sepLine)
	fmt.Fprintln(w, "  ntfy — push notifications for mg jdi (optional)")
	fmt.Fprintln(w, sepLine)
	if have("NTFY_TOPIC") {
		fmt.Fprintf(w, "  ✓ Already configured (NTFY_TOPIC %s).\n", cli.Mask(config.EnvValue("NTFY_TOPIC")))
		return
	}
	fmt.Fprintln(w, "  mg jdi can push a notification to your phone when a run stops")
	fmt.Fprintln(w, "  or when it finds a previous run crashed. Leave NTFY_TOPIC empty")
	fmt.Fprintln(w, "  to keep notifications off.")
	promptValue("  NTFY_URL", "NTFY_URL", "https://ntfy.sh", r, w)
	promptValue("  NTFY_TOPIC", "NTFY_TOPIC", "", r, w)
	promptSecret("  NTFY_TOKEN", "NTFY_TOKEN", r, w)
}

// have reports whether the effective value of key (process env + .env) is
// non-empty — the Go form of setup.sh's have() helper.
func have(key string) bool {
	return config.EnvValue(key) != ""
}
