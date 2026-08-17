package ui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/lmuskalla/manigot/internal/job"
	"github.com/muesli/termenv"
)

// --- the "g" key opens the panel ---------------------------------------------

// TestGitKeyOpensPanel exercises the "g" key from the detail view: it opens
// the git panel (stateGitPanel) without dispatching any command.
func TestGitKeyOpensPanel(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/git01_g", "git01_g", "# Brief: Git\n\nstatus: open\nid: git01\nbranch: feature/git01_g\ndate: 2026-01-01\n")

	jobs, err := job.Discover(dir)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("job.Discover: err=%v jobs=%d", err, len(jobs))
	}

	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	model, cmd := a.updateDetail(keyMsg("g"))
	if cmd != nil {
		t.Errorf("expected no tea.Cmd from opening the panel, got one")
	}
	got := model.(*App)
	if got.state != stateGitPanel {
		t.Errorf("state = %v, want stateGitPanel", got.state)
	}
	if got.gitPanel == nil {
		t.Fatal("gitPanel not set after \"g\"")
	}
}

// TestGitKeyWithoutBranchIsNoop mirrors TestPushKeyWithoutBranchIsNoop: a job
// discovered outside a git repository (Discover's working-tree-only fallback,
// where Branch is left empty) reports a status instead of opening a panel
// whose every action needs the job's worktree branch.
func TestGitKeyWithoutBranchIsNoop(t *testing.T) {
	dir := t.TempDir() // not a git repo: job.Discover falls back to discoverWorkingTree
	jobDir := filepath.Join(dir, "docs", "jobs", "nob01_n")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	brief := "# Brief: NoBranch\n\nstatus: open\nid: nob01\ndate: 2026-01-01\n"
	if err := os.WriteFile(filepath.Join(jobDir, "brief.md"), []byte(brief), 0o644); err != nil {
		t.Fatalf("write brief.md: %v", err)
	}

	jobs, _ := job.Discover(dir)
	if len(jobs) != 1 || jobs[0].Branch != "" {
		t.Fatalf("expected one job with no branch info; got %+v", jobs)
	}
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	_, cmd := a.updateDetail(keyMsg("g"))
	if cmd != nil {
		t.Error("expected no tea.Cmd when the job has no known branch")
	}
	if a.state != stateDetail {
		t.Errorf("state = %v, want stateDetail (panel must not open)", a.state)
	}
	if a.detail.status == "" {
		t.Error("expected a status message explaining why \"g\" did nothing")
	}
}

// --- panel navigation & cancel ----------------------------------------------

// TestGitPanelNavigation covers the picker's cursor movement: ↑/↓ move and
// clamp at the ends, and the cursor starts on the first row. The vim keys j/k
// are regression-pinned as no-ops — the panel dropped them when the project
// decided not to use vim keys to navigate (mirroring the list view's
// TestListJAndKNoLongerMoveCursor precedent).
func TestGitPanelNavigation(t *testing.T) {
	v := newGitPanelView(80, 24)
	if v.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", v.cursor)
	}
	v.update(keyMsg("up")) // already at top — must not go negative
	if v.cursor != 0 {
		t.Errorf("up at top moved cursor to %d, want 0", v.cursor)
	}
	v.update(keyMsg("down"))
	if v.cursor != 1 {
		t.Errorf("down = %d, want 1", v.cursor)
	}
	v.update(keyMsg("down"))
	if v.cursor != 2 {
		t.Errorf("down = %d, want 2", v.cursor)
	}
	v.update(keyMsg("down")) // already at bottom — must not overrun
	if v.cursor != 2 {
		t.Errorf("down at bottom moved cursor to %d, want 2", v.cursor)
	}
	v.update(keyMsg("up"))
	if v.cursor != 1 {
		t.Errorf("up = %d, want 1", v.cursor)
	}

	// Regression: j/k no longer move the cursor — pressed mid-list where a
	// bound j/k would have moved, they must leave the selection alone.
	v.update(keyMsg("j"))
	if v.cursor != 1 {
		t.Errorf("cursor = %d after pressing j, want 1 (j must not move the selection)", v.cursor)
	}
	v.update(keyMsg("k"))
	if v.cursor != 1 {
		t.Errorf("cursor = %d after pressing k, want 1 (k must not move the selection)", v.cursor)
	}

	// Contrast: ↑/↓ still navigate.
	v.update(keyMsg("down"))
	if v.cursor != 2 {
		t.Errorf("cursor = %d after pressing down, want 2 (down must still move the selection)", v.cursor)
	}
}

