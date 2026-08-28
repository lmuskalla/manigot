package serve

import "sync"

// ProjectLocks is the per-project serialization pattern for mutating
// operations — the skeleton the brief's scope item 8 calls for, so job two's
// mutating API inherits the pattern instead of inventing it under pressure.
// internal/git has no locking today: two concurrent mutating operations on
// the same project (e.g. two mg done / mg delete runs, or a create racing a
// delete) would race on the git metadata. Serializing per project root —
// never globally, so independent projects proceed in parallel — is the
// pattern.
//
// Job two's mutating handlers MUST use it: Lock(root) before the critical
// section and Unlock(root) after (defer the unlock, mirroring how every other
// Go mutex is used). The key is the registered project root, so the same
// project is serialized regardless of which URL segment resolved to it.
//
// v1 read endpoints deliberately do NOT take these locks: reads need no
// exclusion, so the structure ships unused by handlers but fully tested —
// the tests below pin the mutual-exclusion and independence properties job
// two's handlers depend on.
type ProjectLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewProjectLocks returns an empty ProjectLocks.
func NewProjectLocks() *ProjectLocks {
	return &ProjectLocks{locks: make(map[string]*sync.Mutex)}
}

// Lock acquires the per-project lock for key (a registered project root),
// blocking until it is free. Lock/Unlock on the same key serialize; different
// keys proceed independently. The lock map is created lazily so a key is only
// ever locked when someone actually contends on it.
func (l *ProjectLocks) Lock(key string) {
	l.mu.Lock()
	m := l.locks[key]
	if m == nil {
		m = &sync.Mutex{}
		l.locks[key] = m
	}
	l.mu.Unlock()
	m.Lock()
}

// Unlock releases the per-project lock for key. It must be called exactly
// once per Lock — conventionally deferred immediately after Lock. An Unlock
// without a prior Lock is a caller bug; it is ignored rather than panicking
// so a misbehaving handler degrades to a no-op instead of crashing the
// daemon.
func (l *ProjectLocks) Unlock(key string) {
	l.mu.Lock()
	m := l.locks[key]
	l.mu.Unlock()
	if m == nil {
		return
	}
	m.Unlock()
}
