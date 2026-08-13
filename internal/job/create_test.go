package job

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmuskalla/manigot/internal/fs"
	"github.com/lmuskalla/manigot/internal/git"
)

// gitCmd runs git -C dir args, failing the test on error, and returns trimmed
// stdout.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// createCheckout builds a scratch git repo at dir with a docs/ dir and an
// initial commit — the project new-job.sh would operate on.
func createCheckout(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "init", "-q", "-b", "main")
	gitCmd(t, dir, "config", "user.name", "Test")
	gitCmd(t, dir, "config", "user.email", "test@x.io")
	if err := os.WriteFile(filepath.Join(dir, "docs", "jobs", ".gitkeep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-q", "-m", "init")
	return dir
}

// fixedID forces CreateJob to use the given id.
func fixedID(id string) func() (string, error) {
	return func() (string, error) { return id, nil }
}

func TestCreateJobFullRoundtrip(t *testing.T) {
	dir := createCheckout(t, t.TempDir())
	var out bytes.Buffer
	res, err := CreateJob(dir, "Add Gallery Block", CreateOptions{RandomID: fixedID("ab12cd")}, &out)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Branch + worktree, laid out as a sibling of the project root.
	if res.Branch != "feature/ab12cd_add-gallery-block" {
		t.Errorf("branch = %q", res.Branch)
	}
	wantWT := filepath.Join(filepath.Dir(dir), ".manigot-worktrees", filepath.Base(dir), "ab12cd_add-gallery-block")
	if res.WorktreePath != wantWT {
		t.Errorf("worktree = %q, want %q", res.WorktreePath, wantWT)
	}
	if res.Job.Name != "ab12cd_add-gallery-block" || res.Job.Dir != filepath.Join(wantWT, "docs", "jobs", "ab12cd_add-gallery-block") {
		t.Errorf("job = %+v", res.Job)
	}

	// The branch exists, points at base, and the worktree resolves to it.
	if ok, err := git.RefExists(dir, "feature/ab12cd_add-gallery-block"); err != nil || !ok {
		t.Fatalf("job branch exists: ok=%v err=%v", ok, err)
	}
	wt, ok, err := git.WorktreeForBranch(dir, "feature/ab12cd_add-gallery-block")
	if err != nil || !ok {
		t.Fatalf("WorktreeForBranch: path=%q ok=%v err=%v", wt, ok, err)
	}
	if filepath.Clean(wt) != filepath.Clean(wantWT) {
		t.Errorf("registered worktree = %q, want %q", wt, wantWT)
	}

	// The main worktree was never switched: it stays on main.
	if cur := gitCmd(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); cur != "main" {
		t.Errorf("main worktree on %q, want main", cur)
	}

	// Four scaffold files, byte-identical to the script's templates.
	for name, want := range expectedScaffold(res.Job) {
		got, err := os.ReadFile(filepath.Join(res.Job.Dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s mismatch:\n got: %q\nwant: %q", name, string(got), want)
		}
	}

	// The scaffold commit exists inside the worktree, on the job branch.
	subject := gitCmd(t, res.WorktreePath, "log", "-1", "--format=%s")
	if subject != "Scaffold job ab12cd_add-gallery-block" {
		t.Errorf("scaffold commit subject = %q", subject)
	}

	// Summary block.
	if !strings.Contains(out.String(), "✓ Job created: ab12cd_add-gallery-block") {
		t.Errorf("summary missing creation line:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "  Branch   : feature/ab12cd_add-gallery-block (based on main)") {
		t.Errorf("summary missing branch line:\n%s", out.String())
	}
}

func TestCreateJobNestedMountPointLayout(t *testing.T) {
	dir := createCheckout(t, t.TempDir())
	// Force the mount-point decision: a sibling would land on another
	// filesystem, so the worktree nests inside the project root.
	res, err := CreateJob(dir, "Nested Case", CreateOptions{
		RandomID:    fixedID("ef34gh"),
		DeviceCheck: func(parent, root string) bool { return true },
	}, io.Discard)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	wantWT := filepath.Join(dir, ".manigot-worktrees", "ef34gh_nested-case")
	if res.WorktreePath != wantWT {
		t.Errorf("nested worktree = %q, want %q", res.WorktreePath, wantWT)
	}
	if !fs.IsDir(wantWT) {
		t.Errorf("nested worktree dir missing")
	}

	// The main worktree's info/exclude carries the pattern so a `git add -A`
	// there never sweeps the nested checkouts into a commit.
	data, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read info/exclude: %v", err)
	}
	if !strings.Contains(string(data), ".manigot-worktrees/") {
		t.Errorf("info/exclude missing pattern: %q", data)
	}

	// The nested worktree's own git status is clean after the scaffold commit
	// (git -C <jobdir> commit ran inside it).
	if out := gitCmd(t, res.WorktreePath, "status", "--porcelain"); out != "" {
		t.Errorf("nested worktree not clean after create:\n%s", out)
	}
}

func TestCreateJobNonGitProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	res, err := CreateJob(dir, "Plain Project", CreateOptions{RandomID: fixedID("jk56lm")}, &out)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if res.Branch != "(no git)" || res.WorktreePath != "" {
		t.Errorf("non-git result = %+v", res)
	}
	wantDir := filepath.Join(dir, "docs", "jobs", "jk56lm_plain-project")
	if res.Job.Dir != wantDir {
		t.Errorf("non-git job dir = %q, want %q", res.Job.Dir, wantDir)
	}
	if !fs.IsDir(wantDir) {
		t.Errorf("non-git job dir missing")
	}
	if !strings.Contains(out.String(), "Warning  : not a git repository — skipping branch/worktree creation") {
		t.Errorf("missing non-git warning:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "✓ Job created: jk56lm_plain-project") {
		t.Errorf("missing creation line:\n%s", out.String())
	}
	// The brief's branch field records "(no git)".
	data, _ := os.ReadFile(filepath.Join(wantDir, "brief.md"))
	if !strings.Contains(string(data), "branch: (no git)") {
		t.Errorf("brief branch field = %q", data)
	}
}

func TestCreateJobFreshRepoNoCommits(t *testing.T) {
	// A freshly `git init`'d repo has an unborn HEAD and zero local branches
	// — the brief's "a fresh repo before its first commit" no-branches case.
	// new-job.sh's git probe (`git rev-parse --abbrev-ref HEAD`) fails there,
	// so the script took the non-git fallback; CreateJob must do the same
	// (a `git symbolic-ref` probe would wrongly succeed and report "main").
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "init", "-q", "-b", "main")
	gitCmd(t, dir, "config", "user.name", "Test")
	gitCmd(t, dir, "config", "user.email", "test@x.io")

	var out bytes.Buffer
	res, err := CreateJob(dir, "Fresh Repo Job", CreateOptions{RandomID: fixedID("mn78op")}, &out)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if res.Branch != "(no git)" || res.WorktreePath != "" {
		t.Errorf("fresh-repo result = %+v, want branch=(no git), no worktree", res)
	}
	wantDir := filepath.Join(dir, "docs", "jobs", "mn78op_fresh-repo-job")
	if res.Job.Dir != wantDir {
		t.Errorf("fresh-repo job dir = %q, want %q", res.Job.Dir, wantDir)
	}
	if !fs.IsDir(wantDir) {
		t.Errorf("fresh-repo job dir missing")
	}
	if !strings.Contains(out.String(), "Warning  : not a git repository — skipping branch/worktree creation") {
		t.Errorf("missing non-git warning:\n%s", out.String())
	}
	// The scaffold lands straight in the project root's docs/jobs/, and the
	// brief records "(no git)" — byte-identical to the script's behavior.
	data, err := os.ReadFile(filepath.Join(wantDir, "brief.md"))
	if err != nil {
		t.Fatalf("read brief.md: %v", err)
	}
	if !strings.Contains(string(data), "branch: (no git)") {
		t.Errorf("brief branch field = %q", data)
	}
}

