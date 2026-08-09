package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lmuskalla/manigot/tui/internal/job"
	"github.com/lmuskalla/manigot/tui/internal/orchestrate"
)

// sidecarExcludePattern is the .gitignore-syntax line ensureSidecarIgnored
// adds to a project's .git/info/exclude — every job's status sidecar, not
// just one, since the pattern is directory-shaped
// ("docs/jobs/.jdi-status/").
var sidecarExcludePattern = filepath.ToSlash(filepath.Join(job.JobsRelDir, job.JDISidecarDirName)) + "/"

// ensureSidecarIgnored appends sidecarExcludePattern to root's
// .git/info/exclude if not already present.
//
// This matters because the status/run.log sidecar (Decision 4) lives inside
// the *target project's* docs/jobs/ — not manigot's own checkout — and the
// whole point of keeping it outside every job's own directory is so it can
// never be swept into an agent's own `git add -A`. That only actually holds
// if *this project's* git configuration excludes it: manigot's own
// .gitignore has no bearing on a project mg-jdi is driving. Rather than
// assume (or silently mutate) the project's own tracked .gitignore,
// .git/info/exclude is the right mechanism — local-only, per-checkout, never
// itself committed — so mg-jdi can guarantee this without touching anything
// the project's own history would ever see. Idempotent: safe to call on
// every mg-jdi run.
//
// Best-effort: a failure here (root isn't a git repo, .git/info isn't
// writable, etc.) is surfaced to the caller to log, but must not abort the
// run — worst case the sidecar risks being swept into a commit, exactly the
// pre-existing behavior before this safeguard existed.
func ensureSidecarIgnored(root string) error {
	excludePath := filepath.Join(root, ".git", "info", "exclude")

	existing, err := os.ReadFile(excludePath)
	switch {
	case err == nil:
		if containsLine(string(existing), sidecarExcludePattern) {
			return nil
		}
	case os.IsNotExist(err):
		existing = nil
	default:
		return err
	}

	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(excludePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	prefix := ""
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		prefix = "\n"
	}
	_, err = f.WriteString(prefix + sidecarExcludePattern + "\n")
	return err
}

// containsLine reports whether body has pattern as one of its lines
// (trimmed), so an existing exclude file with the pattern already present —
// however it got there — isn't duplicated on a second mg-jdi run.
func containsLine(body, pattern string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == pattern {
			return true
		}
	}
	return false
}

// openRunLog ensures the sidecar directory (job.JDIStatusDir — shared with
// the status file WriteJDIStatus writes, Decision 4) exists and opens
// (creating if necessary) run.log for appending — a fresh mg-jdi run
// continues the same job's transcript rather than truncating it, so a human
// can see the full history of every run against this job, not just the most
// recent one.
func openRunLog(root, jobName string) (*os.File, error) {
	dir := job.JDIStatusDir(root, jobName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create sidecar dir %s: %w", dir, err)
	}
	f, err := os.OpenFile(job.JDIRunLogPath(root, jobName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open run log: %w", err)
	}
	return f, nil
}

// logInvocation writes one agent invocation's captured output to w — Run's
// fan-out target (Decision 7), which main() builds as an io.MultiWriter of
// mg-jdi's own stdout and the sidecar's run.log, so both destinations get
// the exact same section: a "=== <timestamp> <agent> (attempt N) ==="
// header, then the agent's extracted final-response text
// (orchestrate.ResultText) — not the raw --output-format json blob, so a
// human reading either destination sees prose, not JSON.
//
// Honesty note (Decision 7's caveat, carried into scripts/entrypoint.sh's
// own comment): this is exactly the agent's *final response text* per
// invocation, not a blow-by-blow of every tool call/file edit — that's all
// `claude --print` (json or plain-text form) actually returns. "See what
// happens" means "see each agent's final answer as it's produced," not a
// live diff of its work.
func logInvocation(w io.Writer, agent string, attempt int, raw []byte) {
	fmt.Fprintf(w, "=== %s %s (attempt %d) ===\n", time.Now().Format(time.RFC3339), agent, attempt)
	text := orchestrate.ResultText(raw)
	if text == "" {
		text = "(no output)"
	}
	if text[len(text)-1] != '\n' {
		text += "\n"
	}
	fmt.Fprint(w, text)
}
