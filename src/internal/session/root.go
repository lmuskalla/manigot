package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lmuskalla/manigot/internal/fs"
	"github.com/lmuskalla/manigot/internal/git"
	"github.com/lmuskalla/manigot/internal/job"
)

// Root holds the resolved project-root state (run.sh's PROJECT_ROOT /
// DOCS_INITIALIZED / INVOCATION_ROOT / GIT_COMMON_DIR block).
type Root struct {
	// ProjectRoot is the mount root: the project root, or the job's own
	// worktree path once --job has resolved one.
	ProjectRoot string

	// InvocationRoot is the original project root, captured before any --job
	// reassignment — used for the banner's "Project" line and the container
	// name.
	InvocationRoot string

	// DocsInitialized reports whether a docs/ directory was found (the walk-up
	// gate that marks an initialized project).
	DocsInitialized bool

	// DocsDir is PROJECT_ROOT/docs (only meaningful when DocsInitialized).
	DocsDir string

	// Job is the resolved job name ("" when no --job was given). For the
	// worktree path this is the branch's id_slug tail segment; for the
	// no-branches fallback, the matched directory's basename.
	Job string

	// GitCommonDir is the main repo's git dir for a job worktree mount ("" for
	// a plain session, where the project's own .git is already mounted).
	GitCommonDir string
}

// ResolveRoot implements run.sh's root resolution and --job worktree
// resolution, walking up from the process working
// directory. The docs walk-up reuses job.FindProjectRoot (the same logic the
// TUI uses); the --job branch scan reuses git.LocalBranches and
// git.WorktreeForBranch rather than re-implementing them. Every error's
// wording matches the script's.
func ResolveRoot(opts Options) (Root, error) {
	dir, err := os.Getwd()
	if err != nil {
		return Root{}, fmt.Errorf("cannot read working directory: %w", err)
	}
	return ResolveRootFrom(opts, dir)
}

// ResolveRootFrom is ResolveRoot's parameterized form: it walks up from
// startDir instead of the process cwd. mg-jdi's --print invocations resolve
// against the project it was started in, the same way run.sh used $PWD.
func ResolveRootFrom(opts Options, startDir string) (Root, error) {
	root, err := job.FindProjectRootFrom(startDir)
	if err != nil {
		return Root{}, fmt.Errorf("cannot read working directory: %w", err)
	}

	r := Root{DocsInitialized: root != ""}
	if root == "" {
		// No docs/: the container boundary falls back to the git root, else
		// $PWD — a plain session, no project context or job workflow.
		toplevel, terr := git.RevParseToplevel(".")
		if terr == nil {
			root = toplevel
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return Root{}, fmt.Errorf("cannot read working directory: %w", err)
			}
			root = cwd
		}
	}
	root = filepath.Clean(root)
	r.ProjectRoot = root
	r.InvocationRoot = root
	r.DocsDir = filepath.Join(root, "docs")

	if opts.Job == "" {
		// Plain session: the resolved root is the worktree the container will
		// mount docs/ into, so keep git from ever tracking the mount target
		// path (.opencode/.claude) in it — an agent staging job files through
		// the mount would otherwise commit a stale duplicate of docs/ (see
		// docs/BUG_report-mg-done-dirty-worktree-stale-job-copy.md).
		if err := git.ExcludeMountTargets(r.ProjectRoot); err != nil {
			return Root{}, err
		}
		return r, nil
	}
	if !r.DocsInitialized {
		return Root{}, fmt.Errorf("--job requires an initialized project (no docs/ found).\nSee 'Per-project setup' in the manigot README, then 'mg job' to create one.")
	}

	branches, berr := git.LocalBranches(root)
	if berr != nil || len(branches) == 0 {
		// Non-git / no-branches fallback: no worktrees are possible, so the
		// job's files live directly in PROJECT_ROOT/docs/jobs/. Exact match
		// first, then a prefix scan of the directory names, excluding archive/.
		jobDir := filepath.Join(root, "docs", "jobs", opts.Job)
		if fs.IsDir(jobDir) {
			r.Job = opts.Job
		} else {
			match := job.PrefixJobDirName(root, opts.Job)
			if match == "" {
				return Root{}, fmt.Errorf("job '%s' not found under docs/jobs/", opts.Job)
			}
			r.Job = match
		}
		if err := git.ExcludeMountTargets(r.ProjectRoot); err != nil {
			return Root{}, err
		}
		return r, nil
	}

	// A job's branch embeds its id_slug as the tail segment: exact match
	// first, then a prefix match, with an ambiguity error for multiple hits.
	branch := git.ExactBranchMatch(branches, opts.Job)
	if branch == "" {
		prefixMatches := git.PrefixBranchMatches(branches, opts.Job)
		switch len(prefixMatches) {
		case 0:
			return Root{}, fmt.Errorf("job '%s' not found among local branches.", opts.Job)
		case 1:
			branch = prefixMatches[0]
		default:
			return Root{}, fmt.Errorf("job '%s' is ambiguous — matches branches: %s", opts.Job, strings.Join(prefixMatches, " "))
		}
	}

	wtPath, ok, werr := git.WorktreeForBranch(root, branch)
	if werr != nil {
		return Root{}, fmt.Errorf("git worktree lookup for branch '%s': %w", branch, werr)
	}
	if !ok {
		// A branch match with no registered worktree is a hard error — never
		// silently fall back to PROJECT_ROOT, which would show the wrong job's
		// content.
		return Root{}, fmt.Errorf("branch '%s' has no git worktree.\nA job's worktree is created by 'mg job' and should always exist for an open job — this is an inconsistent state (worktree creation may have failed, or the worktree was removed by hand).\nRefusing to fall back to mounting %s instead: that would show the wrong job's content.", branch, root)
	}

	r.Job = git.BranchTail(branch)
	r.ProjectRoot = filepath.Clean(wtPath)
	r.DocsDir = filepath.Join(r.ProjectRoot, "docs")
	r.GitCommonDir = git.GitCommonDir(r.ProjectRoot)
	// Defensive consistency check: the resolved worktree must actually carry
	// the job directory (new-job.sh always creates it alongside the worktree).
	if !fs.IsDir(filepath.Join(r.ProjectRoot, "docs", "jobs", r.Job)) {
		return Root{}, fmt.Errorf("resolved job worktree at %s has no docs/jobs/%s directory — inconsistent worktree state.", r.ProjectRoot, r.Job)
	}
	// Same exclusion as the plain-session path: the container mounts docs/
	// into this worktree at the colliding repo path, so git must never track
	// it (see the plain-session comment above).
	if err := git.ExcludeMountTargets(r.ProjectRoot); err != nil {
		return Root{}, err
	}
	return r, nil
}
