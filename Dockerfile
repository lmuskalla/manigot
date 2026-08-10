# manigot — Universal Claude Code environment
# Covers: PHP/Laravel, JS/TS/Svelte, Python, WordPress
# node:22-trixie-slim = Debian 13 (Trixie) — ships PHP 8.4 natively, no third-party repo needed
FROM node:22-trixie-slim

# System dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
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
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Go toolchain + make — needed to build and test the host-side TUI (tui/) from
# inside the container. Debian trixie ships Go 1.24, satisfying tui/go.mod (1.23).
# Kept as its own layer so the PHP/Python layer above stays cached.
RUN apt-get update && apt-get install -y --no-install-recommends \
    make \
    golang-go \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Fail loudly if tui/go.mod ever needs a newer toolchain than the image has,
# instead of silently downloading one at build time.
ENV GOTOOLCHAIN=local

# Composer
RUN curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer

# Claude Code
RUN npm install -g @anthropic-ai/claude-code

# OpenCode — alternative agent CLI, selected at runtime with `mg --tool opencode`.
# The npm package pulls the matching prebuilt binary via optional platform deps.
RUN npm install -g opencode-ai

# Non-root user — node:22-slim already has UID 1000 as 'node', so rename it.
RUN usermod -l claude -d /home/claude -m node \
    && groupmod -n claude node

# HOME must resolve for whatever UID actually runs the container (see below) —
# Docker doesn't derive it from /etc/passwd for an unrecognized UID, it just
# leaves it unset/"/".
ENV HOME=/home/claude

# Global agents — baked into the image, available in every project.
# Project-level agents (mounted at runtime) override these if same name.
COPY --chown=claude:claude agents/ /home/claude/.claude/agents/

# Same agents for OpenCode, which reads global agents from ~/.config/opencode/agents/.
# The markdown body is identical; only the frontmatter differs — OpenCode takes the
# agent name from the filename and expects `tools` to be a map, not a list, and passes
# unknown keys through to the provider. So drop `name:` and `tools:` from this copy.
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

# TASK-0B — pre-warm the Go module cache so `make tui` and `go test ./...` work
# inside the container without network access. Must run as `claude` so the cache
# lands in /home/claude/go/pkg/mod with the right owner. Couples the image to
# tui/go.sum: a dependency bump then needs a `make rebuild`. Delete these two
# instructions if you'd rather rely on the container having network.
COPY --chown=claude:claude tui/go.mod tui/go.sum /tmp/tui/
RUN cd /tmp/tui && go mod download && rm -rf /tmp/tui

# scripts/run.sh runs the container with --user "$(id -u):$(id -g)" (the
# invoking host user) so the bind-mounted /workspace keeps host file
# ownership and stays writable. That means this UID almost never matches the
# baked-in claude (1000), so open up $HOME to any UID — nothing sensitive
# lives here, just agent configs and the Go module cache.
RUN chmod -R o+rwX /home/claude

WORKDIR /workspace

ENTRYPOINT ["/home/claude/entrypoint.sh"]