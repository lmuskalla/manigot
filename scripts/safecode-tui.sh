#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# sc-tui
#
# Host-side terminal UI for managing safecode jobs. This script is the
# PATH-symlink target, mirroring how scripts/run.sh (sc) and
# scripts/new-job.sh (sc-job) are installed:
#
#   make tui
#   make install          # or: make install PREFIX="$HOME/.local"
#
# Then run `sc-tui` from anywhere inside a project that has a docs/.

# ── Resolve repo + binary ───────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(dirname "$SCRIPT_DIR")"
BIN="$ROOT/bin/safecode-tui"

if [[ ! -x "$BIN" ]]; then
    echo "sc-tui: binary not found at $BIN" >&2
    echo "Build it first with:  make tui   (run from $ROOT)" >&2
    exit 1
fi

# Tell the TUI where this checkout is, so it can fall back to
# $SAFECODE_HOME/scripts/*.sh when the launchers are not on $PATH (see
# tui/internal/resolve). Only set if the user has not set it themselves.
export SAFECODE_HOME="${SAFECODE_HOME:-$ROOT}"

exec "$BIN" "$@"
