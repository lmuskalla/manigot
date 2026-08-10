#!/usr/bin/env bash
set -euo pipefail

# ── Usage ───────────────────────────────────────────────────────────────────────
# mg init [--tool claude-code|opencode]
#
# Reached via `mg init`. Bootstraps a project for the manigot job workflow:
# copies project-template/docs/ (AGENTS.md + CLAUDE.md, an empty jobs/, but NOT
# the example job) into the current project if docs/ is absent, then optionally
# hands off to the @prompter agent to write a good project context into
# docs/AGENTS.md. Unlike every other job-workflow subcommand, init CREATES
# docs/, so it deliberately does not use the "walk up to find docs/" gate — it
# runs in uninitialized projects by design. Target dir resolution matches
# run.sh's container-boundary fallback: git top-level if in a repo, else $PWD.

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
MANIGOT_ROOT="$(dirname "$SCRIPT_DIR")"
TEMPLATE_DIR="$MANIGOT_ROOT/project-template/docs"

if [[ ! -d "$TEMPLATE_DIR" ]]; then
    echo "Error: project template not found at $TEMPLATE_DIR." >&2
    exit 1
fi
for f in AGENTS.md CLAUDE.md; do
    if [[ ! -f "$TEMPLATE_DIR/$f" ]]; then
        echo "Error: template file $TEMPLATE_DIR/$f is missing." >&2
        exit 1
    fi
done

# ── Parse args ──────────────────────────────────────────────────────────────────
TOOL=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --tool) TOOL="$2"; shift 2 ;;
        *) echo "Unknown argument: $1" >&2; exit 1 ;;
    esac
done
if [[ -n "$TOOL" ]]; then
    case "$TOOL" in
        claude-code|opencode) ;;
        *) echo "Error: --tool must be 'claude-code' or 'opencode' (got '$TOOL')." >&2; exit 1 ;;
    esac
fi

# ── Resolve target directory ────────────────────────────────────────────────────
# init must CREATE docs/, so it cannot reuse run.sh's "walk up to find docs/"
# logic. Match run.sh's container-boundary fallback instead: git top-level when
# inside a repo, else $PWD.
TARGET="$(git rev-parse --show-toplevel 2>/dev/null || true)"
[[ -z "$TARGET" ]] && TARGET="$PWD"
TARGET="${TARGET%/}"

DOCS_DIR="$TARGET/docs"

# ── Copy template (unless already initialized) ──────────────────────────────────
# Copy AGENTS.md and CLAUDE.md only — explicitly NOT the example job under
# project-template/docs/jobs/6-char-random-id_title-of-job/. Keep an empty
# docs/jobs/ so `mg job` is ready to go (the workflow expects docs/jobs/ to
# exist; new-job.sh mkdir -p's each job dir beneath it).
if [[ -e "$DOCS_DIR" ]]; then
    echo "  Docs     : $DOCS_DIR already exists — skipping template copy."
else
    mkdir -p "$DOCS_DIR"
    cp "$TEMPLATE_DIR/AGENTS.md" "$DOCS_DIR/AGENTS.md"
    cp "$TEMPLATE_DIR/CLAUDE.md" "$DOCS_DIR/CLAUDE.md"
    mkdir -p "$DOCS_DIR/jobs"
    echo "  Created  : $DOCS_DIR/AGENTS.md"
    echo "            $DOCS_DIR/CLAUDE.md"
    echo "            $DOCS_DIR/jobs/ (empty)"
fi

# ── Offer the @prompter hand-off ────────────────────────────────────────────────
# Default no. Non-interactive stdin (piped/closed, e.g. CI) → treat as no with a
# note rather than hanging on a read that can never be answered.
RUN_PROMPTER="no"
if [[ ! -t 0 ]]; then
    echo "  Skipping @prompter offer (stdin is not a terminal)."
else
    read -r -p "Generate a project prompt with @prompter? [y/N] " ANSWER || ANSWER=""
    case "${ANSWER:-}" in
        y|Y|yes|YES) RUN_PROMPTER="yes" ;;
        *) RUN_PROMPTER="no" ;;
    esac
fi

if [[ "$RUN_PROMPTER" == "yes" ]]; then
    # run.sh mounts $TARGET at /workspace, so refer to the container path here.
    # The prompter fills in the placeholder template at docs/AGENTS.md with
    # concrete, accurate details about THIS project (stack, architecture,
    # commands, hard rules) — not generic placeholders.
    INSTRUCTION="Read this project at /workspace — its directory structure, README, config files, package manifests, build/test commands, and key source files — then rewrite /workspace/docs/AGENTS.md as a clear, specific project context. Keep the existing section structure (the title, the brief description, and the ## Stack, ## Architecture, ## Commands, ## Hard rules headings) and fill each one in with concrete, accurate details about THIS project, replacing the placeholder text. Write it tool-neutral (for \"the agent\", not one vendor) and keep it concise. Do not invent things you cannot verify from the project; where the project doesn't dictate a value, leave a short explicit placeholder rather than guessing."

    echo "  Launching @prompter to write your project context…"
    TOOL_ARGS=()
    [[ -n "$TOOL" ]] && TOOL_ARGS=(--tool "$TOOL")
    exec "$SCRIPT_DIR/run.sh" --agent prompter --prompt "$INSTRUCTION" "${TOOL_ARGS[@]+"${TOOL_ARGS[@]}"}"
fi

# ── Next steps ──────────────────────────────────────────────────────────────────
echo ""
echo "✓ Project initialized for manigot."
echo ""
echo "  Next steps:"
echo "  1. Edit your project context:"
echo "     $DOCS_DIR/AGENTS.md"
echo "     (or re-run 'mg init' and answer 'y' to have @prompter draft it)"
echo "  2. Create your first job:"
echo "     mg job \"title of your first job\""
