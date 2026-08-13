package job

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Stage is the workflow stage a job is in, per the ideal-workflow model:
//
//   - define:    brief.md is still the mg-job scaffold   → the user writes the brief
//   - plan:      brief.md written, tasks.md not yet       → owner, analyst
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
// there, but that gate was removed — all agents are always launchable,
// regardless of stage — so Stage no longer has an Agents() method; do not
// reintroduce one as a gate.
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
// Files are always read straight from j.Dir — a job's own worktree (or, for
// an archived job, the main worktree's own archive/) is unconditionally the
// live, correct place to read them from (see the package doc).
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

// fileWritten reports whether the named job file (e.g. "tasks.md") is
// written, reading it straight from j.Dir. The filename is relative to the
// job dir.
func (j Job) fileWritten(filename string) bool {
	data, ok := j.readFile(filename)
	if !ok {
		return false
	}
	return isWritten(data)
}

// readFile returns the raw bytes of a job file (filename relative to the job
// dir), read straight from j.Dir — the same unconditional read fileWritten
// (and, through it, Stage) relies on throughout this file. ok is false when
// the file can't be read at all (missing); that is not itself an error
// condition here, just "no content to look at".
func (j Job) readFile(filename string) (data []byte, ok bool) {
	data, err := os.ReadFile(filepath.Join(j.Dir, filename))
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
	section := verdictOverallSection(string(data))
	if verdictNotApprovedRe.MatchString(section) {
		return false
	}
	return verdictApprovedRe.MatchString(section)
}

// VerdictStatus returns the first verdict-status line under the "## Overall"
// heading of the job's verdict.md — the same computation finish-job.sh's grep
// chain made (`grep -i '^## Overall' -A5 | grep -iE 'APPROVED|REJECTED|NEEDS
// WORK' | head -1`). Returns "" when the file is missing or the status is
// undetermined. Exported so the TUI's done confirmation can show the same
// warnings the CLI does.
func (j Job) VerdictStatus() string {
	data, ok := j.readFile("verdict.md")
	if !ok {
		return ""
	}
	return verdictOverallMatch(data)
}

// VerdictNotApproved reports whether a verdict status line says REJECTED or
// NEEDS WORK (case-insensitive) — the bounce-back test shared by job.Stage,
// finish-job.sh and the TUI's done confirmation.
func VerdictNotApproved(status string) bool {
	return verdictNotApprovedRe.MatchString(status)
}

// verdictOverallSection is the single extraction primitive for the "## Overall"
// region of a verdict file (the other verdict helpers are all built on it):
// the text following the heading up to the next "##" heading (or end of file),
// with HTML comments stripped — the scaffold's guidance comments are not
// verdict content. Returns "" if there is no such heading at all.
//
// The whole section, not grep's -A5 window, is the behavior that wins: a
// status line anywhere under the heading is a verdict statement, and the
// window (a fidelity artifact of finish-job.sh's `grep -A5`) could miss a
// genuine status placed more than five lines down, spuriously warning
// "could not determine verdict status". Pinned by TestVerdictOverallMatch's
// "status beyond line 5" case.
func verdictOverallSection(body string) string {
	body = stripHTMLComments(body)
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

// verdictOverallMatch returns the first verdict-status line (a line containing
// APPROVED, REJECTED, or NEEDS WORK) under the "## Overall" heading — the
// status finish-job.sh surfaced in its warnings — or "" when there is none. It
// reads the whole section (see verdictOverallSection) rather than grep's -A5
// window, so a status line anywhere under the heading is recognised.
func verdictOverallMatch(data []byte) string {
	section := verdictOverallSection(string(data))
	for _, line := range strings.Split(section, "\n") {
		if verdictNotApprovedRe.MatchString(line) || verdictApprovedRe.MatchString(line) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// FileIsWritten reports whether the file at path has real content beyond its
// scaffold template.
//
// "Written" rule (the brief leaves "written" undefined): a job markdown
// file is scaffolded by new-job.sh with a title heading, loose key:value
// frontmatter, HTML-comment guidance, and bare section headers — all of
// which we ignore. The file is "written" when, after stripping HTML
// comments, the remaining body has at least one substantive line (a line of
// ≥8 chars that is not a heading, not frontmatter, and not a TASK- marker)
// OR at least one TASK-... entry outside comments.
//
// One substantive line is enough: real usage regularly produces a brief with
// only a single filled-in section (typically "## What", one paragraph on one
// line) while every other scaffold section is left as its HTML-comment
// placeholder — every substantive sentence in it collapses to one line
// unless the author happens to hit Enter. Requiring two lines misclassified
// that common, genuinely-written case as unwritten (see the "jdi does not
// work" job), which made mg jdi stop immediately with "brief.md is not
// written yet" before ever invoking an agent. One line still correctly
// rejects every new-job.sh scaffold template below, since those have zero
// substantive lines at all — only HTML comments, headings, and frontmatter.
//
// FileIsWritten reads from disk; isWritten is the bytes-only core, used by
// Job.fileWritten.
func FileIsWritten(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return isWritten(data)
}

// isWritten applies the "written" rule (see FileIsWritten) to already-read
// file bytes, sharing the exact same classification logic as the working-tree
// disk read.
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
		if key, value, ok := splitKV(line); ok && (frontmatterKeys[key] || value == "") {
			// Either a recognised frontmatter key: value, or a bare "key:"
			// with nothing filled in yet (e.g. new-job.sh's unfilled
			// "analyst:" / "developer:" / "reviewer:" attribution lines) —
			// neither carries substantive content. Once such a field is
			// actually filled in (value != ""), it's no longer skipped here
			// and falls through to the length check below like any other
			// line.
			continue
		}
		if len(line) >= 8 {
			substantive++
		}
	}
	return substantive >= 1 || hasTask
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
