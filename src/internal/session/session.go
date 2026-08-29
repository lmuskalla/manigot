// Package session ports scripts/run.sh — the docker session launcher — into Go.
// It is the single host-side seam between the orchestrator and the agent
// environment: profile/tool resolution, project-root + --job worktree
// resolution, docker argv/mount/env construction, and the actual run.
//
// Behavior is a 1:1 port of run.sh, including its exact diagnostic wording.
// One deliberate divergence per the consolidation brief (note 1): all
// diagnostics go to stderr. The script's fd-3 juggling existed only to keep
// the agent's stdout clean in --print mode; Go separates stdout/stderr
// natively, so there is nothing to juggle.
package session

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/lmuskalla/manigot/internal/config"
)

// Options carries the parsed session flag set.
type Options struct {
	Agent   string // --agent (or -a)
	Job     string // --job (or -j)
	Prompt  string // --prompt
	Tool    string // --tool (legacy alias)
	Profile string // --profile
	Print   bool   // --print
	Pass    []string
}

// ParseArgs parses the session flags: known --flags (and the -a/-j short
// forms of --agent/--job) consume their value, --print is a bare flag,
// anything else — an unknown flag or a bare word — is passthrough, handed
// verbatim to the container CLI (run.sh's semantics).
//
// The passthrough rule is why the known flags are extracted first (splitFlags)
// and parsed with a flag.FlagSet, rather than feeding flag the raw args:
// flag stops at the first non-flag argument and treats unknown flags as
// errors, neither of which matches "everything unknown goes through".
func ParseArgs(args []string) Options {
	var o Options
	fs := flag.NewFlagSet("mg", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.Agent, "agent", "", "")
	fs.StringVar(&o.Job, "job", "", "")
	fs.StringVar(&o.Agent, "a", "", "") // short form of --agent
	fs.StringVar(&o.Job, "j", "", "")   // short form of --job
	fs.StringVar(&o.Prompt, "prompt", "", "")
	fs.StringVar(&o.Tool, "tool", "", "")
	fs.StringVar(&o.Profile, "profile", "", "")
	fs.BoolVar(&o.Print, "print", false, "")

	var flagArgs []string
	flagArgs, o.Pass = splitFlags(args, sessionValueFlags, sessionBareFlags)
	// The only parse error possible is a known flag left without its value at
	// the end (e.g. a bare "--agent"); flag leaves the field unset, which is
	// the loop's old silent-ignore behavior. There is no error path to the
	// caller.
	_ = fs.Parse(flagArgs)
	return o
}

// sessionValueFlags / sessionBareFlags name the session's own flags for
// ParseArgs's splitFlags extraction. The short forms -a/-j are aliases of
// --agent/--job and are registered in the flag.FlagSet the same way.
var (
	sessionValueFlags = map[string]bool{"--agent": true, "--job": true, "--prompt": true, "--tool": true, "--profile": true, "-a": true, "-j": true}
	sessionBareFlags  = map[string]bool{"--print": true}
)

// splitFlags separates args into the known flag tokens (with their values)
// and everything else, preserving order within each group: the pieces the
// flag.FlagSet parses, and the passthrough remainder. valueFlags take one
// value each; bareFlags take none. Any other token — an unknown flag or a
// bare word — lands in rest.
func splitFlags(args []string, valueFlags, bareFlags map[string]bool) (flagArgs, rest []string) {
	for i := 0; i < len(args); i++ {
		switch {
		case bareFlags[args[i]]:
			flagArgs = append(flagArgs, args[i])
		case valueFlags[args[i]]:
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		default:
			rest = append(rest, args[i])
		}
	}
	return flagArgs, rest
}

// legacyOpenCodeKeys is the nine-key list the legacy profile-less `--tool
// opencode` path forwards. Kept in sync with scripts/entrypoint.sh (the
// container-side safety net); Go pre-validates these keys before launch, so
// drift between the two is harmless.
var legacyOpenCodeKeys = []string{
	"ANTHROPIC_API_KEY",
	"OPENAI_API_KEY",
	"OPENROUTER_API_KEY",
	"GOOGLE_GENERATIVE_AI_API_KEY",
	"GROQ_API_KEY",
	"XAI_API_KEY",
	"DEEPSEEK_API_KEY",
	"OPENCODE_API_KEY",
	"ZHIPU_API_KEY",
}

