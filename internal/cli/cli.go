// Package cli provides the interactive prompt primitives the ported mg
// subcommands share, replacing the bash `read -rp` flows of the old scripts
// with testable Go equivalents.
//
// Every prompt takes an io.Reader for input and an io.Writer for output, so
// tests can drive them with a strings.Reader and assert on a strings.Builder —
// and the exact wording of every prompt and error message matches the script it
// replaces, 1:1 (that wording is the CLI's user-visible contract).
//
// A command that asks several prompts in sequence (the setup wizard, a menu)
// should wrap its input in ONE bufio.Reader and pass that down: readLine reuses
// a *bufio.Reader it is given, but a fresh wrap per prompt would lose whatever
// the previous bufio.Reader buffered past its newline.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// ErrQuit is returned by Select when the user answers q/quit/exit, matching
// the profiles.sh interactive loop's quit path.
var ErrQuit = errors.New("quit")

// IsTerminal reports whether f refers to a terminal (the Go form of bash's
// `[[ -t 0 ]]`). Callers use it to decide whether interactive prompting is
// possible at all — the scripts refuse to prompt (setup) or refuse to pick
// (agents) when stdin is not a TTY.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// Confirm writes prompt to w and reads a single line from r. It reports true
// for y/Y/yes/YES and false for anything else (including empty input and EOF),
// matching the scripts' `[y/N]` default-no confirmations.
func Confirm(prompt string, r io.Reader, w io.Writer) (bool, error) {
	fmt.Fprint(w, prompt)
	line, err := readLine(r)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	switch strings.ToLower(line) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// Mask renders a short, safe display form of a secret, identical to setup.sh's
// mask(): "****" for values of at most 8 characters, else first 4 + "…" + last
// 4.
func Mask(v string) string {
	if len(v) <= 8 {
		return "****"
	}
	return v[:4] + "…" + v[len(v)-4:]
}

// PromptSecret asks for a secret value. When current is non-empty the prompt
// shows its masked form and Enter keeps it; otherwise the prompt is bare and
// empty input means "skip". It returns the value the user typed ("" = keep or
// skip — the caller decides, matching setup.sh's set_env_var refusing empty).
func PromptSecret(label, current string, r io.Reader, w io.Writer) (string, error) {
	if current != "" {
		fmt.Fprintf(w, "  %s [currently %s — Enter keeps it]: ", label, Mask(current))
	} else {
		fmt.Fprintf(w, "  %s: ", label)
	}
	line, err := readLine(r)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return line, nil
}

// PromptValue asks for a non-secret value with a shown default. It returns the
// value to write: what the user typed when non-empty, else current when
// non-empty, else def — mirroring setup.sh's prompt_value (where an empty
// result means nothing is written).
func PromptValue(label, current, def string, r io.Reader, w io.Writer) (string, error) {
	shown := current
	if shown == "" {
		shown = def
	}
	if shown == "" {
		shown = "empty"
	}
	fmt.Fprintf(w, "  %s [%s] — Enter keeps it: ", label, shown)
	line, err := readLine(r)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if line != "" {
		return line, nil
	}
	if current != "" {
		return current, nil
	}
	return def, nil
}

// Select renders a numbered menu (1..n) and loops until a valid selection.
//
// Empty input is accepted (returning 0, nil) when allowEmpty is true — the
// "Enter keeps the current value" path of profiles.sh; when allowEmpty is false
// (agents.sh) empty input re-prompts. q/quit/exit returns ErrQuit. Anything
// else re-prompts with the same error wording the scripts use.
func Select(prompt string, n int, allowEmpty bool, r io.Reader, w io.Writer) (int, error) {
	// One reader for the whole loop — wrapping afresh each round would lose
	// whatever the previous bufio.Reader buffered past the newline.
	br := bufio.NewReader(r)
	for {
		fmt.Fprint(w, prompt)
		line, err := readLine(br)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if allowEmpty {
					return 0, nil
				}
				return 0, fmt.Errorf("unexpected end of input while selecting")
			}
			return 0, err
		}
		if line == "" {
			if allowEmpty {
				return 0, nil
			}
			fmt.Fprintf(w, "Enter a number between 1 and %d.\n", n)
			continue
		}
		switch strings.ToLower(line) {
		case "q", "quit", "exit":
			return 0, ErrQuit
		}
		var choice int
		if _, err := fmt.Sscanf(line, "%d", &choice); err == nil && choice >= 1 && choice <= n {
			return choice, nil
		}
		if allowEmpty {
			fmt.Fprintf(w, "Enter a number between 1 and %d, or q to quit.\n", n)
		} else {
			fmt.Fprintf(w, "Enter a number between 1 and %d.\n", n)
		}
	}
}

// readLine reads one line from r, trimming the trailing newline (and any
// carriage return) and surrounding whitespace, matching `read -rp` semantics.
// It returns io.EOF alongside whatever partial input was available.
func readLine(r io.Reader) (string, error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	line, err := br.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	line = strings.TrimSpace(line)
	if err != nil {
		return line, err
	}
	return line, nil
}
