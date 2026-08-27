package job

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeleteJobFreshlyCreated(t *testing.T) {
	root, res := createWorkedJob(t)

	var out bytes.Buffer
	del, err := DeleteJob(root, "ab12cd", yesConfirm, &out)
	if err != nil {
		t.Fatalf("DeleteJob: %v\n%s", err, out.String())
	}
	if del.JobName != "ab12cd_roundtrip-job" || del.Branch != "feature/ab12cd_roundtrip-job" {
		t.Errorf("DeleteResult = %+v", del)
	}

	// Worktree removed, branch deleted, main worktree untouched.
	if _, err := os.Stat(res.WorktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree still exists after delete")
	}
	if ok, _ := gitExists(root, "feature/ab12cd_roundtrip-job"); ok {
		t.Error("branch still exists after delete")
	}
	if cur := gitCmd(t, root, "rev-parse", "--abbrev-ref", "HEAD"); cur != "main" {
		t.Errorf("main worktree on %q, want main (delete must not touch it)", cur)
	}
	if outStr := out.String(); !strings.Contains(outStr, "✓ Job deleted: ab12cd_roundtrip-job") {
		t.Errorf("missing delete line:\n%s", outStr)
	}
}

func TestDeleteJobDeclined(t *testing.T) {
	root, res := createWorkedJob(t)

	var out bytes.Buffer
	_, err := DeleteJob(root, "ab12cd", noConfirm, &out)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("declined delete: err = %v, want ErrCancelled", err)
	}
	// Nothing happened.
	if _, err := os.Stat(res.WorktreePath); err != nil {
		t.Errorf("worktree removed despite a declined confirmation: %v", err)
	}
	if ok, _ := gitExists(root, "feature/ab12cd_roundtrip-job"); !ok {
		t.Error("branch deleted despite a declined confirmation")
	}
}

func TestDeleteJobDirtyWorktreeWarning(t *testing.T) {
	root, res := createWorkedJob(t)
	// Dirty the worktree.
	writeTestFile(t, filepath.Join(res.Job.Dir, "tasks.md"), "uncommitted edit\n")

	var out bytes.Buffer
	if _, err := DeleteJob(root, "ab12cd", yesConfirm, &out); err != nil {
		t.Fatalf("DeleteJob with dirty worktree: %v", err)
	}
	if !strings.Contains(out.String(), "  Warning  : this worktree has uncommitted changes — they will be discarded.") {
		t.Errorf("missing dirty-worktree warning:\n%s", out.String())
	}
	// The dirty worktree was force-removed anyway.
	if _, err := os.Stat(res.WorktreePath); !os.IsNotExist(err) {
		t.Errorf("dirty worktree still exists after delete")
	}
}

func TestDeleteJobRemovesJDISidecar(t *testing.T) {
	root, res := createWorkedJob(t)
	// A job mg-jdi previously drove: status sidecar + run.log, both outside
	// the job dir, under the project's .manigot/jdi-status/.
	if err := WriteJDIStatus(root, res.Job.Name, JDIStoppedFinished, "reviewer"); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, JDIRunLogPath(root, res.Job.Name), "=== mg jdi started ===\n")

	var out bytes.Buffer
	if _, err := DeleteJob(root, "ab12cd", yesConfirm, &out); err != nil {
		t.Fatalf("DeleteJob: %v\n%s", err, out.String())
	}
	// The sidecar is gone with the job.
	if _, err := os.Stat(JDIStatusDir(root, res.Job.Name)); !os.IsNotExist(err) {
		t.Errorf("mg-jdi sidecar still exists after delete: %v", err)
	}
	if !strings.Contains(out.String(), "→ Removing mg-jdi status for "+res.Job.Name+"...") {
		t.Errorf("missing sidecar-removal line:\n%s", out.String())
	}
}

