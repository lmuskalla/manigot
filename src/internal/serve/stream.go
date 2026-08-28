package serve

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// --- GET /projects/{project}/jobs/{job}/session-log/stream (TASK-12) ---------

// The session-log stream is Server-Sent Events over the plain net/http server:
// no upgrade handshake, no framing protocol, no new dependency — the "less
// code against the existing server" choice the brief's Notes steer toward
// (Go's stdlib has no WebSocket support natively; SSE is just a long-lived
// chunked response with `data:` frames and http.Flusher).
//
// It is the network twin of the TUI's `l` key: the same
// docs/jobs/<id>_<slug>/session.log the TUI tails locally, tailed over HTTP
// instead. The file is a local append-only log, so the "watch for growth"
// mechanism is a plain stat+read-new-bytes poll — no fsnotify/inotify.
//
// Event protocol (consumed by job three's web UI; also human-readable under
// curl):
//
//	event: start
//	data: {"offset":123}
//
//	<one default (message) event per log line, in order>
//	<an idle `: keepalive` comment every streamKeepalive, invisible to EventSource>
//
// Offsets: with no ?from=, the stream starts at the file's current size when
// it already exists (tail -f semantics — only new growth) and at byte 0 when
// it doesn't yet (the launch-then-watch flow: everything from the top once the
// run starts writing). ?from=<byte offset> overrides both — the reconnect
// contract: remember the offset (the client tracks it as the bytes it has
// consumed; `start` reports where a fresh stream began), reconnect with
// from=<offset> after a drop, lose nothing. If the file shrinks below the
// offset (truncated/rotated), the stream resets to byte 0 and says so with a
// fresh `start` event.

// streamPollInterval is how often the handler stats session.log for new bytes.
// A var (not const) purely so tests can shorten it.
var streamPollInterval = 500 * time.Millisecond

// streamKeepalive is how long without an emitted event the handler waits
// before writing an SSE comment (`: keepalive`), which carries no data but
// keeps intermediaries (reverse proxies in particular) from declaring the
// connection dead on idle. A var for the same reason.
var streamKeepalive = 30 * time.Second

// streamLineFlushBytes caps how many bytes may buffer while waiting for a
// newline before the partial line is force-flushed as its own event — a
// pathological no-newline writer (a binary blob tee'd into session.log)
// otherwise grows the handler's line buffer without bound.
const streamLineFlushBytes = 1 << 20 // 1 MiB

