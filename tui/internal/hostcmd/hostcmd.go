// Package hostcmd runs the existing safecode host commands (sc-job, etc.)
// rather than reimplementing them, so the TUI stays in step with the scripts.
//
// Commands are located through the resolve package, never by a hardcoded name,
// so an install under a short or legacy name — or none at all — still works.
package hostcmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/lmuskalla/safecode/tui/internal/resolve"
)

// NewJob runs the host job-creation command — `<cmd> "<title>" [--type <type>]`,
// see scripts/new-job.sh — reusing it verbatim. The command is resolved at call
// time (see resolve.Job), and invoked by absolute path. It executes with cwd and
// $PWD both set to projectRoot, because the script resolves the project root
// from $PWD via its find_project_root helper.
//
// Returns the combined stdout/stderr of the command (the script's job summary)
// so the caller can surface it in a status line.
func NewJob(title, jobType, projectRoot string) (string, error) {
	found, err := resolve.Resolve(resolve.Job())
	if err != nil {
		return "", err
	}

	args := []string{title}
	if jobType != "" {
		args = append(args, "--type", jobType)
	}

	cmd := exec.Command(found.Path, args...)
	cmd.Dir = projectRoot
	// exec.Cmd sets cwd via chdir but does not update the PWD env var; the
	// script reads $PWD, so set it explicitly to match.
	cmd.Env = append(os.Environ(), "PWD="+projectRoot)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s failed: %w", found.Path, err)
	}
	return string(out), nil
}
