package job

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ManigotRelDir is manigot's own directory inside a target project, relative
// to the project root: the home for host-side tooling state that is not job
// content and does not belong in docs/. .manigot/manigot.json holds the
// committable project settings (see the project package); .manigot/jdi-status/
// holds mg-jdi's ephemeral run state (gitignored, see .gitignore).
const ManigotRelDir = ".manigot"

// JDISidecarDirName is the sidecar directory (Decision 4 in the "fully
// autonomous mode" brief) mg-jdi writes its per-job status and run log
// into, under ManigotRelDir — deliberately outside any job's own directory
// (docs/jobs/<name>/) so it can never be swept into an agent's own
// `git add -A`, and gitignored entirely (see .gitignore).
const JDISidecarDirName = "jdi-status"

// jdiStatusFileName / jdiRunLogFileName are the two files living under each
// job's JDIStatusDir.
const (
	jdiStatusFileName = "status"
	jdiRunLogFileName = "run.log"
)

// JDIState is mg-jdi's reported run state for a job, read from its status
// sidecar file.
type JDIState string

const (
	// JDIRunning means mg-jdi is (or, absent a fresher update, very
	// recently was) actively driving this job.
	JDIRunning JDIState = "running"
	// JDIStoppedFinished means mg-jdi's last run ended because the job's
	// verdict was APPROVED.
	JDIStoppedFinished JDIState = "stopped:finished"
	// JDIStoppedNeedsHuman means mg-jdi's last run ended because it needed
	// a human (retry budget exhausted, a NEEDS-HUMAN-INPUT marker, or the
	// stall backstop — see tui/cmd/jdi).
	JDIStoppedNeedsHuman JDIState = "stopped:needs-human"
)

// jdiRunningStaleAfter bounds how long a JDIRunning status is trusted
// without an update before ReadJDIStatus degrades it to "no run in
// progress" (Decision 4a). mg-jdi only updates the status file at
// agent-invocation boundaries, not continuously while one is in flight, so
// this must be generous enough that a single long-running agent call isn't
// mistaken for a killed mg-jdi process.
const jdiRunningStaleAfter = 30 * time.Minute

// JDIStatus is mg-jdi's last-known status for a job.
type JDIStatus struct {
	State   JDIState
	Agent   string
	Updated time.Time
}

// jdiStatusFile is the on-disk JSON shape of the status sidecar file.
type jdiStatusFile struct {
	State   JDIState `json:"state"`
	Agent   string   `json:"agent"`
	Updated string   `json:"updated"` // RFC3339
}

// JDIStatusDir returns the sidecar directory for job jobName under root.
func JDIStatusDir(root, jobName string) string {
	return filepath.Join(root, ManigotRelDir, JDISidecarDirName, jobName)
}

// JDIStatusPath returns the status file path within JDIStatusDir.
func JDIStatusPath(root, jobName string) string {
	return filepath.Join(JDIStatusDir(root, jobName), jdiStatusFileName)
}

// JDIRunLogPath returns the run.log path within JDIStatusDir — the
// append-only transcript tui/cmd/jdi writes (Decision 7) and the TUI's log
// tab (TASK-9) reads.
func JDIRunLogPath(root, jobName string) string {
	return filepath.Join(JDIStatusDir(root, jobName), jdiRunLogFileName)
}

// ReadJDIStatus reads job jobName's mg-jdi status sidecar under root. ok is
// false when there is nothing live to report: no sidecar file exists, it
// can't be parsed, its state isn't one of the three known values, or it
// claims JDIRunning but hasn't been updated in over jdiRunningStaleAfter
// (mg-jdi was almost certainly killed mid-run). Per Decision 4a, a stale or
// unreadable file must never be shown as if it were live — this is the
// normal path through which callers should read it, the one exception being
// StaleRunningJDI's raw read for mg-jdi's own next-start crash check.
func ReadJDIStatus(root, jobName string) (JDIStatus, bool) {
	st, ok := readJDIStatusRaw(root, jobName)
	if !ok {
		return JDIStatus{}, false
	}
	if st.State == JDIRunning && time.Since(st.Updated) > jdiRunningStaleAfter {
		return JDIStatus{}, false
	}
	return st, true
}

