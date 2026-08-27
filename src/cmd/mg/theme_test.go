package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/ui"
)

// themeCheckout builds a minimal fake manigot checkout with an optional .env,
// points $MANIGOT_HOME at it, and clears OPENCODE_THEME from the process
// environment so the tests are hermetic regardless of the host's env.
func themeCheckout(t *testing.T, env string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if env != "" {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("OPENCODE_THEME", "")
	t.Setenv("MANIGOT_HOME", dir)
	return dir
}

func TestThemeHelp(t *testing.T) {
	var out, errOut strings.Builder
	code := runTheme([]string{"--help"}, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
	if code != 0 {
		t.Errorf("help exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "mg theme [name]") {
		t.Errorf("help output missing title:\n%s", out.String())
	}
}

func TestThemeSetKnown(t *testing.T) {
	dir := themeCheckout(t, "# header\nMANIGOT_PROFILE=zai\n")
	var out, errOut strings.Builder
	code := runTheme([]string{"nord"}, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, errOut.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "OPENCODE_THEME=nord") {
		t.Errorf(".env missing OPENCODE_THEME=nord:\n%s", env)
	}
	if !strings.Contains(string(env), "MANIGOT_PROFILE=zai") {
		t.Errorf(".env lost an existing setting:\n%s", env)
	}
	if !strings.Contains(out.String(), "Theme set to nord") {
		t.Errorf("missing set confirmation:\n%s", out.String())
	}
	if strings.Contains(out.String(), "Note:") {
		t.Errorf("a known theme should not print the not-in-reference-list note:\n%s", out.String())
	}
}

// TestThemeSetUnknownAccepted — unlike `mg profiles`, an unrecognized name is
// still accepted (OpenCode may ship themes manigot doesn't know about yet).
func TestThemeSetUnknownAccepted(t *testing.T) {
	dir := themeCheckout(t, "")
	var out, errOut strings.Builder
	code := runTheme([]string{"my-custom-theme"}, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, errOut.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "OPENCODE_THEME=my-custom-theme") {
		t.Errorf(".env missing OPENCODE_THEME=my-custom-theme:\n%s", env)
	}
	if !strings.Contains(out.String(), "Theme set to my-custom-theme") {
		t.Errorf("missing set confirmation:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Note:") {
		t.Errorf("an unknown theme should print the not-in-reference-list note:\n%s", out.String())
	}
}

func TestThemeTooManyArgs(t *testing.T) {
	var out, errOut strings.Builder
	code := runTheme([]string{"nord", "extra"}, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: too many arguments.") {
		t.Errorf("missing too-many-arguments error:\n%s", errOut.String())
	}
}

func TestThemeListNonTTYUnset(t *testing.T) {
	themeCheckout(t, "")
	var out strings.Builder
	code := runTheme([]string{}, strings.NewReader(""), &out, &strings.Builder{}, false, pickerStub(t))
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got := out.String()
	for _, want := range []string{
		"Active theme: (unset",
		"nord",
		"tokyonight",
		"opencode",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("listing missing %q:\n%s", want, got)
		}
	}
}

func TestThemeListNonTTYActive(t *testing.T) {
	themeCheckout(t, "OPENCODE_THEME=nord\n")
	var out strings.Builder
	code := runTheme([]string{}, strings.NewReader(""), &out, &strings.Builder{}, false, pickerStub(t))
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "Active theme: nord") {
		t.Errorf("listing missing active theme:\n%s", got)
	}
	if !strings.Contains(got, "*nord") {
		t.Errorf("listing missing active mark on nord's row:\n%s", got)
	}
}

// TestThemeListInteractiveSelect covers the TTY submit path with an injected
// picker: choosing a theme writes OPENCODE_THEME and prints the confirmation.
func TestThemeListInteractiveSelect(t *testing.T) {
	dir := themeCheckout(t, "# header\n")
	var out strings.Builder
	code := runTheme([]string{}, strings.NewReader(""), &out, &strings.Builder{}, true, pickerChoice("nord", true))
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "OPENCODE_THEME=nord") {
		t.Errorf(".env missing OPENCODE_THEME=nord:\n%s", env)
	}
	if !strings.Contains(out.String(), "Theme set to nord") {
		t.Errorf("missing set confirmation:\n%s", out.String())
	}
}

// TestThemeListInteractiveEnterKeeps covers the "enter keeps the current
// theme" affordance: choosing the already-active theme prints "Keeping X."
// instead of re-writing .env.
func TestThemeListInteractiveEnterKeeps(t *testing.T) {
	themeCheckout(t, "OPENCODE_THEME=nord\n")
	var out strings.Builder
	code := runTheme([]string{}, strings.NewReader(""), &out, &strings.Builder{}, true, pickerChoice("nord", true))
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "Keeping nord.") {
		t.Errorf("missing keeping message:\n%s", out.String())
	}
}

// TestThemeListInteractiveCancel covers the cancelled-picker path: esc/q
// exits 0 quietly.
func TestThemeListInteractiveCancel(t *testing.T) {
	themeCheckout(t, "")
	var out strings.Builder
	code := runTheme([]string{}, strings.NewReader(""), &out, &strings.Builder{}, true, pickerChoice("", false))
	if code != 0 {
		t.Errorf("cancel exit code = %d, want 0", code)
	}
}

// TestThemePickerGetsThemeRows pins the picker wiring: on a TTY the picker is
// fed the title plus one pre-rendered row per known theme (name, padded
// label with the * active mark, search key name+description), and the cursor
// start index is the active theme's position.
func TestThemePickerGetsThemeRows(t *testing.T) {
	themeCheckout(t, "OPENCODE_THEME=nord\n")

	var gotTitle string
	var gotRows []ui.PickerRow
	var gotStart int
	pick := func(title string, rows []ui.PickerRow, start int) (string, bool, error) {
		gotTitle, gotRows, gotStart = title, rows, start
		return "", false, nil // cancelled
	}
	var out strings.Builder
	code := runTheme([]string{}, strings.NewReader(""), &out, &strings.Builder{}, true, pick)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (cancel exits quietly)", code)
	}
	if gotTitle != "Select the OpenCode theme" {
		t.Errorf("picker title = %q, want %q", gotTitle, "Select the OpenCode theme")
	}
	if len(gotRows) != len(knownThemes) {
		t.Fatalf("picker rows = %d, want %d", len(gotRows), len(knownThemes))
	}
	nordIdx := -1
	for i, th := range knownThemes {
		if th.Name == "nord" {
			nordIdx = i
		}
		if gotRows[i].ID != th.Name {
			t.Errorf("row %d ID = %q, want %q", i, gotRows[i].ID, th.Name)
		}
		if !strings.Contains(gotRows[i].SearchKey, th.Name) || !strings.Contains(gotRows[i].SearchKey, th.Description) {
			t.Errorf("row %d search key missing name/description: %q", i, gotRows[i].SearchKey)
		}
	}
	if gotStart != nordIdx {
		t.Errorf("start index = %d, want %d (nord is active)", gotStart, nordIdx)
	}
	if !strings.HasPrefix(gotRows[nordIdx].Label, "*nord") {
		t.Errorf("active row label missing the * mark: %q", gotRows[nordIdx].Label)
	}
}

func TestKnownThemeName(t *testing.T) {
	if !knownThemeName("nord") {
		t.Error("nord should be a known theme")
	}
	if knownThemeName("my-custom-theme") {
		t.Error("my-custom-theme should not be a known theme")
	}
}
