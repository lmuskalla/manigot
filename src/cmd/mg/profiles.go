package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/ui"
)

// profileAuthKeys lists the .env keys a profile needs to be ready, in the order
// the old scripts checked them: claude-pro needs all four subscription keys,
// the opencode profiles need their single auth key.
var profileAuthKeys = map[string][]string{
	config.ProfileClaudePro:       {"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_ACCOUNT_UUID", "CLAUDE_EMAIL", "CLAUDE_ORG_UUID"},
	config.ProfileZAI:             {"ZHIPU_API_KEY"},
	config.ProfileOpenCodeGo:      {"OPENCODE_API_KEY"},
	config.ProfileOpenCodeZen:     {"OPENCODE_API_KEY"},
	config.ProfileOpenCodeZenFree: {"OPENCODE_API_KEY"},
}

// profileModelEnv maps the opencode profiles to the .env key that overrides
// their default model. claude-pro has no such key — the CLI's own default.
var profileModelEnv = map[string]string{
	config.ProfileZAI:             "OPENCODE_ZAI_MODEL",
	config.ProfileOpenCodeGo:      "OPENCODE_GO_MODEL",
	config.ProfileOpenCodeZen:     "OPENCODE_ZEN_MODEL",
	config.ProfileOpenCodeZenFree: "OPENCODE_ZEN_FREE_MODEL",
}

// profileModelDefaults are the model strings shown when no .env override is
// set — the same defaults profiles.sh's table carried.
var profileModelDefaults = map[string]string{
	config.ProfileClaudePro:       "(Claude Code default)",
	config.ProfileZAI:             "zai-coding-plan/glm-5.2",
	config.ProfileOpenCodeGo:      "opencode-go/glm-5.2",
	config.ProfileOpenCodeZen:     "opencode/deepseek-v4-flash",
	config.ProfileOpenCodeZenFree: "opencode/deepseek-v4-flash-free",
}

const profilesHelp = `mg profiles [name]

Lists manigot's subscription profiles — claude-pro, zai, opencode-go,
opencode-zen, opencode-zen-free — showing
which are configured and which is the default used by bare ` + "`mg`" + `. With a name,
sets that profile as the default (written as MANIGOT_PROFILE in manigot/.env);
with no name and an interactive terminal, picks the default via an interactive
picker on a TTY (type to filter, enter to choose, esc/q cancel).

The default is shared: the TUI's settings screen reads and writes the same
MANIGOT_PROFILE, so TUI-launched sessions use whatever this command sets, and
vice versa.
`

// runProfiles implements `mg profiles` — the port of scripts/profiles.sh with
// identical output wording. r is the interactive input (kept for signature
// uniformity with runAgents/runJobs even though the picker reads the terminal
// itself), stdout carries the listing/confirmations, stderr the errors. On a
// TTY the default-profile selection is the injected picker seam (pick) — the
// same one runJobs/runAgents use — so tests never start a real Bubble Tea
// program.
func runProfiles(args []string, r io.Reader, stdout, stderr io.Writer, tty bool, pick pickerRunFunc) int {
	fs := flag.NewFlagSet("mg profiles", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() { fmt.Fprint(stdout, profilesHelp) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0 // usage already printed to stdout
		}
		// An unknown flag — the script's parser had no flags at all, so any
		// single argument was a candidate profile name.
		fmt.Fprintln(stderr, flagParseError(err))
		fmt.Fprintln(stderr, "Usage: mg profiles [name]")
		return 1
	}

	rest := fs.Args()
	if len(rest) == 1 && rest[0] == "help" {
		// The script's bare-word help alias.
		fmt.Fprint(stdout, profilesHelp)
		return 0
	}
	if len(rest) > 1 {
		fmt.Fprintln(stderr, "Error: too many arguments.")
		fmt.Fprintln(stderr, "Usage: mg profiles [name]")
		return 1
	}
	if len(rest) == 1 {
		return profilesSet(rest[0], stdout, stderr)
	}
	return profilesList(r, stdout, tty, pick)
}

// profilesSet writes MANIGOT_PROFILE=<name> into .env and prints the same
// confirmation + missing-credentials warning profiles.sh's confirm_set did.
func profilesSet(name string, stdout, stderr io.Writer) int {
	switch name {
	case config.ProfileClaudePro, config.ProfileZAI, config.ProfileOpenCodeGo, config.ProfileOpenCodeZen, config.ProfileOpenCodeZenFree:
	default:
		fmt.Fprintf(stderr, "Error: unknown profile '%s'.\n", name)
		fmt.Fprintf(stderr, "Valid profiles: %s\n", strings.Join(profileIDs(), " "))
		return 1
	}
	if err := config.UpsertEnv("MANIGOT_PROFILE", name); err != nil {
		fmt.Fprintf(stderr, "mg profiles: %v\n", err)
		return 1
	}
	confirmSet(name, stdout)
	return 0
}

