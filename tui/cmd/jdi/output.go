package main

import (
	"bytes"
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
// recent one. It also reports whether the log already had content when it
// was opened (hadContent), which main() feeds to sectionWriter so the very
// first run ever written to a log starts it cleanly without a leading blank
// line (see sectionWriter's own doc).
func openRunLog(root, jobName string) (*os.File, bool, error) {
	dir := job.JDIStatusDir(root, jobName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, false, fmt.Errorf("create sidecar dir %s: %w", dir, err)
	}
	f, err := os.OpenFile(job.JDIRunLogPath(root, jobName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("open run log: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, false, fmt.Errorf("stat run log: %w", err)
	}
	return f, st.Size() > 0, nil
}

// sectionPrefix is the literal every "=== ... ===" state-log header in this
// file starts with — the five log* functions (logStarted, logAgentInvoked,
// logInvocation, logJobFinished, logImmediateStop) all emit their event
// headers as a line beginning with it. sectionWriter keys off it so it only
// inserts blank lines between actual sections, never inside one (the agent
// text and stop-reason bodies that follow their headers are separate writes
// that don't match).
var sectionPrefix = []byte("===")

// sectionWriter is the writer mg-jdi's log destinations (stdout and the
// sidecar run.log) are wrapped in before anything is logged: it inserts a
// blank line before every "=== ... ===" section header written through it,
// so the log's events don't get crumbled together — either between the
// state events of a single run, or between consecutive mg-jdi runs of the
// same job appending to the same run.log.
//
// wroteSection seeds the writer with the pre-existing state of the
// underlying destination — whether run.log already had content when it was
// opened (see openRunLog). true means the first header written through this
// writer still needs its separating blank line (it's joining prior runs'
// output); false means this is the very first run ever written to the log,
// and the first header starts it at the top of the file with no leading
// blank line. Either way, every subsequent header gets the blank line.
//
// A non-header write (a header's following reason line, or an invocation's
// agent text, each of which arrives as its own write that does not start
// with sectionPrefix) passes through untouched — it belongs to the section
// that just began and must stay flush against its header.
type sectionWriter struct {
	w            io.Writer
	wroteSection bool
}

func (s *sectionWriter) Write(p []byte) (int, error) {
	if bytes.HasPrefix(p, sectionPrefix) {
		if s.wroteSection {
			if _, err := io.WriteString(s.w, "\n\n"); err != nil {
				return 0, err
			}
		}
		s.wroteSection = true
	}
	return s.w.Write(p)
}

// logStarted writes a "mg jdi started" event to w, once, at the very top of
// a run (TASK-1) — main()'s first write to logDest, before Run is even
// invoked, so a human watching run.log sees confirmation the run began
// immediately, rather than the log staying empty until the first agent
// invocation finishes (the brief's own complaint). Mirrors
// logImmediateStop's "=== ... ===" header shape so every event in the log
// looks consistent.
func logStarted(w io.Writer, jobName, profile string) {
	fmt.Fprintf(w, "=== %s mg jdi started: job %s, profile %s ===\n", time.Now().Format(time.RFC3339), jobName, profile)
}

// logAgentInvoked writes an "agent invoked" event to w immediately before
// Run launches an agent (TASK-2) — distinct from logInvocation's own
// post-run header below, so a human watching run.log sees an invoked
// header right away rather than waiting for the (possibly long-running)
// invocation to finish before anything at all appears for it.
func logAgentInvoked(w io.Writer, agent string, attempt int) {
	fmt.Fprintf(w, "=== %s %s invoked (attempt %d) ===\n", time.Now().Format(time.RFC3339), agent, attempt)
}

// logJobFinished writes a "job finished" event to w at Run's loop exit
// (TASK-4) — both terminal outcomes, StopFinished and StopNeedsHuman.
//
// includeReason is false only for the stop-before-any-agent-ran case: that
// path has already printed reason via logImmediateStop immediately above,
// so repeating it here would print the exact same line twice in a row.
// Every other call site has never logged reason before this point, so it's
// included.
func logJobFinished(w io.Writer, kind orchestrate.Kind, reason string, includeReason bool) {
	fmt.Fprintf(w, "=== %s mg jdi finished: %s ===\n", time.Now().Format(time.RFC3339), kind)
	if includeReason && reason != "" {
		fmt.Fprintln(w, reason)
	}
}

// agentTargetFile maps each driven agent (orchestrate.Sequence) to the job
// file it's expected to have written by the time its invocation returns —
// TASK-5's dedup check reads this file fresh off disk via j.Dir, safe to read
// unconditionally because every job lives in its own worktree
// (207bfu_git-worktrees) that is the live, correct place to read it from —
// to compare against the agent's own response text.
var agentTargetFile = map[string]string{
	"analyst":   "tasks.md",
	"developer": "implementation.md",
	"reviewer":  "verdict.md",
}

// isDuplicateOutput reports whether fileContent — the just-run agent's
// target file, read fresh after this invocation — appears, once both sides
// are trimmed and internal whitespace is collapsed, as a substring of text —
// the agent's own response (TASK-5's "already the same ... skip it" rule
// from the brief). Substring rather than exact equality, since a real
// response typically echoes the file's content plus its own surrounding
// commentary (a leading summary, a trailing "done!", etc.).
//
// An empty fileContent (the target file doesn't exist yet, or is itself
// empty) never counts as a duplicate — trivially "contained" in anything,
// but not a meaningful match.
func isDuplicateOutput(text, fileContent string) bool {
	normFile := normalizeWhitespace(fileContent)
	if normFile == "" {
		return false
	}
	return strings.Contains(normalizeWhitespace(text), normFile)
}

// normalizeWhitespace collapses s to single-space-separated fields, so
// isDuplicateOutput's comparison isn't tripped up by incidental reformatting
// (extra blank lines, trailing spaces, etc.) between what's on disk and what
// an agent echoed back.
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// logInvocation writes one agent invocation's captured output to w — Run's
// fan-out target (Decision 7), which main() builds as an io.MultiWriter of
// mg-jdi's own stdout and the sidecar's run.log, so both destinations get
// the exact same section: a "=== <timestamp> <agent> finished (attempt N)
// ===" header — paired with logAgentInvoked's own header just before this
// invocation started (TASK-2/TASK-3) so a reader sees an invoked/finished
// pair per agent call rather than one ambiguous header — then the agent's
// extracted final-response text
// (orchestrate.ResultText) — not the raw --output-format json blob, so a
// human reading either destination sees prose, not JSON.
//
// Honesty note (Decision 7's caveat, carried into scripts/entrypoint.sh's
// own comment): this is exactly the agent's *final response text* per
// invocation, not a blow-by-blow of every tool call/file edit — that's all
// `claude --print` (json or plain-text form) actually returns. "See what
// happens" means "see each agent's final answer as it's produced," not a
// live diff of its work.
//
// targetFile/targetContent (TASK-5) are the just-run agent's expected job
// file and its content read fresh after the invocation (see
// agentTargetFile) — the caller's responsibility, since it already has j.Dir
// on hand. When the response text turns out to be a duplicate of that file
// (isDuplicateOutput), the body is replaced with a short note instead of the
// full (possibly large) text — the header is unaffected, so a reader still
// sees an invoked/finished pair either way. targetFile == "" (e.g. an
// unrecognized agent, or the file couldn't be read) simply skips the check.
func logInvocation(w io.Writer, agent string, attempt int, raw []byte, targetFile, targetContent string) {
	fmt.Fprintf(w, "=== %s %s finished (attempt %d) ===\n", time.Now().Format(time.RFC3339), agent, attempt)
	text := orchestrate.ResultText(raw)
	switch {
	case text == "":
		text = "(no output)"
	case targetFile != "" && isDuplicateOutput(text, targetContent):
		text = fmt.Sprintf("(output matches %s, omitted)", targetFile)
	}
	if text[len(text)-1] != '\n' {
		text += "\n"
	}
	fmt.Fprint(w, text)
}

// logImmediateStop writes an "immediate stop" note to w — orchestrate.Next
// returning a Stop* decision before any agent has ever been invoked (e.g.
// job.StageDefine with an unwritten brief.md), the one case logInvocation
// never runs for. Without this, run.log would be left at 0 bytes for that
// stop, itself confusing (see the "jdi does not work" job's own complaint)
// independent of whether the stop was the correct call. Mirrors
// logInvocation's "=== ... ===" header shape so both look consistent in the
// same log, but names the stop instead of an agent/attempt.
func logImmediateStop(w io.Writer, reason string) {
	fmt.Fprintf(w, "=== %s mg jdi stopped before running any agent ===\n", time.Now().Format(time.RFC3339))
	fmt.Fprintln(w, reason)
}