// ProfileInfo is the outcome of profile/tool resolution. Credential
// validation is a separate step (CheckAuth) so the session flow can match
// run.sh's ordering, where the auth checks run after --job resolution.
type ProfileInfo struct {
	// Profile is the resolved profile id: claude-pro, zai, opencode-go,
	// opencode-zen, opencode-zen-free — or "" for the legacy profile-less
	// --tool opencode path.
	Profile string

	// Tool is the agent CLI to launch: config.ToolClaudeCode or
	// config.ToolOpenCode.
	Tool string

	// AuthKeys are the profile's credential env keys, resolved from
	// config.ProfileByID (the profile's AuthKeys). The claude branch of
	// CheckAuth validates and forwards exactly these; for opencode profiles
	// it is the same set as OpenCodeKeys.
	AuthKeys []string

	// OpenCodeKeys lists the provider keys this opencode run forwards.
	OpenCodeKeys []string

	// OpenCodeModel is the effective OPENCODE_MODEL value for opencode runs
	// ("" when none — only the legacy path can leave it unset).
	OpenCodeModel string

	// ClaudeModel is the effective model override for claude-code runs,
	// resolved the same way as OpenCodeModel (envDefault(p.ModelEnv,
	// p.ModelDefault)) — "" when the profile defines neither, which is what
	// keeps claude-pro's behavior unchanged (no ModelEnv/ModelDefault, so
	// nothing is forwarded and the CLI uses its own default).
	ClaudeModel string

	// OpenCodeTheme is the effective OPENCODE_THEME value for opencode runs
	// ("" when unset — the global theme setting, config.Settings.Theme, is
	// shared across every opencode profile, unlike OpenCodeModel).
	OpenCodeTheme string

	// KeyEnv holds the docker -e arguments for the forwarded credential keys
	// (and the opencode model/theme), e.g. ["-e", "CLAUDE_CODE_OAUTH_TOKEN=...",
	// "-e", "ZHIPU_API_KEY=...", "-e", "OPENCODE_MODEL=...", "-e",
	// "OPENCODE_THEME=..."]. Filled by CheckAuth.
	KeyEnv []string
}

// ResolveProfile implements run.sh's profile resolution block verbatim in
// behavior: precedence is --profile > --tool (legacy map:
// claude-code→claude-pro, opencode→legacy empty-profile mode) >
// $MANIGOT_PROFILE (validated) > claude-pro; then the profile→tool/key/model
// mapping and the --print + legacy-opencode rejection. Values are read via
// config.EnvValue, which mirrors the script's `set -a; source .env` (the .env
// file wins, then the process environment). Every error's wording matches the
// script's output.
//
// The credential auth checks are NOT part of this step — call CheckAuth after
// the root/--job resolution, mirroring run.sh's own ordering (the script
// resolves the job before it checks keys, so a job error takes precedence
// over a missing-key error when both apply).
func ResolveProfile(opts Options) (ProfileInfo, error) {
	profile := opts.Profile
	switch {
	case profile != "":
		if _, ok := config.ProfileByID(profile); !ok {
			return ProfileInfo{}, fmt.Errorf("--profile must be one of: %s (got '%s').", profileValidList(), profile)
		}
	case opts.Tool != "":
		switch opts.Tool {
		case config.ToolClaudeCode:
			profile = config.ProfileClaudePro
		case config.ToolOpenCode:
			profile = "" // legacy opencode mode, handled below
		default:
			return ProfileInfo{}, fmt.Errorf("--tool must be 'claude-code' or 'opencode' (got '%s').", opts.Tool)
		}
	default:
		profile = config.EnvValue("MANIGOT_PROFILE")
		if profile == "" {
			profile = config.ProfileClaudePro
		}
		if _, ok := config.ProfileByID(profile); !ok {
			return ProfileInfo{}, fmt.Errorf("MANIGOT_PROFILE in %s is not a valid profile (got '%s').\nValid profiles: %s", config.EnvFile(), profile, profileValidList())
		}
	}

	info := ProfileInfo{Profile: profile}
	if profile != "" {
		// Data-driven: the tool, auth keys, and model env/default all come
		// from the profile's definition in config, so a user-defined profile
		// resolves exactly like the built-in ones.
		if p, ok := config.ProfileByID(profile); ok {
			info.Tool = p.Tool
			info.AuthKeys = append([]string(nil), p.AuthKeys...)
			if p.Tool == config.ToolOpenCode {
				info.OpenCodeKeys = append([]string(nil), p.AuthKeys...)
				info.OpenCodeModel = envDefault(p.ModelEnv, p.ModelDefault)
			} else if p.Tool == config.ToolClaudeCode {
				info.ClaudeModel = envDefault(p.ModelEnv, p.ModelDefault)
			}
		}
	} else {
		info.Tool = config.ToolOpenCode
		info.OpenCodeKeys = legacyOpenCodeKeys
		info.AuthKeys = legacyOpenCodeKeys
		info.OpenCodeModel = config.EnvValue("OPENCODE_MODEL") // forwarded as-is
	}

	// The theme setting is global — one value shared across every opencode
	// profile (config.Settings.Theme/OPENCODE_THEME), unlike the per-profile
	// model — so it's read once here rather than per-profile above.
	if info.Tool == config.ToolOpenCode {
		info.OpenCodeTheme = config.EnvValue("OPENCODE_THEME")
	}

	// --print is a non-interactive, one-shot invocation built for automated
	// callers like mg-jdi. The legacy, profile-less --tool opencode path is
	// intentionally left rejected here — it predates the profile system.
	if opts.Print && info.Tool == config.ToolOpenCode && info.Profile == "" {
		return ProfileInfo{}, fmt.Errorf("--print is not supported with the legacy --tool opencode (no --profile).\nUse --profile zai, --profile opencode-go, --profile opencode-zen or --profile opencode-zen-free instead.")
	}

	return info, nil
}

