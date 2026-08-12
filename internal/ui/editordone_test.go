package ui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/job"
)

// TestEditorDoneMsgSuccess verifies the footer confirmation and that the tab
// is re-read once the editor exits cleanly.
func TestEditorDoneMsgSuccess(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0001_x")
	os.MkdirAll(jobDir, 0o755)
	brief := filepath.Join(jobDir, "brief.md")
	os.WriteFile(brief, []byte("# Brief: X\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)

	// Simulate the editor having saved new content, then report success.
	os.WriteFile(brief, []byte("# Brief: X\n\nstatus: open\n\nedited by nano\n"), 0o644)

	model, cmd := a.Update(editorDoneMsg{path: brief, err: nil})
	got := model.(*App)

	if got.detail.status != "edited brief.md" {
		t.Errorf("status = %q, want %q", got.detail.status, "edited brief.md")
	}
	if !strings.Contains(got.detail.tabs[0].viewer.View(), "edited by nano") {
		t.Errorf("tab was not reloaded after the editor exited")
	}
	if cmd == nil {
		t.Fatal("expected a follow-up auto-commit cmd for brief.md, got nil")
	}
}

// TestEditorDoneMsgAutoCommitsBrief verifies that a successful brief.md edit
// is committed automatically in the job's own worktree, with a subject
// following the same "[ID] <type>: <summary>" convention every agent's own
// commits use, so a job's brief never lingers as an uncommitted change the
// way finish-job.sh's clean-tree check would otherwise reject. The commit
// must land on the job worktree's branch (git -C <worktree>), not the main
// worktree — see commitBriefCmd's doc.
func TestEditorDoneMsgAutoCommitsBrief(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/ab0001_x", "ab0001_x", "# Brief: X\n\nstatus: open\nid: ab0001\nbranch: feature/ab0001_x\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%d", err, len(jobs))
	}

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)

	brief := filepath.Join(wtPath, "docs", "jobs", "ab0001_x", "brief.md")
	os.WriteFile(brief, []byte("# Brief: X\n\nstatus: open\nid: ab0001\nbranch: feature/ab0001_x\n\nedited by nano\n"), 0o644)

	_, cmd := a.Update(editorDoneMsg{path: brief, err: nil})
	if cmd == nil {
		t.Fatal("expected a follow-up auto-commit cmd, got nil")
	}

	msg := cmd()
	commitM, ok := msg.(commitMsg)
	if !ok {
		t.Fatalf("expected commitMsg, got %T", msg)
	}
	if commitM.err != nil {
		t.Fatalf("auto-commit failed: %v", commitM.err)
	}

	model, _ := a.Update(commitM)
	got := model.(*App)
	if got.detail.status != "edited and committed brief.md" {
		t.Errorf("status = %q, want %q", got.detail.status, "edited and committed brief.md")
	}

	// The commit must be in the job worktree's history (on the job branch),
	// and the main worktree must be untouched.
	out, err := exec.Command("git", "-C", wtPath, "log", "-1", "--format=%s").Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if subject := strings.TrimSpace(string(out)); subject != "[ab0001] brief: edit via TUI" {
		t.Errorf("commit subject = %q, want %q", subject, "[ab0001] brief: edit via TUI")
	}

	statusOut, err := exec.Command("git", "-C", wtPath, "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if len(statusOut) != 0 {
		t.Errorf("expected a clean job worktree after auto-commit, got:\n%s", statusOut)
	}

	mainLog, err := exec.Command("git", "-C", dir, "log", "--format=%s", "-1").Output()
	if err != nil {
		t.Fatalf("git log (main): %v", err)
	}
	if strings.Contains(string(mainLog), "[ab0001] brief: edit via TUI") {
		t.Errorf("auto-commit leaked into the main worktree's history:\n%s", mainLog)
	}
}

// TestEditorDoneMsgCreatesMissingFile verifies that editing a tab whose file
// doesn't exist yet (shown as its "(label)" placeholder) flips over to real
// content once the editor creates it on save — the same reload path used for
// an existing file, just starting from a placeholder instead.
func TestEditorDoneMsgCreatesMissingFile(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0003_z")
	os.MkdirAll(jobDir, 0o755)
	// No brief.md written yet — job.Discover/ReadJob tolerate that (see
	// job.ReadJob's docs), returning defaults.

	jobs, err := job.Discover(root)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: %v jobs=%d", err, len(jobs))
	}

	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)

	brief := filepath.Join(jobDir, "brief.md")
	if a.detail.tabs[0].exists {
		t.Fatal("brief.md tab should start out as the not-written placeholder")
	}
	if !strings.Contains(a.detail.tabs[0].viewer.View(), "has not been written yet") {
		t.Errorf("expected placeholder content before the file exists, got:\n%s", a.detail.tabs[0].viewer.View())
	}

	// Simulate the editor creating brief.md on save.
	os.WriteFile(brief, []byte("# Brief: Z\n\nstatus: open\n"), 0o644)

	model, _ := a.Update(editorDoneMsg{path: brief, err: nil})
	got := model.(*App)

	if !got.detail.tabs[0].exists {
		t.Errorf("tab still marked as not existing after the editor created the file")
	}
	if strings.Contains(got.detail.tabs[0].viewer.View(), "has not been written yet") {
		t.Errorf("tab still shows the placeholder after the editor created the file")
	}
}

// TestEditorDoneMsgError verifies a failed edit (editor not found, or exited
// non-zero) surfaces through the same cmdErrorText formatting the agent
// launch errors use, rather than a bespoke message.
func TestEditorDoneMsgError(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "docs", "jobs", "ab0001_x")
	os.MkdirAll(jobDir, 0o755)
	os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: X\n\nstatus: open\n"), 0o644)

	jobs, _ := job.Discover(root)
	a := NewApp(root, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(a.jobs[0], 80, 24)

	wantErr := errors.New("exit status 127")
	model, _ := a.Update(editorDoneMsg{path: filepath.Join(jobDir, "brief.md"), err: wantErr})
	got := model.(*App)

	want := cmdErrorText(wantErr)
	if got.detail.status != want {
		t.Errorf("status = %q, want %q", got.detail.status, want)
	}
}
