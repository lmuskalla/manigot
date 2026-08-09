#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# mg-jdi --job <id-or-name>
#
# Drives a job's @analyst -> @developer -> @reviewer sequence end to end,
# without a human manually triggering each stage — see
# docs/jobs/vu33rn_fully-autonomous-mode/. This script is the PATH-symlink
# target, mirroring how scripts/tui.sh (mg-tui) is installed:
#
#   make jdi
#   make install          # or: make install PREFIX="$HOME/.local"
#
# Then run `mg-jdi --job <id-or-name>` from anywhere inside a project that
# has a docs/.

# ── Resolve repo + binary ───────────────────────────────────────────────────────
# Follow symlinks to the real script location — this is installed as a
# PATH symlink (make install), and `dirname "${BASH_SOURCE[0]}"` alone would
# resolve to the symlink's directory (e.g. /usr/local/bin), not the checkout.
resolve_script_dir() {
    local src="${BASH_SOURCE[0]}" dir
    while [[ -h "$src" ]]; do
        dir="$(cd -P "$(dirname "$src")" && pwd)"
        src="$(readlink "$src")"
        [[ "$src" != /* ]] && src="$dir/$src"
    done
    cd -P "$(dirname "$src")" && pwd
}
SCRIPT_DIR="$(resolve_script_dir)"
ROOT="$(dirname "$SCRIPT_DIR")"
BIN="$ROOT/bin/manigot-jdi"

if [[ ! -x "$BIN" ]]; then
    echo "mg-jdi: binary not found at $BIN" >&2
    echo "Build it first with:  make jdi   (run from $ROOT)" >&2
    exit 1
fi

# Tell mg-jdi where this checkout is, so it can fall back to
# $MANIGOT_HOME/scripts/*.sh when the launchers are not on $PATH (see
# tui/internal/resolve, which mg-jdi shares with mg-tui). Only set if the
# user has not set it themselves.
export MANIGOT_HOME="${MANIGOT_HOME:-$ROOT}"

exec "$BIN" "$@"
