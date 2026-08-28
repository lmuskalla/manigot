package job

import (
	"fmt"
	"io"
	"os"
	"time"
)

// OpenSessionLog opens the job's session.log for live appending and writes
// the section header for the invocation about to start — a blank-line
// separator (skipped for the very first section ever written to a fresh
// file) plus the "=== <RFC3339 timestamp> <agent> (attempt N) ===" header,
// the same sectioned shape appendSessionLog used to write post-hoc. The
// returned *SessionLog is the writer the invocation's raw bytes are tee'd
// into as they arrive (see mg-jdi's commandAgentRunner.Run's io.MultiWriter),
// so the verbose log grows live during the run instead of only after it
// finishes — the whole point of the tailing fix. Callers Close it after the
// invocation so the trailing-newline guarantee holds.
//
// This lives in internal/job (rather than staying private to cmd/mg) so both
// mg-jdi's loop and internal/serve's one-shot detached agent-run primitive
// can write a live, tailable session.log — a bare `mg --print --agent X
// --job Y` invocation outside the mg-jdi loop had no session.log writer at
// all before this relocation, despite docs/listener.md's claim that
// session.log is "the captured stream for both containerized and host-mode
// runs".
func OpenSessionLog(path, agent string, attempt int) (*SessionLog, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	// Blank-line separator between sections — the very first section ever
	// written to a fresh file starts at the top instead.
	if st, err := f.Stat(); err != nil {
		f.Close()
		return nil, err
	} else if st.Size() > 0 {
		if _, err := io.WriteString(f, "\n"); err != nil {
			f.Close()
			return nil, err
		}
	}

	if _, err := fmt.Fprintf(f, "=== %s %s (attempt %d) ===\n", time.Now().Format(time.RFC3339), agent, attempt); err != nil {
		f.Close()
		return nil, err
	}
	return &SessionLog{f: f}, nil
}

// SessionLog is the live, per-invocation writer behind a job's session.log
// (docs/jobs/<id>_<slug>/session.log): it owns the open file handle and
// tracks the last byte written through it, so Close can guarantee the
// section ends with a newline — the same trailing-newline guarantee
// appendSessionLog used to make post-hoc, without needing to read the file
// back. The file is opened in append mode, so concurrent appends (a second
// mg-jdi run against the same job, or the next invocation's own section)
// can never overwrite this section.
type SessionLog struct {
	f        *os.File
	lastByte byte // last byte written through this writer; 0 when nothing was written
}

// Write appends p to the underlying file and records the last byte, so Close
// knows whether a trailing newline is still needed.
func (s *SessionLog) Write(p []byte) (int, error) {
	if len(p) > 0 {
		s.lastByte = p[len(p)-1]
	}
	return s.f.Write(p)
}

// Close finishes the section and closes the file: a trailing newline is
// written when the last write didn't already end with one, so the next
// section's header never glues to this section's raw output. Nothing written
// at all (an invocation that produced no output) leaves the header's own
// trailing newline as the section end.
func (s *SessionLog) Close() error {
	if s.lastByte != 0 && s.lastByte != '\n' {
		if _, err := s.f.Write([]byte("\n")); err != nil {
			s.f.Close()
			return err
		}
	}
	return s.f.Close()
}

// EnsureSessionLogFile creates the job's session.log if it doesn't exist yet
// (an empty file is fine — the per-invocation OpenSessionLog appends the
// first section header on demand), so the TUI's "l" tail gate — the file's
// existence — is stable from the moment a run begins, mirroring run.log's
// own up-front creation in openRunLog. Best-effort: a failure only warns;
// the caller still creates the file per invocation.
func EnsureSessionLogFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}
