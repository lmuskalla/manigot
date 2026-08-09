package job

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lmuskalla/safecode/tui/internal/git"
)

// Stage is the workflow stage a job is in, per the ideal-workflow model:
//
//   - analyze: open job, tasks not yet written   → product-owner, analyst
//   - develop: tasks.md written                  → developer
//   - review:  implementation.md written         → reviewer, security
//
// Stage is informational only: it labels which step of the ideal workflow a
// job's files say it has reached, shown as a hint in the TUI's detail view
// ("stage: <name>"). It used to also gate which agents could be launched from
// there, but per the "launch agents without workflow" brief that gate was
// removed — all agents are always launchable, regardless of stage — so Stage
// no longer has an Agents() method; do not reintroduce one as a gate.
type Stage string

const (
	StageAnalyze Stage = "analyze"
	StageDevelop Stage = "develop"
	StageReview  Stage = "review"
)

// Stage derives the job's current workflow stage from which files are written.
// Precedence: implementation.md written → review; else tasks.md written →
// develop; else analyze. This matches the brief's stage model exactly.
//
// For a job on the current branch the files are read from the working tree
// (so uncommitted edits still count); for a job discovered on another branch
// they are read via `git show <Branch>:…` from that branch — otherwise every
// cross-branch job would falsely report stage analyze because its files
// aren't in the working tree at all.
func (j Job) Stage() Stage {
	switch {
	case j.fileWritten("implementation.md"):
		return StageReview
	case j.fileWritten("tasks.md"):
		return StageDevelop
	default:
		return StageAnalyze
	}
}

// fileWritten reports whether the named job file (e.g. "tasks.md") is written,
// reading it from the working tree for current-branch jobs and via git show
// for jobs living on another branch. The filename is relative to the job dir.
func (j Job) fileWritten(filename string) bool {
	if j.OnCurrentBranch {
		return FileIsWritten(filepath.Join(j.Dir, filename))
	}
	// Off-branch: read the file's bytes from its branch and apply the same
	// "written" rule FileIsWritten uses. A missing file (git show →
	// os.ErrNotExist) is simply not written, same as a missing working-tree
	// file in the current-branch path.
	rel := filepath.ToSlash(filepath.Join(JobsRelDir, j.Name, filename))
	data, err := git.ShowFile(j.Root, j.Branch, rel)
	if err != nil {
		return false
	}
	return isWritten(data)
}

// FileIsWritten reports whether the file at path has real content beyond its
// scaffold template.
//
// "Written" rule (pinned here for TASK-7; the brief leaves "written"
// undefined): a job markdown file is scaffolded by new-job.sh with a title
// heading, loose key:value frontmatter, HTML-comment guidance, and bare
// section headers — all of which we ignore. The file is "written" when, after
// stripping HTML comments, the remaining body has at least two substantive
// lines (a line of ≥8 chars that is not a heading, not frontmatter, and not a
// TASK- marker) OR at least one TASK-... entry outside comments.
//
// This correctly classifies the new-job.sh templates as unwritten and the
// analyst/developer/reviewer-filled versions as written.
//
// FileIsWritten reads from disk; isWritten is the bytes-only core, used by
// Job.fileWritten for off-branch jobs whose contents arrive via git show.
func FileIsWritten(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return isWritten(data)
}

// isWritten applies the "written" rule (see FileIsWritten) to already-read
// file bytes, so the off-branch path (git show → bytes) shares the exact same
// classification logic as the working-tree path (disk read).
func isWritten(data []byte) bool {
	body := stripHTMLComments(string(data))

	substantive := 0
	hasTask := false
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "#"):
			continue // title / section heading
		case strings.HasPrefix(line, "TASK-"):
			hasTask = true
			continue
		}
		if key, _, ok := splitKV(line); ok && frontmatterKeys[key] {
			continue // loose frontmatter key: value
		}
		if len(line) >= 8 {
			substantive++
		}
	}
	return substantive >= 2 || hasTask
}

// stripHTMLComments removes <!-- … --> spans (including multi-line comments)
// from s. An unterminated comment drops the remainder.
func stripHTMLComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if strings.HasPrefix(s[i:], "<!--") {
			rest := s[i+4:]
			end := strings.Index(rest, "-->")
			if end < 0 {
				return b.String() // unterminated — drop the rest
			}
			i += 4 + end + 3
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