// handleSessionLogStream streams a job's session.log growth as Server-Sent
// Events. No lock: a read-only file tail, per the brief's Notes — streaming
// logs doesn't touch git state and must not block behind an unrelated
// done/delete.
//
// It returns when the client disconnects (r.Context) OR the daemon shuts down
// (s.shutdownCtx — cancelled at the top of Server.Shutdown, because
// http.Server.Shutdown alone only stops accepting new connections and waits
// for in-flight handlers: it never interrupts a handler stuck in its own poll
// loop, so without this select every open stream would force the drain to wait
// out its full timeout).
func (s *Server) handleSessionLogStream(w http.ResponseWriter, r *http.Request) {
	root, ok := s.resolveProject(w, r.PathValue("project"))
	if !ok {
		return
	}
	j, ok := s.resolveJob(w, root, r.PathValue("job"))
	if !ok {
		return
	}
	logPath := filepath.Join(j.Dir, "session.log") // from the resolved job — never the URL segment

	// Parse ?from= before any SSE headers are written, so a bad value is a
	// clean 400 JSON envelope rather than an error mid-stream.
	fromSet := false
	var from int64
	if raw := r.URL.Query().Get("from"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			writeError(w, http.StatusBadRequest, "from must be a non-negative byte offset")
			return
		}
		from, fromSet = v, true
	}

	flusher, canFlush := w.(http.Flusher)
	if !canFlush {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	// Hint for reverse proxies (nginx): do not buffer this response.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Determine the starting offset. See the event-protocol comment above for
	// the three cases (explicit from / existing file / file not yet created).
	var offset int64
	switch {
	case fromSet:
		offset = from
	default:
		if fi, err := os.Stat(logPath); err == nil {
			offset = fi.Size() // tail -f: only new growth
		} else {
			offset = 0 // no file yet: capture the whole run when it starts
		}
	}

	write := func(event, data string) bool {
		if err := writeSSE(w, event, data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !write("start", startEventData(offset)) {
		return
	}

	var pending []byte   // bytes of a not-yet-complete line (no \n yet)
	lastEmit := time.Now()

	emitLines := func(chunk []byte) bool {
		pending = append(pending, chunk...)
		for {
			idx := bytes.IndexByte(pending, '\n')
			force := idx < 0 && len(pending) >= streamLineFlushBytes
			if idx < 0 && !force {
				break
			}
			var line string
			if idx >= 0 {
				line, pending = string(pending[:idx]), pending[idx+1:]
			} else {
				line, pending = string(pending), nil
			}
			if !write("", line) {
				return false
			}
			lastEmit = time.Now()
		}
		return true
	}

	ticker := time.NewTicker(streamPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done(): // client disconnected
			flushPartial(write, &pending)
			return
		case <-s.shutdownCtx.Done(): // daemon draining
			flushPartial(write, &pending)
			return
		case <-ticker.C:
			fi, err := os.Stat(logPath)
			switch {
			case os.IsNotExist(err):
				// Still waiting for the file to appear — nothing to read.
			case err != nil:
				// Transient stat failure; retry on the next tick.
			case fi.Size() < offset:
				// Truncated/rotated below the offset: restart from the top.
				offset = 0
				if !write("start", startEventData(0)) {
					return
				}
				lastEmit = time.Now()
			case fi.Size() > offset:
				chunk, err := readBytesFrom(logPath, offset)
				if err != nil {
					// Read raced with a writer/truncation; the next tick
					// re-stats and recovers.
					continue
				}
				offset += int64(len(chunk))
				if !emitLines(chunk) {
					return
				}
			}
			if time.Since(lastEmit) >= streamKeepalive {
				if !write("", "") { // `: keepalive` comment — see writeSSE
					return
				}
				lastEmit = time.Now()
			}
		}
	}
}

// flushPartial emits whatever bytes remain in the line buffer as one final
// event, so a trailing line without its newline is not lost when the stream
// ends. Best-effort: the connection may already be gone on a disconnect.
func flushPartial(write func(event, data string) bool, pending *[]byte) {
	if len(*pending) > 0 {
		write("", string(*pending))
		*pending = nil
	}
}

// writeSSE writes one SSE frame. event is the named-event type ("" = the
// default "message" event); data is the payload — each of its lines is
// prefixed `data:` per the SSE wire format, and an empty data writes a bare
// `:` comment (the keepalive form, ignored by every SSE parser).
func writeSSE(w http.ResponseWriter, event, data string) error {
	var b strings.Builder
	if event != "" {
		b.WriteString("event: " + event + "\n")
	}
	if data == "" {
		b.WriteString(": keepalive\n\n")
	} else {
		for _, line := range strings.Split(data, "\n") {
			b.WriteString("data: " + line + "\n")
		}
		b.WriteString("\n")
	}
	_, err := fmt.Fprint(w, b.String())
	return err
}

// readBytesFrom reads the file's bytes from offset to its current end — a
// fresh open per poll, which sidesteps inode rotation entirely (a held file
// handle would silently keep reading the replaced file forever).
func readBytesFrom(path string, offset int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Seek(offset, 0); err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}

// startEventData builds the `start` event's data payload: a JSON object
// carrying the byte offset the stream began (or restarted) at. JSON (not a
// bare number) so the payload is self-describing and forward-compatible —
// the shape the doc comment at the top of this file and startEventOffset
// (the test-side reader) both expect.
func startEventData(offset int64) string {
	return fmt.Sprintf(`{"offset":%d}`, offset)
}

// startEventOffset decodes the `start` event's data payload. Test-side helper
// (the handler writes it; tests — and job three, eventually — read it).
func startEventOffset(t testingT, data string) int64 {
	var v struct {
		Offset int64 `json:"offset"`
	}
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		t.Fatalf("start event data %q is not an offset: %v", data, err)
	}
	return v.Offset
}

// testingT is the subset of testing.T the helpers above need.
type testingT interface {
	Fatalf(format string, args ...any)
}
