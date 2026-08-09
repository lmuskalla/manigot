package job

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lmuskalla/safecode/tui/internal/git"
)

// Stage is the workflow stage a job is in, per the ideal-workflow model:
//
//   - define:    brief.md is still the sc-job scaffold   → the user writes the brief
//   - plan:      brief.md written, tasks.md not yet       → product-owner, analyst
//   - implement: tasks.md written, implementation.md not  → developer
//   - review:    implementation.md written, verdict not   → reviewer, security
//   - finished:  verdict.md written, and its "## Overall" verdict is APPROVED
//
// A verdict.md that has been written but was NOT approved (REJECTED, NEEDS
// WORK, or any other/missing verdict) bounces the stage back to implement
// rather than resolving to review or finished — the job needs more
// development work before it can go through review again.
//
// Stage is informational only: it labels which step of the ideal workflow a
// job's files say it has reached, shown as a horizontal timeline in the TUI's
// detail view. It used to also gate which agents could be launched from
// there, but per the "launch agents without workflow" brief that gate was
// removed — all agents are always launchable, regardless of stage — so Stage
// no longer has an Agents() method; do not reintroduce one as a gate.
type Stage string

const (
	StageDefine    Stage = "define"
	StagePlan      Stage = "plan"
	StageImplement Stage = "implement"
	StageReview    Stage = "review"
	StageFinished  Stage = "finished"
)

// Stages is every stage in workflow order, for rendering a timeline of the
// job's progress.
var Stages = []Stage{StageDefine, StagePlan, StageImplement, StageReview, StageFinished}

// Stage derives the job's current workflow stage from which files are
// written and, once a verdict exists, whether it was approved. See the Stage
// type's doc for the precedence and the review/approval bounce-back rule.
//
// For a job on the current branch the files are read from the working tree
// (so uncommitted edits still count); for a job discovered on another branch
// they are read via `git show <Branch>:…` from that branch — otherwise every
// cross-branch job would falsely report stage define because its files
// aren't in the working tree at all.
func (j Job) Stage() Stage {
	switch {
	case !j.fileWritten("brief.md"):
		return StageDefine
	case !j.fileWritten("tasks.md"):
		return StagePlan
	case !j.fileWritten("implementation.md"):
		return StageImplement
	case !j.fileWritten("verdict.md"):
		return StageReview
	case j.verdictApproved():
		return StageFinished
	default:
		return StageImplement
	}
}

// fileWritten reports whether the named job file (e.g. "tasks.md") is written,
// reading it from the working tree for current-branch jobs and via git show
// for jobs living on another branch. The filename is relative to the job dir.
func (j Job) fileWritten(filename string) bool {
	data, ok := j.readFile(filename)
	if !ok {
		return false
	}
	return isWritten(data)
}

// readFile returns the raw bytes of a job file (filename relative to the job
// dir), reading from the working tree for a current-branch job or via `git
// show` for a job discovered on another branch — the same dual-path strategy
// fileWritten (and, through it, Stage) relies on throughout this file. ok is
// false when the file can't be read at all (missing, or a git show failure);
// that is not itself an error condition here, just "no content to look at".
func (j Job) readFile(filename string) (data []byte, ok bool) {
	if j.OnCurrentBranch {
		data, err := os.ReadFile(filepath.Join(j.Dir, filename))
		if err != nil {
			return nil, false
		}
		return data, true
	}
	rel := filepath.ToSlash(filepath.Join(JobsRelDir, j.Name, filename))
	data, err := git.ShowFile(j.Root, j.Branch, rel)
	if err != nil {
		return nil, false
	}
	return data, true
}

// verdictNotApprovedRe / verdictApprovedRe classify the text under verdict.md's
// "## Overall" heading, mirroring finish-job.sh's own
// `grep -i '^## Overall' -A5 | grep -iE 'APPROVED|REJECTED|NEEDS WORK'` check
// closely enough for stage purposes. The not-approved pattern is checked
// first and wins on conflict, so a reviewer who writes "REJECTED — needs
// another APPROVED pass on TASK-3" still bounces the stage back rather than
// resolving finished.
var (
	verdictOverallRe     = regexp.MustCompile(`(?im)^##\s*overall\s*$`)
	verdictNotApprovedRe = regexp.MustCompile(`(?i)REJECTED|NEEDS WORK`)
	verdictApprovedRe    = regexp.MustCompile(`(?i)\bAPPROVED\b`)
)

// verdictApproved reports whether the job's verdict.md, under its "##
// Overall" heading, says APPROVED. Called only once verdict.md is already
// known to be written (see Stage); a missing/unreadable file or a section
// that says neither APPROVED nor REJECTED/NEEDS WORK defaults to false — the
// same "don't assume approval" default finish-job.sh applies when it can't
// determine the verdict status either.
func (j Job) verdictApproved() bool {
	data, ok := j.readFile("verdict.md")
	if !ok {
		return false
	}
	section := verdictOverallSection(stripHTMLComments(string(data)))
	if verdictNotApprovedRe.MatchString(section) {
		return false
	}
	return verdictApprovedRe.MatchString(section)
}

// verdictOverallSection extracts the text following a "## Overall" heading up
// to the next "##" heading (or end of file), from HTML-comment-stripped
// verdict.md content. Returns "" if there is no such heading at all.
func verdictOverallSection(body string) string {
	loc := verdictOverallRe.FindStringIndex(body)
	if loc == nil {
		return ""
	}
	rest := body[loc[1]:]
	if next := strings.Index(rest, "\n##"); next >= 0 {
		rest = rest[:next]
	}
	return rest
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
