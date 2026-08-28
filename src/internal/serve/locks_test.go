package serve

import (
	"sync"
	"testing"
	"time"
)

// TestProjectLocksMutualExclusionWithinKey: two Lock calls on the same
// project key serialize — the second blocks until the first releases — the
// property job two's mutating handlers depend on.
func TestProjectLocksMutualExclusionWithinKey(t *testing.T) {
	locks := NewProjectLocks()
	const key = "/some/project"

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		locks.Lock(key)
		defer locks.Unlock(key)
		close(entered) // inside the critical section
		<-release      // hold the lock until the test tries the second Lock
		close(done)
	}()
	<-entered

	locked := make(chan struct{})
	go func() {
		locks.Lock(key)
		defer locks.Unlock(key)
		close(locked)
	}()

	// The second acquisition must block while the first holds the lock.
	select {
	case <-locked:
		t.Fatal("second Lock on the same key returned while the first held it")
	case <-time.After(50 * time.Millisecond):
		// blocked as expected
	}

	close(release)
	<-done

	// After the first releases, the second proceeds.
	select {
	case <-locked:
	case <-time.After(2 * time.Second):
		t.Fatal("second Lock did not proceed after the first released")
	}
}

// TestProjectLocksIndependenceAcrossKeys: locks on different project keys are
// independent — one project's critical section never blocks another's, the
// property that keeps a multi-project daemon from serializing everything.
func TestProjectLocksIndependenceAcrossKeys(t *testing.T) {
	locks := NewProjectLocks()

	enteredA := make(chan struct{})
	releaseA := make(chan struct{})
	doneA := make(chan struct{})
	go func() {
		locks.Lock("projectA")
		defer locks.Unlock("projectA")
		close(enteredA)
		<-releaseA
		close(doneA)
	}()
	<-enteredA

	// Locking a different key while projectA is held must succeed immediately.
	gotB := make(chan struct{})
	go func() {
		locks.Lock("projectB")
		defer locks.Unlock("projectB")
		close(gotB)
	}()
	select {
	case <-gotB:
		// independent — B proceeded while A was held
	case <-time.After(2 * time.Second):
		t.Fatal("Lock on a different key blocked behind projectA — locks must be per-project")
	}

	close(releaseA)
	<-doneA
}

// TestProjectLocksAPIReentrantShape pins the API shape job two's handlers
// will call: New → Lock(root) → Unlock(root), repeatedly and across many
// keys, without panics or cross-talk — the "Lock/Unlock serialize one key,
// ignore others" contract in its simplest form.
func TestProjectLocksAPIReentrantShape(t *testing.T) {
	locks := NewProjectLocks()
	keys := []string{"/a", "/b", "/a", "/c", "/b", "/a"}
	for i := 0; i < 3; i++ {
		for _, k := range keys {
			locks.Lock(k)
			// The lock is held for the critical section, then released.
			locks.Unlock(k)
		}
	}
	// An Unlock without a Lock is a no-op, not a panic (a caller bug must not
	// crash the daemon).
	locks.Unlock("/never-locked")
}

// TestProjectLocksContentionStress: many goroutines hammering the same key
// never overlap their critical sections (a statistical smoke test of the
// mutual exclusion under real concurrency).
func TestProjectLocksContentionStress(t *testing.T) {
	locks := NewProjectLocks()
	const key = "/contended"

	var mu sync.Mutex
	inCritical := 0
	maxConcurrent := 0

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			locks.Lock(key)
			defer locks.Unlock(key)
			mu.Lock()
			inCritical++
			if inCritical > maxConcurrent {
				maxConcurrent = inCritical
			}
			mu.Unlock()
			time.Sleep(time.Millisecond)
			mu.Lock()
			inCritical--
			mu.Unlock()
		}()
	}
	wg.Wait()

	if maxConcurrent != 1 {
		t.Errorf("max concurrent critical sections on one key = %d, want 1", maxConcurrent)
	}
	if inCritical != 0 {
		t.Errorf("inCritical after all goroutines = %d, want 0 (balanced Lock/Unlock)", inCritical)
	}
}
