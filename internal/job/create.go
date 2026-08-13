package job

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/project"
)

// CreateOptions carries the optional knobs of CreateJob — the flags
// new-job.sh accepted after the title.
type CreateOptions struct {
	// Type is the job type: feature (the default), fix, or chore.
	Type string

	// BaseBranchOverride replaces the project's configured base branch for
	// this one invocation (new-job.sh's --base-branch).
	BaseBranchOverride string

	// DeviceCheck reports whether a worktree sibling of root would land on a
	// different filesystem (root itself is a mount point), which forces the
	// nested layout. nil uses the real stat-dev comparison. Injectable so
	// tests can force the nested layout without an actual second mount.
	DeviceCheck func(parent, root string) bool

	// RandomID generates the 6-char job id (a-z0-9). nil uses crypto/rand.
	// Injectable so tests get a deterministic id.
	RandomID func() (string, error)
}

// CreateResult is the outcome of a successful CreateJob.
type CreateResult struct {
	// Job is the created job, read back from disk (Dir is the absolute job
	// directory).
	Job Job

	// Branch is the resolved branch name, or "(no git)" for a non-git project
	// (mirroring the brief.md branch field new-job.sh wrote).
	Branch string

	// WorktreePath is the job's own worktree ("" for a non-git project).
	WorktreePath string
}

