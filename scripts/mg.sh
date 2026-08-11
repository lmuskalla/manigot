#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# mg                              # start a session (today's run.sh behavior)
# mg --profile zai                # ...with a specific subscription profile
# mg --agent analyst --job <id>
# mg profiles                     # list profiles, then pick the default (TTY)
# mg profiles zai                 # ...make bare `mg` use the zai profile
# mg setup                        # configure credentials for your subscriptions
# mg agents                       # today's agents.sh — pick an agent, then run.sh
# mg crew                         # thematic alias of `mg agents`, same script
# mg job "title" [--type ...]     # today's new-job.sh
# mg tui                          # today's tui.sh
# mg jdi --job <id>               # today's jdi.sh
# mg made-man --job <id>          # thematic alias of `mg jdi`, same script
# mg done <id>                    # today's finish-job.sh
# mg delete <id>                  # today's delete-job.sh
# mg init [--tool ...]            # today's init.sh
# mg -h | --help | help           # print_help below, no subcommand exec'd
#
# Single dispatcher: the only script `make install` symlinks onto PATH as
# `mg`. It inspects $1 — if it's -h/--help/help, prints usage and exits
# (before any subcommand runs, so it needs no docker/auth setup); if it
# exactly matches one of the subcommand names above, it execs the
# matching sibling script with the remaining args unchanged; anything else
# (no args at all, or any other first token, including run.sh's own
# --agent/--job/--tool/--profile/--print flags) falls through to run.sh with
# all original args untouched. None of the underlying scripts change their own
# logic, flags, or behavior — see docs/jobs/9pze1x_better-cli-syntax/brief.md.
# `crew`/`made-man` are purely thematic aliases of `agents`/`jdi` — same
# scripts, same behavior, optional flavor only (see
# docs/jobs/tt45uz_naming-features/brief.md); `agents`/`jdi` remain the
# documented primary names.
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

Profiles bundle an agent CLI with one of your subscriptions:
  claude-pro    Claude Code, billed to your Claude Pro/Max subscription
  zai           OpenCode, billed to your Z.AI Coding Plan
  opencode-go   OpenCode, billed to the OpenCode Go subscription

Usage:
  mg                              Start a session in the current project
  mg --profile <name>             ...with the given profile (claude-pro|zai|opencode-go)
  mg --agent <name>               Start straight in an agent (e.g. analyst)
  mg --job <id>                   Start with a job's brief.md as the prompt
  mg --agent <name> --job <id>    Combine the two

Commands:
  mg profiles [name]              List the profiles (and which is the default),
                                   set the default used by bare `mg`, or pick
                                   one interactively (no name, on a TTY)
  mg setup [name] [--check]       Configure credentials for your subscriptions,
                                   interactively, or report status with --check
  mg agents                       List available agents and pick one to start
                                   (thematic alias: mg crew)
  mg init [--profile <name>]      Bootstrap this project for the job workflow
  mg job "<title>" [--type feature|fix|chore]
                                   Create a job directory + branch + worktree
                                   (off main)
  mg done <id>                    Archive a finished job (merge + remove worktree)
  mg delete <id>                  Permanently delete a job (worktree + branch, no merge)
  mg tui                          Terminal UI for browsing jobs and firing agents
  mg jdi --job <id> [--profile <name>]
                                   Drive a job's analyst -> developer ->
                                   reviewer sequence unattended
                                   (thematic alias: mg made-man)

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
    profiles)
        shift
        exec "$SCRIPT_DIR/profiles.sh" "$@"
        ;;
    setup)
        shift
        exec "$SCRIPT_DIR/setup.sh" "$@"
        ;;
    agents|crew)
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
    jdi|made-man)
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
