package main

import (
	"fmt"
	"strings"
)

// flagParseError converts a flag.FlagSet parse error into the scripts'
// "Unknown argument: <flag>" wording, which the command tests pin. flag
// reports unknown flags and missing values as "-name" (single-dash); the
// original hand-rolled parsers echoed the token as typed, which is "--name"
// in every pinned case.
func flagParseError(err error) error {
	msg := err.Error()
	for _, prefix := range []string{"flag provided but not defined: -", "flag needs an argument: -"} {
		if strings.HasPrefix(msg, prefix) {
			return fmt.Errorf("Unknown argument: --%s", strings.TrimPrefix(msg, prefix))
		}
	}
	return err
}

// splitFlags separates args into the known flag tokens (with their values)
// and everything else, preserving order within each group: the pieces the
// flag.FlagSet parses, and the positional/passthrough remainder. valueFlags
// are the flags that take one value; bareFlags take none. Any other token —
// an unknown flag or a bare word — lands in rest.
//
// Needed where flags must be parseable in any position relative to
// non-flag arguments: Go's flag package stops at the first non-flag
// argument, so "zai --check" would otherwise leave "--check" unparsed.
func splitFlags(args []string, valueFlags, bareFlags map[string]bool) (flagArgs, rest []string) {
	for i := 0; i < len(args); i++ {
		switch {
		case bareFlags[args[i]]:
			flagArgs = append(flagArgs, args[i])
		case valueFlags[args[i]]:
			flagArgs = append(flagArgs, args[i])
			if i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		default:
			rest = append(rest, args[i])
		}
	}
	return flagArgs, rest
}
