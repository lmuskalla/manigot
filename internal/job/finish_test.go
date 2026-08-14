package job

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// yesConfirm answers every prompt with yes.
func yesConfirm(prompt string) (bool, error) { return true, nil }

// noConfirm answers every prompt with no.
func noConfirm(prompt string) (bool, error) { return false, nil }

// recordingConfirm answers with yes and records every prompt asked.
func recordingConfirm(recorded *[]string) ConfirmFunc {
	return func(prompt string) (bool, error) {
		*recorded = append(*recorded, prompt)
		return true, nil
	}
}

// createWorkedJob creates a job and adds some work (implementation.md +
// verdict.md + commits), returning the create result. The job's worktree is
// left clean and on the job branch.
func createWorkedJob(t *testing.T) (root string, res CreateResult) {
	t.Helper()
	root = createCheckout(t, t.TempDir())
	var out bytes.Buffer
	res, err := CreateJob(root, "Roundtrip Job", CreateOptions{RandomID: fixedID("ab12cd")}, &out)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	jobDir := res.Job.Dir
	writeTestFile(t, filepath.Join(jobDir, "implementation.md"), "# Implementation: Roundtrip Job\n\n## Summary\n\nDone.\n")
	writeTestFile(t, filepath.Join(jobDir, "tasks.md"), "# Tasks: Roundtrip Job\n\nTASK-1: implement\n")
	writeTestFile(t, filepath.Join(jobDir, "verdict.md"), "# Verdict: Roundtrip Job\n\n## Overall\n\nAPPROVED\n")
	gitCmd(t, res.WorktreePath, "add", "-A")
	gitCmd(t, res.WorktreePath, "commit", "-q", "-m", "[ab12cd] implementation: add summary")
	return root, res
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFinishJobFullRoundtrip(t *testing.T) {
	root, res := createWorkedJob(t)

	var out bytes.Buffer
	fin, err := FinishJob(root, "ab12cd", yesConfirm, &out)
	if err != nil {
		t.Fatalf("FinishJob: %v\n%s", err, out.String())
	}
	if fin.JobName != "ab12cd_roundtrip-job" || fin.Branch != "feature/ab12cd_roundtrip-job" || fin.BaseBranch != "main" {
		t.Errorf("FinishResult = %+v", fin)
	}

	// Branch deleted.
	if ok, _ := gitExists(root, "feature/ab12cd_roundtrip-job"); ok {
		t.Error("job branch still exists after FinishJob")
	}
	// Main worktree back on the base branch.
	if cur := gitCmd(t, root, "rev-parse", "--abbrev-ref", "HEAD"); cur != "main" {
		t.Errorf("main worktree on %q, want main", cur)
	}
	// Job worktree removed.
	if _, err := os.Stat(res.WorktreePath); !os.IsNotExist(err) {
		t.Errorf("job worktree still exists after FinishJob")
	}
	// The job's work is on the base branch, archived under docs/jobs/archive/.
	archived := filepath.Join(root, "docs", "jobs", "archive", "ab12cd_roundtrip-job")
	if _, err := os.Stat(filepath.Join(archived, "implementation.md")); err != nil {
		t.Errorf("archived implementation.md missing: %v", err)
	}
	brief, _ := os.ReadFile(filepath.Join(archived, "brief.md"))
	if !strings.Contains(string(brief), "status: done") {
		t.Errorf("archived brief not marked done:\n%s", brief)
	}
	// One squash commit on main carrying the whole job.
	log := gitCmd(t, root, "log", "--oneline", "main")
	if n := len(strings.Split(strings.TrimSpace(log), "\n")); n != 2 {
		t.Errorf("main has %d commits after finish, want 2 (init + squash):\n%s", n, log)
	}
	if outStr := out.String(); !strings.Contains(outStr, "✓ Job finished: ab12cd_roundtrip-job") {
		t.Errorf("missing finish line:\n%s", outStr)
	}
}

func TestFinishJobRemovesJDISidecar(t *testing.T) {
	root, res := createWorkedJob(t)
	// A job mg-jdi previously drove: status sidecar + run.log under the
	// project's .manigot/jdi-status/.
	if err := WriteJDIStatus(root, res.Job.Name, JDIStoppedFinished, "reviewer"); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, JDIRunLogPath(root, res.Job.Name), "=== mg jdi started ===\n")

	var out bytes.Buffer
	if _, err := FinishJob(root, "ab12cd", yesConfirm, &out); err != nil {
		t.Fatalf("FinishJob: %v\n%s", err, out.String())
	}
	// The sidecar is gone with the finished job — the archive keeps the job's
	// docs, and mg-jdi never runs against an archived job.
	if _, err := os.Stat(JDIStatusDir(root, res.Job.Name)); !os.IsNotExist(err) {
		t.Errorf("mg-jdi sidecar still exists after finish: %v", err)
	}
	if !strings.Contains(out.String(), "→ Removing mg-jdi status for "+res.Job.Name+"...") {
		t.Errorf("missing sidecar-removal line:\n%s", out.String())
	}
}