// confirmSet prints the shared set confirmation, warning when the profile's
// auth key is not in .env yet.
func confirmSet(name string, w io.Writer) {
	envFile := config.EnvFile()
	fmt.Fprintf(w, "Default profile set to %s (MANIGOT_PROFILE in %s).\n", name, envFile)
	fmt.Fprintln(w, "Bare `mg` sessions and TUI-launched sessions share this default.")

	if key := profileAuthKeys[name][0]; config.EnvValue(key) == "" {
		fmt.Fprintf(w, "Warning: %s is not set in %s — run 'mg setup %s' first, or sessions will fail at launch.\n", key, envFile, name)
	}
}

// profilesList prints the profile table, then offers the interactive picker
// on a TTY — same listing, and the picker rows carry the same padded columns
// (byte-padded by padBytes, matching bash printf's byte-based padding: the
// label column contains a multi-byte ·, so rune-based fmt padding would
// misalign every row by one). The picker replaces the old numbered
// `[1-N, Enter keeps X, q quits]` loop; the cursor opens on the active
// default so a bare enter keeps it. Off a TTY the listing alone is the whole
// command (exit 0 — listing is `mg profiles`' documented purpose, unlike
// jobs/agents which refuse because selection is their only purpose).
func profilesList(r io.Reader, w io.Writer, tty bool, pick pickerRunFunc) int {
	active := config.EnvValue("MANIGOT_PROFILE")
	if active == "" {
		active = config.ProfileClaudePro
	}
	fmt.Fprintf(w, "Active default: %s   (shared with the TUI; switch with: mg profiles <name>, or pick one below)\n", active)
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "  %-13s %-28s %-10s %-26s %s\n", "profile", "label", "tool", "model", "creds")
	fmt.Fprintf(w, "  %-13s %-28s %-10s %-26s %s\n", "---------", "----------------------------", "----------", "--------------------------", "-----")
	for _, p := range config.Profiles() {
		mark := " "
		if p.ID == active {
			mark = "*"
		}
		row := "  " + mark + padBytes(p.ID, 12) + " " + padBytes(p.Label, 28) + " " +
			padBytes(p.Tool, 10) + " " + padBytes(profileModel(p.ID), 26) + " " + creds(p.ID)
		fmt.Fprintln(w, row)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  * = default. Configure credentials with: mg setup [name]")

	if !tty {
		return 0
	}

	profiles := config.Profiles()
	rows := make([]ui.PickerRow, 0, len(profiles))
	start := 0
	for i, p := range profiles {
		if p.ID == active {
			start = i
		}
		model := profileModel(p.ID)
		cred := creds(p.ID)
		mark := " "
		if p.ID == active {
			mark = "*"
		}
		rows = append(rows, ui.PickerRow{
			ID:        p.ID,
			SearchKey: p.ID + " " + p.Label + " " + p.Tool + " " + model + " " + cred,
			// The same padded columns as the plain listing (minus the leading
			// indent, which the picker's own cursor/blank prefix provides),
			// keeping the * active mark.
			Label: mark + padBytes(p.ID, 12) + " " + padBytes(p.Label, 28) + " " +
				padBytes(p.Tool, 10) + " " + padBytes(model, 26) + " " + cred,
		})
	}

	chosen, ok, err := pick("Select the default profile", rows, start)
	if err != nil {
		fmt.Fprintf(w, "mg profiles: %v\n", err)
		return 1
	}
	if !ok {
		// Cancelled (esc/q) — a quiet exit 0, not the old "quit" error.
		return 0
	}
	if chosen == active {
		fmt.Fprintf(w, "Keeping %s.\n", active)
		return 0
	}
	fmt.Fprintln(w, "")
	if err := config.UpsertEnv("MANIGOT_PROFILE", chosen); err != nil {
		fmt.Fprintf(w, "mg profiles: %v\n", err)
		return 1
	}
	confirmSet(chosen, w)
	return 0
}

// profileModel returns the model column for a profile: the effective .env /
// environment override when set, else the compiled-in default.
func profileModel(id string) string {
	if env, ok := profileModelEnv[id]; ok {
		if v := config.EnvValue(env); v != "" {
			return v
		}
	}
	return profileModelDefaults[id]
}

// creds returns the creds column for a profile: "✓ ready" when every required
// key is set, else "✗ missing <first missing key>". Checks the effective value
// (process env + .env), matching the scripts' sourced-env ${!KEY:-} reads.
func creds(id string) string {
	for _, k := range profileAuthKeys[id] {
		if config.EnvValue(k) == "" {
			return "✗ missing " + k
		}
	}
	return "✓ ready"
}

// profileIDs returns the ordered profile ids for error messages.
func profileIDs() []string {
	ps := config.Profiles()
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	return ids
}

// padBytes returns s padded with trailing spaces to the given byte width —
// bash printf pads %s by byte length, and the profile table's label column
// contains a multi-byte ·, so fmt's rune-based width would misalign.
func padBytes(s string, width int) string {
	if n := len(s); n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
