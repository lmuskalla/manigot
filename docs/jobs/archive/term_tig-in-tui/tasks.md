# Tasks: tig in tui

id: term
status: open
analyst: architect (analysis pass, unattended)
date: 2026-08-16

<!-- Produced by @analyst from brief.md. -->

## Decisions this breakdown locks in

**1. Key: `t`, from the job detail view only.** The brief suggests `t` and
scopes reachability to "inside a job detail page". The list view already has
`j`/`o`/`a`/`n`/`s`/`ctrl+r`; no list-view change. `t` collides with no
existing detail-view binding (tab/1-6, e, D, j, x/del, P, esc/q, ctrl+r,
agent keys a/o/d/r/s, scroll keys).

**2. Launch mechanism: reuse `internal/launch`'s detached spawn ("just like
agents").** A new `launch.Tig` builds `cd '<projectRoot>' && '<mg-binary>'
diff '<job-name>' --tig` and hands it to the existing `launchDetached`,
which already implements exactly what the brief asks for: a tmux split pane
(with the replace policy — at most one manigot pane) when the TUI is inside
tmux, falling back to `config.Settings.Terminal`, Terminal.app, or a Linux
terminal emulator otherwise. No new tmux/terminal code is written. The CLI
path `mg diff --tig` already exists (`cmd/mg/diff.go` runTig): it resolves
the job's branch + the project's base branch, builds the three-dot range
`<base>...<branch>`, runs `tig <range>`, and errors clearly when tig is not
installed; `holdOnFailure` keeps the pane open on any failure.

**3. Identifier passed to `mg diff`: the job's `Name` (id_slug), not its
`ID`.** `mg diff` resolves via `resolveJobBranch` — exact match on the
branch's id_slug tail segment first, then unique prefix. `job.Name`
(e.g. `term_tig-in-tui`) equals the tail segment of the job's branch
(`feature/term_tig-in-tui`) exactly, so resolution is unambiguous. Do NOT
cargo-cult `job.ID` from the `launch.Agent` call site — `--job` in the
session launcher and the job arg in `mg diff` resolve differently.

**4. "if available" = gate the hint and the key on tig being installed on
the host.** `launch.TigAvailable()` (an `exec.LookPath` behind an exported
`launch.TigLookPath` seam, mirroring `ExeOverride`/`JdiExe`) is queried when
the detail view is constructed and cached on the view. The footer hint
`· t tig` renders only when available AND the job has a branch; pressing `t`
when unavailable reports a one-line status instead of silently doing nothing;
a branch-less job gets the same "no branch known for this job" guard the `P`
push key already uses. `launch.Tig` re-checks availability itself (authoritative
backstop) so a stale cached gate surfaces a synchronous status error instead
of a doomed pane. Alternative considered and rejected: always binding `t` and
letting the pane show `mg diff --tig`'s tig-missing error — less
discoverable, and "if available" reads as an availability gate, not an error
path.

**5. Hint placement: footer, not the action bar.** The action bar's stage
line already carries the stage timeline + `[D] Done` + `[j] just do it` +
an optional running badge and does not truncate at narrow widths — adding
`[t] tig` there risks overflowing at 80 columns. The footer's conditional
hint pattern (`e edit` only on editable tabs) is the established home.

**6. Out of scope.** No changes to `mg diff`/`runTig` themselves, no
list-view key, no tig installation/management, no action-bar button, no
`--profile` flag on `mg diff` (it takes none — `mg diff` is a host git
command, not a session launch).

## Task breakdown

TASK-1: `internal/launch` — add the tig availability query: an exported
package-level `var TigLookPath = exec.LookPath` (the test seam, mirroring
`ExeOverride`/`JdiExe` and `cmd/mg`'s unexported `tigLookPath`) and
`func TigAvailable() bool` returning whether tig resolves on the host.
files: internal/launch/launch.go
depends: none
risk: low — a thin LookPath wrapper with an established seam shape.

TASK-2: `internal/launch` — add `Tig(jobID, projectRoot, terminal string)
(string, error)` plus a `tigShellCommand(manigotPath, jobID, projectRoot
string) string` helper (deliberately its own function, per this file's
"separate function per launch path" convention). The inner command is
`cd '<projectRoot>' && '<manigot>' diff '<jobID>' --tig` (cd-first,
shellQuote-everything, NO `--profile`), wrapped in `holdOnFailure`, spawned
via the existing `launchDetached` so tmux split-pane / override / Terminal.app
/ Linux-emulator behavior and the replace policy come for free. `Tig` first
checks `TigAvailable()` and returns the "tig is not installed on the host"
error synchronously (backstop behind the TUI's cached gate, TASK-4/5).
Returns the same short "where it opened" description as Agent/Quick.
files: internal/launch/launch.go
depends: TASK-1
risk: low-medium — mirrors the Agent/Quick/AgentQuick pattern exactly; the
only novel bit is getting the shell-command format right and not passing a
profile flag `mg diff` doesn't accept.

TASK-3: `internal/launch` tests — pin the new launch path: `tigShellCommand`
format (cd + `diff <name> --tig`, holdOnFailure wrap, quote/space escaping,
no `--profile`/`--agent`/`--job` flags), `TigAvailable` true/false via a
stubbed `TigLookPath`, `Tig`'s tig-missing error path, `Tig`'s ExeOverride
resolution-failure path, and a `Tig` success path through the existing
`tmuxStub` (desc "tmux pane", split-window invoked with the tig inner
command, pane tagged/recorded — mirror `TestAgentAndQuickShareOneTrackedPane`).
files: internal/launch/launch_test.go
depends: TASK-1, TASK-2
risk: low — the tmuxStub and Agent-test patterns already exist in this
package; this is an additive clone.

TASK-4: `internal/ui` detail view — add a `tigAvailable bool` field to
`detailView`, set in `newDetailView` via `launch.TigAvailable()` (cached for
the view's lifetime; re-checked on each job open), and extend
`renderFooter`'s hint with `· t tig` only when `tigAvailable` AND
`d.job.Branch != ""` — mirroring the existing conditional `e edit` hint.
files: internal/ui/detail.go
depends: TASK-1
risk: low — a conditional hint string following an established pattern; the
zero-value default (unavailable) keeps existing footer tests green.

TASK-5: `internal/ui` app — add a `"t"` case to `updateDetail` (before the
agentForKey fallthrough): if `!a.detail.tigAvailable`, set the status line
(suggested: "tig is not installed on the host — install it, or use the diff
tab") and return; if `a.detail.job.Branch == ""`, set the same "no branch
known for this job" guard the `P` key uses and return; otherwise call
`launch.Tig(a.detail.job.Name, a.root, a.settings.Terminal)` and set
`"→ tig in " + desc` or `cmdErrorText(err)`, mirroring the agent-launch
status reporting. Update the `agentMeta` doc comment's list of non-colliding
detail-view bindings to include `t`.
files: internal/ui/app.go, internal/ui/agents.go (comment only)
depends: TASK-2, TASK-4
risk: low — a new key case mirroring the existing `P` and agent-launch cases;
`t` is unbound today so nothing regresses.

TASK-6: `internal/ui` tests — cover the hint and the key: footer shows
`t tig` when available + branch set and omits it when unavailable or
branch-less (stub `launch.TigLookPath` or set `d.tigAvailable` directly);
`t` when tig unavailable reports the not-installed status and launches
nothing (stub `launch.ExeOverride` to a marker stub and assert it did not
run, mirroring jdilaunch_test's marker pattern); `t` on a branch-less job
reports "no branch known"; `t` with tig available but an ExeOverride
resolution failure surfaces the error (proves routing to `launch.Tig`); `t`
success path lands `→ tig in tmux pane` using a locally replicated tmux stub
on PATH + `$TMUX` (the codebase convention of self-contained test helpers —
see detail_test.go's gitRun/gitInitRepo/addJobWorktree copies), asserting the
tmux call log contains the `diff <name> --tig` inner command.
files: internal/ui/tig_test.go (new; or app_test.go/detail_test.go)
depends: TASK-4, TASK-5
risk: medium — the success-path test needs a tmux stub replicated in this
package (or accept a weaker assertion via the failure paths only); the
gating and failure-path tests are straightforward.

TASK-7: documentation — add the `t` row to the README's detail-view
Keybindings table ("open the job's branch diff in tig — spawns in a tmux
split pane / new terminal like agent launches; only when tig is installed on
the host"), and a matching sentence in the TUI section of `docs/AGENTS.md`
plus `project-template/docs/AGENTS.md` (the hard-rule sync pair).
files: README.md, docs/AGENTS.md, project-template/docs/AGENTS.md
depends: TASK-5
risk: low — prose additions in three documented locations; keep the three
descriptions consistent with each other.
