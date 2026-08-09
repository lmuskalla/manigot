// Package job reads safecode job data from the filesystem.
//
// A job lives at <project-root>/docs/jobs/<id>_<slug>/ and contains four
// markdown files: brief.md, tasks.md, implementation.md and verdict.md. This
// package deals only with locating jobs and parsing the loose (non-YAML)
// frontmatter block at the top of brief.md. See
// docs/jobs/archive/irw320_tui/tasks.md ("Implementation notes") for the exact
// format contract this parser must honour.
package job

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// Job is one safecode job directory under docs/jobs/.
type Job struct {
	// Name is the directory name, e.g. "irw320_tui". It is the machine
	// identity of the job.
	Name string

	// Dir is the absolute path to the job directory.
	Dir string

	// ID is the 6-char job id. It comes from the brief.md frontmatter when
	// present, otherwise it is derived from the leading segment of Name
	// (split on the first '_').
	ID string

	// Title is the human title, taken from the "# Brief: <title>" heading.
	// Falls back to Name when the heading is absent.
	Title string

	// Type is the job type: feature (default), fix, or chore.
	Type string

	// Status is open (default) or done. done is set by finish-job.sh when a
	// job is archived.
	Status string

	// Branch is the git branch for the job.
	Branch string

	// Date is the brief.md date field (YYYY-MM-DD).
	Date string

	// Author is the brief.md author field (may contain spaces).
	Author string
}

// frontmatterKeys are the recognised bare "key: value" keys at the top of
// brief.md. Loose parsing: these exact keys are matched line-anchored anywhere
// in the file, matching how scripts/finish-job.sh reads them with
// `grep '^branch:'`. Body text never legitimately starts with these keys.
var frontmatterKeys = map[string]bool{
	"status": true,
	"type":   true,
	"id":     true,
	"branch": true,
	"date":   true,
	"author": true,
}

// ReadJob reads the job at dir and returns it with frontmatter populated.
//
// It never returns an error for a missing or malformed brief.md: the job is
// returned with sensible defaults (type=feature, status=open, title=Name) so
// the UI can still list a half-formed job. The bash scripts make the same
// assumption — new-job.sh always writes a brief.md, so an absent one is a
// best-effort case.
func ReadJob(dir string) (Job, error) {
	name := filepath.Base(dir)
	j := Job{
		Name:   name,
		Dir:    dir,
		ID:     idFromName(name),
		Title:  name,
		Type:   "feature",
		Status: "open",
	}
	data, err := os.ReadFile(filepath.Join(dir, "brief.md"))
	if err != nil {
		return j, nil
	}
	parseBrief(data, &j)
	return j, nil
}

// idFromName splits "irw320_tui" → "irw320". With no underscore, the whole
// name is the id.
func idFromName(name string) string {
	if i := strings.IndexByte(name, '_'); i >= 0 {
		return name[:i]
	}
	return name
}

// parseBrief populates j from the brief.md contents. The format is:
//
//	# Brief: <title>
//
//	status: open
//	type: feature
//	id: irw320
//	branch: feature/irw320_tui
//	date: 2026-08-08
//	author: Leander Muskalla
//
//	## What
//	...
//
// No "---" YAML delimiters. The first "# Brief:" line supplies Title; any line
// beginning with a recognised key followed by ": " supplies that field. \r is
// trimmed so CRLF files parse the same.
func parseBrief(data []byte, j *Job) {
	for _, raw := range bytes.Split(data, []byte("\n")) {
		line := strings.TrimRight(string(raw), "\r")

		if strings.HasPrefix(line, "# Brief:") {
			if t := strings.TrimSpace(strings.TrimPrefix(line, "# Brief:")); t != "" && j.Title == j.Name {
				j.Title = t
			}
			continue
		}

		// Only consider lines that look like "key: value" with a known key.
		key, value, ok := splitKV(line)
		if !ok || !frontmatterKeys[key] {
			continue
		}
		switch key {
		case "status":
			j.Status = value
		case "type":
			j.Type = value
		case "id":
			j.ID = value
		case "branch":
			j.Branch = value
		case "date":
			j.Date = value
		case "author":
			j.Author = value
		}
	}
}

// splitKV splits "key: value" into ("key", "value"). The value keeps internal
// colons (e.g. author names, branch names). Returns ok=false if there is no
// colon or the key contains whitespace / isn't a simple token.
func splitKV(line string) (key, value string, ok bool) {
	idx := strings.IndexByte(line, ':')
	if idx <= 0 {
		return "", "", false
	}
	key = line[:idx]
	if strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	value = strings.TrimSpace(line[idx+1:])
	return key, value, true
}
