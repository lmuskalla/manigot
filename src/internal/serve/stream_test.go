package serve

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// shortenStreamPoll shortens the stream handler's poll interval for the
// duration of a test (the handler's live-growth loop runs on it), restoring
// the package default afterwards.
func shortenStreamPoll(t *testing.T) {
	t.Helper()
	old := streamPollInterval
	streamPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { streamPollInterval = old })
}

// appendToFile appends s to the file at path (creating it if needed).
func appendToFile(path, s string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(s)
	return err
}

// sseFrame is one parsed Server-Sent Events frame: the named event ("" = the
// default message event) and its data payload.
type sseFrame struct {
	event string
	data  string
}

// readSSEFrame reads one SSE frame (terminated by a blank line) from br. A
// frame is a sequence of `event:` / `data:` lines followed by an empty line.
func readSSEFrame(br *bufio.Reader) (sseFrame, error) {
	var f sseFrame
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return f, err
		}
		line = strings.TrimRight(line, "\n")
		if line == "" {
			return f, nil // end of frame
		}
		switch {
		case strings.HasPrefix(line, "event: "):
			f.event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			if f.data != "" {
				f.data += "\n"
			}
			f.data += strings.TrimPrefix(line, "data: ")
		}
	}
}

// readSSEFrameTimeout reads one frame, failing the test if it does not arrive
// within the timeout — the guard against a broken stream hanging the test.
func readSSEFrameTimeout(t *testing.T, br *bufio.Reader, timeout time.Duration) sseFrame {
	t.Helper()
	type res struct {
		f   sseFrame
		err error
	}
	ch := make(chan res, 1)
	go func() {
		f, err := readSSEFrame(br)
		ch <- res{f, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("read SSE frame: %v", r.err)
		}
		return r.f
	case <-time.After(timeout):
		t.Fatalf("timed out after %v waiting for an SSE frame", timeout)
		return sseFrame{}
	}
}

// streamURL builds the session-log stream URL for a job under the given
// project base name.
func streamURL(base, job string, from string) string {
	u := "/projects/" + base + "/jobs/" + job + "/session-log/stream"
	if from != "" {
		u += "?from=" + from
	}
	return u
}

// --- live growth -------------------------------------------------------------

