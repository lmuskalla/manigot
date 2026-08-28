package session

import (
	"errors"
	"strings"
	"testing"
)

// TestImagePresentTrueWhenInspectSucceeds pins the happy path: docker image
// inspect manigot succeeding (the image is present) reports true, and the
// exact invocation is pinned.
func TestImagePresentTrueWhenInspectSucceeds(t *testing.T) {
	old := dockerCommand
	dockerCommand = func(args ...string) ([]byte, error) {
		if got := strings.Join(args, " "); got != "image inspect manigot" {
			t.Errorf("docker args = %q, want %q", got, "image inspect manigot")
		}
		return []byte("[]\n"), nil
	}
	t.Cleanup(func() { dockerCommand = old })

	if !ImagePresent() {
		t.Error("ImagePresent with a present image = false, want true")
	}
}

// TestImagePresentFalseOnEveryFailureMode pins the degrade: docker missing,
// the daemon down, or the image absent all report false — never an error.
func TestImagePresentFalseOnEveryFailureMode(t *testing.T) {
	failures := []struct {
		name string
		out  []byte
		err  error
	}{
		{"docker missing", nil, errors.New(`exec: "docker": executable file not found in $PATH`)},
		{"daemon down", []byte("Cannot connect to the Docker daemon\n"), errors.New("exit status 1")},
		{"image absent", []byte("Error response from daemon: No such image: manigot\n"), errors.New("exit status 1")},
	}
	for _, f := range failures {
		t.Run(f.name, func(t *testing.T) {
			old := dockerCommand
			dockerCommand = func(args ...string) ([]byte, error) { return f.out, f.err }
			t.Cleanup(func() { dockerCommand = old })

			if ImagePresent() {
				t.Errorf("ImagePresent on %s = true, want false", f.name)
			}
		})
	}
}