// TestGitPanelSelected pins selected() to the row under the cursor.
func TestGitPanelSelected(t *testing.T) {
	v := newGitPanelView(80, 24)
	if got, ok := v.selected(); !ok || got != gitActionCommitAll {
		t.Errorf("selected() row 0 = %v, %v, want gitActionCommitAll", got, ok)
	}
	v.cursor = 1
	if got, ok := v.selected(); !ok || got != gitActionPush {
		t.Errorf("selected() row 1 = %v, %v, want gitActionPush", got, ok)
	}
	v.cursor = 2
	if got, ok := v.selected(); !ok || got != gitActionMerge {
		t.Errorf("selected() row 2 = %v, %v, want gitActionMerge", got, ok)
	}
}

// TestGitPanelActions pins the update() actions: enter submits, esc/q cancel,
// and any other key is a no-op.
func TestGitPanelActions(t *testing.T) {
	v := newGitPanelView(80, 24)
	if v.update(keyMsg("enter")) != gpSubmit {
		t.Error("enter should submit")
	}
	if v.update(keyMsg("esc")) != gpCancel {
		t.Error("esc should cancel")
	}
	if v.update(keyMsg("q")) != gpCancel {
		t.Error("q should cancel")
	}
	if v.update(keyMsg("x")) != gpNone {
		t.Error("unbound key should be a no-op")
	}
}

// TestGitPanelRender pins the rendered surface: the title, all three action
// rows, and the key hint.
func TestGitPanelRender(t *testing.T) {
	v := newGitPanelView(80, 24)
	out := v.render()
	for _, want := range []string{"Git", "Commit all", "Push to origin", "Merge default branch", "enter run", "esc/q cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q:\n%s", want, out)
		}
	}
}

// TestGitPanelCancelReturnsToDetail verifies esc and q both close the panel
// and return to the detail view without dispatching anything.
func TestGitPanelCancelReturnsToDetail(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/git03_g", "git03_g", "# Brief: Git\n\nstatus: open\nid: git03\nbranch: feature/git03_g\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.gitPanel = newGitPanelView(80, 24)
	a.state = stateGitPanel

	for _, key := range []string{"esc", "q"} {
		model, cmd := a.updateGitPanel(keyMsg(key))
		if cmd != nil {
			t.Errorf("%q: expected no tea.Cmd from cancelling, got one", key)
		}
		got := model.(*App)
		if got.state != stateDetail {
			t.Errorf("%q: state = %v, want stateDetail", key, got.state)
		}
		if got.gitPanel != nil {
			t.Errorf("%q: gitPanel should be cleared after cancel", key)
		}
		// Re-open for the next key.
		a.gitPanel = newGitPanelView(80, 24)
		a.state = stateGitPanel
	}
}

// --- enter dispatches the right cmd -----------------------------------------

