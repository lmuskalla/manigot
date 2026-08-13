package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/job"
	"github.com/lmuskalla/manigot/internal/ui"
)

// jobsBrief builds a brief.md body with frontmatter for a hermetic fixture.
func jobsBrief(title, id, typ, date string) string {
	return "# Brief: " + title + "\n\n" +
		"status: open\ntype: " + typ + "\nid: " + id + "\ndate: " + date + "\n"
}

// jobsCheckout builds a project root (a non-git temp dir — exercising
// job.Discover's working-tree fallback) with docs/jobs/ populated from a map
// of job name → brief body. Returns the root, already chdir'd into.
func jobsCheckout(t *testing.T, briefs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, brief := range briefs {
		dir := filepath.Join(root, "docs", "jobs", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "brief.md"), []byte(brief), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	return root
}

// jobsJDIStatus writes an mg-jdi status sidecar under root, using the job
// package's own writer so the on-disk format is exactly the real one.
func jobsJDIStatus(t *testing.T, root, jobName string, state job.JDIState, agent string) {
	t.Helper()
	if err := job.WriteJDIStatus(root, jobName, state, agent); err != nil {
		t.Fatal(err)
	}
}

// pickerStub returns a pickerRunFunc that fails the test if invoked — for
// tests that must never reach the selection step (non-TTY paths, empty job
// lists, orphan-only flows).
func pickerStub(t *testing.T) pickerRunFunc {
	t.Helper()
	return func(title string, rows []ui.PickerRow) (string, bool, error) {
		t.Fatalf("picker unexpectedly run (title %q)", title)
		return "", false, nil
	}
}

// pickerChoice returns a pickerRunFunc that reports the given result without
// touching a terminal — the fake the wiring tests inject.
func pickerChoice(id string, ok bool) pickerRunFunc {
	return func(title string, rows []ui.PickerRow) (string, bool, error) {
		return id, ok, nil
	}
}

func TestJobsListsWithStateAndBadge(t *testing.T) {
	root := jobsCheckout(t, map[string]string{
		"aaa01_alpha": jobsBrief("Alpha Job", "aaa01", "feature", "2026-01-01"),
		"def02_beta":  jobsBrief("Beta Job", "def02", "fix", "2026-02-02"),
	})
	// mg-jdi sidecar: a live running badge for the newer job.
	jobsJDIStatus(t, root, "def02_beta", job.JDIRunning, "developer")

	var out strings.Builder
	code := runJobs(nil, strings.NewReader(""), &out, &strings.Builder{}, false, pickerStub(t))
	// Non-TTY: after listing it must refuse to pick, like mg agents.
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (non-TTY refusal)", code)
	}
	got := out.String()
	for _, want := range []string{
		"def02", "fix", "2026-02-02", "Beta Job", "[running @developer]",
		"aaa01", "feature", "2026-01-01", "Alpha Job",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("listing missing %q:\n%s", want, got)
		}
	}
	// Newest first (date desc), same ordering as the TUI.
	for _, want := range []string{"1) def02", "2) aaa01"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected date-desc order, missing %q:\n%s", want, got)
		}
	}
}

func TestJobsNonTTYRefusal(t *testing.T) {
	jobsCheckout(t, map[string]string{"aaa01_alpha": jobsBrief("Alpha Job", "aaa01", "feature", "2026-01-01")})
	var out, errOut strings.Builder
	code := runJobs(nil, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: mg jobs needs an interactive terminal to select a job.") {
		t.Errorf("missing non-TTY refusal:\n%s", errOut.String())
	}
}

// TestJobsSelectWritesChosenAndLaunches covers the TTY submit path with an
// injected picker (the seam — no real Bubble Tea program): the chosen job's
// ID is passed through to the launch line and the re-exec of
// os.Executable().
func TestJobsSelectWritesChosenAndLaunches(t *testing.T) {
	jobsCheckout(t, map[string]string{"aaa01_alpha": jobsBrief("Alpha Job", "aaa01", "feature", "2026-01-01")})
	var out strings.Builder
	code := runJobs([]string{"--profile", "zai"}, strings.NewReader(""), &out, &strings.Builder{}, true, pickerChoice("aaa01", true))
	// The launch re-execs os.Executable() — the go test binary — with
	// --job aaa01 --profile zai, which it rejects as unknown flags and exits
	// non-zero. What matters here is the menu output up to the launch.
	if code == 0 {
		t.Fatalf("unexpected success; the re-exec should not accept the flags")
	}
	if !strings.Contains(out.String(), "→ Starting a session in aaa01...") {
		t.Errorf("missing launch line:\n%s", out.String())
	}
}

