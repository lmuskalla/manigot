package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lmuskalla/manigot/tui/internal/config"
)

func TestShellCommandFormat(t *testing.T) {
	got := shellCommand("/usr/local/bin/manigot", "developer", "irw320", "/home/me/proj", "claude-pro")
	wantInner := "cd '/home/me/proj' && '/usr/local/bin/manigot' --profile 'claude-pro' --agent 'developer' --job 'irw320'"
	if !strings.HasPrefix(got, wantInner) {
		t.Errorf("shellCommand =\n %q\nwant prefix\n %q", got, wantInner)
	}
	// The inner command must be wrapped so a failure holds the window open
	// (TASK-6) rather than the terminal closing immediately.
	if got != holdOnFailure(wantInner) {
		t.Errorf("shellCommand does not wrap its inner command with holdOnFailure:\n%q", got)
	}
}

// --- quick (bare-session) launcher ------------------------------------------

func TestQuickShellCommandFormat(t *testing.T) {
	got := quickShellCommand("/usr/local/bin/manigot", "/home/me/proj", "claude-pro")
	wantInner := "cd '/home/me/proj' && '/usr/local/bin/manigot' --profile 'claude-pro'"
	if !strings.HasPrefix(got, wantInner) {
		t.Errorf("quickShellCommand =\n %q\nwant prefix\n %q", got, wantInner)
	}
	// Must be wrapped by holdOnFailure, exactly like the agent path.
	if got != holdOnFailure(wantInner) {
		t.Errorf("quickShellCommand does not wrap its inner command with holdOnFailure:\n%q", got)
	}
}

// A bare session must never carry --agent or --job — that's the whole point of
// the quick launch path.
func TestQuickShellCommandOmitsAgentAndJob(t *testing.T) {
	got := quickShellCommand("/usr/local/bin/manigot", "/home/me/proj", "claude-pro")
	if strings.Contains(got, "--agent") {
		t.Errorf("quickShellCommand unexpectedly contains --agent: %q", got)
	}
	if strings.Contains(got, "--job") {
		t.Errorf("quickShellCommand unexpectedly contains --job: %q", got)
	}
}

// An empty profile must default to claude-pro, mirroring the agent path and
// scripts/run.sh's own default.
func TestQuickShellCommandDefaultsEmptyProfile(t *testing.T) {
	got := quickShellCommand("/bin/manigot", "/home/me/proj", "")
	if !strings.Contains(got, "--profile 'claude-pro'") {
		t.Errorf("quickShellCommand with empty profile = %q, want it to default to claude-pro", got)
	}
}

func TestQuickShellCommandPassesZAIProfile(t *testing.T) {
	got := quickShellCommand("/bin/manigot", "/home/me/proj", "zai")
	if !strings.Contains(got, "--profile 'zai'") {
		t.Errorf("quickShellCommand with zai profile = %q, want --profile 'zai'", got)
	}
}

// A checkout in a directory with spaces must still produce a single word for
// the manigot path, since the string is re-parsed by osascript / bash -lc.
func TestQuickShellCommandQuotesPathWithSpaces(t *testing.T) {
	got := quickShellCommand("/Users/me/My Projects/manigot/scripts/run.sh", "/tmp/p", "claude-pro")
	if !strings.Contains(got, `'/Users/me/My Projects/manigot/scripts/run.sh'`) {
		t.Errorf("manigot path not quoted as one word in %q", got)
	}
}

// --- agent-quick (jobless agent session) launcher ---------------------------

func TestAgentQuickShellCommandFormat(t *testing.T) {
	got := agentQuickShellCommand("/usr/local/bin/manigot", "developer", "/home/me/proj", "claude-pro")
	wantInner := "cd '/home/me/proj' && '/usr/local/bin/manigot' --profile 'claude-pro' --agent 'developer'"
	if !strings.HasPrefix(got, wantInner) {
		t.Errorf("agentQuickShellCommand =\n %q\nwant prefix\n %q", got, wantInner)
	}
	// Must be wrapped by holdOnFailure, exactly like the other launch paths.
	if got != holdOnFailure(wantInner) {
		t.Errorf("agentQuickShellCommand does not wrap its inner command with holdOnFailure:\n%q", got)
	}
}

// An agent-quick session must never carry --job — that's what distinguishes
// it from the job-scoped Agent launch path.
func TestAgentQuickShellCommandOmitsJob(t *testing.T) {
	got := agentQuickShellCommand("/usr/local/bin/manigot", "developer", "/home/me/proj", "claude-pro")
	if strings.Contains(got, "--job") {
		t.Errorf("agentQuickShellCommand unexpectedly contains --job: %q", got)
	}
}

func TestAgentQuickShellCommandDefaultsEmptyProfile(t *testing.T) {
	got := agentQuickShellCommand("/bin/manigot", "developer", "/home/me/proj", "")
	if !strings.Contains(got, "--profile 'claude-pro'") {
		t.Errorf("agentQuickShellCommand with empty profile = %q, want it to default to claude-pro", got)
	}
}

