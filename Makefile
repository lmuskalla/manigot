.PHONY: build rebuild run mg install uninstall check help

IMAGE  := manigot
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# ── Build ───────────────────────────────────────────────────────────────────────

build: ## Build the manigot image (skip if already built)
	docker build -t $(IMAGE) .

rebuild: ## Force rebuild with no cache (use after Claude Code updates)
	docker build --no-cache --pull -t $(IMAGE) .

# ── Host-side binary ────────────────────────────────────────────────────────────
# The one `mg` binary is the entire host-side tool: session, profiles, setup,
# agents, job, done, delete, init, tui and jdi are all subcommands of it. The
# only bash left is scripts/entrypoint.sh, which runs inside the container
# image. The Go module lives in src/ (go.mod, cmd/, internal/).

MG_BIN := bin/mg

mg: ## Build the host-side mg binary into bin/
	@mkdir -p bin
	cd src && CGO_ENABLED=0 go build -trimpath \
		-ldflags "-X main.version=$(VERSION) -X main.tuiVersion=$(VERSION) -X main.jdiVersion=$(VERSION)" \
		-o "$(CURDIR)/$(MG_BIN)" ./cmd/mg
	@echo "Built $(MG_BIN) ($(VERSION))"
	@echo "Install:  make install   (into ~/.local/bin; override with PREFIX=...)"

# ── Run ─────────────────────────────────────────────────────────────────────────

run: mg ## Start manigot in the current project (requires docs/ in project root)
	./$(MG_BIN)

# ── Install ─────────────────────────────────────────────────────────────────────
# Symlinks the single `mg` binary into PREFIX/bin. Symlink (not a copy) so a
# `git pull` + rebuild updates the installed command too. Never a prerequisite
# of another target — this is the only thing here that writes outside the repo.
#
#   make install                      → ~/.local/bin, no sudo needed (default)
#   make install PREFIX=/usr/local    → system-wide, may need sudo

PREFIX  ?= $(HOME)/.local
BINDIR  := $(PREFIX)/bin

install: mg ## Symlink the mg binary into PREFIX/bin (default ~/.local/bin)
	@mkdir -p "$(BINDIR)"
	@ln -sf "$(CURDIR)/$(MG_BIN)" "$(BINDIR)/mg" \
		&& echo "  $(BINDIR)/mg -> $(MG_BIN)"
	@echo "Installed into $(BINDIR)."
	@case ":$$PATH:" in *":$(BINDIR):"*) ;; \
		*) echo "Warning: $(BINDIR) is not on your PATH." ;; esac

uninstall: ## Remove the symlink created by `make install`
	@if [ -L "$(BINDIR)/mg" ]; then \
		rm -f "$(BINDIR)/mg" && echo "  removed $(BINDIR)/mg"; \
	fi
	@echo "Removed manigot symlink from $(BINDIR)."

# ── Check ───────────────────────────────────────────────────────────────────────
# The verification target the brief calls for (note 13): the Go suite plus
# shellcheck on the one remaining script. shellcheck runs only when installed;
# the Go checks always run. CI wiring (a GitHub Actions workflow, if that's
# the intended vehicle) is deliberately left out — the repo has no CI today.

check: ## Run go vet + go test (from src/), and shellcheck on scripts/entrypoint.sh (when installed)
	cd src && go vet ./...
	cd src && go test ./...
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck scripts/entrypoint.sh; \
	else \
		echo "shellcheck not installed — skipping scripts/entrypoint.sh check"; \
	fi

# ── Helpers ─────────────────────────────────────────────────────────────────────

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
