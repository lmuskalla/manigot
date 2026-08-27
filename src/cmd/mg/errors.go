package main

import (
	"fmt"
	"io"
)

// cliError writes a command error to stderr with the standard "Error: "
// framing — the one place the CLI owns presentation of domain errors.
// internal/job and internal/session return bare "what happened" facts; the
// presentation prefix belongs here,
// so the rendered output stays identical to the old "Error: ..." domain
// strings.
func cliError(stderr io.Writer, err error) {
	fmt.Fprintf(stderr, "Error: %v\n", err)
}