func TestAgentQuickShellCommandPassesZAIProfile(t *testing.T) {
	got := agentQuickShellCommand("/bin/manigot", "developer", "/home/me/proj", "zai")
	if !strings.Contains(got, "--profile 'zai'") {
		t.Errorf("agentQuickShellCommand with zai profile = %q, want --profile 'zai'", got)
	}
}

// A checkout in a directory with spaces must still produce a single word for
// the manigot path, since the string is re-parsed by osascript / bash -lc.
func TestAgentQuickShellCommandQuotesPathWithSpaces(t *testing.T) {
	got := agentQuickShellCommand("/Users/me/My Projects/manigot/scripts/run.sh", "developer", "/tmp/p", "claude-pro")
	if !strings.Contains(got, `'/Users/me/My Projects/manigot/scripts/run.sh'`) {
		t.Errorf("manigot path not quoted as one word in %q", got)
	}
}

// An empty profile must default to claude-pro, matching scripts/run.sh's own
// default, rather than passing an empty --profile value through.
func TestShellCommandDefaultsEmptyProfile(t *testing.T) {
	got := shellCommand("/bin/manigot", "developer", "irw320", "/home/me/proj", "")
	if !strings.Contains(got, "--profile 'claude-pro'") {
		t.Errorf("shellCommand with empty profile = %q, want it to default to claude-pro", got)
	}
}

func TestShellCommandPassesZAIProfile(t *testing.T) {
	got := shellCommand("/bin/manigot", "developer", "irw320", "/home/me/proj", "zai")
	if !strings.Contains(got, "--profile 'zai'") {
		t.Errorf("shellCommand with zai profile = %q, want --profile 'zai'", got)
	}
}

// TestHoldOnFailureExitsCleanlyOnSuccess documents the intended shape of the
// wrapper: it must not alter the wrapped command itself, only append
// exit-code-conditional hold logic after it.
func TestHoldOnFailureExitsCleanlyOnSuccess(t *testing.T) {
	got := holdOnFailure("true")
	if !strings.HasPrefix(got, "true;") {
		t.Errorf("holdOnFailure should prefix the wrapped command unchanged, got %q", got)
	}
	if !strings.Contains(got, `ec=$?`) {
		t.Errorf("holdOnFailure should capture the exit code, got %q", got)
	}
	if !strings.Contains(got, `if [ "$ec" -ne 0 ]`) {
		t.Errorf("holdOnFailure should only hold on a non-zero exit, got %q", got)
	}
	if !strings.Contains(got, `exit "$ec"`) {
		t.Errorf("holdOnFailure should preserve the original exit code, got %q", got)
	}
}

// A checkout in a directory with spaces must still produce a single word for
// the manigot path, since the string is re-parsed by osascript / bash -lc.
func TestShellCommandQuotesPathWithSpaces(t *testing.T) {
	got := shellCommand("/Users/me/My Projects/manigot/scripts/run.sh", "reviewer", "abc123", "/tmp/p", "claude-code")
	if !strings.Contains(got, `'/Users/me/My Projects/manigot/scripts/run.sh'`) {
		t.Errorf("manigot path not quoted as one word in %q", got)
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"":           "''",
		"plain":      "'plain'",
		"with space": "'with space'",
		"a'b":        "'a'\\''b'", // embedded quote escaped via '\''
		`back\slash`: `'back\slash'`,
	}
	for in, want := range cases {
		got := shellQuote(in)
		if got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestShellQuoteNeutralizesInjection ensures a value engineered to break out of
// its quotes cannot inject shell metacharacters: after quoting, the dangerous
// substring must be contained inside the escaped sequence, not terminate the
// outer single-quoted word.
func TestShellQuoteNeutralizesInjection(t *testing.T) {
	mal := "'; rm -rf /; '"
	q := shellQuote(mal)
	// The result must start and end with a single quote and contain no raw
	// unescaped "; rm -rf" outside of quotes — i.e. the only way "rm" appears
	// is inside the escaped inner segment.
	if !strings.HasPrefix(q, "'") || !strings.HasSuffix(q, "'") {
		t.Fatalf("quoted value not single-quote wrapped: %q", q)
	}
	// Re-splitting on the escape idiom, no segment should be a bare shell
	// command. Concretely: there must be no occurrence of "; '" (close+open
	// without the escape) which would indicate breakout.
	if strings.Contains(q, "; '") && !strings.Contains(q, `'\''`) {
		t.Errorf("quoting allowed breakout in %q", q)
	}
	// And the canonical escaped form must be present.
	if !strings.Contains(q, `'\''`) {
		t.Errorf("expected the \\'' escape idiom in %q", q)
	}
}

func TestShellCommandQuoteEscape(t *testing.T) {
	// A projectRoot with a quote still ends up as one safe argument.
	got := shellCommand("/bin/manigot", "a", "b", "/path/with'quote", "claude-pro")
	// The root must appear escaped, not as a raw close-quote.
	if !strings.Contains(got, `'/path/with'\''quote'`) {
		t.Errorf("root quote not escaped in %q", got)
	}
}

// --- tmux split-pane construction (TASK-2) -----------------------------

// TestBuildCmdTmuxUsesSplitWindow confirms buildCmd's tmux branch runs
// `tmux split-window -h -l 35%` (side-by-side, new pane sized to 35% of the
// window so the TUI's own pane keeps the majority share — TASK-1 scope
// decision) rather than `tmux new-window`, and describes the result as a
// "tmux pane". The -P -F '#{pane_id}' flags are TASK-3's replace-policy
// plumbing: they make the split-window client print the new pane's id, which
// launchTmuxPane records and tags (select-pane -T — split-window itself has
// no -T on any released tmux, so tagging is deliberately not constructed
// here). This only exercises the command *construction*; it stubs `tmux` on
// PATH rather than running a real tmux server, so it cannot verify the pane
// actually lands next to the TUI at runtime — see TASK-2's own risk note and
// TASK-4/TASK-5's manual-verification caveat for that gap.
func TestBuildCmdTmuxUsesSplitWindow(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "tmux")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	cmd, desc, err := buildCmd("echo hi", "")
	if err != nil {
		t.Fatalf("buildCmd: %v", err)
	}
	if desc != "tmux pane" {
		t.Errorf("desc = %q, want %q", desc, "tmux pane")
	}
	if cmd.Path != stub {
		t.Errorf("cmd.Path = %q, want %q (resolved tmux binary)", cmd.Path, stub)
	}
	want := []string{"tmux", "split-window", "-h", "-l", "35%", "-P", "-F", "#{pane_id}", "bash", "-lc", "echo hi"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, want)
	}
	for i, a := range want {
		if cmd.Args[i] != a {
			t.Errorf("cmd.Args[%d] = %q, want %q (full: %v)", i, cmd.Args[i], a, cmd.Args)
		}
	}
}

