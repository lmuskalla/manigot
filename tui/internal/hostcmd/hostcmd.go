// Package hostcmd runs the existing manigot host commands (mg-job, etc.)
// rather than reimplementing them, so the TUI stays in step with the scripts.
//
// Commands are located through the resolve package, never by a hardcoded name,
// so an install under a short or legacy name — or none at all — still works.
package hostcmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/lmuskalla/manigot/tui/internal/resolve"
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

// DoneCommand resolves the host job-completion command — `<cmd> "<jobName>"`,
// see scripts/finish-job.sh — and returns the *exec.Cmd ready to run, but does
// not run it: unlike NewJob, finish-job.sh's several `read -rp` confirmations
// need a real interactive terminal, so the caller runs this in the foreground
// (tea.ExecProcess) rather than capturing its output.
//
// jobName is the job directory's exact name (e.g. "8g06st_launch-agents"),
// not an ID prefix, for an unambiguous match against scripts/finish-job.sh's
// own "exact match first" resolution.
//
// The command is resolved at call time (see resolve.Done) and invoked by
// absolute path, with cwd and $PWD both set to projectRoot — the same
// pattern NewJob uses, because finish-job.sh locates the project root from
// $PWD via the same find_project_root helper new-job.sh uses.
func DoneCommand(jobName, projectRoot string) (*exec.Cmd, error) {
	found, err := resolve.Resolve(resolve.Done())
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(found.Path, jobName)
	cmd.Dir = projectRoot
	// exec.Cmd sets cwd via chdir but does not update the PWD env var; the
	// script reads $PWD, so set it explicitly to match.
	cmd.Env = append(os.Environ(), "PWD="+projectRoot)

	return cmd, nil
}

// DeleteCommand resolves the host job-deletion command — `<cmd> "<jobName>"`,
// see scripts/delete-job.sh — and returns the *exec.Cmd ready to run, but
// does not run it: like DoneCommand, delete-job.sh's `read -rp` confirmation
// needs a real interactive terminal, so the caller runs this in the
// foreground (tea.ExecProcess) rather than capturing its output.
//
// jobName is the job directory's exact name, same convention as DoneCommand.
//
// The command is resolved at call time (see resolve.Delete) and invoked by
// absolute path, with cwd and $PWD both set to projectRoot — same pattern
// DoneCommand uses, because delete-job.sh locates the project root from $PWD
// via the same find_project_root helper the other scripts use.
func DeleteCommand(jobName, projectRoot string) (*exec.Cmd, error) {
	found, err := resolve.Resolve(resolve.Delete())
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(found.Path, jobName)
	cmd.Dir = projectRoot
	// exec.Cmd sets cwd via chdir but does not update the PWD env var; the
	// script reads $PWD, so set it explicitly to match.
	cmd.Env = append(os.Environ(), "PWD="+projectRoot)

	return cmd, nil
}
