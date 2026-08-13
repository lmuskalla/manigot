// Package fs holds the tiny filesystem predicates shared across the host-side
// packages (session, job, agentlist, cmd/mg) — each used to have its own
// one-line isDir/isFile copy. One definition per predicate, one place to
// change the semantics.
package fs

import "os"

// IsDir reports whether path exists and is a directory.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// IsFile reports whether path exists and is a regular file (not a directory).
func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