// --- tmux replace/reuse tracking (TASK-3, TASK-5) ----------------------

// tmuxStub is a fake `tmux` binary used to exercise the replace/reuse
// tracking logic without a real tmux server. It records every invocation to a
// log file (so tests can assert both the content and the order of the commands
// manigot *runs*) and answers like a minimal tmux:
//
//   - list-panes prints the contents of panesFile verbatim — each line is one
//     "<pane_id> <pane_title>", matching what `list-panes -s -F '#{pane_id}
//     #{pane_title}'` prints in a real tmux.
//   - split-window prints the next "%N" pane id (from an incrementing
//     counterFile, starting at %100 so ids never collide with the listed
//     panes), matching what `-P -F '#{pane_id}'` prints for a real split.
//   - select-pane does nothing, unless TMUX_STUB_SELECT_PANE_FAIL (exit 1 —
//     the "tagging failed but the pane is live" case) is set.
//   - kill-pane does nothing, unless TMUX_STUB_KILL_FAIL (exit 1 — the "user
//     already closed the pane" case) or TMUX_STUB_SPLIT_FAIL (exit 1 — the
//     "tmux is broken" case for split-window) is set.
//
// Like the existing TestJdiStartsResolvedCommandDetached stub, this can only
// confirm that the commands manigot runs are the right ones — real tmux
// session behavior (actual pane creation/listing/killing) cannot be verified
// without a live tmux server, which this environment does not have (TASK-5
// risk note).
type tmuxStub struct {
	log     string
	panes   string
	counter string
	path    string
}

