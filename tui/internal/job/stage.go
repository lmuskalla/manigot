package job

import (
	"os"
	"path/filepath"
	"strings"
)

// Stage is the workflow stage a job is in, per the brief's stage model:
//
//   - analyze: open job, tasks not yet written   → product-owner, analyst
//   - develop: tasks.md written                  → developer
//   - review:  implementation.md written         → reviewer, security
type Stage string

const (
	StageAnalyze Stage = "analyze"
	StageDevelop Stage = "develop"
	StageReview  Stage = "review"
)

// stageAgents maps each stage to the agents the brief says to surface in the
// action bar. Order is the display order.
var stageAgents = map[Stage][]string{
	StageAnalyze: {"product-owner", "analyst"},
	StageDevelop: {"developer"},
	StageReview:  {"reviewer", "security"},
}

// Agents returns the agents to surface for this stage (empty for an unknown
// stage).
func (s Stage) Agents() []string {
	return stageAgents[s]
}

// Stage derives the job's current workflow stage from which files are written.
// Precedence: implementation.md written → review; else tasks.md written →
// develop; else analyze. This matches the brief's stage model exactly.
func (j Job) Stage() Stage {
	switch {
	case FileIsWritten(filepath.Join(j.Dir, "implementation.md")):
		return StageReview
	case FileIsWritten(filepath.Join(j.Dir, "tasks.md")):
		return StageDevelop
	default:
		return StageAnalyze
	}
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
func FileIsWritten(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
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
