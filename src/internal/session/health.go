package session

// ImagePresent reports whether the manigot container image is present on the
// local docker daemon, via `docker image inspect manigot` — the
// health-endpoint's image check (a daemon whose image is missing cannot start
// sessions until `make build` runs). It shells out through the same
// stubbable dockerCommand seam as PruneOrphans, so tests never need docker.
//
// Every failure mode degrades to false, never an error: docker missing on
// PATH, the daemon down, or the image simply absent all report "not ready" —
// the caller (the /health endpoint) surfaces the boolean, and a missing image
// is an informational gap, not a crash.
func ImagePresent() bool {
	_, err := dockerCommand("image", "inspect", dockerImageName)
	return err == nil
}