// TestJobsPickerGetsJobRows pins the picker wiring: on a TTY the picker is
// fed a title plus one pre-rendered row per job (ID/status/type/date/title +
// jdi badge, search key id + title), and a cancelled picker exits 0 quietly.
func TestJobsPickerGetsJobRows(t *testing.T) {
	root := jobsCheckout(t, map[string]string{
		"aaa01_alpha": jobsBrief("Alpha Job", "aaa01", "feature", "2026-01-01"),
		"def02_beta":  jobsBrief("Beta Job", "def02", "fix", "2026-02-02"),
	})
	jobsJDIStatus(t, root, "def02_beta", job.JDIRunning, "developer")

	var gotTitle string
	var gotRows []ui.PickerRow
	pick := func(title string, rows []ui.PickerRow) (string, bool, error) {
		gotTitle, gotRows = title, rows
		return "", false, nil // cancelled
	}
	var out strings.Builder
	code := runJobs(nil, strings.NewReader(""), &out, &strings.Builder{}, true, pick)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (cancel exits quietly)", code)
	}
	if gotTitle != "Select a job" {
		t.Errorf("picker title = %q, want %q", gotTitle, "Select a job")
	}
	if len(gotRows) != 2 {
		t.Fatalf("picker rows = %d, want 2", len(gotRows))
	}
	if gotRows[0].ID != "def02" || gotRows[0].SearchKey != "def02 Beta Job" {
		t.Errorf("row 0 = %+v, want def02 with id+title search key", gotRows[0])
	}
	if !strings.Contains(gotRows[0].Label, "def02") || !strings.Contains(gotRows[0].Label, "fix") ||
		!strings.Contains(gotRows[0].Label, "2026-02-02") || !strings.Contains(gotRows[0].Label, "Beta Job") {
		t.Errorf("row 0 label missing listing columns: %q", gotRows[0].Label)
	}
	if !strings.Contains(gotRows[0].Label, "[running @developer]") {
		t.Errorf("row 0 label missing jdi badge: %q", gotRows[0].Label)
	}
}

func TestJobsEmptyList(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	var out strings.Builder
	code := runJobs(nil, strings.NewReader(""), &out, &strings.Builder{}, false, pickerStub(t))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (empty list is not an error)", code)
	}
	if !strings.Contains(out.String(), "No jobs yet — run 'mg job \"<title>\"' to create one.") {
		t.Errorf("missing empty-list message:\n%s", out.String())
	}
}

func TestJobsNoProjectRoot(t *testing.T) {
	t.Chdir(t.TempDir()) // no docs/ anywhere up the tree
	var out, errOut strings.Builder
	code := runJobs(nil, strings.NewReader(""), &out, &errOut, false, pickerStub(t))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error: could not find project root (no docs/ directory found).") {
		t.Errorf("missing no-project-root error:\n%s", errOut.String())
	}
}

// TestJobsBadge pins the plain-text badge wording for each mg-jdi state,
// mirroring the TUI's jdiStatusBadge without styling or the spinner frame.
func TestJobsBadge(t *testing.T) {
	root := t.TempDir()
	j := job.Job{Name: "aaa01_alpha"}

	if badge := jobsBadge(root, j); badge != "" {
		t.Errorf("no sidecar yet, badge = %q, want \"\"", badge)
	}

	jobsJDIStatus(t, root, j.Name, job.JDIRunning, "developer")
	if badge := jobsBadge(root, j); badge != "[running @developer]" {
		t.Errorf("running badge = %q, want [running @developer]", badge)
	}

	jobsJDIStatus(t, root, j.Name, job.JDIStoppedFinished, "developer")
	if badge := jobsBadge(root, j); badge != "[finished]" {
		t.Errorf("finished badge = %q, want [finished]", badge)
	}

	jobsJDIStatus(t, root, j.Name, job.JDIStoppedNeedsHuman, "developer")
	if badge := jobsBadge(root, j); badge != "[needs human]" {
		t.Errorf("needs-human badge = %q, want [needs human]", badge)
	}
}