// TestSessionLogStreamLiveGrowth: a stream opened on an existing session.log
// starts at EOF (the `start` event reports the current byte size) and emits
// each new line as it is appended — the network twin of the TUI's `l` tail.
func TestSessionLogStreamLiveGrowth(t *testing.T) {
	shortenStreamPoll(t)
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	logPath := filepath.Join(root, "docs", "jobs", "wood_oak", "session.log")
	if err := os.WriteFile(logPath, []byte("line one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	base := filepath.Base(root)

	resp, err := http.Get(ts.URL + streamURL(base, "wood_oak", ""))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	br := bufio.NewReader(resp.Body)

	// The start event reports the current file size (tail -f: only new growth).
	start := readSSEFrameTimeout(t, br, 5*time.Second)
	if start.event != "start" {
		t.Fatalf("first frame = %+v, want the start event", start)
	}
	if off := startEventOffset(t, start.data); off != int64(len("line one\n")) {
		t.Errorf("start offset = %d, want %d (the file's current size)", off, len("line one\n"))
	}

	// Append a line → it must arrive as a data frame.
	if err := appendToFile(logPath, "line two\n"); err != nil {
		t.Fatal(err)
	}
	f := readSSEFrameTimeout(t, br, 5*time.Second)
	if f.event != "" || f.data != "line two" {
		t.Errorf("frame after append = %+v, want data: line two", f)
	}
}

// TestSessionLogStreamFromOffset: ?from=<byte offset> resumes at that offset
// (the reconnect contract — a client that dropped reconnects with the offset
// it had consumed and loses nothing).
func TestSessionLogStreamFromOffset(t *testing.T) {
	shortenStreamPoll(t)
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	logPath := filepath.Join(root, "docs", "jobs", "wood_oak", "session.log")
	if err := os.WriteFile(logPath, []byte("line one\nline two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	base := filepath.Base(root)

	resp, err := http.Get(ts.URL + streamURL(base, "wood_oak", "9"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	br := bufio.NewReader(resp.Body)

	start := readSSEFrameTimeout(t, br, 5*time.Second)
	if start.event != "start" || startEventOffset(t, start.data) != 9 {
		t.Fatalf("start frame = %+v, want offset 9 (the ?from value)", start)
	}
	// Bytes from offset 9 onward: "line two\n" → one frame "line two".
	f := readSSEFrameTimeout(t, br, 5*time.Second)
	if f.data != "line two" {
		t.Errorf("first data frame = %+v, want line two", f)
	}
}

// TestSessionLogStreamFileAppearsAfterStart: a stream opened before the
// session.log exists (the launch-then-watch flow) starts at byte 0 and emits
// everything from the top once the file appears.
func TestSessionLogStreamFileAppearsAfterStart(t *testing.T) {
	shortenStreamPoll(t)
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	logPath := filepath.Join(root, "docs", "jobs", "wood_oak", "session.log")
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	base := filepath.Base(root)

	resp, err := http.Get(ts.URL + streamURL(base, "wood_oak", ""))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	br := bufio.NewReader(resp.Body)

	start := readSSEFrameTimeout(t, br, 5*time.Second)
	if start.event != "start" || startEventOffset(t, start.data) != 0 {
		t.Fatalf("start frame = %+v, want offset 0 (no file yet)", start)
	}
	// The file appears (a launch started writing) → the stream captures it.
	if err := os.WriteFile(logPath, []byte("first line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := readSSEFrameTimeout(t, br, 5*time.Second)
	if f.data != "first line" {
		t.Errorf("frame after file creation = %+v, want first line", f)
	}
}

// TestSessionLogStreamBadFrom: a negative or unparseable ?from= is a clean 400
// JSON error before any SSE headers are written.
func TestSessionLogStreamBadFrom(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	for _, from := range []string{"-1", "abc", "1.5"} {
		rec := get(t, srv, streamURL(base, "wood_oak", from), "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("?from=%s: status = %d, want 400 (body %s)", from, rec.Code, rec.Body.String())
		}
	}
}

// --- disconnect + shutdown ---------------------------------------------------

// signalWriter is a ResponseWriter that records writes and signals each write
// on a channel — so a test can observe a streaming handler's progress without
// polling (and without racing on a bytes.Buffer). It implements http.Flusher
// (a no-op that still signals), the surface the SSE handler requires.
type signalWriter struct {
	header http.Header
	mu     sync.Mutex
	body   bytes.Buffer
	writes chan struct{}
}

func newSignalWriter() *signalWriter {
	return &signalWriter{header: http.Header{}, writes: make(chan struct{}, 64)}
}

func (w *signalWriter) Header() http.Header { return w.header }

func (w *signalWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.body.Write(p)
	w.mu.Unlock()
	w.signal()
	return len(p), nil
}

func (w *signalWriter) WriteHeader(int) {}

// Flush signals without actually flushing — the observation point a test
// needs, with no real buffered writer behind it.
func (w *signalWriter) Flush() { w.signal() }

func (w *signalWriter) signal() {
	select {
	case w.writes <- struct{}{}:
	default:
	}
}

// bodyText returns the recorded body under the mutex.
func (w *signalWriter) bodyText() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}

// waitForWrite waits until the handler has written (and flushed) at least one
// frame, or fails the test after the timeout.
func waitForWrite(t *testing.T, w *signalWriter, timeout time.Duration) {
	t.Helper()
	select {
	case <-w.writes:
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for the handler to write; body so far: %q", w.bodyText())
	}
}

// serveStream starts the stream handler in a goroutine, returning the request
// (whose context the test cancels to simulate a disconnect) and a done channel
// closed when the handler returns.
func serveStream(t *testing.T, srv *Server, path string) (cancel func(), done chan struct{}) {
	t.Helper()
	ctx, cancelFn := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	w := newSignalWriter()
	done = make(chan struct{})
	go func() {
		srv.Handler().ServeHTTP(w, req)
		close(done)
	}()
	waitForWrite(t, w, 5*time.Second) // the handler is inside its poll loop now
	return cancelFn, done
}

// TestSessionLogStreamDisconnectStopsHandler: cancelling the client's request
// context makes the handler return promptly — no goroutine leak per stream.
func TestSessionLogStreamDisconnectStopsHandler(t *testing.T) {
	shortenStreamPoll(t)
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	cancel, done := serveStream(t, srv, streamURL(base, "wood_oak", ""))
	cancel()

	select {
	case <-done:
		// handler returned promptly — no leak
	case <-time.After(5 * time.Second):
		t.Fatal("stream handler did not return within 5s of the client disconnect")
	}
}

// TestSessionLogStreamShutdownReturnsPromptly: a graceful Server.Shutdown
// cancels the server's shutdown context before draining, so an open stream
// unblocks and Shutdown returns promptly instead of waiting out the drain
// timeout — the one correctness requirement the brief explicitly calls out.
func TestSessionLogStreamShutdownReturnsPromptly(t *testing.T) {
	shortenStreamPoll(t)
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	_, done := serveStream(t, srv, streamURL(base, "wood_oak", ""))

	start := time.Now()
	shutdownDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownDone <- srv.Shutdown(ctx)
	}()

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Errorf("Shutdown took %v, want prompt (an open stream must not hold the drain)", elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return within 5s with an open stream attached")
	}

	// The stream handler itself also returned (the server context was
	// cancelled before the drain).
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stream handler did not return after Shutdown")
	}
}

// TestSessionLogStreamUnknownJobIs404: an unknown job is a 404 before any SSE
// headers — the stream is scoped to resolved jobs only.
func TestSessionLogStreamUnknownJobIs404(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	rec := get(t, srv, streamURL(base, "no-such-job", ""), "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
}