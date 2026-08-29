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
  claude-pro        Claude Code, billed to your Claude Pro/Max subscription
  zai               OpenCode, billed to your Z.AI Coding Plan
  opencode-go       OpenCode, billed to the OpenCode Go subscription
  opencode-zen      OpenCode, billed to OpenCode Zen (DeepSeek V4 Flash)
  opencode-zen-free OpenCode, billed to OpenCode Zen (DeepSeek V4 Flash Free)

With no profile, the wizard walks through every profile — the built-ins above
plus any user-defined ones you've added (see 'mg profiles add') — and the
optional ntfy push-notification settings (NTFY_URL/NTFY_TOPIC/NTFY_TOKEN) for
mg jdi. --check reports which profiles are ready without prompting. Values are
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
		fmt.Fprintln(stderr, "Usage: mg setup [<profile>] [--check]")
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
		if _, ok := config.ProfileByID(arg); ok {
			if target != "" {
				fmt.Fprintln(stderr, "Error: give a single profile, not several.")
				return 1
			}
			target = arg
		} else {
			fmt.Fprintf(stderr, "Error: unknown argument '%s'.\n", arg)
			fmt.Fprintln(stderr, "Usage: mg setup [<profile>] [--check]")
			return 1
		}
	}

	if *check {
		if target != "" {
			checkProfile(target, stdout)
		} else {
			// Data-driven: check every profile (built-in + user-defined) in
			// canonical order.
			for _, p := range config.Profiles() {
				checkProfile(p.ID, stdout)
			}
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
		if p, ok := config.ProfileByID(target); ok {
			setupProfile(p, br, stdout)
		}
	} else {
		// Data-driven: walk every profile (built-in + user-defined), then the
		// optional ntfy block.
		for _, p := range config.Profiles() {
			setupProfile(p, br, stdout)
		}
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
	prof, ok := config.ProfileByID(p)
	if !ok {
		fmt.Fprintf(w, "  \u2717 %-12s unknown profile\n", p)
		return
	}
	var missing []string
	for _, k := range prof.AuthKeys {
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

func setupClaudePro(p config.Profile, r io.Reader, w io.Writer) {
	fmt.Fprintln(w, sepLine)
	fmt.Fprintln(w, "  claude-pro — Claude Code, billed to your Claude Pro/Max subscription")
	fmt.Fprintln(w, sepLine)
	if have("CLAUDE_CODE_OAUTH_TOKEN") && have("CLAUDE_ACCOUNT_UUID") && have("CLAUDE_EMAIL") && have("CLAUDE_ORG_UUID") {
		fmt.Fprintf(w, "  ✓ Already configured (token %s, %s).\n", cli.Mask(config.EnvValue("CLAUDE_CODE_OAUTH_TOKEN")), config.EnvValue("CLAUDE_EMAIL"))
		setupClaudeModel(p, r, w)
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
			setupClaudeModel(p, r, w)
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
	setupClaudeModel(p, r, w)
}

// setupClaudeModel is the optional model block of a claude-code profile's
// setup wizard, analogous to setupOpenCodeProfile's model prompt: it uses the
// profile's own ModelEnv/ModelDefault, so it is a complete no-op for
// claude-pro (which defines neither) — unchanged output there. A user-defined
// claude-code profile that does define a model gets prompted the same way an
// opencode profile does.
func setupClaudeModel(p config.Profile, r io.Reader, w io.Writer) {
	if p.ModelEnv == "" && p.ModelDefault == "" {
		return
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  Optional — the model this profile defaults to (e.g. sonnet, opus).")
	if p.ModelEnv != "" {
		promptValue("  "+p.ModelEnv, p.ModelEnv, p.ModelDefault, r, w)
		return
	}
	fmt.Fprintf(w, "  Default model: %s (no .env override key configured for this profile)\n", p.ModelDefault)
}

// setupProfile runs the setup wizard block for one profile, dispatching on its
// tool: the claude-code flow (the bespoke OAuth account wizard) and the
// data-driven opencode flow (setupOpenCodeProfile + setupSpecs). Called for
// every profile in config.Profiles(), so a user-defined profile gets a wizard
// too — a generic spec when its id has no bespoke entry in setupSpecs.
func setupProfile(p config.Profile, br *bufio.Reader, w io.Writer) {
	if p.Tool == config.ToolClaudeCode {
		setupClaudePro(p, br, w)
		return
	}
	spec, ok := setupSpecs[p.ID]
	if !ok {
		spec = genericOpenCodeSpec(p)
	}
	setupOpenCodeProfile(p, spec, br, w)
}

// profileSetupSpec carries the prose a profile's opencode wizard block prints.
// The built-in opencode profiles each have bespoke billing instructions; a
// user-defined profile (no entry in setupSpecs) falls back to the generic
// spec built by genericOpenCodeSpec.
type profileSetupSpec struct {
	title     string   // the line under sepLine
	intro     []string // paragraphs printed before the credential prompt
	modelHint []string // extra lines printed before the model prompt
}

// setupSpecs holds the per-profile wizard prose for the built-in opencode
// profiles, so the data-driven setupOpenCodeProfile reproduces their exact
// wording. claude-pro is handled by setupClaudePro (the claude-tool flow).
var setupSpecs = map[string]profileSetupSpec{
	config.ProfileZAI: {
		title: "  zai — OpenCode, billed to your Z.AI Coding Plan",
		intro: []string{
			"  OpenCode authenticates to Z.AI with an API key from your Z.AI",
			"  Coding Plan. Get one from the Z.AI / BigModel console",
			"  (https://www.bigmodel.cn), then paste it below.",
		},
	},
	config.ProfileOpenCodeGo: {
		title: "  opencode-go — OpenCode, billed to the OpenCode Go subscription",
		intro: []string{
			"  OpenCode Go uses your OpenCode API key — the same key you get at",
			"  https://opencode.ai/auth, billed against your Go subscription.",
			"",
			"  1. Open https://opencode.ai/auth and sign in",
			"  2. Subscribe to OpenCode Go (if you haven't already)",
			"  3. Copy your API key and paste it below",
		},
		modelHint: []string{
			"  Go model ids use the opencode-go/ prefix (e.g. opencode-go/deepseek-v4-flash).",
		},
	},
	config.ProfileOpenCodeZen: {
		title: "  opencode-zen — OpenCode, billed to OpenCode Zen (DeepSeek V4 Flash)",
		intro: []string{
			"  OpenCode Zen uses your OpenCode API key — the same key you get at",
			"  https://opencode.ai/auth. This DeepSeek V4 Flash model is billed",
			"  pay-as-you-go against your Zen account credits — add billing at",
			"  https://opencode.ai/auth if you haven't. For the no-cost variant, use",
			"  the opencode-zen-free profile instead.",
			"",
			"  1. Open https://opencode.ai/auth and sign in",
			"  2. Copy your API key and paste it below",
		},
		modelHint: []string{
			"  Zen's (billed) DeepSeek V4 Flash model is opencode/deepseek-v4-flash.",
		},
	},
	config.ProfileOpenCodeZenFree: {
		title: "  opencode-zen-free — OpenCode, billed to OpenCode Zen (DeepSeek V4 Flash Free)",
		intro: []string{
			"  OpenCode Zen uses your OpenCode API key — the same key you get at",
			"  https://opencode.ai/auth. The free DeepSeek V4 Flash model costs",
			"  no credits, so this profile works with a key alone.",
			"",
			"  1. Open https://opencode.ai/auth and sign in",
			"  2. Copy your API key and paste it below",
		},
		modelHint: []string{
			"  Zen's free DeepSeek V4 Flash model is opencode/deepseek-v4-flash-free.",
		},
	},
}

// genericOpenCodeSpec builds a wizard spec for a user-defined opencode profile
// that has no bespoke entry in setupSpecs.
func genericOpenCodeSpec(p config.Profile) profileSetupSpec {
	key := "the profile's API key"
	if len(p.AuthKeys) > 0 {
		key = p.AuthKeys[0]
	}
	return profileSetupSpec{
		title: fmt.Sprintf("  %s — OpenCode profile billed via %s", p.ID, key),
		intro: []string{
			fmt.Sprintf("  This user-defined profile bills against the %s credential.", key),
			"  Add it to manigot/.env (or inherit it from your environment) to",
			"  make the profile ready.",
		},
	}
}

// setupOpenCodeProfile is the data-driven opencode wizard block: the key, model
// env key, and model default all come from the profile; the surrounding prose
// from its spec. It reproduces the built-in opencode wizards byte-for-byte via
// setupSpecs.
func setupOpenCodeProfile(p config.Profile, spec profileSetupSpec, br *bufio.Reader, w io.Writer) {
	fmt.Fprintln(w, sepLine)
	fmt.Fprintln(w, spec.title)
	fmt.Fprintln(w, sepLine)
	key := ""
	if len(p.AuthKeys) > 0 {
		key = p.AuthKeys[0]
	}
	if key != "" && have(key) {
		fmt.Fprintf(w, "  ✓ Already configured (%s %s).\n", key, cli.Mask(config.EnvValue(key)))
	} else {
		for _, line := range spec.intro {
			fmt.Fprintln(w, line)
		}
		if key != "" {
			promptSecret("  "+key, key, br, w)
		}
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  Optional — the model this profile defaults to, as provider/model.")
	for _, line := range spec.modelHint {
		fmt.Fprintln(w, line)
	}
	promptValue("  "+p.ModelEnv, p.ModelEnv, p.ModelDefault, br, w)
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