// jobsOrphanWorktree builds a git project root with a dead worktree directory
// under its .manigot-worktrees sibling parent — the orphan shape DiscoverOrphans
// detects — and returns the orphan's Name and absolute Dir. It is the git
// counterpart of jobsCheckout (which builds non-git fixtures). The orphan has
// no branch and no worktree registration, mirroring a job scaffolded and then
// abandoned.
func jobsOrphanWorktree(t *testing.T, root, name string) (orphanName, orphanDir string) {
	t.Helper()
	wtParent := filepath.Join(filepath.Dir(root), ".manigot-worktrees", filepath.Base(root))
	wtPath := filepath.Join(wtParent, name)
	if err := os.MkdirAll(wtParent, 0o755); err != nil {
		t.Fatal(err)
	}
	mgGit(t, root, "worktree", "add", wtPath, "-b", "feature/"+name)
	// Read the gitdir the .git file names, then delete that metadata and the
	// branch behind git's back — leaving wtPath as a dead directory.
	data, err := os.ReadFile(filepath.Join(wtPath, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(data)), "gitdir:"))
	if err := os.RemoveAll(gitdir); err != nil {
		t.Fatal(err)
	}
	mgGit(t, root, "branch", "-D", "feature/"+name)
	return name, wtPath
}

func TestJobsListsOrphans(t *testing.T) {
	root := mgCheckout(t)
	orphanName, _ := jobsOrphanWorktree(t, root, "o3kk3n_jdi-is-broken")
	t.Chdir(root)

	var out strings.Builder
	code := runJobs(nil, strings.NewReader(""), &out, &strings.Builder{}, false, pickerStub(t))
	// No jobs, so nothing to pick — the listing + orphan surfacing is the
	// whole command, and an empty job list is not an error (exit 0).
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (no jobs to select)", code)
	}
	got := out.String()
	if !strings.Contains(got, "Orphaned worktrees") {
		t.Errorf("missing orphan heading:\n%s", got)
	}
	if !strings.Contains(got, orphanName) {
		t.Errorf("missing orphan %q in listing:\n%s", orphanName, got)
	}
	if !strings.Contains(got, "Remove them with: mg delete <name>") {
		t.Errorf("missing non-TTY removal hint:\n%s", got)
	}
}

func TestJobsOrphanRemovalOffer(t *testing.T) {
	root := mgCheckout(t)
	orphanName, orphanDir := jobsOrphanWorktree(t, root, "o3kk3n_jdi-is-broken")
	t.Chdir(root)

	var out strings.Builder
	code := runJobs(nil, strings.NewReader("y\n"), &out, &strings.Builder{}, true, pickerStub(t))
	// No jobs, so after removing the orphan the command ends cleanly (exit 0).
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if _, err := os.Stat(orphanDir); !os.IsNotExist(err) {
		t.Errorf("orphan dir still exists after removal: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "✓ Orphan removed: "+orphanName) {
		t.Errorf("missing removal confirmation:\n%s", got)
	}
	if !strings.Contains(got, "This cannot be undone.") {
		t.Errorf("missing 'This cannot be undone.':\n%s", got)
	}
}

func TestJobsOrphanRemovalDeclined(t *testing.T) {
	root := mgCheckout(t)
	_, orphanDir := jobsOrphanWorktree(t, root, "o3kk3n_jdi-is-broken")
	t.Chdir(root)

	var out strings.Builder
	code := runJobs(nil, strings.NewReader("n\n"), &out, &strings.Builder{}, true, pickerStub(t))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (declined removal is not an error)", code)
	}
	if _, err := os.Stat(orphanDir); err != nil {
		t.Errorf("orphan dir removed despite a declined confirmation: %v", err)
	}
}
