package cli

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestConfirmAcceptsYesAnswers(t *testing.T) {
	for _, in := range []string{"y", "Y", "yes", "YES", "y\n"} {
		var w strings.Builder
		got, err := Confirm("Continue? [y/N] ", strings.NewReader(in), &w)
		if err != nil {
			t.Fatalf("Confirm(%q): %v", in, err)
		}
		if !got {
			t.Errorf("Confirm(%q) = false, want true", in)
		}
		if w.String() != "Continue? [y/N] " {
			t.Errorf("prompt written = %q, want %q", w.String(), "Continue? [y/N] ")
		}
	}
}

func TestConfirmDefaultsNo(t *testing.T) {
	for _, in := range []string{"", "\n", "n", "N", "no", "maybe"} {
		got, err := Confirm("Continue? [y/N] ", strings.NewReader(in), &strings.Builder{})
		if err != nil {
			t.Fatalf("Confirm(%q): %v", in, err)
		}
		if got {
			t.Errorf("Confirm(%q) = true, want false", in)
		}
	}
}

func TestConfirmEOFIsNo(t *testing.T) {
	got, err := Confirm("Continue? [y/N] ", strings.NewReader(""), &strings.Builder{})
	if err != nil {
		t.Fatalf("Confirm on empty reader: %v", err)
	}
	if got {
		t.Error("Confirm on EOF = true, want false")
	}
}

