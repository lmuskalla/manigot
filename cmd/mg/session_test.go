package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/session"
)

// pathWithRealGitOnly returns a PATH value containing git but nothing else,
// so the docker launch inside the session flow fails deterministically —
// never starting a real container on a machine that has docker, and failing
// with the same clear error on one that doesn't. The real git (not the
// manigot git shim, which may sit on PATH and refuses some session-flow git
// calls) is resolved explicitly.
func pathWithRealGitOnly(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	realGit := "/usr/bin/git"
	if _, err := os.Stat(realGit); err != nil {
		var lerr error
		realGit, lerr = exec.LookPath("git")
		if lerr != nil {
			t.Fatalf("could not locate a real git: %v", lerr)
		}
	}
	if err := os.Symlink(realGit, filepath.Join(binDir, "git")); err != nil {
		t.Fatal(err)
	}
	return binDir
}

// fakeDockerOnPath returns a PATH value with real git plus a fake `docker`
// executable that runs body (default "exit 0") — so the session flow's
// docker invocation "runs" without a real container, letting the post-run
// sweep logic be exercised end to end (the launch-path prune also shells out
// to docker; an exit-0 docker with no output makes it a quiet success).
func fakeDockerOnPath(t *testing.T, body string) string {
	t.Helper()
	binDir := pathWithRealGitOnly(t)
	if body == "" {
		body = "exit 0"
	}
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binDir
}

// sweepTestJob builds a real worktree job (the mg job flow) and returns the
// project root, the job name, and the job worktree path — the scaffolding the
// sweep-wiring tests share. Callers set PATH first (fakeDockerOnPath or
// pathWithRealGitOnly) and chdir into the returned root.
func sweepTestJob(t *testing.T) (dir, jobName, wt string) {
	t.Helper()
	dir = mgCheckout(t)
	t.Chdir(dir)
	var out strings.Builder
	if code := runJob([]string{"Sweep Job"}, &out, &strings.Builder{}); code != 0 {
		t.Fatalf("runJob: %d %s", code, out.String())
	}
	branch := mgJobBranch(t, dir)
	jobName = strings.TrimPrefix(branch, "feature/")
	wt = mgWorktreePath(t, dir, branch)
	return dir, jobName, wt
}

// TestRunSessionPruneCalledBeforeRun pins the launch-path wiring: runSession
// calls pruneOrphans (stubbed — docker is not on the test PATH) before the
// docker run, and a successful prune is quiet. The run then proceeds to the
// docker launch, which fails on the git-only PATH — proving the prune
// happened before the run and did not replace it.
func TestRunSessionPruneCalledBeforeRun(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	dir := mgCheckout(t)
	t.Chdir(dir)
	profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")

	var pruneCalls int
	old := pruneOrphans
	pruneOrphans = func(diag io.Writer) (session.PruneResult, error) {
		pruneCalls++
		return session.PruneResult{Removed: 1, Running: 0}, nil
	}
	t.Cleanup(func() { pruneOrphans = old })

	var out, errOut strings.Builder
	if code := runSession(nil, os.Stdin, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1 (the docker launch fails on the git-only PATH)", code)
	}
	if pruneCalls != 1 {
		t.Errorf("pruneOrphans called %d times, want 1", pruneCalls)
	}
	if !strings.Contains(errOut.String(), "executable file not found") {
		t.Errorf("the docker launch must have been attempted after the prune:\n%s", errOut.String())
	}
	if strings.Contains(errOut.String(), "could not prune") {
		t.Errorf("a successful prune must not warn:\n%s", errOut.String())
	}
}