func newTmuxStub(t *testing.T) *tmuxStub {
	t.Helper()
	dir := t.TempDir()
	s := &tmuxStub{
		log:     filepath.Join(dir, "calls.log"),
		panes:   filepath.Join(dir, "panes.txt"),
		counter: filepath.Join(dir, "counter.txt"),
		path:    filepath.Join(dir, "tmux"),
	}
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> '" + s.log + "'\n" +
		"case \"$1\" in\n" +
		// Only shell builtins below: PATH is set to the stub dir alone (so
		// exec.LookPath("tmux") resolves the stub, not a real tmux), so any
		// external command would not be found.
		"  list-panes) while read -r line; do echo \"$line\"; done < '" + s.panes + "' ;;\n" +
		"  split-window) [ -n \"$TMUX_STUB_SPLIT_FAIL\" ] && exit 1; n=0; [ -f '" + s.counter + "' ] && read -r n < '" + s.counter + "'; n=$((n+1)); echo \"$n\" > '" + s.counter + "'; echo \"%$n\" ;;\n" +
		"  kill-pane) [ -n \"$TMUX_STUB_KILL_FAIL\" ] && exit 1 ;;\n" +
		"  select-pane) [ -n \"$TMUX_STUB_SELECT_PANE_FAIL\" ] && exit 1 ;;\n" +
		"esac\n"
	if err := os.WriteFile(s.path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// Empty panes file (list-panes succeeds but lists nothing) and a counter
	// starting at 99, so the first split prints %100 — never colliding with
	// the small ids tests use for the panes they list.
	if err := os.WriteFile(s.panes, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.counter, []byte("99"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	return s
}

// setPanes replaces the pane listing list-panes will print. Each line is one
// pane: "<pane_id> <pane_title>".
func (s *tmuxStub) setPanes(lines ...string) {
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(s.panes, []byte(content), 0o644); err != nil {
		panic(err)
	}
}

// calls returns the recorded invocation log, one `tmux ...` line per call.
func (s *tmuxStub) calls() string {
	raw, err := os.ReadFile(s.log)
	if err != nil {
		return ""
	}
	return string(raw)
}

// indexCalls returns the byte offset of needle in the invocation log, or -1.
func (s *tmuxStub) indexCalls(needle string) int {
	return strings.Index(s.calls(), needle)
}

// TestKillPreviousTmuxPaneKillsTrackedAndTaggedOnly is the core safety test
// for the replace policy's "must never kill a pane it didn't itself open":
// the previously tracked pane id and every pane whose title is tmuxPaneTag
// are killed, and nothing else in the session is touched.
func TestKillPreviousTmuxPaneKillsTrackedAndTaggedOnly(t *testing.T) {
	stub := newTmuxStub(t)
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = "%1" // tracked from a previous launch in this TUI process
	stub.setPanes(
		"%2 "+tmuxPaneTag, // tagged — must be killed
		"%3 other pane",   // untagged — must be left alone
		"%4",              // no title at all — must be left alone
	)

	if err := killPreviousTmuxPane(); err != nil {
		t.Fatalf("killPreviousTmuxPane: %v", err)
	}
	calls := stub.calls()
	for _, want := range []string{
		"list-panes -s -F #{pane_id} #{pane_title}",
		"kill-pane -t %1",
		"kill-pane -t %2",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("missing %q in stub call log:\n%s", want, calls)
		}
	}
	for _, not := range []string{"kill-pane -t %3", "kill-pane -t %4"} {
		if strings.Contains(calls, not) {
			t.Errorf("unexpectedly killed a pane manigot did not open (%q):\n%s", not, calls)
		}
	}
}

// TestKillPreviousTmuxPaneSkipsWhenNothingToReplace: with no tracked pane and
// no tagged pane in the session, replace must be a complete no-op — not a
// single kill-pane. Also exercises the empty list-panes output edge case.
func TestKillPreviousTmuxPaneSkipsWhenNothingToReplace(t *testing.T) {
	stub := newTmuxStub(t)
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = ""
	stub.setPanes("%9 unrelated")

	if err := killPreviousTmuxPane(); err != nil {
		t.Fatalf("killPreviousTmuxPane: %v", err)
	}
	if strings.Contains(stub.calls(), "kill-pane") {
		t.Errorf("kill-pane called with nothing to replace:\n%s", stub.calls())
	}
}

// TestKillPreviousTmuxPaneToleratesAlreadyClosedPane: a kill-pane failure
// (the user closed the pane themselves in the meantime) is not an error — it
// is the tolerated no-op case from TASK-3.
func TestKillPreviousTmuxPaneToleratesAlreadyClosedPane(t *testing.T) {
	newTmuxStub(t) // sets up the stubbed tmux on PATH + $TMUX; no panes listed
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = "%1"
	t.Setenv("TMUX_STUB_KILL_FAIL", "1")

	if err := killPreviousTmuxPane(); err != nil {
		t.Fatalf("killPreviousTmuxPane should tolerate a missing pane, got: %v", err)
	}
}

// TestKillPreviousTmuxPaneListFailure: when tmux cannot list panes at all,
// the error is surfaced so the caller can decide — launchTmuxPane continues
// anyway (see TestLaunchTmuxPaneContinuesAfterListFailure).
func TestKillPreviousTmuxPaneListFailure(t *testing.T) {
	stub := newTmuxStub(t)
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = ""
	if err := os.Remove(stub.panes); err != nil {
		t.Fatal(err) // a missing file makes the stub's `cat` fail, like a real tmux error
	}

	err := killPreviousTmuxPane()
	if err == nil {
		t.Fatal("expected an error when list-panes fails")
	}
	if !strings.Contains(err.Error(), "list-panes") {
		t.Errorf("error should mention list-panes, got: %v", err)
	}
}

// TestLaunchTmuxPaneReplacesBeforeSplittingAndRecordsPaneID exercises the
// whole TASK-3 sequence: the previous manigot pane (identified by its title
// tag — a fresh TUI process has no tracked id) is killed before the new
// split, the new pane's id (printed by split-window -P) is tagged via
// select-pane -T (the replace-policy tag survives a TUI restart) and recorded
// for the next replace. The log's ordering proves kill-before-split and
// tag-after-split, and that select-pane targets only the pane just created.
func TestLaunchTmuxPaneReplacesBeforeSplittingAndRecordsPaneID(t *testing.T) {
	stub := newTmuxStub(t)
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = "" // fresh TUI process: only the tag identifies the old pane
	stub.setPanes("%1 " + tmuxPaneTag)

	desc, err := launchTmuxPane("echo hi")
	if err != nil {
		t.Fatalf("launchTmuxPane: %v", err)
	}
	if desc != "tmux pane" {
		t.Errorf("desc = %q, want %q", desc, "tmux pane")
	}
	if tmuxLastPaneID != "%100" {
		t.Errorf("tmuxLastPaneID = %q, want %q (the id split-window printed)", tmuxLastPaneID, "%100")
	}
	calls := stub.calls()
	if !strings.Contains(calls, "kill-pane -t %1") {
		t.Errorf("previous tagged pane not killed:\n%s", calls)
	}
	if !strings.Contains(calls, "split-window -h -l 35% -P -F #{pane_id} bash -lc echo hi") {
		t.Errorf("split-window invocation missing or wrong:\n%s", calls)
	}
	// The tag must be applied with select-pane (split-window has no -T on any
	// released tmux), to the pane the split just created — %100 — and to no
	// other pane. ("-t %1 -T" rather than "-t %1" so the %100 line cannot
	// false-positive the check.)
	if !strings.Contains(calls, "select-pane -t %100 -T manigot") {
		t.Errorf("select-pane tagging of the new pane missing or wrong:\n%s", calls)
	}
	if strings.Contains(calls, "select-pane -t %1 -T") {
		t.Errorf("select-pane tagged the previous pane instead of the new one:\n%s", calls)
	}
	if i, j := stub.indexCalls("kill-pane -t %1"), stub.indexCalls("split-window"); i < 0 || j < 0 || i > j {
		t.Errorf("kill-pane (%d) must precede split-window (%d):\n%s", i, j, calls)
	}
	if i, j := stub.indexCalls("split-window"), stub.indexCalls("select-pane"); i < 0 || j < 0 || i > j {
		t.Errorf("split-window (%d) must precede select-pane (%d):\n%s", i, j, calls)
	}
}

// TestLaunchTmuxPaneSecondLaunchReplacesRecordedPane: two launches through
// launchTmuxPane — the second must kill the pane the first recorded (by id).
// This is the case the in-memory tracking exists for: the running agent may
// have overwritten the pane title in the meantime, so the tag alone could not
// find it.
func TestLaunchTmuxPaneSecondLaunchReplacesRecordedPane(t *testing.T) {
	stub := newTmuxStub(t)
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = ""

	if _, err := launchTmuxPane("echo one"); err != nil {
		t.Fatalf("first launchTmuxPane: %v", err)
	}
	first := tmuxLastPaneID
	if first == "" {
		t.Fatal("first launch did not record a pane id")
	}

	// The old pane no longer appears tagged in list-panes (as if the running
	// agent retitled it) — only the in-memory id can find it now.
	stub.setPanes("%9 unrelated")
	if _, err := launchTmuxPane("echo two"); err != nil {
		t.Fatalf("second launchTmuxPane: %v", err)
	}
	if want := "kill-pane -t " + first; !strings.Contains(stub.calls(), want) {
		t.Errorf("second launch did not kill the recorded pane %s:\n%s", first, stub.calls())
	}
}

// TestLaunchTmuxPaneContinuesAfterListFailure documents the best-effort
// replace decision from TASK-3: a list-panes failure must not abort the new
// launch — if tmux is in a state where list-panes fails, the split below will
// fail with the accurate error instead.
func TestLaunchTmuxPaneContinuesAfterListFailure(t *testing.T) {
	stub := newTmuxStub(t)
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = ""
	if err := os.Remove(stub.panes); err != nil {
		t.Fatal(err)
	}

	desc, err := launchTmuxPane("echo hi")
	if err != nil {
		t.Fatalf("launchTmuxPane should continue after list-panes failure, got: %v", err)
	}
	if desc != "tmux pane" {
		t.Errorf("desc = %q, want %q", desc, "tmux pane")
	}
	if tmuxLastPaneID == "" {
		t.Error("launch succeeded but no pane id was recorded")
	}
}

// TestLaunchTmuxPaneSplitFailureReturnsError: a split-window failure (the
// tmux server is gone, say) is surfaced as the launch error.
func TestLaunchTmuxPaneSplitFailureReturnsError(t *testing.T) {
	newTmuxStub(t) // sets up the stubbed tmux on PATH + $TMUX; no panes listed
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = ""
	t.Setenv("TMUX_STUB_SPLIT_FAIL", "1")

	_, err := launchTmuxPane("echo hi")
	if err == nil {
		t.Fatal("expected an error when split-window fails")
	}
	if !strings.Contains(err.Error(), "tmux pane") {
		t.Errorf("error should say where the launch was attempted, got: %v", err)
	}
}

// TestLaunchDetachedRoutesToTmuxPane: with $TMUX set and a tmux binary on
// PATH, launchDetached (the shared tail of Agent and Quick) must route to the
// synchronous tmux-pane path rather than the detached terminal-emulator tail.
func TestLaunchDetachedRoutesToTmuxPane(t *testing.T) {
	stub := newTmuxStub(t)
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = ""

	desc, err := launchDetached("echo hi", "")
	if err != nil {
		t.Fatalf("launchDetached: %v", err)
	}
	if desc != "tmux pane" {
		t.Errorf("desc = %q, want %q", desc, "tmux pane")
	}
	if !strings.Contains(stub.calls(), "split-window -h -l 35% -P -F #{pane_id} bash -lc echo hi") {
		t.Errorf("tmux split-window not invoked via launchDetached:\n%s", stub.calls())
	}
}

// TestLaunchTmuxPaneContinuesAfterSelectPaneFailure documents the best-effort
// tagging decision from the TASK-2/TASK-3 rework: the pane is already live and
// the in-memory tmuxLastPaneID covers replacement within this TUI process, so
// a select-pane -T failure (the tag that would survive a TUI restart) must not
// abort a launch that already succeeded.
func TestLaunchTmuxPaneContinuesAfterSelectPaneFailure(t *testing.T) {
	newTmuxStub(t) // sets up the stubbed tmux on PATH + $TMUX; no panes listed
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = ""
	t.Setenv("TMUX_STUB_SELECT_PANE_FAIL", "1")

	desc, err := launchTmuxPane("echo hi")
	if err != nil {
		t.Fatalf("launchTmuxPane should continue after select-pane failure, got: %v", err)
	}
	if desc != "tmux pane" {
		t.Errorf("desc = %q, want %q", desc, "tmux pane")
	}
	if tmuxLastPaneID == "" {
		t.Error("launch succeeded but no pane id was recorded")
	}
}

// TestAgentAndQuickShareOneTrackedPane is the requirement behind "a single
// shared tracked pane across both launch.Agent and launch.Quick": a Quick
// launch after an Agent launch must replace the pane the Agent opened, so at
// most one manigot pane exists regardless of which entry point opened it.
func TestAgentAndQuickShareOneTrackedPane(t *testing.T) {
	stub := newTmuxStub(t)
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = ""

	manigot := filepath.Join(t.TempDir(), "manigot")
	if err := os.WriteFile(manigot, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_BIN", manigot)
	root := t.TempDir()

	if _, err := Agent("developer", "t5oc4j", root, "claude-pro", ""); err != nil {
		t.Fatalf("Agent: %v", err)
	}
	agentPane := tmuxLastPaneID
	if agentPane == "" {
		t.Fatal("Agent launch did not record a pane id")
	}

	if _, err := Quick(root, "claude-pro", ""); err != nil {
		t.Fatalf("Quick: %v", err)
	}
	if want := "kill-pane -t " + agentPane; !strings.Contains(stub.calls(), want) {
		t.Errorf("Quick launch did not replace the pane Agent opened (%s):\n%s", agentPane, stub.calls())
	}
}

func TestBuildCmdSmoke(t *testing.T) {
	// buildCmd must not panic and must return either a runnable command or a
	// descriptive error, depending on what is installed in this environment.
	cmd, desc, err := buildCmd("echo hi", "")
	if err != nil {
		if !strings.Contains(err.Error(), "terminal launcher") {
			t.Errorf("unexpected error: %v", err)
		}
		// desc should be empty on error.
		if desc != "" {
			t.Errorf("on error, desc = %q, want empty", desc)
		}
		return
	}
	if cmd == nil {
		t.Fatal("buildCmd returned nil cmd and nil error")
	}
	if cmd.Path == "" {
		t.Error("returned cmd has empty Path")
	}
	if desc == "" {
		t.Error("returned cmd has empty description")
	}
}

// --- terminal override (config.Settings.Terminal, TASK-3) ------------------

// stubBinary writes an executable no-op shell script at dir/name and returns
// its path, so exec.LookPath(name) resolves it when dir is on $PATH.
func stubBinary(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestBuildCmdOverrideBypassesTmux confirms an override takes over even
// inside tmux (TASK-1 point 2): with $TMUX set and a tmux binary present,
// buildCmd must still invoke the override, not the tmux split-pane branch.
func TestBuildCmdOverrideBypassesTmux(t *testing.T) {
	dir := t.TempDir()
	stubBinary(t, dir, "tmux")
	kitty := stubBinary(t, dir, "kitty")
	t.Setenv("PATH", dir)
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	cmd, desc, err := buildCmd("echo hi", "kitty")
	if err != nil {
		t.Fatalf("buildCmd: %v", err)
	}
	if desc != "kitty" {
		t.Errorf("desc = %q, want %q", desc, "kitty")
	}
	if cmd.Path != kitty {
		t.Errorf("cmd.Path = %q, want %q", cmd.Path, kitty)
	}
}

// TestLaunchDetachedOverrideBypassesTmuxPane confirms the same at the
// launchDetached level, which is what actually decides whether to route to
// launchTmuxPane at all.
func TestLaunchDetachedOverrideBypassesTmuxPane(t *testing.T) {
	dir := t.TempDir()
	stubBinary(t, dir, "tmux")
	stubBinary(t, dir, "kitty")
	t.Setenv("PATH", dir)
	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	t.Cleanup(func() { tmuxLastPaneID = "" })
	tmuxLastPaneID = ""

	desc, err := launchDetached("echo hi", "kitty")
	if err != nil {
		t.Fatalf("launchDetached: %v", err)
	}
	if desc != "kitty" {
		t.Errorf("desc = %q, want %q (tmux must be bypassed)", desc, "kitty")
	}
	if tmuxLastPaneID != "" {
		t.Errorf("tmuxLastPaneID = %q, want empty (tmux path must not have run)", tmuxLastPaneID)
	}
}

// TestBuildOverrideCmdKnownNameReusesConvention confirms an override whose
// binary matches one of terminalCandidates' own names reuses that name's
// existing flag convention rather than defaulting to -e (TASK-1 point 3).
func TestBuildOverrideCmdKnownNameReusesConvention(t *testing.T) {
	dir := t.TempDir()
	konsole := stubBinary(t, dir, "konsole")
	gnomeTerminal := stubBinary(t, dir, "gnome-terminal")
	t.Setenv("PATH", dir)

	cmd, desc, err := buildOverrideCmd("echo hi", "konsole")
	if err != nil {
		t.Fatalf("buildOverrideCmd: %v", err)
	}
	if desc != "konsole" {
		t.Errorf("desc = %q, want %q", desc, "konsole")
	}
	want := []string{"konsole", "-e", "bash", "-lc", "echo hi"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, want)
	}
	for i, a := range want {
		if cmd.Args[i] != a {
			t.Errorf("cmd.Args[%d] = %q, want %q (full: %v)", i, cmd.Args[i], a, cmd.Args)
		}
	}
	if cmd.Path != konsole {
		t.Errorf("cmd.Path = %q, want %q", cmd.Path, konsole)
	}

	cmd, desc, err = buildOverrideCmd("echo hi", "gnome-terminal")
	if err != nil {
		t.Fatalf("buildOverrideCmd: %v", err)
	}
	if desc != "gnome-terminal" {
		t.Errorf("desc = %q, want %q", desc, "gnome-terminal")
	}
	want = []string{"gnome-terminal", "--", "bash", "-lc", "echo hi"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, want)
	}
	for i, a := range want {
		if cmd.Args[i] != a {
			t.Errorf("cmd.Args[%d] = %q, want %q (full: %v)", i, cmd.Args[i], a, cmd.Args)
		}
	}
	if cmd.Path != gnomeTerminal {
		t.Errorf("cmd.Path = %q, want %q", cmd.Path, gnomeTerminal)
	}
}

// TestBuildOverrideCmdKnownNameCaseInsensitive confirms the known-name match
// is case-insensitive.
func TestBuildOverrideCmdKnownNameCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	stubBinary(t, dir, "Xterm")
	t.Setenv("PATH", dir)

	cmd, _, err := buildOverrideCmd("echo hi", "Xterm")
	if err != nil {
		t.Fatalf("buildOverrideCmd: %v", err)
	}
	if len(cmd.Args) < 2 || cmd.Args[1] != "-e" {
		t.Errorf("cmd.Args = %v, want [Xterm -e ...] (case-insensitive known-name match)", cmd.Args)
	}
}

// TestBuildOverrideCmdUnknownNameDefaultsToDashE confirms an override binary
// not in terminalCandidates falls back to the -e convention (TASK-1 point 3).
func TestBuildOverrideCmdUnknownNameDefaultsToDashE(t *testing.T) {
	dir := t.TempDir()
	stubBinary(t, dir, "alacritty")
	t.Setenv("PATH", dir)

	cmd, desc, err := buildOverrideCmd("echo hi", "alacritty")
	if err != nil {
		t.Fatalf("buildOverrideCmd: %v", err)
	}
	if desc != "alacritty" {
		t.Errorf("desc = %q, want %q", desc, "alacritty")
	}
	want := []string{"alacritty", "-e", "bash", "-lc", "echo hi"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, want)
	}
	for i, a := range want {
		if cmd.Args[i] != a {
			t.Errorf("cmd.Args[%d] = %q, want %q (full: %v)", i, cmd.Args[i], a, cmd.Args)
		}
	}
}

