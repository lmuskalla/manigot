package job

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeJob writes a minimal docs/processes/<name>/brief.md.
func writeJob(t *testing.T, root, name, brief string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "processes", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if brief != "" {
		if err := os.WriteFile(filepath.Join(dir, "brief.md"), []byte(brief), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestParseBrief(t *testing.T) {
	const brief = "# Brief: TUI\n\n" +
		"status: open\n" +
		"type: feature\n" +
		"id: irw320\n" +
		"branch: feature/irw320_tui\n" +
		"date: 2026-08-08\n" +
		"author: Leander Muskalla\n\n" +
		"## What\n\nbody text mentioning branch: should-not-match\n"

	// Parse via a temp job dir so ReadJob actually reads the file.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "brief.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	job, err := ReadJob(dir)
	if err != nil {
		t.Fatalf("ReadJob: unexpected error %v", err)
	}

	if job.Title != "TUI" {
		t.Errorf("Title = %q, want %q", job.Title, "TUI")
	}
	if job.ID != "irw320" {
		t.Errorf("ID = %q, want %q", job.ID, "irw320")
	}
	if job.Type != "feature" {
		t.Errorf("Type = %q, want %q", job.Type, "feature")
	}
	if job.Status != "open" {
		t.Errorf("Status = %q, want %q", job.Status, "open")
	}
	if job.Branch != "feature/irw320_tui" {
		t.Errorf("Branch = %q, want %q", job.Branch, "feature/irw320_tui")
	}
	if job.Date != "2026-08-08" {
		t.Errorf("Date = %q, want %q", job.Date, "2026-08-08")
	}
	if job.Author != "Leander Muskalla" {
		// Author has a space — confirms we keep the full value, unlike the
		// bash `awk '{print $2}'` which would truncate to "Leander".
		t.Errorf("Author = %q, want %q", job.Author, "Leander Muskalla")
	}
}

func TestParseBriefDefaultsWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	// No brief.md at all.
	job, err := ReadJob(dir)
	if err != nil {
		t.Fatalf("ReadJob: unexpected error %v", err)
	}
	name := filepath.Base(dir)
	if job.Title != name {
		t.Errorf("Title = %q, want dir name %q", job.Title, name)
	}
	if job.Type != "feature" {
		t.Errorf("Type default = %q, want feature", job.Type)
	}
	if job.Status != "open" {
		t.Errorf("Status default = %q, want open", job.Status)
	}
	if job.ID == "" {
		t.Errorf("ID should be derived from dir name, got empty")
	}
}

func TestIDDerivedFromDirName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "irw320_tui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	job, _ := ReadJob(dir)
	if job.ID != "irw320" {
		t.Errorf("derived ID = %q, want irw320", job.ID)
	}
	if job.Name != "irw320_tui" {
		t.Errorf("Name = %q, want irw320_tui", job.Name)
	}
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()

	// Two real jobs with known dates, one archived (must be excluded), and a
	// stray non-directory file (must be skipped).
	writeJob(t, root, "aaaa01_older", strings.Repeat("x", 0)+
		"# Brief: Older\n\nstatus: open\ntype: fix\ndate: 2025-01-01\n")
	writeJob(t, root, "zzzz99_newer", "# Brief: Newer\n\nstatus: done\ndate: 2026-12-31\n")
	writeJob(t, root, "archive/q_archived", "# Brief: Archived\n\ndate: 2020-01-01\n")
	if err := os.WriteFile(filepath.Join(root, "docs", "processes", "README.md"),
		[]byte("not a job"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2 (archive + stray file excluded); got %+v", len(jobs), jobs)
	}
	// Sorted by date desc → Newer first.
	if jobs[0].Title != "Newer" {
		t.Errorf("jobs[0].Title = %q, want Newer (date desc)", jobs[0].Title)
	}
	if jobs[1].Title != "Older" {
		t.Errorf("jobs[1].Title = %q, want Older", jobs[1].Title)
	}
	// Archived job must not appear.
	for _, j := range jobs {
		if strings.Contains(j.Name, "archive") {
			t.Errorf("archive/ job leaked into results: %+v", j)
		}
	}
}

func TestDiscoverEmptyWhenNoProcesses(t *testing.T) {
	root := t.TempDir()
	// docs/ exists but no docs/processes/.
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	jobs, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover on missing processes dir: unexpected error %v", err)
	}
	if jobs != nil && len(jobs) != 0 {
		t.Errorf("expected no jobs, got %d", len(jobs))
	}
}

func TestFindProjectRootWalksUp(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Chdir two levels deep inside the fake project.
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(deep); err != nil {
		t.Fatal(err)
	}
	got, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot: %v", err)
	}
	if got != root {
		t.Errorf("FindProjectRoot = %q, want %q", got, root)
	}
}

func TestFindProjectRootNotFound(t *testing.T) {
	// /tmp should not itself contain a docs/ in an ancestor chain that we own;
	// use a fresh temp dir and chdir into it.
	root := t.TempDir()
	// Make absolutely sure no docs/ exists in the chain by not creating one.
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	got, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot: unexpected error %v", err)
	}
	if got != "" {
		t.Errorf("FindProjectRoot = %q, want empty string (not found) — an ancestor has docs/", got)
	}
}