// CheckAuth validates the resolved profile's credentials (run.sh's auth
// block, which ran after --job resolution) and fills in KeyEnv — the docker
// -e arguments for the forwarded opencode keys and model.
func (info *ProfileInfo) CheckAuth() error {
	if info.Tool == config.ToolClaudeCode {
		// The first auth key is the OAuth token every claude profile requires
		// (the built-in's is CLAUDE_CODE_OAUTH_TOKEN); the rest are optional
		// account details. The message keeps the built-in's exact wording.
		tokenKey := "CLAUDE_CODE_OAUTH_TOKEN"
		if len(info.AuthKeys) > 0 {
			tokenKey = info.AuthKeys[0]
		}
		if config.EnvValue(tokenKey) == "" {
			return fmt.Errorf("%s is not set.\nAdd it to %s, or run 'mg setup %s' for help:\n  %s=sk-ant-oat01-...", tokenKey, config.EnvFile(), info.Profile, tokenKey)
		}
		// Subscription protection: an API key would override the mounted
		// OAuth credentials and bill per token.
		if v := config.EnvValue("ANTHROPIC_API_KEY"); v != "" {
			return fmt.Errorf("ANTHROPIC_API_KEY is set — this overrides your subscription and bills per token.\nRemove it from your environment before running mg with the claude-pro profile.")
		}
		// Forward the subscription credentials into the container. The
		// empty-value filter matches the opencode handling below, so a
		// non-claude profile's docker argv never carries -e CLAUDE_*==""
		// noise (the token was just validated; the other three are optional).
		for _, key := range info.AuthKeys {
			if v := config.EnvValue(key); v != "" {
				info.KeyEnv = append(info.KeyEnv, "-e", key+"="+v)
			}
		}
		return nil
	}

	// OpenCode is multi-provider: forward exactly the key(s) the profile is
	// billed against; any one of them is enough to start.
	for _, key := range info.OpenCodeKeys {
		if v := config.EnvValue(key); v != "" {
			info.KeyEnv = append(info.KeyEnv, "-e", key+"="+v)
		}
	}
	if len(info.KeyEnv) == 0 {
		var msg string
		if info.Profile != "" {
			msg = fmt.Sprintf("profile '%s' is missing its API key.\nAdd it to %s, or run 'mg setup %s' for help:", info.Profile, config.EnvFile(), info.Profile)
		} else {
			msg = fmt.Sprintf("--tool opencode needs at least one provider API key.\nAdd one of these to %s:", config.EnvFile())
		}
		for _, k := range info.OpenCodeKeys {
			msg += "\n  " + k
		}
		return errors.New(msg)
	}

	// The model each profile defaults to, consumed by scripts/entrypoint.sh
	// via the {env:OPENCODE_MODEL} config substitution.
	if info.OpenCodeModel != "" {
		info.KeyEnv = append(info.KeyEnv, "-e", "OPENCODE_MODEL="+info.OpenCodeModel)
	}
	// The global theme setting, forwarded independent of which profile/API
	// key is in use, consumed by scripts/entrypoint.sh via the
	// {env:OPENCODE_THEME} config substitution.
	if info.OpenCodeTheme != "" {
		info.KeyEnv = append(info.KeyEnv, "-e", "OPENCODE_THEME="+info.OpenCodeTheme)
	}
	return nil
}

// profileValidList is the pipe-joined list of known profile ids used in error
// messages, derived from config.Profiles() so a user-defined profile is
// reported as valid too. For the default (built-ins only) store it is exactly
// "claude-pro|zai|opencode-go|opencode-zen|opencode-zen-free".
func profileValidList() string {
	ps := config.Profiles()
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	return strings.Join(ids, "|")
}

// envDefault returns the effective value of key, or def when it is unset —
// the Go form of bash's ${VAR:-default}.
func envDefault(key, def string) string {
	if v := config.EnvValue(key); v != "" {
		return v
	}
	return def
}