// TestBuildOverrideCmdPassesLeadingArgs confirms trailing tokens in the
// override value (e.g. "alacritty --some-flag") are passed through as leading
// args before the flag convention, mirroring config.Settings.Editor's own
// "trailing args" allowance.
func TestBuildOverrideCmdPassesLeadingArgs(t *testing.T) {
	dir := t.TempDir()
	stubBinary(t, dir, "alacritty")
	t.Setenv("PATH", dir)

	cmd, _, err := buildOverrideCmd("echo hi", "alacritty --some-flag")
	if err != nil {
		t.Fatalf("buildOverrideCmd: %v", err)
	}
	want := []string{"alacritty", "--some-flag", "-e", "bash", "-lc", "echo hi"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, want)
	}
	for i, a := range want {
		if cmd.Args[i] != a {
			t.Errorf("cmd.Args[%d] = %q, want %q (full: %v)", i, cmd.Args[i], a, cmd.Args)
		}
	}
}

// TestBuildOverrideCmdMissingBinaryErrors confirms a typo'd/missing override
// binary surfaces a clear error instead of silently falling back to
// auto-detect (TASK-1 point 3).
func TestBuildOverrideCmdMissingBinaryErrors(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty PATH: nothing resolves

	_, _, err := buildOverrideCmd("echo hi", "not-a-real-terminal")
	if err == nil {
		t.Fatal("expected an error for a missing override binary")
	}
	if !strings.Contains(err.Error(), "not-a-real-terminal") {
		t.Errorf("error should name the missing binary, got: %v", err)
	}
}