// readJDIStatusRaw is the shared parse of the status sidecar: it returns the
// parsed status with ok=false when the file is missing, unparseable, or
// holds an unknown state value. Unlike ReadJDIStatus it does NOT degrade a
// stale JDIRunning entry away — staleness is the caller's business.
func readJDIStatusRaw(root, jobName string) (JDIStatus, bool) {
	data, err := os.ReadFile(JDIStatusPath(root, jobName))
	if err != nil {
		return JDIStatus{}, false
	}

	var raw jdiStatusFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return JDIStatus{}, false
	}

	updated, err := time.Parse(time.RFC3339, raw.Updated)
	if err != nil {
		return JDIStatus{}, false
	}

	switch raw.State {
	case JDIRunning, JDIStoppedFinished, JDIStoppedNeedsHuman:
	default:
		return JDIStatus{}, false
	}

	return JDIStatus{State: raw.State, Agent: raw.Agent, Updated: updated}, true
}

// StaleRunningJDI reports whether job jobName's mg-jdi sidecar claims a run
// is in progress (JDIRunning) but has not been updated in over
// jdiRunningStaleAfter — the on-disk signature of an mg-jdi process that was
// killed (SIGKILL, OOM, host reboot) without ever reporting a stop. ok is
// true only for that stale-running case: a stopped:* state, a fresh running
// state, or a missing/unparseable sidecar all report ok=false. It is the raw
// counterpart to ReadJDIStatus's degradation (same staleness constant, same
// parse) and exists so mg-jdi's next-start crash check (see
// cmd/mg/jdi.go's notifyCrashedRun) can distinguish "no run in progress"
// from "a run died mid-flight".
func StaleRunningJDI(root, jobName string) (JDIStatus, bool) {
	st, ok := readJDIStatusRaw(root, jobName)
	if !ok || st.State != JDIRunning {
		return JDIStatus{}, false
	}
	if time.Since(st.Updated) <= jdiRunningStaleAfter {
		return JDIStatus{}, false
	}
	return st, true
}

// WriteJDIStatus writes/overwrites job jobName's status sidecar under root.
// Called by tui/cmd/jdi at each state transition (Decision 4a): once before
// invoking an agent (JDIRunning, that agent's name) and once more at loop
// exit (JDIStoppedFinished or JDIStoppedNeedsHuman, the last agent that
// ran). Lives here (not in tui/cmd/jdi) so the write and read sides share
// one on-disk format definition rather than risking drift between two
// independent copies.
func WriteJDIStatus(root, jobName string, state JDIState, agent string) error {
	dir := JDIStatusDir(root, jobName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw := jdiStatusFile{
		State:   state,
		Agent:   agent,
		Updated: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(JDIStatusPath(root, jobName), data, 0o644)
}

// jdiRunLogTailBytes bounds how much of a job's run.log ReadJDIRunLogTail
// loads into memory — a tail, not "load everything" (TASK-9's own
// requirement for a large/growing file), the same kind of bounded-viewport
// approach the markdown viewer already uses for job files. Generous enough
// to comfortably hold many invocations' worth of text while staying a small,
// fixed cost regardless of how long a job's full history grows.
const jdiRunLogTailBytes = 256 * 1024 // 256 KiB

// ReadJDIRunLogTail reads up to the last jdiRunLogTailBytes of job jobName's
// run.log sidecar (JDIRunLogPath) — the TUI's log tab (TASK-9), the primary
// way to see what mg-jdi is doing from inside the TUI (Decision 7/7a).
//
// ok is false only when there is no run.log at all yet — the common case of
// a job mg-jdi has never driven — so callers can show a distinct "no run
// yet" placeholder instead of an empty tab. An existing-but-empty file (the
// sidecar directory exists but no invocation has completed yet) returns
// ("", true): still "there is a run happening," just nothing logged yet.
func ReadJDIRunLogTail(root, jobName string) (content string, ok bool) {
	f, err := os.Open(JDIRunLogPath(root, jobName))
	if err != nil {
		return "", false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", false
	}
	size := info.Size()
	if size == 0 {
		return "", true
	}

	start := int64(0)
	truncated := false
	if size > jdiRunLogTailBytes {
		start = size - jdiRunLogTailBytes
		truncated = true
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", false
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return "", false
	}

	text := string(data)
	if truncated {
		// Drop a possibly-partial first line so the tail starts cleanly at
		// an invocation boundary rather than mid-line.
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			text = text[idx+1:]
		}
		text = "… (log truncated — showing the most recent portion)\n\n" + text
	}
	return text, true
}
