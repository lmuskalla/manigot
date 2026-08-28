package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// This file pins the ProjectLocks boundary across the handler surface: which
// operations take s.locks.Lock(root) and which deliberately do not. The
// boundary is a real correctness property — over-serializing (a launch that
// waits behind an unrelated done) defeats the point of a responsive control
// plane, under-serializing (two git-mutating ops racing on the same project)
// corrupts git metadata. Each test holds the project lock itself (simulating
// an in-flight mutating operation on that project) and asserts the request
// under test either blocks behind it (a lock-taking handler) or completes
// immediately (a non-locking handler).

// lockBlockTimeout is how long a request that SHOULD block behind the held
// project lock is given before the test concludes it blocked. A handler that
// wrongly skipped the lock would complete in microseconds, so 200ms is an
// enormous margin.
const lockBlockTimeout = 200 * time.Millisecond

// lockReleaseTimeout is how long a blocked request is given to complete after
// the lock is released.
const lockReleaseTimeout = 3 * time.Second

// assertBlocksWhileLockHeld starts fn in a goroutine, holds the project lock
// for root, and asserts fn does NOT complete until the lock is released (a
// lock-taking handler). fn is the request's completion signal — it must close
// done when the request finishes. On return the lock has been released and the
// goroutine has completed (the test waits for it, so a fixture-dir cleanup
// never races the still-running handler).
func assertBlocksWhileLockHeld(t *testing.T, srv *Server, root string, fn func(done chan struct{})) {
	t.Helper()
	srv.locks.Lock(root)

	done := make(chan struct{})
	go fn(done)

	select {
	case <-done:
		srv.locks.Unlock(root)
		t.Fatal("request completed while the project lock was held — a mutating handler must serialize behind s.locks.Lock(root)")
	case <-time.After(lockBlockTimeout):
		// blocked as expected
	}

	srv.locks.Unlock(root)
	select {
	case <-done:
	case <-time.After(lockReleaseTimeout):
		t.Fatal("request did not proceed after the project lock was released")
	}
}

// releaseAndExpectDone releases the lock and asserts the blocked request now
// completes.
func releaseAndExpectDone(t *testing.T, srv *Server, root string, done chan struct{}) {
	t.Helper()
	srv.locks.Unlock(root)
	select {
	case <-done:
	case <-time.After(lockReleaseTimeout):
		t.Fatal("request did not proceed after the project lock was released")
	}
}

// assertCompletesWhileLockHeld holds the project lock and asserts fn completes
// promptly — a handler that must NOT take the lock (launch-agent, launch-jdi,
// prune, reads).
func assertCompletesWhileLockHeld(t *testing.T, srv *Server, root string, fn func(done chan struct{})) {
	t.Helper()
	srv.locks.Lock(root)
	defer srv.locks.Unlock(root)

	done := make(chan struct{})
	go fn(done)

	select {
	case <-done:
		// completed without waiting — the boundary holds
	case <-time.After(lockBlockTimeout):
		t.Fatal("request did not complete while the project lock was held — a non-mutating handler must not block behind s.locks.Lock(root)")
	}
}

// --- lock-taking handlers ----------------------------------------------------

// TestCreateBlocksBehindProjectLock: create-job takes the lock (git-mutating:
// creates a branch + worktree) — a create issued while another mutating
// operation holds the same project's lock must wait.
func TestCreateBlocksBehindProjectLock(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	assertBlocksWhileLockHeld(t, srv, root, func(done chan struct{}) {
		post(t, srv, "/projects/"+base+"/jobs", "", `{"title":"Blocked Create"}`)
		close(done)
	})
}

// TestEditBriefBlocksBehindProjectLock: edit-brief takes the lock (the
// post-write commit touches git) — a write issued behind another mutating
// operation on the same project must wait, so the commit can never race a
// concurrent done/delete on the same worktree.
func TestEditBriefBlocksBehindProjectLock(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	assertBlocksWhileLockHeld(t, srv, root, func(done chan struct{}) {
		put(t, srv, "/projects/"+base+"/jobs/wood_oak/files/brief", "", "# Brief: X\n")
		close(done)
	})
}

// TestDoneBlocksBehindProjectLock: done takes the lock (git-mutating: archive
// + squash merge + branch delete). force:true is required so the request
// reaches the lock rather than returning early on the verdict warning.
func TestDoneBlocksBehindProjectLock(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	assertBlocksWhileLockHeld(t, srv, root, func(done chan struct{}) {
		post(t, srv, "/projects/"+base+"/jobs/wood_oak/done", "", `{"force":true}`)
		close(done)
	})
}

// TestDeleteBlocksBehindProjectLock: delete takes the lock (git-mutating:
// worktree force-remove + branch -D).
func TestDeleteBlocksBehindProjectLock(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	assertBlocksWhileLockHeld(t, srv, root, func(done chan struct{}) {
		post(t, srv, "/projects/"+base+"/jobs/wood_oak/delete", "", "")
		close(done)
	})
}

// TestPushBlocksBehindProjectLock: push takes the lock (git-mutating: pushes
// the branch).
func TestPushBlocksBehindProjectLock(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	assertBlocksWhileLockHeld(t, srv, root, func(done chan struct{}) {
		post(t, srv, "/projects/"+base+"/jobs/wood_oak/push", "", "")
		close(done)
	})
}

