# Handover: bvi7n6 — adjust tui to be dynamic

date: 2026-08-08
branch: feature/bvi7n6_adjust-tui-to-be-dynamic
written by: @developer (session 1)

Session 1 did **exploration only — no code was changed**. It stopped because
the container image has neither `make` nor Go, so nothing in TASK-1..14 (all Go)
can be compiled or tested. This file records what was found, the exact Dockerfile
change needed, and the decisions already settled, so session 2 can start writing
code immediately without re-reading the whole repo.

---

## 1. What the user needs to do before session 2

Apply the Dockerfile change in section 2, then:

```bash
cd manigot/
make rebuild       # or: make build — the apt layer changes, so it re-runs either way
```

Then re-run the developer agent on this job. Section 4 is the pick-up point.

---

## 2. Dockerfile change (TASK-0A, and TASK-0B if agreed)

### Current state

`Dockerfile` installs git, curl, unzip, ca-certificates, gnupg, php8.4-*,
python3* in one `apt-get` layer. There is **no `make` and no Go**. Verified
inside the running container: `command -v go make` → both missing.

Base image is `node:22-trixie-slim` (Debian 13). Debian trixie's `golang-go`
is Go 1.24, which satisfies the `go 1.23` directive in `tui/go.mod`.

### TASK-0A — recommended: separate apt layer

Insert this **after** the existing `apt-get install` layer and **before** the
Composer line. A separate layer (rather than extending the existing list) keeps
the Docker cache for the PHP/Python layer intact and makes the diff obvious.

```dockerfile
# Go toolchain + make — needed to build and test the host-side TUI (tui/) from
# inside the container. Debian trixie ships Go 1.24, satisfying tui/go.mod (1.23).
RUN apt-get update && apt-get install -y --no-install-recommends \
    make \
    golang-go \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*
```

Cost: roughly +500 MB installed (`golang-go` pulls a fair amount of build
tooling). Everyone must rebuild once.

### TASK-0A — alternative: official tarball

Smaller and version-pinned, at the price of a hardcoded version to maintain.
`dpkg --print-architecture` returns `amd64`/`arm64`, which match Go's own
release naming, so this works on Apple Silicon too. `make` still needs apt.

```dockerfile
ENV GO_VERSION=1.23.6
RUN apt-get update && apt-get install -y --no-install-recommends make \
    && apt-get clean && rm -rf /var/lib/apt/lists/* \
    && ARCH="$(dpkg --print-architecture)" \
    && curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tgz \
    && tar -C /usr/local -xzf /tmp/go.tgz \
    && rm /tmp/go.tgz
ENV PATH="/usr/local/go/bin:${PATH}"
```

Pick one. The tasks do not care which.

### TASK-0B — pre-warm the module cache (Q4, still undecided)

Only if you answer Q4 with yes. Must go **after** `USER claude`, so the module
cache lands in `/home/claude/go/pkg/mod` and is owned by the user that will
later run `go build`. Place it directly before `WORKDIR /workspace`:

```dockerfile
# Pre-warm the Go module cache so `make tui` / `go test ./...` work inside the
# container without network access. Couples the image to tui/go.sum: a
# dependency bump needs a `make rebuild`.
COPY --chown=claude:claude tui/go.mod tui/go.sum /tmp/tui/
RUN cd /tmp/tui && go mod download && rm -rf /tmp/tui
```

Optional hardening, either way: `ENV GOTOOLCHAIN=local` makes Go fail loudly if
`tui/go.mod` ever requires a newer toolchain than the image has, instead of
silently downloading one at build time.

**Q4 is still open.** Session 1 recommended yes (fits the containment premise,
and TASK-0C documents the rebuild). Network *is* reachable from the container
today — verified with a request to `proxy.golang.org` (HTTP 200) — so without
0B builds still work, they just need the network. If the answer is no, session 2
should mark TASK-0B as "dropped per Q4" in `implementation.md` and skip it.

### Note

There is no `.dockerignore`, so the build context includes `.env`. Nothing
`COPY`s it, so it does not reach the image — out of scope here, but worth a
separate chore job.

---

## 3. Decisions already settled

| # | Question | Answer |
|---|---|---|
| Q1 | short names | install `manigot-job`/`mg-job` and `manigot-done`/`mg-done` only — **no** bare `mg`, no `mg-tui` |
| Q2 | legacy names | resolver accepts `new-job`/`finish-job` silently, no deprecation warning |
| Q3 | install mechanism | `make install` / `make uninstall` are in scope (TASK-10) |
| Q4 | Go module cache | **open** — see above |

Naming table (from `tasks.md`, unchanged):

| script | canonical | short | legacy |
|---|---|---|---|
| `scripts/run.sh` | `manigot` | — | — |
| `scripts/new-job.sh` | `manigot-job` | `mg-job` | `new-job` |
| `scripts/finish-job.sh` | `manigot-done` | `mg-done` | `finish-job` |
| `scripts/manigot-tui.sh` | `manigot-tui` | — | — |

