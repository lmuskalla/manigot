package main

import (
	"os"
	"path/filepath"
	"slices"
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

// jobsWriteJobFile writes a job file (name relative to the job dir) into the
// checkout's docs/jobs/<jobName>/ directory — the extension to jobsCheckout
// that lets a fixture land on any workflow stage (mirroring the filled-*
// content shapes in internal/job/stage_test.go).
func jobsWriteJobFile(t *testing.T, root, jobName, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "docs", "jobs", jobName, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// jobsWrittenBrief builds a brief.md with real prose beyond the frontmatter —
// the same past-scaffold shape the job package's stage tests use, so
// job.Stage() lands past define (jobsBrief alone is frontmatter-only and
// counts as unwritten).
func jobsWrittenBrief(title, id, typ, date string) string {
	return jobsBrief(title, id, typ, date) + "## What\n\nAdd a widget so users can schedule recurring exports.\n"
}

// The past-scaffold, real content for each job file — the "written" side of
// the stage fixtures, mirroring internal/job/stage_test.go's filled-*.
const (
	jobsFilledTasks          = "# Tasks: Alpha Job\n\nid: aaa01\n\n## Task breakdown\n\nTASK-1: real work here\n"
	jobsFilledImplementation = "# Implementation: Alpha Job\n\nid: aaa01\n\n## Summary\n\nReal prose line one here.\nMore real prose line two.\n"
	jobsApprovedVerdict      = "# Verdict: Alpha Job\n\nid: aaa01\n\n## Review\n\nTASK-1: PASS — matches the brief.\n\n## Overall\n\nAPPROVED — nice work.\n"
	jobsRejectedVerdict      = "# Verdict: Alpha Job\n\nid: aaa01\n\n## Review\n\nTASK-1: FAIL — off-by-one in the export loop.\n\n## Overall\n\nREJECTED — TASK-1 has a bug.\n"
)

// pickerStub returns a pickerRunFunc that fails the test if invoked — for
// tests that must never reach the selection step (non-TTY paths, empty job
// lists, orphan-only flows).
func pickerStub(t *testing.T) pickerRunFunc {
	t.Helper()
	return func(title string, rows []ui.PickerRow, start int) (string, bool, error) {
		t.Fatalf("picker unexpectedly run (title %q)", title)
		return "", false, nil
	}
}

// pickerChoice returns a pickerRunFunc that reports the given result without
// touching a terminal — the fake the wiring tests inject.
func pickerChoice(id string, ok bool) pickerRunFunc {
	return func(title string, rows []ui.PickerRow, start int) (string, bool, error) {
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
	// Newest first (date desc), same ordering as the TUI — the ids carry
	// their display "#" prefix in the listing.
	for _, want := range []string{"1) #def02", "2) #aaa01"} {
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
	if !strings.Contains(out.String(), "→ Starting a session in #aaa01...") {
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
	pick := func(title string, rows []ui.PickerRow, start int) (string, bool, error) {
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
	if gotRows[0].ID != "def02" || gotRows[0].SearchKey != "#def02 Beta Job" {
		t.Errorf("row 0 = %+v, want def02 (raw ID) with #-prefixed id+title search key", gotRows[0])
	}
	if !strings.Contains(gotRows[0].Label, "#def02") || !strings.Contains(gotRows[0].Label, "fix") ||
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

// TestJobsStageAgent pins the stage → agent mapping behind the mg-jdi-less
// CLI launch: plan → analyst, implement → developer, review → reviewer, and
// "" for the edge stages with no fitting agent (define, finished).
func TestJobsStageAgent(t *testing.T) {
	cases := map[job.Stage]string{
		job.StageDefine:    "",
		job.StagePlan:      "analyst",
		job.StageImplement: "developer",
		job.StageReview:    "reviewer",
		job.StageFinished:  "",
	}
	for stage, want := range cases {
		if got := stageAgent(stage); got != want {
			t.Errorf("stageAgent(%s) = %q, want %q", stage, got, want)
		}
	}
}

// TestJobsStageGuidance pins the edge-stage heads-up lines: define and
// finished launch agent-less but print a short guidance line naming the
// situation, while the mapped stages get no guidance at all.
func TestJobsStageGuidance(t *testing.T) {
	if got := stageGuidance(job.StageDefine); !strings.Contains(got, "brief.md is not written yet") {
		t.Errorf("define guidance = %q, want brief-not-written heads-up", got)
	}
	if got := stageGuidance(job.StageFinished); !strings.Contains(got, "run mg done to merge") {
		t.Errorf("finished guidance = %q, want mg-done heads-up", got)
	}
	for _, stage := range []job.Stage{job.StagePlan, job.StageImplement, job.StageReview} {
		if got := stageGuidance(stage); got != "" {
			t.Errorf("stageGuidance(%s) = %q, want \"\" (mapped stages need no guidance)", stage, got)
		}
	}
}

// TestJobsLaunchLine pins the launch-line wording: with an agent it names it
// ("→ Starting a session in @analyst for #aaa01..."), without one it stays the
// plain agent-less line — the id always carries its display "#" prefix.
func TestJobsLaunchLine(t *testing.T) {
	if got, want := jobsLaunchLine("aaa01", "analyst"), "→ Starting a session in @analyst for #aaa01..."; got != want {
		t.Errorf("jobsLaunchLine with agent = %q, want %q", got, want)
	}
	if got, want := jobsLaunchLine("aaa01", ""), "→ Starting a session in #aaa01..."; got != want {
		t.Errorf("jobsLaunchLine without agent = %q, want %q", got, want)
	}
}

// TestJobsLaunchArgsStageDerivesAgent pins the re-exec launch-argument
// construction for every stage: the mapped stages launch with
// --agent <name> (plan→analyst, implement→developer, review→reviewer), the
// edge stages (define, finished) launch agent-less, and an explicit
// --agent/-a in passthrough wins over the stage-derived default (TASK-3 —
// the derived flag is skipped entirely, so session.ParseArgs's last-wins
// semantics can't silently override the user's choice).
func TestJobsLaunchArgsStageDerivesAgent(t *testing.T) {
	cases := []struct {
		name        string
		stage       job.Stage
		passthrough []string
		want        []string
	}{
		{"define", job.StageDefine, nil, []string{"--job", "aaa01"}},
		{"plan", job.StagePlan, nil, []string{"--job", "aaa01", "--agent", "analyst"}},
		{"implement", job.StageImplement, nil, []string{"--job", "aaa01", "--agent", "developer"}},
		{"review", job.StageReview, nil, []string{"--job", "aaa01", "--agent", "reviewer"}},
		{"finished", job.StageFinished, nil, []string{"--job", "aaa01"}},
		// The derived agent sits between --job and the passthrough — the
		// `mg --job <id> --agent <name> <passthrough>` shape.
		{"passthrough preserved", job.StagePlan, []string{"--profile", "zai"}, []string{"--job", "aaa01", "--agent", "analyst", "--profile", "zai"}},
		// Explicit user agent beats the derived one: the derived flag is
		// skipped, leaving the user's --agent (in its original position) as
		// the only one the re-exec parses.
		{"explicit --agent wins over plan", job.StagePlan, []string{"--agent", "security"}, []string{"--job", "aaa01", "--agent", "security"}},
		{"explicit -a wins over implement", job.StageImplement, []string{"-a", "owner"}, []string{"--job", "aaa01", "-a", "owner"}},
		{"explicit --agent wins on edge stage too", job.StageFinished, []string{"--agent", "reviewer"}, []string{"--job", "aaa01", "--agent", "reviewer"}},
		{"explicit --agent after passthrough flags", job.StagePlan, []string{"--profile", "zai", "--agent", "developer"}, []string{"--job", "aaa01", "--profile", "zai", "--agent", "developer"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := jobsLaunchArgs("aaa01", c.stage, c.passthrough)
			if !slices.Equal(got, c.want) {
				t.Errorf("jobsLaunchArgs = %v, want %v", got, c.want)
			}
		})
	}
}

// TestJobsSelectStageLaunchOutput covers the TTY submit path end to end per
// stage (the injected picker seam, same as the other wiring tests): the
// launch line names the stage-derived agent for the mapped stages, and the
// edge stages (define, finished) stay agent-less and print their guidance
// line. The re-exec always exits non-zero (the go test binary rejects the
// launch flags), which is what pins that the launch was reached.
func TestJobsSelectStageLaunchOutput(t *testing.T) {
	planFixture := func(t *testing.T, root string) {
		jobsWriteJobFile(t, root, "aaa01_alpha", "brief.md", jobsWrittenBrief("Alpha Job", "aaa01", "feature", "2026-01-01"))
	}
	cases := []struct {
		name     string
		setup    func(t *testing.T, root string)
		wantLine string
		wantGuid string // "" asserts no guidance line is printed
	}{
		{
			name:     "define stays agent-less with guidance",
			setup:    func(t *testing.T, root string) {}, // jobsBrief alone is frontmatter-only (unwritten)
			wantLine: "→ Starting a session in #aaa01...",
			wantGuid: "brief.md is not written yet — write it first",
		},
		{
			name:     "plan launches in analyst",
			setup:    planFixture,
			wantLine: "→ Starting a session in @analyst for #aaa01...",
		},
		{
			name: "implement launches in developer",
			setup: func(t *testing.T, root string) {
				planFixture(t, root)
				jobsWriteJobFile(t, root, "aaa01_alpha", "tasks.md", jobsFilledTasks)
			},
			wantLine: "→ Starting a session in @developer for #aaa01...",
		},
		{
			name: "review launches in reviewer",
			setup: func(t *testing.T, root string) {
				planFixture(t, root)
				jobsWriteJobFile(t, root, "aaa01_alpha", "tasks.md", jobsFilledTasks)
				jobsWriteJobFile(t, root, "aaa01_alpha", "implementation.md", jobsFilledImplementation)
			},
			wantLine: "→ Starting a session in @reviewer for #aaa01...",
		},
		{
			name: "finished stays agent-less with guidance",
			setup: func(t *testing.T, root string) {
				planFixture(t, root)
				jobsWriteJobFile(t, root, "aaa01_alpha", "tasks.md", jobsFilledTasks)
				jobsWriteJobFile(t, root, "aaa01_alpha", "implementation.md", jobsFilledImplementation)
				jobsWriteJobFile(t, root, "aaa01_alpha", "verdict.md", jobsApprovedVerdict)
			},
			wantLine: "→ Starting a session in #aaa01...",
			wantGuid: "verdict is APPROVED — run mg done to merge",
		},
		{
			name: "rejected verdict bounces back to developer",
			setup: func(t *testing.T, root string) {
				planFixture(t, root)
				jobsWriteJobFile(t, root, "aaa01_alpha", "tasks.md", jobsFilledTasks)
				jobsWriteJobFile(t, root, "aaa01_alpha", "implementation.md", jobsFilledImplementation)
				jobsWriteJobFile(t, root, "aaa01_alpha", "verdict.md", jobsRejectedVerdict)
			},
			wantLine: "→ Starting a session in @developer for #aaa01...",
		},
		{
			name: "explicit passthrough agent names the launch line",
			setup: func(t *testing.T, root string) {
				planFixture(t, root)
			},
			wantLine: "→ Starting a session in @security for #aaa01...",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := jobsCheckout(t, map[string]string{"aaa01_alpha": jobsBrief("Alpha Job", "aaa01", "feature", "2026-01-01")})
			c.setup(t, root)
			var passthrough []string
			if strings.Contains(c.wantLine, "@security") {
				passthrough = []string{"--agent", "security"}
			}
			var out strings.Builder
			code := runJobs(passthrough, strings.NewReader(""), &out, &strings.Builder{}, true, pickerChoice("aaa01", true))
			if code == 0 {
				t.Fatalf("unexpected success; the re-exec should not accept the launch flags")
			}
			got := out.String()
			if !strings.Contains(got, c.wantLine) {
				t.Errorf("missing launch line %q:\n%s", c.wantLine, got)
			}
			if c.wantGuid != "" {
				if !strings.Contains(got, c.wantGuid) {
					t.Errorf("missing guidance line %q:\n%s", c.wantGuid, got)
				}
			} else {
				for _, stray := range []string{"brief.md is not written yet", "run mg done to merge"} {
					if strings.Contains(got, stray) {
						t.Errorf("unexpected guidance line %q:\n%s", stray, got)
					}
				}
			}
		})
	}
}