// TestOrphanDeleteBlocksBehindProjectLock: orphan-delete takes the lock
// (RemoveOrphansConfirmed calls git.WorktreePrune, which touches git worktree
// metadata) — this task's own reasoned addition to the boundary, since the
// brief's Notes don't name orphan cleanup in the git-mutating set.
func TestOrphanDeleteBlocksBehindProjectLock(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	fakeOrphan(t, root, "abandoned_1")
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	assertBlocksWhileLockHeld(t, srv, root, func(done chan struct{}) {
		post(t, srv, "/projects/"+base+"/orphans/abandoned_1/delete", "", "")
		close(done)
	})
}

// --- non-locking handlers ----------------------------------------------------

// TestLaunchAgentDoesNotBlockBehindProjectLock: launch-agent takes no lock
// (per the brief's Notes — launching an agent doesn't touch git state and
// must not wait behind an unrelated done/delete). A launch issued while
// another mutating operation holds the same project's lock completes
// immediately.
func TestLaunchAgentDoesNotBlockBehindProjectLock(t *testing.T) {
	fakeCheckout(t, map[string]string{
		"analyst": "name: analyst\ndescription: Breaks requests into tasks.\n",
	})
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	assertCompletesWhileLockHeld(t, srv, root, func(done chan struct{}) {
		rec := post(t, srv, "/projects/"+base+"/jobs/wood_oak/agents/analyst", "", "")
		if rec.Code != http.StatusAccepted {
			t.Errorf("launch-agent status = %d, want 202", rec.Code)
		}
		close(done)
	})
}

// TestLaunchJDIDoesNotBlockBehindProjectLock: launch-jdi takes no lock (same
// reasoning as launch-agent).
func TestLaunchJDIDoesNotBlockBehindProjectLock(t *testing.T) {
	stubJdiExe(t, "/bin/true")
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	assertCompletesWhileLockHeld(t, srv, root, func(done chan struct{}) {
		rec := post(t, srv, "/projects/"+base+"/jobs/wood_oak/jdi", "", "")
		if rec.Code != http.StatusAccepted {
			t.Errorf("launch-jdi status = %d, want 202", rec.Code)
		}
		close(done)
	})
}

// TestPruneDoesNotBlockBehindProjectLock: prune takes no lock (no git or
// project-scoped state at all — orphaned containers are not partitioned by
// project). It completes even while a project lock is held.
func TestPruneDoesNotBlockBehindProjectLock(t *testing.T) {
	srv := New(&Registry{}, "test-version", "", nil)

	assertCompletesWhileLockHeld(t, srv, "", func(done chan struct{}) {
		post(t, srv, "/prune", "", "")
		close(done)
	})
}

// TestReadsDoNotBlockBehindProjectLock: read endpoints take no locks — a
// listing issued while a mutating operation holds the project lock completes
// immediately (v1's deliberate no-lock reads).
func TestReadsDoNotBlockBehindProjectLock(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	assertCompletesWhileLockHeld(t, srv, root, func(done chan struct{}) {
		get(t, srv, "/projects/"+base+"/jobs", "")
		close(done)
	})
}

// --- per-project independence ------------------------------------------------

// TestDifferentProjectsProceedIndependently: the lock is keyed by project
// root — a mutating call on project B proceeds while project A's lock is held
// (the property that keeps a multi-project daemon responsive).
func TestDifferentProjectsProceedIndependently(t *testing.T) {
	rootA := fakeJobProject(t, "project_a", minimalBrief("A", "a", "open", "feature", "2026-08-01"))
	rootB := fakeJobProject(t, "project_b", minimalBrief("B", "b", "open", "feature", "2026-08-01"))
	reg := &Registry{entries: []Entry{entryFor(rootA), entryFor(rootB)}}
	srv := New(reg, "test-version", "", nil)
	baseB := filepath.Base(rootB)

	// Hold A's lock, then create a job in B — B must proceed immediately.
	assertCompletesWhileLockHeld(t, srv, rootA, func(done chan struct{}) {
		rec := post(t, srv, "/projects/"+baseB+"/jobs", "", `{"title":"Independent B"}`)
		if rec.Code != http.StatusCreated {
			t.Errorf("create in project B status = %d, want 201", rec.Code)
		}
		close(done)
	})
}

// TestConcurrentMutatingCallsSerializeSameProject: two mutating calls against
// the SAME project serialize end to end — a blocked create behind a held lock
// proceeds (and succeeds) once the lock is released, proving the handler
// acquires the lock for its whole critical section, not just part of it.
func TestConcurrentMutatingCallsSerializeSameProject(t *testing.T) {
	root := fakeJobProject(t, "wood_oak", minimalBrief("Oak", "wod", "open", "feature", "2026-08-01"))
	srv := New(&Registry{entries: []Entry{entryFor(root)}}, "test-version", "", nil)
	base := filepath.Base(root)

	srv.locks.Lock(root)
	done := make(chan struct{})
	var rec *httptest.ResponseRecorder
	go func() {
		rec = post(t, srv, "/projects/"+base+"/jobs", "", `{"title":"Serialized Create"}`)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("create completed while the project lock was held")
	case <-time.After(lockBlockTimeout):
	}

	srv.locks.Unlock(root)
	select {
	case <-done:
	case <-time.After(lockReleaseTimeout):
		t.Fatal("create did not proceed after the project lock was released")
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("create status = %d, want 201 after the lock was released", rec.Code)
	}
	// The created job's directory actually exists — the critical section ran
	// to completion, not just past the lock acquisition.
	var body createJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("create body not JSON: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "jobs", body.Job.Name)); err != nil {
		t.Errorf("created job dir missing after serialized create: %v", err)
	}
}