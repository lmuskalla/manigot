package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/lmuskalla/manigot/internal/config"
	"github.com/lmuskalla/manigot/internal/ui"
)

// knownTheme names one of OpenCode's built-in themes, for `mg theme`'s
// reference listing and picker rows only — never for validating a set (an
// unrecognized name is still accepted, see runTheme).
type knownTheme struct {
	Name        string
	Description string
}

// knownThemes is the reference list of OpenCode's built-in themes, verified
// against https://opencode.ai/docs/themes/ ("Built-in themes" table) at
// implementation time. OpenCode "constantly adds new themes" per its own
// docs, so this list is informational only — it is never used to reject a
// name passed to `mg theme <name>`, since manigot would otherwise go stale
// the moment OpenCode ships a new one.
var knownThemes = []knownTheme{
	{Name: "opencode", Description: "OpenCode's own default theme"},
	{Name: "system", Description: "Adapts to your terminal's background color"},
	{Name: "tokyonight", Description: "Based on the Tokyonight theme"},
	{Name: "everforest", Description: "Based on the Everforest theme"},
	{Name: "ayu", Description: "Based on the Ayu dark theme"},
	{Name: "catppuccin", Description: "Based on the Catppuccin theme"},
	{Name: "catppuccin-macchiato", Description: "Based on the Catppuccin theme"},
	{Name: "gruvbox", Description: "Based on the Gruvbox theme"},
	{Name: "kanagawa", Description: "Based on the Kanagawa theme"},
	{Name: "nord", Description: "Based on the Nord theme"},
	{Name: "matrix", Description: "Hacker-style green on black theme"},
	{Name: "one-dark", Description: "Based on the Atom One Dark theme"},
}

const themeHelp = `mg theme [name]

Shows or sets manigot's global OpenCode theme (` + "`OPENCODE_THEME`" + ` in manigot/.env) —
one value shared across every OpenCode profile (zai, opencode-go, opencode-zen,
opencode-zen-free), forwarded into the container and applied via OpenCode's
tui.json. Claude Code already respects your terminal's own theme, so this has
no effect there.

With no name, lists the current value plus a reference list of OpenCode's
known built-in themes (https://opencode.ai/docs/themes/); on an interactive
terminal it also offers a picker (type to filter, enter to choose, esc/q
cancel). With a name, sets it — any name is accepted, even one not in the
reference list (OpenCode may ship themes manigot doesn't know about yet;
OpenCode itself rejects an invalid name at runtime).
`

// runTheme implements `mg theme` — the port of `mg profiles`' shape to the
// global theme setting. r is the interactive input (kept for signature
// uniformity with runProfiles/runAgents/runJobs), stdout carries the
// listing/confirmations, stderr the errors.
func runTheme(args []string, r io.Reader, stdout, stderr io.Writer, tty bool, pick pickerRunFunc) int {
	fs := flag.NewFlagSet("mg theme", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() { fmt.Fprint(stdout, themeHelp) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0 // usage already printed to stdout
		}
		fmt.Fprintln(stderr, flagParseError(err))
		fmt.Fprintln(stderr, "Usage: mg theme [name]")
		return 1
	}

	rest := fs.Args()
	if len(rest) == 1 && rest[0] == "help" {
		fmt.Fprint(stdout, themeHelp)
		return 0
	}
	if len(rest) > 1 {
		fmt.Fprintln(stderr, "Error: too many arguments.")
		fmt.Fprintln(stderr, "Usage: mg theme [name]")
		return 1
	}
	if len(rest) == 1 {
		return themeSet(rest[0], stdout, stderr)
	}
	return themeList(r, stdout, tty, pick)
}

// themeSet writes OPENCODE_THEME=<name> into .env and prints a confirmation,
// matching profilesSet/confirmSet's style. Unlike a profile id, name is never
// validated — OpenCode's own theme list may have grown since knownThemes was
// last verified, and OpenCode itself rejects an invalid name at runtime.
func themeSet(name string, stdout, stderr io.Writer) int {
	if err := config.UpsertEnv("OPENCODE_THEME", name); err != nil {
		fmt.Fprintf(stderr, "mg theme: %v\n", err)
		return 1
	}
	envFile := config.EnvFile()
	fmt.Fprintf(stdout, "Theme set to %s (OPENCODE_THEME in %s).\n", name, envFile)
	fmt.Fprintln(stdout, "OpenCode sessions (zai, opencode-go, opencode-zen, opencode-zen-free) share this default.")
	if !knownThemeName(name) {
		fmt.Fprintf(stdout, "Note: %q is not in mg theme's reference list — OpenCode will reject it at launch if it isn't a real theme name.\n", name)
	}
	return 0
}

// themeList prints the current theme plus the reference table, then offers
// the interactive picker on a TTY — same shape as profilesList. Off a TTY the
// listing alone is the whole command (exit 0).
func themeList(r io.Reader, w io.Writer, tty bool, pick pickerRunFunc) int {
	active := config.EnvValue("OPENCODE_THEME")
	if active == "" {
		fmt.Fprintln(w, "Active theme: (unset — OpenCode uses its own default/config)")
	} else {
		fmt.Fprintf(w, "Active theme: %s\n", active)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "  %-22s %s\n", "theme", "description")
	fmt.Fprintf(w, "  %-22s %s\n", "----------------------", "-----------")
	for _, th := range knownThemes {
		mark := " "
		if th.Name == active {
			mark = "*"
		}
		fmt.Fprintf(w, "  %s%-22s %s\n", mark, th.Name, th.Description)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  * = active. OpenCode may ship themes not listed here — set with: mg theme <name>")

	if !tty {
		return 0
	}

	rows := make([]ui.PickerRow, 0, len(knownThemes))
	start := 0
	for i, th := range knownThemes {
		if th.Name == active {
			start = i
		}
		mark := " "
		if th.Name == active {
			mark = "*"
		}
		rows = append(rows, ui.PickerRow{
			ID:        th.Name,
			SearchKey: th.Name + " " + th.Description,
			Label:     mark + padBytes(th.Name, 22) + " " + th.Description,
		})
	}

	chosen, ok, err := pick("Select the OpenCode theme", rows, start)
	if err != nil {
		fmt.Fprintf(w, "mg theme: %v\n", err)
		return 1
	}
	if !ok {
		return 0 // cancelled — quiet exit 0, matching mg profiles
	}
	if chosen == active {
		fmt.Fprintf(w, "Keeping %s.\n", active)
		return 0
	}
	fmt.Fprintln(w, "")
	return themeSet(chosen, w, w)
}

// knownThemeName reports whether name matches one of knownThemes — used only
// to decide whether to print the "not in the reference list" note, never to
// reject a set.
func knownThemeName(name string) bool {
	for _, th := range knownThemes {
		if th.Name == name {
			return true
		}
	}
	return false
}