func TestMask(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "****"},
		{"short", "****"},
		{"12345678", "****"},
		{"sk-ant-oat01-abcdefghijkl", "sk-a…ijkl"},
	}
	for _, tc := range cases {
		if got := Mask(tc.in); got != tc.want {
			t.Errorf("Mask(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPromptSecretShowsMaskWhenCurrent(t *testing.T) {
	var w strings.Builder
	got, err := PromptSecret("  CLAUDE_CODE_OAUTH_TOKEN", "sk-ant-oat01-abcdef", strings.NewReader("\n"), &w)
	if err != nil {
		t.Fatalf("PromptSecret: %v", err)
	}
	if got != "" {
		t.Errorf("empty Enter = %q, want empty (keep)", got)
	}
	// The label carries the leading indent, so it appears doubled — matching
	// setup.sh's `  $label [...]` formatting exactly.
	if want := "    CLAUDE_CODE_OAUTH_TOKEN [currently sk-a…cdef — Enter keeps it]: "; w.String() != want {
		t.Errorf("prompt = %q, want %q", w.String(), want)
	}
}

func TestPromptSecretBareWhenNoCurrent(t *testing.T) {
	var w strings.Builder
	got, err := PromptSecret("  ZHIPU_API_KEY", "", strings.NewReader("sk-zhipu-123\n"), &w)
	if err != nil {
		t.Fatalf("PromptSecret: %v", err)
	}
	if got != "sk-zhipu-123" {
		t.Errorf("typed value = %q, want %q", got, "sk-zhipu-123")
	}
	if want := "    ZHIPU_API_KEY: "; w.String() != want {
		t.Errorf("prompt = %q, want %q", w.String(), want)
	}
}

func TestPromptValueTypedWins(t *testing.T) {
	var w strings.Builder
	got, err := PromptValue("  OPENCODE_ZAI_MODEL", "zai-coding-plan/old", "zai-coding-plan/glm-5.2", strings.NewReader("zai-coding-plan/new\n"), &w)
	if err != nil {
		t.Fatalf("PromptValue: %v", err)
	}
	if got != "zai-coding-plan/new" {
		t.Errorf("typed value = %q, want the typed value", got)
	}
	if want := "    OPENCODE_ZAI_MODEL [zai-coding-plan/old] — Enter keeps it: "; w.String() != want {
		t.Errorf("prompt = %q, want %q", w.String(), want)
	}
}

func TestPromptValueEnterKeepsCurrent(t *testing.T) {
	got, err := PromptValue("  OPENCODE_ZAI_MODEL", "zai-coding-plan/current", "zai-coding-plan/glm-5.2", strings.NewReader("\n"), &strings.Builder{})
	if err != nil {
		t.Fatalf("PromptValue: %v", err)
	}
	if got != "zai-coding-plan/current" {
		t.Errorf("Enter = %q, want current kept", got)
	}
}

func TestPromptValueEnterAppliesDefaultWhenNoCurrent(t *testing.T) {
	got, err := PromptValue("  OPENCODE_GO_MODEL", "", "opencode-go/glm-5.2", strings.NewReader("\n"), &strings.Builder{})
	if err != nil {
		t.Fatalf("PromptValue: %v", err)
	}
	if got != "opencode-go/glm-5.2" {
		t.Errorf("Enter with no current = %q, want default", got)
	}
}

func TestPromptValueEmptyShown(t *testing.T) {
	var w strings.Builder
	_, err := PromptValue("  OPENCODE_GO_MODEL", "", "", strings.NewReader("\n"), &w)
	if err != nil {
		t.Fatalf("PromptValue: %v", err)
	}
	// The label already carries the leading indent, so it appears doubled —
	// matching setup.sh's `  $label [...]` formatting exactly.
	if want := "    OPENCODE_GO_MODEL [empty] — Enter keeps it: "; w.String() != want {
		t.Errorf("prompt = %q, want %q", w.String(), want)
	}
}

func TestSelectValidNumber(t *testing.T) {
	got, err := Select("Select an agent [1-3]: ", 3, false, strings.NewReader("2\n"), &strings.Builder{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got != 2 {
		t.Errorf("Select = %d, want 2", got)
	}
}

func TestSelectRepromptsOnInvalidThenAccepts(t *testing.T) {
	var w strings.Builder
	got, err := Select("Select an agent [1-3]: ", 3, false, strings.NewReader("9\nabc\n3\n"), &w)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got != 3 {
		t.Errorf("Select = %d, want 3", got)
	}
	// The prompt is printed once per iteration, like bash's read -rp loop.
	want := "Select an agent [1-3]: Enter a number between 1 and 3.\n" +
		"Select an agent [1-3]: Enter a number between 1 and 3.\n" +
		"Select an agent [1-3]: "
	if w.String() != want {
		t.Errorf("output = %q, want %q", w.String(), want)
	}
}

func TestSelectEmptyRepromptsWhenNotAllowed(t *testing.T) {
	_, err := Select("Select an agent [1-3]: ", 3, false, strings.NewReader("\n1\n"), &strings.Builder{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
}

func TestSelectEmptyAllowedReturnsZero(t *testing.T) {
	got, err := Select("  [1-3, Enter keeps zai, q quits]: ", 3, true, strings.NewReader("\n"), &strings.Builder{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got != 0 {
		t.Errorf("Select empty = %d, want 0 (keep)", got)
	}
}

func TestSelectQuitReturnsErrQuit(t *testing.T) {
	for _, in := range []string{"q\n", "Q\n", "quit\n", "exit\n"} {
		_, err := Select("  [1-3, Enter keeps zai, q quits]: ", 3, true, strings.NewReader(in), &strings.Builder{})
		if !errors.Is(err, ErrQuit) {
			t.Errorf("Select(%q) error = %v, want ErrQuit", in, err)
		}
	}
}

func TestSelectQuitWordingWhenAllowed(t *testing.T) {
	var w strings.Builder
	got, err := Select("  [1-3, Enter keeps zai, q quits]: ", 3, true, strings.NewReader("9\n2\n"), &w)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got != 2 {
		t.Errorf("Select = %d, want 2", got)
	}
	// Prompt prints per iteration; the invalid answer gets the ", or q to quit."
	// wording that profiles.sh uses.
	want := "  [1-3, Enter keeps zai, q quits]: Enter a number between 1 and 3, or q to quit.\n" +
		"  [1-3, Enter keeps zai, q quits]: "
	if w.String() != want {
		t.Errorf("output = %q, want %q", w.String(), want)
	}
}

func TestSelectEOFWhenEmptyNotAllowedErrors(t *testing.T) {
	_, err := Select("Select an agent [1-3]: ", 3, false, strings.NewReader(""), &strings.Builder{})
	if err == nil {
		t.Fatal("expected an error on EOF when empty is not allowed")
	}
	if errors.Is(err, io.EOF) {
		t.Errorf("error should wrap a descriptive message, not raw EOF: %v", err)
	}
}
