// Package editor resolves which editor command to run for interactive
// (terminal) editing of a file, mirroring the resolution order used by git
// and most other CLI tools: $VISUAL, then $EDITOR, then a fallback editor
// found on $PATH.
package editor

import (
	"errors"
	"os"
	"os/exec"
)

// LookPath resolves fallback editor candidates against $PATH. It is a
// package-level var (rather than calling exec.LookPath directly) so tests
// can inject a fake instead of depending on the host's actually installed
// editors.
var LookPath = exec.LookPath

// fallbacks are tried in order, via LookPath, once neither $VISUAL nor
// $EDITOR is set.
var fallbacks = []string{"nano", "vi"}

// ErrNotFound is returned by Resolve when neither $VISUAL, $EDITOR, nor any
// fallback editor could be located.
var ErrNotFound = errors.New("no editor found: set $VISUAL or $EDITOR, or install nano or vi")

// Resolve returns the editor command to run: $VISUAL, then $EDITOR, then the
// first of the fallback editors (nano, then vi) found via LookPath. It
// returns ErrNotFound if none of those resolve.
//
// The returned string may include arguments, as $VISUAL/$EDITOR commonly do
// (e.g. "code --wait"), so it is meant to be run through a shell rather than
// exec'd directly — see Command.
func Resolve() (string, error) {
	if v := os.Getenv("VISUAL"); v != "" {
		return v, nil
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e, nil
	}
	for _, c := range fallbacks {
		if _, err := LookPath(c); err == nil {
			return c, nil
		}
	}
	return "", ErrNotFound
}

// Command builds the *exec.Cmd that opens path in the resolved editor. It
// runs the editor through "sh -c" so a $VISUAL/$EDITOR value containing
// arguments (e.g. "code --wait") still works; path is passed positionally as
// "$1" rather than interpolated into the shell string, so it needs no
// shell-quoting even if it contains spaces or special characters.
func Command(path string) (*exec.Cmd, error) {
	ed, err := Resolve()
	if err != nil {
		return nil, err
	}
	return exec.Command("sh", "-c", ed+` "$1"`, "--", path), nil
}