func TestFinishJobDeclinedProceed(t *testing.T) {
	root, res := createWorkedJob(t)
	defer os.RemoveAll(res.WorktreePath) // cleanup: nothing was done

	var out bytes.Buffer
	_, err := FinishJob(root, "ab12cd", noConfirm, &out)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("declined proceed: err = %v, want ErrCancelled", err)
	}
	// Nothing happened: branch + worktree intact.
	if ok, _ := gitExists(root, "feature/ab12cd_roundtrip-job"); !ok {
		t.Error("branch was deleted despite a declined confirmation")
	}
}

func TestFinishJobNotApprovedVerdictWarns(t *testing.T) {
	root, res := createWorkedJob(t)
	// Overwrite the verdict with a NEEDS WORK one and commit.
	writeTestFile(t, filepath.Join(res.Job.Dir, "verdict.md"), "# Verdict\n\n## Overall\n\nNEEDS WORK — TASK-1 incomplete\n")
	gitCmd(t, res.WorktreePath, "add", "-A")
	gitCmd(t, res.WorktreePath, "commit", "-q", "-m", "[ab12cd] verdict: needs work")

	var prompts []string
	var out bytes.Buffer
	_, err := FinishJob(root, "ab12cd", recordingConfirm(&prompts), &out)
	if err != nil {
		t.Fatalf("FinishJob: %v", err)
	}
	if !strings.Contains(out.String(), "Warning: verdict is 'NEEDS WORK — TASK-1 incomplete' — job is not approved.") {
		t.Errorf("missing not-approved warning:\n%s", out.String())
	}
	if len(prompts) < 2 || prompts[0] != "  Continue anyway? [y/N] " || prompts[1] != "  Proceed? [y/N] " {
		t.Errorf("prompts = %v", prompts)
	}
}

func TestFinishJobNoVerdictWarns(t *testing.T) {
	root, res := createWorkedJob(t)
	os.Remove(filepath.Join(res.Job.Dir, "verdict.md"))
	gitCmd(t, res.WorktreePath, "add", "-A")
	gitCmd(t, res.WorktreePath, "commit", "-q", "-m", "[ab12cd] drop verdict")

	var prompts []string
	var out bytes.Buffer
	if _, err := FinishJob(root, "ab12cd", recordingConfirm(&prompts), &out); err != nil {
		t.Fatalf("FinishJob: %v", err)
	}
	if !strings.Contains(out.String(), "Warning: no verdict.md found — job has not been reviewed.") {
		t.Errorf("missing no-verdict warning:\n%s", out.String())
	}
	if len(prompts) < 1 || prompts[0] != "  Continue anyway? [y/N] " {
		t.Errorf("prompts = %v", prompts)
	}
}

func TestFinishJobDirtyWorktreeRejected(t *testing.T) {
	root, res := createWorkedJob(t)
	// Uncommitted change in the worktree.
	writeTestFile(t, filepath.Join(res.Job.Dir, "tasks.md"), "uncommitted edit\n")

	var out bytes.Buffer
	_, err := FinishJob(root, "ab12cd", yesConfirm, &out)
	if err == nil || !strings.Contains(err.Error(), "uncommitted changes in the worktree for branch 'feature/ab12cd_roundtrip-job'. Commit or stash before finishing.") {
		t.Errorf("dirty-tree error = %v", err)
	}
}