func TestDeleteJobNoJDISidecarPrintsNothing(t *testing.T) {
	root, _ := createWorkedJob(t)
	// The common case: mg-jdi never drove this job, so no sidecar exists and
	// the output must not claim any sidecar cleanup.
	var out bytes.Buffer
	if _, err := DeleteJob(root, "ab12cd", yesConfirm, &out); err != nil {
		t.Fatalf("DeleteJob: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "mg-jdi status") {
		t.Errorf("sidecar-removal output printed with no sidecar present:\n%s", out.String())
	}
}

func TestDeleteJobMainWorktreeCase(t *testing.T) {
	// A pre-worktree job: the branch is checked out in the main worktree, so
	// the main worktree is switched onto the base branch and no worktree
	// removal happens.
	root := createCheckout(t, t.TempDir())
	gitCmd(t, root, "checkout", "-q", "-b", "feature/ef34gh_main-wt")
	jobDir := filepath.Join(root, "docs", "jobs", "ef34gh_main-wt")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(jobDir, "brief.md"), "# Brief: Main WT\n\nstatus: open\nid: ef34gh\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "[ef34gh] work")

	var out bytes.Buffer
	del, err := DeleteJob(root, "ef34gh", yesConfirm, &out)
	if err != nil {
		t.Fatalf("DeleteJob (main-worktree case): %v\n%s", err, out.String())
	}
	if del.Branch != "feature/ef34gh_main-wt" {
		t.Errorf("branch = %q", del.Branch)
	}
	if !strings.Contains(out.String(), "→ Job's worktree is the main worktree — skipping worktree removal.") {
		t.Errorf("missing skip-removal line:\n%s", out.String())
	}
	if cur := gitCmd(t, root, "rev-parse", "--abbrev-ref", "HEAD"); cur != "main" {
		t.Errorf("main worktree on %q, want main", cur)
	}
	if ok, _ := gitExists(root, "feature/ef34gh_main-wt"); ok {
		t.Error("branch still exists")
	}
}

func TestDeleteJobNonGit(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "jobs", "jk56lm_plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "docs", "jobs", "jk56lm_plain", "brief.md"), "# Brief: Plain\n\nstatus: open\nid: jk56lm\n")
	// A sidecar mg-jdi left for the same job, in the project's .manigot/.
	if err := WriteJDIStatus(root, "jk56lm_plain", JDIStoppedNeedsHuman, "analyst"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	del, err := DeleteJob(root, "jk56lm", yesConfirm, &out)
	if err != nil {
		t.Fatalf("DeleteJob (non-git): %v\n%s", err, out.String())
	}
	if del.JobName != "jk56lm_plain" || del.Branch != "" {
		t.Errorf("non-git result = %+v", del)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "jobs", "jk56lm_plain")); !os.IsNotExist(err) {
		t.Errorf("non-git job dir still exists")
	}
	if _, err := os.Stat(JDIStatusDir(root, "jk56lm_plain")); !os.IsNotExist(err) {
		t.Errorf("mg-jdi sidecar still exists after non-git delete: %v", err)
	}
	if !strings.Contains(out.String(), "✓ Job deleted: jk56lm_plain") {
		t.Errorf("missing delete line:\n%s", out.String())
	}
}

func TestDeleteJobNonGitPrefixMatch(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "jobs", "jk56lm_alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	// archive/ is never matched.
	if err := os.MkdirAll(filepath.Join(root, "docs", "jobs", "archive", "jk56lm_old"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	del, err := DeleteJob(root, "jk56lm", yesConfirm, &out)
	if err != nil {
		t.Fatalf("DeleteJob (non-git prefix): %v", err)
	}
	if del.JobName != "jk56lm_alpha" {
		t.Errorf("prefix match = %q, want jk56lm_alpha", del.JobName)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "jobs", "archive", "jk56lm_old")); err != nil {
		t.Errorf("archive job was deleted: %v", err)
	}
}

func TestDeleteJobNonGitNotFound(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_, err := DeleteJob(root, "zzzz99", yesConfirm, &out)
	if err == nil || !strings.Contains(err.Error(), "job 'zzzz99' not found under docs/jobs/") {
		t.Errorf("not-found error = %v", err)
	}
}

func TestDeleteJobNotFound(t *testing.T) {
	root := createCheckout(t, t.TempDir())
	var out bytes.Buffer
	_, err := DeleteJob(root, "zzzz99", yesConfirm, &out)
	if err == nil || !strings.Contains(err.Error(), "job 'zzzz99' not found among local branches.") {
		t.Errorf("not-found error = %v", err)
	}
}
