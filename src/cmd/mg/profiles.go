package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/lmuskalla/manigot/internal/cli"
	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/ui"
)

const profilesHelp = `mg profiles [name|add|rm]

Lists manigot's subscription profiles — the built-ins claude-pro, zai,
opencode-go, opencode-zen, opencode-zen-free plus any you've added — showing
which are configured and which is the default used by bare ` + "`mg`" + `. With a name,
sets that profile as the default (written as MANIGOT_PROFILE in manigot/.env);
with no name and an interactive terminal, picks the default via an interactive
picker on a TTY (type to filter, enter to choose, esc/q cancel).
sets that profile as the default (written as MANIGOT_PROFILE in manigot/.env);
with no name and an interactive terminal, picks the default via an interactive
picker on a TTY (type to filter, enter to choose, esc/q cancel).

  mg profiles add [<id>]   interactively create a user-defined profile
                           (e.g. OpenCode Zen with a different model),
                           stored in config/profiles.json
  mg profiles rm <id>      remove a user-defined profile; removing the current
                           default falls back to claude-pro (built-ins cannot
                           be removed)

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
	if len(rest) == 0 {
		return profilesList(r, stdout, tty, pick)
	}
	switch rest[0] {
	case "add":
		return profilesAdd(rest[1:], r, stdout, stderr, tty)
	case "rm", "remove":
		return profilesRm(rest[1:], stdout, stderr)
	}
	if len(rest) > 1 {
		fmt.Fprintln(stderr, "Error: too many arguments.")
		fmt.Fprintln(stderr, "Usage: mg profiles [name|add|rm]")
		return 1
	}
	return profilesSet(rest[0], stdout, stderr)
}

// profilesAdd implements `mg profiles add [<id>]` — the interactive
// user-defined-profile creation wizard. On a TTY it walks the fields of a
// profile (tool, credential key(s), model env/default for opencode) with the
// built-ins' values offered as defaults, then persists the profile to
// config/profiles.json via config.AddProfile. Off a TTY it refuses — profile
// creation is inherently interactive, matching `mg setup`'s stance.
func profilesAdd(args []string, r io.Reader, stdout, stderr io.Writer, tty bool) int {
	if !tty {
		fmt.Fprintln(stderr, "mg profiles add: interactive profile creation needs a terminal.")
		fmt.Fprintln(stderr, "Define the profile in config/profiles.json instead, or run inside a terminal.")
		return 1
	}
	if len(args) > 1 {
		fmt.Fprintln(stderr, "Error: mg profiles add takes at most a single profile id.")
		return 1
	}

	// One buffered reader for the whole wizard — a fresh bufio.Reader per
	// prompt would lose whatever the previous one buffered past its newline.
	br := bufio.NewReader(r)

	id := ""
	if len(args) == 1 {
		id = args[0]
	}
	if strings.TrimSpace(id) == "" {
		v, err := promptLine("  Profile id: ", br, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "mg profiles add: %v\n", err)
			return 1
		}
		id = v
	}
	if strings.TrimSpace(id) == "" {
		fmt.Fprintln(stderr, "Error: a profile id is required.")
		return 1
	}
	if _, ok := config.ProfileByID(id); ok {
		fmt.Fprintf(stderr, "Error: a profile with id %q already exists.\n", id)
		return 1
	}

	tool, _ := cli.PromptValue("Tool (claude-code|opencode)", "", config.ToolOpenCode, br, stdout)
	if tool != config.ToolClaudeCode && tool != config.ToolOpenCode {
		fmt.Fprintf(stderr, "Error: tool must be 'claude-code' or 'opencode' (got '%s').\n", tool)
		return 1
	}
	label, _ := cli.PromptValue("Label", "", id, br, stdout)
	if label == "" {
		label = id
	}

	defKey := "OPENCODE_API_KEY"
	if tool == config.ToolClaudeCode {
		defKey = "CLAUDE_CODE_OAUTH_TOKEN"
	}
	authKeysRaw, _ := cli.PromptValue("Credential key(s), comma-separated", "", defKey, br, stdout)
	var authKeys []string
	for _, k := range strings.Split(authKeysRaw, ",") {
		if k = strings.TrimSpace(k); k != "" {
			authKeys = append(authKeys, k)
		}
	}
	if len(authKeys) == 0 {
		fmt.Fprintln(stderr, "Error: at least one credential key is required.")
		return 1
	}
	billed, _ := cli.PromptValue("Billed via (description)", "", authKeys[0], br, stdout)
	if billed == "" {
		billed = authKeys[0]
	}

	p := config.Profile{
		ID:       id,
		Label:    label,
		Tool:     tool,
		Auth:     billed,
		AuthKeys: authKeys,
	}
	if tool == config.ToolOpenCode {
		modelEnv, _ := cli.PromptValue("Model .env key", "", "OPENCODE_MODEL", br, stdout)
		modelDefault, _ := cli.PromptValue("Default model", "", "", br, stdout)
		if strings.TrimSpace(modelDefault) == "" {
			fmt.Fprintln(stderr, "Error: an opencode profile needs a default model.")
			return 1
		}
		p.ModelEnv = modelEnv
		p.ModelDefault = modelDefault
	}

	if err := config.AddProfile(p); err != nil {
		fmt.Fprintf(stderr, "mg profiles add: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Added profile '%s' (%s).\n", p.ID, p.Label)
	fmt.Fprintf(stdout, "Configure its credentials with: mg setup %s\n", p.ID)
	fmt.Fprintf(stdout, "Make it the default with: mg profiles %s\n", p.ID)
	return 0
}

// profilesRm implements `mg profiles rm <id>` — removes a user-defined
// profile from config/profiles.json. Built-in profiles are not deletable
// (they are the defaults every user starts from), so rm only ever touches the
// user store. Removing the currently-set MANIGOT_PROFILE default falls back
// to claude-pro so bare `mg` keeps a valid default.
func profilesRm(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: mg profiles rm <id>")
		return 1
	}
	id := args[0]
	if _, ok := config.ProfileByID(id); !ok {
		fmt.Fprintf(stderr, "Error: unknown profile '%s'.\n", id)
		return 1
	}
	if err := config.RemoveProfile(id); err != nil {
		// Covers the built-in profiles (not present in the user store) and
		// any corrupt-store/unresolvable-home write failure.
		fmt.Fprintf(stderr, "mg profiles rm: %v\n", err)
		return 1
	}
	if config.EnvValue("MANIGOT_PROFILE") == id {
		if err := config.UpsertEnv("MANIGOT_PROFILE", config.ProfileClaudePro); err != nil {
			fmt.Fprintf(stderr, "mg profiles rm: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Removed profile '%s'. The default fell back to %s.\n", id, config.ProfileClaudePro)
		return 0
	}
	fmt.Fprintf(stdout, "Removed profile '%s'.\n", id)
	return 0
}

// promptLine prints label and reads one trimmed line from br (no default
// shown) — for the add wizard's required, default-less fields like the profile
// id. On EOF it returns whatever was read, like cli's readLine.
func promptLine(label string, br *bufio.Reader, w io.Writer) (string, error) {
	fmt.Fprint(w, label)
	line, err := br.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// profilesSet writes MANIGOT_PROFILE=<name> into .env and prints the same
// confirmation + missing-credentials warning profiles.sh's confirm_set did.
func profilesSet(name string, stdout, stderr io.Writer) int {
	if _, ok := config.ProfileByID(name); !ok {
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

	if p, ok := config.ProfileByID(name); ok && len(p.AuthKeys) > 0 {
		if key := p.AuthKeys[0]; config.EnvValue(key) == "" {
			fmt.Fprintf(w, "Warning: %s is not set in %s — run 'mg setup %s' first, or sessions will fail at launch.\n", key, envFile, name)
		}
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
// environment override when set, else the compiled-in default. Both come from
// the profile's definition (config.ProfileByID), so a user-defined profile
// renders its own model env/default.
func profileModel(id string) string {
	p, ok := config.ProfileByID(id)
	if !ok {
		return ""
	}
	if p.ModelEnv != "" {
		if v := config.EnvValue(p.ModelEnv); v != "" {
			return v
		}
	}
	return p.ModelDefault
}

// creds returns the creds column for a profile: "✓ ready" when every required
// key is set, else "✗ missing <first missing key>". The required keys are the
// profile's AuthKeys. Checks the effective value (process env + .env),
// matching the scripts' sourced-env ${!KEY:-} reads.
func creds(id string) string {
	p, ok := config.ProfileByID(id)
	if !ok {
		// An unknown id has no keys, so the old loop ran zero times and
		// reported "ready" — preserved for identical output.
		return "✓ ready"
	}
	for _, k := range p.AuthKeys {
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