// TestBuildCmdUnsetTerminalUnchanged is a smoke check that buildCmd with an
// empty terminal still goes through the auto-detect chain (not the override
// path) — the substantive coverage is TestBuildCmdSmoke/TestBuildCmdTmux*,
// this just confirms an empty string doesn't accidentally match
// buildOverrideCmd's "blank" guard as a real override.
func TestBuildCmdUnsetTerminalUnchanged(t *testing.T) {
	cmd, desc, err := buildCmd("echo hi", "")
	if err != nil {
		if !strings.Contains(err.Error(), "terminal launcher") {
			t.Errorf("unexpected error for empty terminal: %v", err)
		}
		return
	}
	if cmd == nil || desc == "" {
		t.Errorf("buildCmd with empty terminal returned incomplete result: cmd=%v desc=%q", cmd, desc)
	}
}

// --- Jdi (detached, no terminal window — Decision 7a) -----------------------

func TestJdiUnresolvable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("MANIGOT_JDI_BIN", "")
	t.Setenv("MANIGOT_HOME", "")

	err := Jdi("ab0001_x", t.TempDir(), "")
	if err == nil {
		t.Fatal("expected an error when mg-jdi cannot be resolved")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should explain the command is missing; got: %v", err)
	}
}

// The resolved command must be started detached (no terminal emulator
// involved at all, unlike Agent/Quick), by absolute path, in projectRoot,
// with $PWD matching — the same invocation contract hostcmd's NewJob uses.
func TestJdiStartsResolvedCommandDetached(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("MANIGOT_HOME", "")

	root := t.TempDir()
	out := filepath.Join(root, "args.txt")

	stub := filepath.Join(t.TempDir(), "stub.sh")
	script := "#!/bin/sh\n" +
		"{ echo \"argv0=$0\"; echo \"pwd=$PWD\"; echo \"cwd=$(pwd)\"; " +
		"for a in \"$@\"; do echo \"arg=$a\"; done; } > " + out + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_JDI_BIN", stub)

	if err := Jdi("ab0001_x", root, ""); err != nil {
		t.Fatalf("Jdi: %v", err)
	}

	// Jdi starts the process detached and reaps it asynchronously — Start()
	// only guarantees the process began, not that it has finished writing
	// its marker file yet, so poll briefly rather than reading immediately.
	var raw []byte
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(out); err == nil {
			raw = data
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if raw == nil {
		t.Fatal("stub did not run within the timeout")
	}

	recorded := string(raw)
	for _, want := range []string{
		"argv0=" + stub, // invoked by absolute path, not by bare name
		"pwd=" + root,   // $PWD explicitly set for job.FindProjectRoot
		"cwd=" + root,   // and the real cwd agrees
		"arg=--job",
		"arg=ab0001_x",
		"arg=--profile",
		"arg=" + config.ProfileClaudePro, // empty profile defaults to claude-pro, matching Agent/Quick
	} {
		if !strings.Contains(recorded, want) {
			t.Errorf("missing %q in stub record:\n%s", want, recorded)
		}
	}
}

// TestJdiForwardsGivenProfile confirms a non-empty profile (TASK-6) is
// forwarded to mg-jdi as its own --profile flag unchanged, not overridden by
// the claude-pro default TestJdiStartsResolvedCommandDetached exercises.
func TestJdiForwardsGivenProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("MANIGOT_HOME", "")

	root := t.TempDir()
	out := filepath.Join(root, "args.txt")

	stub := filepath.Join(t.TempDir(), "stub.sh")
	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"arg=$a\"; done > " + out + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANIGOT_JDI_BIN", stub)

	if err := Jdi("ab0001_x", root, config.ProfileZAI); err != nil {
		t.Fatalf("Jdi: %v", err)
	}

	var raw []byte
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(out); err == nil {
			raw = data
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if raw == nil {
		t.Fatal("stub did not run within the timeout")
	}

	recorded := string(raw)
	for _, want := range []string{"arg=--profile", "arg=" + config.ProfileZAI} {
		if !strings.Contains(recorded, want) {
			t.Errorf("missing %q in stub record:\n%s", want, recorded)
		}
	}
}
