#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# mg                              # start a session (today's run.sh behavior)
# mg --tool opencode              # ...with flags, unchanged
# mg --agent analyst --job <id>
# mg agents                       # today's agents.sh — pick an agent, then run.sh
# mg job "title" [--type ...]     # today's new-job.sh
# mg tui                          # today's tui.sh
# mg jdi --job <id>               # today's jdi.sh
# mg done <id>                    # today's finish-job.sh
# mg delete <id>                  # today's delete-job.sh
# mg init [--tool ...]            # today's init.sh
# mg -h | --help | help           # print_help below, no subcommand exec'd
#
# Single dispatcher: the only script `make install` symlinks onto PATH as
# `mg`. It inspects $1 — if it's -h/--help/help, prints usage and exits
# (before any subcommand runs, so it needs no docker/auth setup); if it
# exactly matches one of the seven subcommand names above, it execs the
# matching sibling script with the remaining args unchanged; anything else
# (no args at all, or any other first token, including run.sh's own
# --agent/--job/--tool/--print flags) falls through to run.sh with all
# original args untouched. None of the underlying scripts change their own
# logic, flags, or behavior — see docs/jobs/9pze1x_better-cli-syntax/brief.md.
# init is the one exception to "needs an initialized project" — it creates
# docs/, so it deliberately works without one already existing.

# ── Resolve repo ────────────────────────────────────────────────────────────────
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

print_help() {
    cat <<'EOF'
mg — isolated agent environment per project (Claude Code or OpenCode, sandboxed in Docker)

Usage:
  mg                              Start a session in the current project
  mg --tool opencode              ...with OpenCode instead of Claude Code
  mg --agent <name>               Start straight in an agent (e.g. analyst)
  mg --job <id>                   Start with a job's brief.md as the prompt
  mg --agent <name> --job <id>    Combine the two

Commands:
  mg agents                       List available agents and pick one to start
  mg init [--tool claude-code|opencode]
                                   Bootstrap this project for the job workflow
  mg job "<title>" [--type feature|fix|chore]
                                   Create a job directory + branch (off main)
  mg done <id>                    Archive a finished job
  mg delete <id>                  Permanently delete a job (dir + branch, no merge)
  mg tui                          Terminal UI for browsing jobs and firing agents
  mg jdi --job <id>                Drive a job's analyst -> developer ->
                                   reviewer sequence unattended (Claude Code only)

  mg -h, --help, help             Show this help

A bare `mg` works in any project — no setup required. Project context and the
job workflow (mg job/tui/jdi) additionally need a docs/ directory; see
'Per-project setup' in the manigot README to add one.
EOF
}

# ── Dispatch ────────────────────────────────────────────────────────────────────
case "${1:-}" in
    -h|--help|help)
        print_help
        exit 0
        ;;
    agents)
        shift
        exec "$SCRIPT_DIR/agents.sh" "$@"
        ;;
    job)
        shift
        exec "$SCRIPT_DIR/new-job.sh" "$@"
        ;;
    tui)
        shift
        exec "$SCRIPT_DIR/tui.sh" "$@"
        ;;
    jdi)
        shift
        exec "$SCRIPT_DIR/jdi.sh" "$@"
        ;;
    done)
        shift
        exec "$SCRIPT_DIR/finish-job.sh" "$@"
        ;;
    delete)
        shift
        exec "$SCRIPT_DIR/delete-job.sh" "$@"
        ;;
    init)
        shift
        exec "$SCRIPT_DIR/init.sh" "$@"
        ;;
    *)
        exec "$SCRIPT_DIR/run.sh" "$@"
        ;;
esac