// CreateJob ports scripts/new-job.sh — the job-creation lifecycle: resolve the
// settings, generate the id/slug/branch, create the job's own git worktree (or
// fall back to a plain directory for a non-git project), write the four
// scaffold files byte-identically to the script's templates, and make the
// "Scaffold job <id>_<slug>" commit inside the worktree. The script's stdout
// summary goes to out; all errors are returned with the script's exact
// "Error: ..." wording (the caller decides where to print them).
//
// root is the project root (the docs/-walk-up the CLI resolves before calling
// in; the TUI passes its own a.root). It is explicit rather than
// cwd-derived so the TUI's in-process calls target the project the TUI was
// opened on, not the TUI process's working directory.
func CreateJob(root, title string, opts CreateOptions, out io.Writer) (CreateResult, error) {
	if root == "" {
		return CreateResult{}, fmt.Errorf("could not find project root (no docs/ directory found).")
	}

	// ── Resolve type + base branch + prefix ─────────────────────────────────
	jobType := opts.Type
	if jobType == "" {
		jobType = "feature"
	}
	switch jobType {
	case "feature", "fix", "chore":
	default:
		return CreateResult{}, fmt.Errorf("Invalid type '%s'. Use: feature, fix, or chore.", jobType)
	}

	settings, err := project.Load(root)
	if err != nil {
		return CreateResult{}, err
	}
	baseBranch := settings.BaseBranchValue()
	if opts.BaseBranchOverride != "" {
		baseBranch = opts.BaseBranchOverride
	}

	// ── Generate id, slug, date, author ─────────────────────────────────────
	id := opts.RandomID
	if id == nil {
		id = randomID
	}
	jobID, err := id()
	if err != nil {
		return CreateResult{}, fmt.Errorf("generating job id: %w", err)
	}
	slug := slugify(title)
	date := time.Now().Format("2006-01-02")
	author := git.ConfigUserName(root)
	if author == "" {
		author = "unknown"
	}

	branch := settings.JobBranchPrefix
	if branch != "" {
		branch += "/"
	}
	branch += jobType + "/" + jobID + "_" + slug

	// Git vs non-git is decided on the no-branches signal, exactly like
	// new-job.sh: the script ran `git rev-parse --abbrev-ref HEAD`, which
	// FAILS on an unborn HEAD (a fresh repo before its first commit), so such
	// a repo took the non-git fallback. `git symbolic-ref --quiet --short HEAD`
	// instead SUCCEEDS on an unborn HEAD (it prints the unborn branch name),
	// which is why CurrentBranch is the wrong probe here. A repo with zero
	// local branches (fresh `git init`) takes the plain-directory fallback;
	// a non-repo (ErrNotARepo) takes it too. Any other git failure is real
	// and returned.
	branches, err := git.LocalBranches(root)
	if err != nil && !errors.Is(err, git.ErrNotARepo) {
		return CreateResult{}, err
	}

	if len(branches) > 0 {
		// ── Git path: branch + worktree ────────────────────────────────────
		if ok, err := git.RefExists(root, baseBranch); err != nil {
			return CreateResult{}, err
		} else if !ok {
			return CreateResult{}, fmt.Errorf("base branch '%s' does not exist; cannot create job branch from it.", baseBranch)
		}

		if err := checkBranchNamespace(root, branch); err != nil {
			return CreateResult{}, err
		}

		// Worktree layout: sibling of the project root by default; nested
		// inside it (with a .git/info/exclude entry) when the root is itself
		// a mount point and a sibling would land on another filesystem.
		parent := filepath.Join(filepath.Dir(root), ".manigot-worktrees", filepath.Base(root))
		dc := opts.DeviceCheck
		if dc == nil {
			dc = func(parent, root string) bool { return !sameDevice(parent, root) }
		}
		if dc(filepath.Dir(root), root) {
			parent = filepath.Join(root, ".manigot-worktrees")
			if err := git.ExcludePath(root, ".manigot-worktrees/"); err != nil && !errors.Is(err, git.ErrNotARepo) {
				return CreateResult{}, err
			}
		}

		wtPath := filepath.Join(parent, jobID+"_"+slug)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return CreateResult{}, err
		}
		if err := git.WorktreeAdd(root, wtPath, branch, baseBranch); err != nil {
			return CreateResult{}, err
		}
		fmt.Fprintf(out, "  Branch   : %s (based on %s)\n", branch, baseBranch)
		fmt.Fprintf(out, "  Worktree : %s\n", wtPath)

		jobDir := filepath.Join(wtPath, JobsRelDir, jobID+"_"+slug)
		if err := os.MkdirAll(jobDir, 0o755); err != nil {
			return CreateResult{}, err
		}
		if err := writeScaffold(jobDir, title, jobType, jobID, branch, date, author); err != nil {
			return CreateResult{}, err
		}

		// The commit runs inside the job's own worktree (git -C <job-dir>),
		// staging exactly the four new files.
		if err := git.Stage(jobDir, "."); err != nil {
			return CreateResult{}, err
		}
		if err := git.CommitStaged(jobDir, "Scaffold job "+jobID+"_"+slug); err != nil {
			return CreateResult{}, err
		}

		j, _ := ReadJob(jobDir)
		res := CreateResult{Job: j, Branch: branch, WorktreePath: wtPath}
		printCreateSummary(out, res)
		return res, nil
	}

	// ── Non-git path: plain directory scaffold ─────────────────────────────
	fmt.Fprintln(out, "  Warning  : not a git repository — skipping branch/worktree creation")
	jobDir := filepath.Join(root, JobsRelDir, jobID+"_"+slug)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return CreateResult{}, err
	}
	if err := writeScaffold(jobDir, title, jobType, jobID, "(no git)", date, author); err != nil {
		return CreateResult{}, err
	}
	j, _ := ReadJob(jobDir)
	res := CreateResult{Job: j, Branch: "(no git)"}
	printCreateSummary(out, res)
	return res, nil
}

// printCreateSummary writes the script's closing "✓ Job created" block to out.
func printCreateSummary(out io.Writer, r CreateResult) {
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "✓ Job created: %s\n", r.Job.Name)
	fmt.Fprintf(out, "  Dir    : %s\n", r.Job.Dir)
	fmt.Fprintf(out, "  Branch : %s\n", r.Branch)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  Next steps:")
	fmt.Fprintln(out, "  1. Edit brief.md:")
	fmt.Fprintf(out, "     %s/brief.md\n", r.Job.Dir)
	fmt.Fprintln(out, "  2. Run @owner or @analyst inside manigot")
	fmt.Fprintln(out, "  3. Implement on this branch")
	fmt.Fprintln(out, "  4. Merge when verdict is APPROVED")
}