// TestRunSessionPruneFailureIsFailSoft pins the fail-soft contract: a prune
// failure only warns on stderr and never aborts the launch — the run proceeds
// to the docker launch (which fails on the git-only PATH) despite the prune
// error.
func TestRunSessionPruneFailureIsFailSoft(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	dir := mgCheckout(t)
	t.Chdir(dir)
	profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")

	var pruneCalls int
	old := pruneOrphans
	pruneOrphans = func(diag io.Writer) (session.PruneResult, error) {
		pruneCalls++
		return session.PruneResult{}, errors.New("Cannot connect to the Docker daemon")
	}
	t.Cleanup(func() { pruneOrphans = old })

	var out, errOut strings.Builder
	if code := runSession(nil, os.Stdin, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1 (the docker launch fails on the git-only PATH)", code)
	}
	if pruneCalls != 1 {
		t.Errorf("pruneOrphans called %d times, want 1", pruneCalls)
	}
	if !strings.Contains(errOut.String(), "mg: warning: could not prune orphaned containers") {
		t.Errorf("missing the prune warning:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "executable file not found") {
		t.Errorf("the launch must proceed past a prune failure (docker attempted):\n%s", errOut.String())
	}
}

// TestRunSessionSweepsJobWorktree pins the job-worktree sweep wiring at the
// session level: after a job-worktree session whose container "ran" (a fake
// docker that exits 0), whatever the agent left uncommitted in the job
// worktree is swept into a single [<id>] chore: commit all commit and the
// worktree is left clean.
func TestRunSessionSweepsJobWorktree(t *testing.T) {
	t.Setenv("PATH", fakeDockerOnPath(t, ""))
	dir, jobName, wt := sweepTestJob(t)
	profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")

	// The "agent" leaves a leftover in the job worktree (the analyst's
	// tasks.md shape — written, never committed).
	if err := os.WriteFile(filepath.Join(wt, "docs", "jobs", jobName, "tasks.md"), []byte("# Tasks\n\nTASK-1: do the thing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut strings.Builder
	if code := runSession([]string{"--job", jobName}, os.Stdin, &out, &errOut); code != 0 {
		t.Fatalf("runSession exit code = %d, stderr:\n%s", code, errOut.String())
	}
	if want := "[" + strings.Split(jobName, "_")[0] + "] chore: commit all"; mgGit(t, wt, "log", "-1", "--format=%s") != want {
		t.Errorf("sweep commit subject = %q, want %q", mgGit(t, wt, "log", "-1", "--format=%s"), want)
	}
	if status := mgGit(t, wt, "status", "--porcelain"); status != "" {
		t.Errorf("job worktree not clean after sweep:\n%s", status)
	}
	// The plain session at the project root must not be swept (no --job).
	if status := mgGit(t, dir, "status", "--porcelain"); status != "" {
		t.Errorf("project root must be untouched by a job-worktree sweep:\n%s", status)
	}
}

// TestRunSessionDoesNotSweepWhenDockerFailed pins the "no sweep when no agent
// ran" contract at the session level: when the docker launch itself fails
// (docker missing on the git-only PATH), the leftover in the job worktree is
// left exactly as it was — an agent that never started must not trigger a
// commit.
func TestRunSessionDoesNotSweepWhenDockerFailed(t *testing.T) {
	t.Setenv("PATH", pathWithRealGitOnly(t))
	_, jobName, wt := sweepTestJob(t)
	profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")

	leftover := filepath.Join(wt, "docs", "jobs", jobName, "tasks.md")
	if err := os.WriteFile(leftover, []byte("# Tasks\n\nTASK-1: do the thing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut strings.Builder
	if code := runSession([]string{"--job", jobName}, os.Stdin, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1 (the docker launch fails on the git-only PATH)", code)
	}
	if got := mgGit(t, wt, "log", "-1", "--format=%s"); strings.HasPrefix(got, "[") {
		t.Errorf("a sweep commit was made despite docker never running (head = %q)", got)
	}
	if status := mgGit(t, wt, "status", "--porcelain"); !strings.Contains(status, "tasks.md") {
		t.Errorf("leftover must remain uncommitted after a failed launch, status:\n%s", status)
	}
}

// TestRunSessionDoesNotSweepFlatScanFallback pins the TASK-3 blocker fix at
// the session level: the --job flat-scan fallback — a git repo with no local
// branches (here: a fresh git init before its first commit), where the job's
// files live directly in the main project root — must never be swept, even
// when the container "ran" (a fake docker that exits 0). Sweeping there would
// run git add -A over the user's own uncommitted work — .env included — and
// create the repo's first commit. The repo keeps its unborn HEAD and the
// leftovers stay untracked.
func TestRunSessionDoesNotSweepFlatScanFallback(t *testing.T) {
	t.Setenv("PATH", fakeDockerOnPath(t, ""))
	dir := t.TempDir()
	mgGit(t, dir, "init", "-q")
	mgGit(t, dir, "config", "user.name", "Test")
	mgGit(t, dir, "config", "user.email", "test@x.io")
	if err := os.MkdirAll(filepath.Join(dir, "docs", "jobs", "ab12cd_flat-scan"), 0o755); err != nil {
		t.Fatal(err)
	}
	profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")

	// The user's own uncommitted work, .env included — the exact contents the
	// flat-scan fallback must never sweep into the repo's first commit.
	if err := os.WriteFile(filepath.Join(dir, "work.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=token\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	var out, errOut strings.Builder
	if code := runSession([]string{"--job", "ab12cd_flat-scan"}, os.Stdin, &out, &errOut); code != 0 {
		t.Fatalf("runSession exit code = %d, stderr:\n%s", code, errOut.String())
	}
	if err := exec.Command("git", "-C", dir, "rev-parse", "--verify", "HEAD").Run(); err == nil {
		t.Error("flat-scan fallback must not create the repo's first commit")
	}
	status := mgGit(t, dir, "status", "--porcelain")
	if !strings.Contains(status, "work.txt") || !strings.Contains(status, ".env") {
		t.Errorf("leftovers must remain untracked after a flat-scan no-op, status:\n%s", status)
	}
	if strings.Contains(errOut.String(), "chore: commit all") {
		t.Errorf("no sweep note may appear for a flat-scan fallback:\n%s", errOut.String())
	}
}

// TestRunSessionDoesNotSweepPreWorktreeJob pins the second TASK-3 blocker
// shape at the session level: a pre-worktree job — its branch checked out in
// the main worktree itself, an explicitly supported transitional state (see
// internal/job/discover.go:76-82) — resolves --job to the main project root
// (git.WorktreeForBranch matches the main worktree's own porcelain entry), so
// it must never be swept either, even though GitCommonDir resolves non-empty
// there (the repo's own .git — unlike the flat-scan fallback, where
// GitCommonDir stays ""). Sweeping would commit the user's own uncommitted
// work — .env included — onto the job branch.
func TestRunSessionDoesNotSweepPreWorktreeJob(t *testing.T) {
	t.Setenv("PATH", fakeDockerOnPath(t, ""))
	dir := mgCheckout(t)
	t.Chdir(dir)
	mgGit(t, dir, "checkout", "-q", "-b", "feature/ab12cd_main-wt")
	jobDir := filepath.Join(dir, "docs", "jobs", "ab12cd_main-wt")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief\n\nstatus: open\nid: ab12cd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mgGit(t, dir, "add", "-A")
	mgGit(t, dir, "commit", "-q", "-m", "[ab12cd] scaffold")
	profileCheckout(t, "CLAUDE_CODE_OAUTH_TOKEN=t\nCLAUDE_ACCOUNT_UUID=u\nCLAUDE_EMAIL=e\nCLAUDE_ORG_UUID=o\n")

	// The user's own uncommitted work, .env included — must never be swept
	// onto the job branch.
	if err := os.WriteFile(filepath.Join(dir, "work.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=token\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut strings.Builder
	if code := runSession([]string{"--job", "ab12cd_main-wt"}, os.Stdin, &out, &errOut); code != 0 {
		t.Fatalf("runSession exit code = %d, stderr:\n%s", code, errOut.String())
	}
	if got := mgGit(t, dir, "log", "-1", "--format=%s"); got != "[ab12cd] scaffold" {
		t.Errorf("a sweep commit was made over the pre-worktree job's main worktree (head = %q)", got)
	}
	status := mgGit(t, dir, "status", "--porcelain")
	if !strings.Contains(status, "work.txt") || !strings.Contains(status, ".env") {
		t.Errorf("leftovers must remain untracked after a pre-worktree-job no-op, status:\n%s", status)
	}
	if strings.Contains(errOut.String(), "chore: commit all") {
		t.Errorf("no sweep note may appear for a pre-worktree job:\n%s", errOut.String())
	}
}