Script *filenames* do not change — only the installed command names.

---

## 4. Pick-up point

Nothing implemented. Start at TASK-0A (or TASK-0C, if the user already applied
the Dockerfile change and committed it — check `git log` first to avoid
committing it twice).

Order from `tasks.md`: 0A → 0B → 0C → 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 →
10 → 11 → 12 → 13 → 14.

Commit format: `[bvi7n6] TASK-N: short description`.

---

## 5. Findings that save re-reading

### The two hardcoded call sites (the whole point of the job)

- `tui/internal/hostcmd/hostcmd.go:19` — `exec.LookPath("new-job")`, then
  `exec.Command("new-job", args...)` at line 28. Sets `cmd.Dir = projectRoot`
  **and** `cmd.Env = append(os.Environ(), "PWD="+projectRoot)` — the script
  resolves the project root from `$PWD`, and `exec.Cmd` does not update `PWD`
  on its own. **Keep both when switching to the resolved absolute path.**
- `tui/internal/launch/launch.go:57-60` — `shellCommand` returns
  `cd <root> && manigot --agent <a> --job <j>` with the literal word
  `manigot`. `shellQuote` (line 64) is the existing `'…'\''…'` quoter; reuse it
  for the resolved path so paths with spaces survive `osascript` and `bash -lc`.

### Tests that will need updating

- `tui/internal/hostcmd/hostcmd_test.go` — `TestNewJobMissingOnPath` skips if
  `new-job` is on PATH and asserts the error contains `"not found"`. TASK-5/7
  change the error text; keep a `"not found"` substring or update the assertion.
  It must now also be immune to `manigot-job`/`mg-job` being installed, and to
  `manigot_JOB_BIN` being set — clear the env vars in the test.
- `tui/internal/launch/launch_test.go:10` — `TestShellCommandFormat` asserts the
  exact string `cd '/home/me/proj' && manigot --agent 'developer' --job 'irw320'`.
  TASK-6 breaks this. Best fix: make `shellCommand` take the resolved manigot
  path as a parameter, so the test can pass a fixed value instead of depending
  on the environment.
- Other tests in that file (`TestShellQuote*`, `TestBuildCmdSmoke`) are
  unaffected.

### Where the resolver gets wired in

- `tui/internal/ui/app.go:175` — `hostcmd.NewJob(title, typ, a.root)`, error goes
  to `a.newJob.status` as `"error: " + err.Error()`.
- `tui/internal/ui/app.go:213` — `launch.Agent(agent, a.detail.job.ID, a.root)`,
  error goes to `a.detail.status`. These two strings are what TASK-7 improves.
- `tui/main.go` — TASK-4's `os.Executable()` fallback hooks in here.

### Scripts

- `scripts/manigot-tui.sh:17-19` already computes `SCRIPT_DIR` and `ROOT`.
  TASK-3 is literally adding `export manigot_HOME="$ROOT"` before the final
  `exec "$BIN" "$@"`. The usage comment at line 9 also mentions `new-job` and
  needs the rename.
- `scripts/new-job.sh` — usage comments lines 5-7, usage string line 15.
- `scripts/finish-job.sh` — usage comments lines 5-6, usage string line 14.
- Both scripts find the project root by walking up from `$PWD` looking for
  `docs/` — unchanged by this job, but it is why `PWD` must be set explicitly.

### Docs to update, with exact locations

- `README.md` — line 22 (repo tree, `new-job.sh` comment; also add
  `finish-job.sh`, which the tree omits entirely), lines 100-102 (install
  symlinks — replace with `make install`), lines 173-175 (usage examples),
  line 237 and line 249 (workflow examples), line 275 (TUI intro prose),
  lines 292-301 (TUI build & install), line 318 (keybinding table),
  line 346 (`new-job` scaffold wording). TASK-11 also adds the
  "installing without symlinks" subsection (aliases + env overrides).
- `docs/AGENTS.md` — lines 12, 24, 54, 62. **`docs/CLAUDE.md` is an empty file
  (0 bytes)** despite being listed in TASK-12 — nothing to sync there; say so in
  `implementation.md` rather than inventing content.
- `project-template/docs/AGENTS.md` — grep finds **no** `new-job`/`finish-job`
  occurrences (only `manigot` at lines 5-7). Likely nothing to change; confirm
  and note it.
- `docs/TASKS.md` — line 4 and line 34 (TASK-13). Also note line 70,
  "Add `make install` target that sets up symlinks automatically", which TASK-10
  fulfils — tick it off.

### Makefile

`tui:` target ends with an install hint printing a raw `ln -s` line — TASK-10
replaces that with the `make install` hint. `PREFIX ?= /usr/local` for the new
targets; `install` must not become a prerequisite of any other target.

### Environment constraints

- `docs/` is not the mount here: this session sees the **whole** repo at
  `/workspace`, including `tui/`, `Makefile`, `Dockerfile`.
- Network is available (verified).
- `git user.email` is `leomuck@posteo.de`; commits work.