func TestCreateJobBaseBranchMissing(t *testing.T) {
	dir := createCheckout(t, t.TempDir())
	_, err := CreateJob(dir, "X", CreateOptions{BaseBranchOverride: "develop"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "base branch 'develop' does not exist; cannot create job branch from it.") {
		t.Errorf("missing-base-branch error = %v", err)
	}
}

func TestCreateJobNamespaceCollision(t *testing.T) {
	dir := createCheckout(t, t.TempDir())
	// A plain branch named exactly "feature" blocks the feature/... namespace.
	gitCmd(t, dir, "branch", "feature")

	_, err := CreateJob(dir, "X", CreateOptions{RandomID: fixedID("ab12cd")}, io.Discard)
	if err == nil {
		t.Fatal("expected a namespace-collision error")
	}
	if !strings.Contains(err.Error(), "a branch named 'feature' already exists, which blocks the 'feature/...' namespace.") {
		t.Errorf("collision error = %v", err)
	}
}

func TestCreateJobInvalidType(t *testing.T) {
	dir := createCheckout(t, t.TempDir())
	_, err := CreateJob(dir, "X", CreateOptions{Type: "bogus"}, io.Discard)
	if err == nil || err.Error() != "Invalid type 'bogus'. Use: feature, fix, or chore." {
		t.Errorf("invalid-type error = %v", err)
	}
}

func TestCreateJobNoProjectRoot(t *testing.T) {
	_, err := CreateJob("", "X", CreateOptions{}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "could not find project root (no docs/ directory found).") {
		t.Errorf("no-project-root error = %v", err)
	}
}

func TestExistingJobIDs(t *testing.T) {
	dir := createCheckout(t, t.TempDir())

	// An open, worktree-backed job.
	if _, err := CreateJob(dir, "Open Job", CreateOptions{RandomID: fixedID("open01")}, io.Discard); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// An archived job under the main worktree's docs/jobs/archive/.
	archiveJob := filepath.Join(dir, "docs", "jobs", "archive", "old99_archived")
	if err := os.MkdirAll(archiveJob, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archiveJob, "brief.md"), []byte("# Brief: Archived\n\nid: old99\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ids, err := existingJobIDs(dir)
	if err != nil {
		t.Fatalf("existingJobIDs: %v", err)
	}
	for _, want := range []string{"open01", "old99"} {
		if !ids[want] {
			t.Errorf("existingJobIDs missing %q: %v", want, ids)
		}
	}
}

func TestExistingJobIDsNonGit(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}

	// A working-tree-only job (the non-git / no-branches fallback).
	jobDir := filepath.Join(dir, "docs", "jobs", "plain01_plain")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte("# Brief: Plain\n\nid: plain01\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ids, err := existingJobIDs(dir)
	if err != nil {
		t.Fatalf("existingJobIDs: %v", err)
	}
	if !ids["plain01"] {
		t.Errorf("existingJobIDs missing working-tree job id: %v", ids)
	}
}

func TestCreateJobSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Add Gallery Block", "add-gallery-block"},
		{"  Spaces  ", "spaces"},
		{"UPPER/lower_Mixed!!", "upper-lower-mixed"},
		{"a--b---c", "a-b-c"},
		{"-leading-and-trailing-", "leading-and-trailing"},
		{"keep.123", "keep-123"},
	}
	for _, c := range cases {
		if got := slugify(c.in); got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// expectedScaffold builds the byte-exact scaffold content for a created job —
// an independent re-construction of new-job.sh's heredocs used to verify the
// written files.
func expectedScaffold(j Job) map[string]string {
	return map[string]string{
		"brief.md": fmt.Sprintf(`# Brief: Add Gallery Block

status: open
type: feature
id: ab12cd
branch: feature/ab12cd_add-gallery-block
date: %s
author: Test

## What

<!-- What needs to be done? Be specific. -->

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->
`, j.Date),
		"tasks.md": fmt.Sprintf(`# Tasks: Add Gallery Block

id: ab12cd
status: open
analyst:
date:

<!-- Produced by @analyst from brief.md. -->

## Task breakdown

<!-- TASK-1: description
     files: list of files likely affected
     depends: none
     risk: low / medium / high — reason

TASK-2: ...
-->
`),
		"implementation.md": `# Implementation: Add Gallery Block

id: ab12cd
status: open
developer:
date:

<!-- Produced by @developer after implementation. -->

## Summary

<!-- What was implemented, task by task. Reference task IDs. -->

## Changes

<!-- List of files changed and what changed in each. -->

## Known issues / follow-ups

<!-- Anything that came up during implementation that wasn't in scope but should be tracked. -->
`,
		"verdict.md": `# Verdict: Add Gallery Block

id: ab12cd
status: open
reviewer:
date:

<!-- Produced by @reviewer and/or @security after implementation. -->

## Review

<!-- TASK-1: PASS / FAIL / PARTIAL
     notes: ...

TASK-2: ...
-->

## Security

<!-- Any security findings from @security, or "none" if not run. -->

## Overall

<!-- APPROVED / REJECTED / NEEDS WORK -->
<!-- Summary of what needs to change before this can be approved, if anything. -->
`,
	}
}

// io.Discard is used directly for tests that don't care about CreateJob's
// output.