// TestGitPanelEnterDispatchesCommitAll opens the panel and presses enter on
// the Commit all row: the dispatched tea.Cmd runs git's commit-all for real
// in the job's own worktree and reports a commitAllMsg.
func TestGitPanelEnterDispatchesCommitAll(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/git02_g", "git02_g", "# Brief: Git\n\nstatus: open\nid: git02\nbranch: feature/git02_g\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.gitPanel = newGitPanelView(80, 24)
	a.state = stateGitPanel

	// Dirty the job worktree so the commit has something to sweep.
	if err := os.WriteFile(filepath.Join(wtPath, "leftover.txt"), []byte("agent leftover\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	model, cmd := a.updateGitPanel(keyMsg("enter")) // row 0 = Commit all
	if cmd == nil {
		t.Fatal("expected enter on Commit all to return a tea.Cmd, got nil")
	}
	got := model.(*App)
	if got.state != stateDetail {
		t.Errorf("state = %v, want stateDetail after dispatch", got.state)
	}
	if got.gitPanel != nil {
		t.Error("gitPanel should be cleared after submit")
	}

	msg := cmd()
	commitAllM, ok := msg.(commitAllMsg)
	if !ok {
		t.Fatalf("expected commitAllMsg, got %T", msg)
	}
	if commitAllM.err != nil {
		t.Fatalf("commit all failed: %v", commitAllM.err)
	}
	// The commit really landed on the job branch.
	out, err := exec.Command("git", "-C", wtPath, "log", "-1", "--format=%s").Output()
	if err != nil {
		t.Fatal(err)
	}
	if subject := strings.TrimSpace(string(out)); subject != "[git02] chore: commit all" {
		t.Errorf("commit subject = %q, want %q", subject, "[git02] chore: commit all")
	}
}

// TestGitPanelEnterDispatchesPush opens the panel, moves to the Push row and
// presses enter: the dispatched tea.Cmd pushes the job's branch to origin for
// real and reports a pushMsg.
func TestGitPanelEnterDispatchesPush(t *testing.T) {
	dir, _ := gitInitRepo(t)
	origin := gitAddBareOrigin(t, dir)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/git04_g", "git04_g", "# Brief: Git\n\nstatus: open\nid: git04\nbranch: feature/git04_g\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.gitPanel = newGitPanelView(80, 24)
	a.state = stateGitPanel

	a.gitPanel.update(keyMsg("down")) // row 1 = Push to origin
	model, cmd := a.updateGitPanel(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("expected enter on Push to return a tea.Cmd, got nil")
	}
	got := model.(*App)
	if got.state != stateDetail {
		t.Errorf("state = %v, want stateDetail after dispatch", got.state)
	}

	msg := cmd()
	pushM, ok := msg.(pushMsg)
	if !ok {
		t.Fatalf("expected pushMsg, got %T", msg)
	}
	if pushM.err != nil {
		t.Fatalf("push failed: %v", pushM.err)
	}
	if pushM.branch != "feature/git04_g" {
		t.Errorf("pushMsg.branch = %q, want feature/git04_g", pushM.branch)
	}
	// The branch ref really landed on the bare remote.
	localHash, execErr := exec.Command("git", "-C", dir, "rev-parse", "feature/git04_g").Output()
	if execErr != nil {
		t.Fatal(execErr)
	}
	remoteHash, execErr := exec.Command("git", "-C", origin, "rev-parse", "feature/git04_g").Output()
	if execErr != nil {
		t.Fatalf("rev-parse on remote after push: %v", execErr)
	}
	if strings.TrimSpace(string(localHash)) != strings.TrimSpace(string(remoteHash)) {
		t.Errorf("remote feature/git04_g = %q, want %q (local)", remoteHash, localHash)
	}
}

// --- merge end-to-end and mergeMsg status handling ---------------------------

// TestGitPanelMergeBringsBaseCommitsIntoJobWorktree is the end-to-end merge
// test: the base branch moves on after the job worktree was created (the
// diverged-base fixture), then opening the panel and selecting Merge runs the
// merge for real — the base commit lands in the job worktree and the job
// branch tip becomes a merge commit — and feeding the resulting mergeMsg back
// through Update sets the detail status line.
func TestGitPanelMergeBringsBaseCommitsIntoJobWorktree(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/git05_g", "git05_g", "# Brief: Git\n\nstatus: open\nid: git05\nbranch: feature/git05_g\ndate: 2026-01-01\n")

	// The base branch (main, per gitInitRepo's -b main) moves on while the
	// job worktree is out — the job branch and base have diverged.
	if err := os.WriteFile(filepath.Join(dir, "base-work.txt"), []byte("base work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "base work")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	// Open the panel and select Merge (row 2).
	a.gitPanel = newGitPanelView(80, 24)
	a.state = stateGitPanel
	a.gitPanel.update(keyMsg("down"))
	a.gitPanel.update(keyMsg("down"))
	model, cmd := a.updateGitPanel(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("expected enter on Merge to return a tea.Cmd, got nil")
	}
	got := model.(*App)
	if got.state != stateDetail {
		t.Errorf("state = %v, want stateDetail after dispatch", got.state)
	}

	msg := cmd()
	mergeM, ok := msg.(mergeMsg)
	if !ok {
		t.Fatalf("expected mergeMsg, got %T", msg)
	}
	if mergeM.err != nil {
		t.Fatalf("merge failed: %v", mergeM.err)
	}
	if mergeM.base != "main" {
		t.Errorf("mergeMsg.base = %q, want main", mergeM.base)
	}

	// The base commit really landed in the job worktree...
	if _, err := os.Stat(filepath.Join(wtPath, "base-work.txt")); err != nil {
		t.Errorf("base-work.txt missing in the job worktree after merge: %v", err)
	}
	// ...and the diverged merge produced a merge commit (two parents) on the
	// job branch.
	out, err := exec.Command("git", "-C", wtPath, "log", "-1", "--format=%P").Output()
	if err != nil {
		t.Fatal(err)
	}
	if parents := strings.Fields(string(out)); len(parents) != 2 {
		t.Errorf("tip of feature/git05_g has %d parents, want 2 (a merge commit)", len(parents))
	}

	// Feeding the mergeMsg back through Update reports the success status.
	model2, followUp := a.Update(mergeM)
	if followUp != nil {
		t.Errorf("expected no follow-up cmd from mergeMsg handling, got one")
	}
	got2 := model2.(*App)
	if got2.detail == nil {
		t.Fatal("detail view should stay open after a successful merge")
	}
	if want := "→ merged main into feature/git05_g"; got2.detail.status != want {
		t.Errorf("status = %q, want %q", got2.detail.status, want)
	}
}

// TestGitPanelMergeConflictSurfacesInStatus verifies a conflicting merge is
// reported in the detail view's status line as an error mentioning the
// conflict — the tree is left in the conflicted state for the user to resolve
// manually, and the app keeps running.
func TestGitPanelMergeConflictSurfacesInStatus(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	wtPath := addJobWorktree(t, dir, wts, "feature/git06_g", "git06_g", "# Brief: Git\n\nstatus: open\nid: git06\nbranch: feature/git06_g\ndate: 2026-01-01\n")

	// Both sides modify the same file differently — the diverged conflicting
	// fixture.
	if err := os.WriteFile(filepath.Join(wtPath, "shared.txt"), []byte("feature version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wtPath, "add", "-A")
	gitRun(t, wtPath, "commit", "-q", "-m", "feature work")

	if err := os.WriteFile(filepath.Join(dir, "shared.txt"), []byte("base version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "base work")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	// Open the panel and select Merge (row 2).
	a.gitPanel = newGitPanelView(80, 24)
	a.state = stateGitPanel
	a.gitPanel.update(keyMsg("down"))
	a.gitPanel.update(keyMsg("down"))
	_, cmd := a.updateGitPanel(keyMsg("enter"))
	if cmd == nil {
		t.Fatal("expected enter on Merge to return a tea.Cmd, got nil")
	}

	msg := cmd()
	mergeM, ok := msg.(mergeMsg)
	if !ok {
		t.Fatalf("expected mergeMsg, got %T", msg)
	}
	if mergeM.err == nil {
		t.Fatal("expected the conflicting merge to report an error, got nil")
	}
	if !strings.Contains(mergeM.err.Error(), "CONFLICT") {
		t.Errorf("merge error should mention CONFLICT, got: %v", mergeM.err)
	}

	// Feeding the mergeMsg back through Update surfaces it as a status error
	// without crashing or closing the detail view.
	model, followUp := a.Update(mergeM)
	if followUp != nil {
		t.Errorf("expected no follow-up cmd from a failed merge, got one")
	}
	got := model.(*App)
	if got.detail == nil {
		t.Fatal("detail view should stay open after a conflicting merge")
	}
	if want := cmdErrorText(mergeM.err); got.detail.status != want {
		t.Errorf("status = %q, want %q", got.detail.status, want)
	}
}

// TestMergeMsgErrorSurfacesInDetailStatus verifies an arbitrary merge failure
// (e.g. a dirty-worktree refusal) is reported in the detail view's status
// line via the usual cmdErrorText formatting.
func TestMergeMsgErrorSurfacesInDetailStatus(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/git07_g", "git07_g", "# Brief: Git\n\nstatus: open\nid: git07\nbranch: feature/git07_g\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.state = stateDetail

	wantErr := errors.New("git merge main: exit status 128: fatal: something failed")
	model, cmd := a.Update(mergeMsg{base: "main", err: wantErr})
	if cmd != nil {
		t.Errorf("expected no follow-up cmd, got one")
	}
	got := model.(*App)
	if got.detail == nil {
		t.Fatal("detail view should stay open after a failed merge")
	}
	if want := cmdErrorText(wantErr); got.detail.status != want {
		t.Errorf("status = %q, want %q", got.detail.status, want)
	}
}

// --- popup overlay -----------------------------------------------------------

// TestGitPanelOverlayKeepsDetailVisible verifies the stateGitPanel render is
// now a popup over the detail view rather than a full-frame swap: App.View
// still shows the job detail underneath the panel box, alongside the panel's
// own title, rows, and key hint.
func TestGitPanelOverlayKeepsDetailVisible(t *testing.T) {
	dir, _ := gitInitRepo(t)
	wts := t.TempDir()
	addJobWorktree(t, dir, wts, "feature/git08_g", "git08_g", "# Brief: Git\n\nstatus: open\nid: git08\nbranch: feature/git08_g\ndate: 2026-01-01\n")

	jobs, _ := job.Discover(dir)
	a := NewApp(dir, jobs)
	a.width, a.height = 80, 24
	a.detail = newDetailView(jobs[0], 80, 24)
	a.gitPanel = newGitPanelView(80, 24)
	a.state = stateGitPanel

	out := a.View()
	// The panel's own surface is all present...
	for _, want := range []string{"Git", "Commit all", "Push to origin", "Merge default branch", "enter run", "esc/q cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("overlay missing panel row %q:\n%s", want, out)
		}
	}
	// ...and so is the underlying detail view — the popup floats over it
	// instead of replacing it.
	if !strings.Contains(out, a.detail.job.Title) {
		t.Errorf("overlay lost the underlying detail title %q:\n%s", a.detail.job.Title, out)
	}
	// The box carries the modal's border and background, marking it as a
	// floating dialog rather than a plain full-frame list.
	if !strings.Contains(out, "╭") || !strings.Contains(out, "╰") {
		t.Errorf("overlay lacks the rounded modal border:\n%s", out)
	}
}

// TestGitPanelOverlayCentersBoxInFrame pins the popup geometry: the box's
// top-left corner lands centered within the viewport, not pinned to the top
// left or anywhere off-frame, and the rows above and below it stay untouched.
func TestGitPanelOverlayCentersBoxInFrame(t *testing.T) {
	// gitPanelModalStyle renders "xx\nyy" as a 6-row (2 content + padding +
	// border) × 8-column box. Over a 12×10 base that places it at x=2, y=2.
	modal := gitPanelModalStyle.Render("xx\nyy")
	var baseLines []string
	for i := 0; i < 10; i++ {
		baseLines = append(baseLines, strings.Repeat(" ", 12))
	}
	out := placeOverlay(strings.Join(baseLines, "\n"), modal, 12, 10)
	lines := strings.Split(out, "\n")
	if len(lines) != 10 {
		t.Fatalf("overlay height = %d lines, want 10:\n%s", len(lines), out)
	}
	// The box top border sits at row 2, indented to column 2 within a 12-wide
	// row; content rows carry the box's own padding.
	if got := ansi.StringWidth(lines[2]); got != 12 {
		t.Errorf("box top row width = %d, want 12", got)
	}
	if !strings.HasPrefix(ansi.Strip(lines[2]), "  ╭──────╮") {
		t.Errorf("box top border misplaced:\n%q", ansi.Strip(lines[2]))
	}
	if !strings.Contains(ansi.Strip(lines[4]), "xx") || !strings.Contains(ansi.Strip(lines[5]), "yy") {
		t.Errorf("box content misplaced:\n%s", out)
	}
	if !strings.Contains(ansi.Strip(lines[7]), "╰──────╯") {
		t.Errorf("box bottom border missing:\n%s", out)
	}
	// Rows above and below the box are untouched frame space.
	for _, i := range []int{0, 1, 8, 9} {
		if strings.TrimSpace(ansi.Strip(lines[i])) != "" {
			t.Errorf("row %d should be untouched frame space, got %q", i, ansi.Strip(lines[i]))
		}
	}
}

// TestPlaceOverlayMergesCellsOverAnsiBase pins the cell-by-cell merge: the
// box replaces exactly the base cells it covers and the base content — and
// its ANSI styling — survives on both sides, with the frame width preserved.
func TestPlaceOverlayMergesCellsOverAnsiBase(t *testing.T) {
	forceColor(t)
	// 12-cell base line: "aaaa" + bold "BBBB" + "cccc".
	base := "aaaa" + lipgloss.NewStyle().Bold(true).Render("BBBB") + "cccc"
	// 2-cell-wide box centered on the 12-cell frame → covers columns 5–6
	// (the middle of the bold run).
	out := placeOverlay(base, "xx\nyy", 12, 1)

	if got := ansi.StringWidth(out); got != 12 {
		t.Errorf("overlay width = %d, want 12", got)
	}
	if want := "aaaaB" + "xx" + "Bcccc"; ansi.Strip(out) != want {
		t.Errorf("overlay = %q, want %q", ansi.Strip(out), want)
	}
}

// TestCutDropsMiddleRangeWithoutSplittingAnsi verifies cut removes exactly
// the [lo, hi) cell range of a rendered line, leaving the ANSI intact on both
// sides and keeping the display widths of the two halves correct.
func TestCutDropsMiddleRangeWithoutSplittingAnsi(t *testing.T) {
	forceColor(t)
	// "ab" + red("cd") + "ef": dropping columns 2–3 must remove the red "cd"
	// without tearing the color codes open or shifting the surviving cells.
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	line := "ab" + red.Render("cd") + "ef"
	left, right := cut(line, 2, 4)

	if plain := ansi.Strip(left + right); plain != "abef" {
		t.Errorf("cut produced %q, want %q", plain, "abef")
	}
	if wl, wr := ansi.StringWidth(left), ansi.StringWidth(right); wl != 2 || wr != 2 {
		t.Errorf("cut widths = %d/%d, want 2/2", wl, wr)
	}
}

// forceColor switches the lipgloss renderer to a color profile for the
// duration of a test, so styles actually emit their ANSI sequences instead of
// degrading to plain text under the default no-color test profile. Restored
// on the way out so later tests keep seeing the same rendering they expect.
func forceColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

// TestCutClampsOutOfRangeColumns pins cut's boundary behavior: a range past
// the line's end or a line shorter than the range degrades to no-op rather
// than corrupting the line.
func TestCutClampsOutOfRangeColumns(t *testing.T) {
	line := "abcdef"
	left, right := cut(line, 0, 3)
	if left != "" || right != "def" {
		t.Errorf("cut(0,3) = %q + %q, want %q + %q", left, right, "", "def")
	}
	left, right = cut(line, 0, 99)
	if left != "" || right != "" {
		t.Errorf("cut past end left %q + %q, want empty both", left, right)
	}
	left, right = cut("ab", 4, 6)
	if left != "ab" || right != "" {
		t.Errorf("short line cut = %q + %q, want %q + %q", left, right, "ab", "")
	}
}