// checkBranchNamespace ports new-job.sh's pre-flight namespace-collision
// check: git stores refs as filesystem paths, so a plain branch "feature"
// blocks the whole "feature/..." namespace. Every ancestor path segment of the
// branch name except the leaf <id>_<slug> is checked; a collision returns the
// script's exact error + fix hint.
func checkBranchNamespace(root, branch string) error {
	seg := branch
	if i := strings.LastIndex(seg, "/"); i >= 0 {
		seg = seg[:i]
	} else {
		return nil // no ancestor segments
	}
	for seg != "" {
		ok, err := git.RefExists(root, seg)
		if err != nil {
			return err
		}
		if ok {
			return fmt.Errorf("cannot create job branch '%s': a branch named '%s' already exists, which blocks the '%s/...' namespace.\n  Set jobBranchPrefix in .manigot/manigot.json (or rename the conflicting branch) and retry.", branch, seg, seg)
		}
		if !strings.Contains(seg, "/") {
			break // a bare segment (e.g. "jobs") is the last ancestor
		}
		seg = seg[:strings.LastIndex(seg, "/")]
	}
	return nil
}

// slugify is the script's slug pipeline: lowercase, non-alphanumeric → "-",
// collapse runs, trim leading/trailing "-".
func slugify(title string) string {
	s := strings.ToLower(title)
	s = regexpNonAlnum.ReplaceAllString(s, "-")
	s = regexpCollapseDash.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

var (
	regexpNonAlnum     = regexp.MustCompile(`[^a-z0-9]`)
	regexpCollapseDash = regexp.MustCompile(`-+`)
)

// randomID generates a 6-char id from a-z0-9 via crypto/rand — the Go form of
// `LC_ALL=C tr -dc 'a-z0-9' < /dev/urandom | head -c 6`.
func randomID() (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	// Rejection sampling: 256 is not a multiple of 36, so a naive
	// `byte % 36` overweights the first few chars (values 252-255 would all
	// map onto them). Draw a fresh byte until it lands in [0, 252) — the
	// largest multiple of 36 below 256 — so every char is exactly as likely
	// as every other. The bias was negligible for job ids, but the correct
	// shape costs nothing.
	const limit = byte(256 - 256%len(chars)) // 252
	b := make([]byte, 6)
	for i := range b {
		for {
			if _, err := rand.Read(b[i : i+1]); err != nil {
				return "", err
			}
			if b[i] < limit {
				break
			}
		}
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b), nil
}

// writeScaffold writes the four job files with content byte-identical to
// new-job.sh's heredocs. A slice of {name, content} pairs, not a map: the
// write order is deterministic (the files are independent, so a map's random
// order was harmless — fixed iteration is simply tidier).
func writeScaffold(dir, title, jobType, jobID, branch, date, author string) error {
	files := []struct {
		name    string
		content string
	}{
		{"brief.md", fmt.Sprintf(`# Brief: %s

status: open
type: %s
id: %s
branch: %s
date: %s
author: %s

## What

<!-- What needs to be done? Be specific. -->

## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->
`, title, jobType, jobID, branch, date, author)},
		{"tasks.md", fmt.Sprintf(`# Tasks: %s

id: %s
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
`, title, jobID)},
		{"implementation.md", fmt.Sprintf(`# Implementation: %s

id: %s
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
`, title, jobID)},
		{"verdict.md", fmt.Sprintf(`# Verdict: %s

id: %s
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
`, title, jobID)},
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// sameDevice reports whether parent and root are on the same filesystem — the
// mount-point test of new-job.sh (`stat -c %d "$(dirname …)"` vs
// `stat -c %d …`). A difference means root crosses a mount boundary, so a
// sibling worktree would land outside the project's persistent storage.
func sameDevice(parent, root string) bool {
	ip, err := os.Stat(parent)
	if err != nil {
		return true
	}
	ir, err := os.Stat(root)
	if err != nil {
		return true
	}
	sp, ok1 := ip.Sys().(*syscall.Stat_t)
	sr, ok2 := ir.Sys().(*syscall.Stat_t)
	if !ok1 || !ok2 {
		return true
	}
	return sp.Dev == sr.Dev
}
