.PHONY: build rebuild run tui install uninstall help

IMAGE  := safecode
SCRIPT := ./scripts/run.sh

# ── Build ───────────────────────────────────────────────────────────────────────

build: ## Build the safecode image (skip if already built)
	docker build -t $(IMAGE) .

rebuild: ## Force rebuild with no cache (use after Claude Code updates)
	docker build --no-cache --pull -t $(IMAGE) .

# ── Run ─────────────────────────────────────────────────────────────────────────

run: ## Start safecode in the current project (requires claude/ dir in project root)
	$(SCRIPT)

# ── TUI ─────────────────────────────────────────────────────────────────────────
# The TUI is a host-side binary (not part of the container image). Build it with
# `make tui`, then `make install` to put the launchers on your PATH — see README.

TUI_BIN    := bin/safecode-tui
TUI_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

tui: ## Build the host-side TUI binary into bin/
	@mkdir -p bin
	cd tui && CGO_ENABLED=0 go build -trimpath \
		-ldflags "-X main.version=$(TUI_VERSION)" \
		-o "$(CURDIR)/$(TUI_BIN)" .
	@echo "Built $(TUI_BIN) ($(TUI_VERSION))"
	@echo "Install:  make install   (or: make install PREFIX=\$$HOME/.local)"

# ── Install ─────────────────────────────────────────────────────────────────────
# Symlinks the launchers into PREFIX/bin under their canonical sc- names.
# Symlinks (not copies) so a `git pull` updates the installed commands too.
# Never a prerequisite of another target — this is the only thing here that
# writes outside the repo.
#
#   make install                      → /usr/local/bin (may need sudo)
#   make install PREFIX=$HOME/.local  → ~/.local/bin, no sudo needed

PREFIX  ?= /usr/local
BINDIR  := $(PREFIX)/bin

# <installed name>:<script>.
LINKS := \
	sc:run.sh \
	sc-tui:tui.sh \
	sc-job:new-job.sh \
	sc-done:finish-job.sh

install: ## Symlink the safecode launchers into PREFIX/bin (default /usr/local)
	@mkdir -p "$(BINDIR)"
	@for pair in $(LINKS); do \
		name="$${pair%%:*}"; script="$${pair#*:}"; \
		ln -sf "$(CURDIR)/scripts/$$script" "$(BINDIR)/$$name" \
			&& echo "  $(BINDIR)/$$name -> scripts/$$script"; \
	done
	@echo "Installed into $(BINDIR)."
	@case ":$$PATH:" in *":$(BINDIR):"*) ;; \
		*) echo "Warning: $(BINDIR) is not on your PATH." ;; esac
	@test -x "$(TUI_BIN)" || echo "Note: run 'make tui' to build the TUI binary sc-tui needs."

uninstall: ## Remove the symlinks created by `make install`
	@for pair in $(LINKS); do \
		name="$${pair%%:*}"; \
		if [ -L "$(BINDIR)/$$name" ]; then \
			rm -f "$(BINDIR)/$$name" && echo "  removed $(BINDIR)/$$name"; \
		fi; \
	done
	@echo "Removed safecode symlinks from $(BINDIR)."

# ── Helpers ─────────────────────────────────────────────────────────────────────

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help