func TestFinishJobMainWorktreeCase(t *testing.T) {
	// A pre-worktree job: the job's branch is checked out in the main
	// worktree itself, so WorktreeForBranch resolves to the main worktree and
	// the removal step must be skipped.
	root := createCheckout(t, t.TempDir())
	gitCmd(t, root, "checkout", "-q", "-b", "feature/ef34gh_main-wt")
	jobDir := filepath.Join(root, "docs", "jobs", "ef34gh_main-wt")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(jobDir, "brief.md"), "# Brief: Main WT\n\nstatus: open\nid: ef34gh\n")
	writeTestFile(t, filepath.Join(jobDir, "tasks.md"), "# Tasks\n\nTASK-1\n")
	writeTestFile(t, filepath.Join(jobDir, "implementation.md"), "# Implementation\n\nDone.\n")
	writeTestFile(t, filepath.Join(jobDir, "verdict.md"), "# Verdict\n\n## Overall\n\nAPPROVED\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "[ef34gh] work")

	var out bytes.Buffer
	fin, err := FinishJob(root, "ef34gh", yesConfirm, &out)
	if err != nil {
		t.Fatalf("FinishJob (main-worktree case): %v\n%s", err, out.String())
	}
	if fin.BaseBranch != "main" {
		t.Errorf("base branch = %q", fin.BaseBranch)
	}
	if !strings.Contains(out.String(), "→ Worktree is the main worktree — skipping worktree removal.") {
		t.Errorf("missing skip-removal line:\n%s", out.String())
	}
	// The main worktree was switched onto the base branch.
	if cur := gitCmd(t, root, "rev-parse", "--abbrev-ref", "HEAD"); cur != "main" {
		t.Errorf("main worktree on %q, want main", cur)
	}
	// Branch deleted; work archived on main.
	if ok, _ := gitExists(root, "feature/ef34gh_main-wt"); ok {
		t.Error("job branch still exists")
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "jobs", "archive", "ef34gh_main-wt", "implementation.md")); err != nil {
		t.Errorf("archived work missing: %v", err)
	}
}

func TestFinishJobJobNotFound(t *testing.T) {
	root := createCheckout(t, t.TempDir())
	var out bytes.Buffer
	_, err := FinishJob(root, "zzzz99", yesConfirm, &out)
	if err == nil || !strings.Contains(err.Error(), "job 'zzzz99' not found among local branches.") {
		t.Errorf("not-found error = %v", err)
	}
	if !strings.Contains(err.Error(), "Active job branches:") {
		t.Errorf("not-found error missing branch listing: %v", err)
	}
}

func TestFinishJobAmbiguous(t *testing.T) {
	root := createCheckout(t, t.TempDir())
	gitCmd(t, root, "branch", "feature/ab12cd_alpha")
	gitCmd(t, root, "branch", "feature/ab12cd_beta")
	// A branch with no worktree can't be finished — but ambiguity is reported
	// first, matching the script's resolution order.
	var out bytes.Buffer
	_, err := FinishJob(root, "ab12cd", yesConfirm, &out)
	if err == nil || !strings.Contains(err.Error(), "job 'ab12cd' is ambiguous — matches branches:") {
		t.Errorf("ambiguity error = %v", err)
	}
}

func TestBriefTitle(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "brief.md"), "# Brief: My Job Title\n\nstatus: open\n")
	if got := briefTitle(filepath.Join(dir, "brief.md")); got != "My Job Title" {
		t.Errorf("briefTitle = %q", got)
	}
	// A first line that isn't a Brief heading stays as-is (sed's no-match
	// behavior).
	writeTestFile(t, filepath.Join(dir, "brief.md"), "Some other first line\n")
	if got := briefTitle(filepath.Join(dir, "brief.md")); got != "Some other first line" {
		t.Errorf("briefTitle (no heading) = %q", got)
	}
}

// gitExists reports whether the local branch ref exists.
func gitExists(root, branch string) (bool, error) {
	_, err := execGit(root, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}
	return false, nil
}

// execGit runs `git -C root args` and returns trimmed combined output.
func execGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
