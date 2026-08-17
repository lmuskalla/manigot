# manigot — Universal Claude Code environment
# Covers: PHP/Laravel, JS/TS/Svelte, Python, WordPress
# node:22-trixie-slim = Debian 13 (Trixie) — ships PHP 8.4 natively, no third-party repo needed
FROM node:22-trixie-slim

# System dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    tig \
    curl \
    unzip \
    ca-certificates \
    gnupg \
    php8.4-cli \
    php8.4-mbstring \
    php8.4-xml \
    php8.4-curl \
    php8.4-zip \
    php8.4-mysql \
    php8.4-pgsql \
    php8.4-sqlite3 \
    php8.4-bcmath \
    php8.4-gd \
    python3 \
    python3-pip \
    python3-venv \
    fonts-liberation \
    fonts-dejavu-core \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Go toolchain + make — needed to build and test the host-side tool (cmd/) from
# inside the container. Debian trixie ships Go 1.24, satisfying go.mod (1.23).
# Kept as its own layer so the PHP/Python layer above stays cached.
RUN apt-get update && apt-get install -y --no-install-recommends \
    make \
    golang-go \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Fail loudly if go.mod ever needs a newer toolchain than the image has,
# instead of silently downloading one at build time.
ENV GOTOOLCHAIN=local

# Composer
RUN curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer

# Claude Code
RUN npm install -g @anthropic-ai/claude-code

# OpenCode — alternative agent CLI, selected at runtime by an opencode profile
# (e.g. `mg --profile zai` / `mg --profile opencode-go`).
# The npm package pulls the matching prebuilt binary via optional platform deps.
RUN npm install -g opencode-ai

# Non-root user — node:22-slim already has UID 1000 as 'node', so rename it.
RUN usermod -l claude -d /home/claude -m node \
    && groupmod -n claude node

# HOME must resolve for whatever UID actually runs the container (see below) —
# Docker doesn't derive it from /etc/passwd for an unrecognized UID, it just
# leaves it unset/"/".
ENV HOME=/home/claude

# Playwright — the `shot` render tool's browser engine (see docs/PLAYWRIGHT.md).
# Version resolved from the registry at build time (npm view playwright version)
# and pinned for this build — the registry is the authority, and the browser
# revision stays deterministic per build. Debian 13 (trixie) is an officially
# supported distro for this version, so `install --with-deps` resolves the
# headless-shell system libraries itself — no hand-enumerated apt list to
# drift. Only the headless shell is installed (~90MB), not full Chromium
# (~300MB); one browser, no cross-browser matrix. The font layer (Liberation +
# DejaVu) is installed in the system-deps layer above — typography review
# against tofu is worse than no review.
#
# PLAYWRIGHT_BROWSERS_PATH must point inside /home/claude: the install runs as
# root (the default /root/.cache is unreachable at runtime, when the container
# runs as the invoking host UID with HOME=/home/claude). Baking the env var in
# makes the install land there AND keeps runtime lookups on the same path. The
# chmod right after the install runs as root so the browser dir is opened to
# every session UID — the final `chmod -R o+rwX /home/claude` also runs as root
# (see below), because claude (UID 1000) cannot chmod root-owned files.
ENV PLAYWRIGHT_BROWSERS_PATH=/home/claude/.cache/ms-playwright
RUN PW_VERSION="$(npm view playwright version)" \
    && npm install -g "playwright@${PW_VERSION}" \
    && npx playwright install --with-deps chromium-headless-shell \
    && chmod -R o+rwX /home/claude/.cache

# The `shot` render tool — a Node script baked into the image (see
# docs/PLAYWRIGHT.md). Landed on PATH as /usr/local/bin/shot with a node
# shebang; NODE_PATH points at the global node_modules so `require('playwright')`
# resolves without growing entrypoint.sh's bash. Node's require() searches
# NODE_PATH as a fallback after the script's own node_modules chain.
ENV NODE_PATH=/usr/local/lib/node_modules
COPY --chown=claude:claude scripts/shot.js /usr/local/bin/shot
RUN chmod +x /usr/local/bin/shot

# Global agents — baked into the image, available in every project.
# Project-level agents (mounted at runtime) override these if same name.
COPY --chown=claude:claude agents/ /home/claude/.claude/agents/

# Same agents for OpenCode, which reads global agents from ~/.config/opencode/agents/.
# The markdown body is identical; only the frontmatter differs — OpenCode takes the
# agent name from the filename and expects `tools` to be a map, not a list, and passes
# unknown keys through to the provider. So drop `name:` and `tools:` from this copy —
# a `permission:` block passes through untouched, which is how the read-only agents
# (reviewer/security/analyst/owner) express their restriction under OpenCode.
RUN mkdir -p /home/claude/.config/opencode/agents \
    && for f in /home/claude/.claude/agents/*.md; do \
        awk 'BEGIN{fm=0} /^---$/{fm++; print; next} fm==1 && /^(name|tools):/{next} {print}' "$f" \
            > "/home/claude/.config/opencode/agents/$(basename "$f")"; \
    done \
    && chown -R claude:claude /home/claude/.config

# Entrypoint script — writes claude.json to bypass onboarding before Claude starts
COPY --chown=claude:claude scripts/entrypoint.sh /home/claude/entrypoint.sh
RUN chmod +x /home/claude/entrypoint.sh

USER claude

# Pre-warm the Go module cache so `make build` and `go test ./...` work inside
# the container without network access. Must run as `claude` so the cache
# lands in /home/claude/go/pkg/mod with the right owner.
# Couples the image to go.sum: a dependency bump then needs a `make rebuild`.
# Delete these two instructions if you'd rather rely on the container having
# network.
COPY --chown=claude:claude go.mod go.sum /tmp/tui/
RUN cd /tmp/tui && go mod download && rm -rf /tmp/tui

# The host-side session launcher (cmd/mg's session subcommand) runs the
# container with --user "$(id -u):$(id -g)" (the invoking host user) so the
# bind-mounted /workspace keeps host file ownership and stays writable. That
# means this UID almost never matches the baked-in claude (1000), so open up
# $HOME to any UID — nothing sensitive lives here, just agent configs and the
# Go module cache.
#
# Must run as root: $HOME mixes claude-owned files (agent configs, the Go
# module cache from the step above) with root-owned files (.npm, the Playwright
# browser cache). chmod requires ownership of every file it touches, so an
# unprivileged build user fails on the root-owned ones even though they are
# already world-accessible.
USER root
RUN chmod -R o+rwX /home/claude
USER claude

WORKDIR /workspace

ENTRYPOINT ["/home/claude/entrypoint.sh"]