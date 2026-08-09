# Tasks: TUI

id: irw320
status: open
analyst: leomuck@posteo.de
date: 2026-08-08

<!-- Produced by @analyst from brief.md. -->

## Scope note

The TUI is **host-side**: it runs on the user's machine, NOT inside the
safecode container. It reads a project's job directories and shells out to
the existing host PATH commands — `safecode`, `new-job`, and `finish-job`
(see discrepancy #3). It does not run in Docker and needs no credentials
itself.

## Open questions (resolve before / during TASK-1)

The brief's Why / Out of scope / Notes sections are empty, so confirm with
the author before locking scope:

1. **Target platforms for "new terminal window/pane" (TASK-8):** macOS only?
   macOS + Linux? tmux required? Windows? This drives both the stack choice
   (TASK-1) and most of the implementation complexity.
2. **Surface `finish-job` too?** Should the TUI also offer merge/archive, or
   only `safecode` and `new-job`? `finish-job.sh` exists but is undocumented
   in the README. Recommend including it in a later phase.
3. **Distribution model:** prebuilt binary, run-from-source script, or the
   existing symlink-to-PATH pattern used by `safecode`/`new-job`?

## Conventions (now consistent across repo)

Resolved as part of this job's analysis (docs + scripts aligned):

1. **Jobs live in `docs/processes/`.** Scripts (`run.sh:107`, `new-job.sh:10`,
   `finish-job.sh:9`) and docs (`README.md`, `AGENTS.md`) now agree — the TUI
   scans `docs/processes/`. `docs/templates/processes/` (and
   `project-template/docs/templates/processes/`) is a **reference example
   only**, not where live jobs go; exclude it from the job list.
2. **Developer file is `implementation.md`.** `new-job.sh` now creates
   `implementation.md`, matching `agents/developer.md`, `agents/reviewer.md`,
   and the docs; existing `result.md` files were renamed. The TUI renders
   `implementation.md`.

Still open (follow-up chore, out of scope for this job):
3. **`finish-job.sh` is undocumented.** `scripts/finish-job.sh` exists
   (archives a job to `docs/processes/archive/`, sets `status: done`, merges
   the branch) but is missing from the README's "where everything lives"
   section. TASK-11/TASK-12 should account for it.

## Implementation notes (apply across tasks)

- **Job frontmatter is loose, not YAML.** Each job file begins with
  `# Title`, a blank line, then bare `key: value` lines — **no `---`
  delimiters.** Keys seen in `brief.md`: `status`, `type`, `id`, `branch`,
  `date`, `author`. The existing scripts parse these with plain
  `grep '^branch:'` / `grep '^status:'` (see `finish-job.sh:75,145`). The TUI
  parser must match this loose format; do not assume YAML.
- **Status values:** `open` (default) and `done` (set by
  `finish-job.sh:145` via `sed`). TASK-10 only needs these two.
- **Title source:** the `# Brief: <title>` heading line is the human title;
  the directory name `<id>_<slug>` is the machine identity.
- **Exclude `archive/`** from the job list — `finish-job.sh` moves finished
  jobs into `docs/processes/archive/` and itself filters with `-v '/archive'`
  (`finish-job.sh:51,58`).
- **Project-root detection:** walk up from `$PWD` until a `docs/` directory is
  found. Identical helper in all three scripts (`run.sh:46`, `new-job.sh:37`,
  `finish-job.sh:21`). Reuse this algorithm; do not invent a new one.
- **Agent invocation contract (confirmed real — `run.sh:16-17,125-137`):**
  `safecode --agent <name> --job <id>` starts the chosen tool on that job,
  injecting the "read brief.md first" prompt. The TUI only needs to spawn this
  in a new terminal. Valid agent names: `analyst`, `developer`, `reviewer`,
  `security`, `product-owner`, `designer`.
- **Stage model for the action bar (TASK-7), per the brief:**
  - job open, no tasks written → Product Owner, Analyst
  - `tasks.md` written → Developer
  - `implementation.md` written → Reviewer, Security

  "Written" is the least-defined term in the brief. Recommended rule: a file
  is "written" when it has content beyond its empty placeholder comment block.
  Pin this down before implementing TASK-7.

## Task breakdown

TASK-1: Evaluate and pick the TUI stack.
     files: docs/processes/irw320_tui/brief.md (Notes), this tasks.md
     depends: none
     risk: low — research/decision only, but gates TASK-2 onward. Compare
            Bubble Tea (Go) vs Textual (Python) vs Ink (Node) against the
            constraints above (host-side, shells out to bash, renders
            markdown, cross-platform terminal spawning per Open Question #1).
            The brief specifically asks about Bubble Tea — address it
            explicitly. Record the decision + rationale.

TASK-2: Scaffold the TUI project.
     files: new `tui/` dir at repo root; dep manifest (go.mod /
            pyproject.toml / package.json)
     depends: TASK-1
     risk: medium — module layout and build story ripple into every later
            task. Provide an entry point and a local run command (e.g.
            `go run ./tui`, a venv script, or `npm`), runnable from the repo
            root without a full install.

TASK-3: Project-root + job discovery.
     files: `tui/` source (discovery + frontmatter modules)
     depends: TASK-2
     risk: medium — loose frontmatter parsing (not YAML). Implement
            the walk-up-to-`docs/` root detection and enumerate job dirs under
            `docs/processes/`, excluding `archive/`. Parse loose `brief.md`
            frontmatter (id, type, status, branch, date, author) and the
            `# Brief:` title. Follow the Implementation Notes exactly.

TASK-4: Job list view.
     files: `tui/` source (list view)
     depends: TASK-3
     risk: low. Display all discovered jobs with ID, title, type, status;
           keyboard-selectable; sorted by date desc or id.

TASK-5: Markdown rendering.
     files: `tui/` source (markdown renderer + viewer)
     depends: TASK-3
     risk: medium — markdown rendering + in-TUI scrolling is the fiddly part.
            Render `brief.md`, `tasks.md`, `implementation.md`, `verdict.md`
            as readable, scrollable markdown. Source files live at
            `docs/processes/<job>/`.

TASK-6: Job detail view (composition).
     files: `tui/` source (detail view + navigation)
     depends: TASK-4, TASK-5
     risk: low. Navigate from the list into a detail view showing the four
            rendered files; keyboard nav back/forth between list and detail,
            and between files.

TASK-7: Agent action bar keyed to job stage.
     files: `tui/` source (stage detection + action bar)
     depends: TASK-3, TASK-6
     risk: medium — the stage-detection rule is the least-defined part of the
            brief. Detect stage per the Stage Model above and surface the
            matching agent actions. Define "written" before implementing.

TASK-8: Fire an agent in a new terminal window/pane.
     files: `tui/` source (terminal launcher, per-platform)
     depends: TASK-7
     risk: high — cross-platform terminal spawning is genuinely fiddly and
            platform-dependent; the platform scope (Open Question #1) directly
            drives effort. On action, spawn
            `safecode --agent <name> --job <id>` in a new terminal (e.g. tmux
            split, macOS osascript/Terminal, linux `x-terminal-emulator`).

TASK-9: new-job shortcut.
     files: `tui/` source (new-job form + shell-out)
     depends: TASK-2
     risk: low–medium. From within the TUI, trigger
            `new-job "<title>" [--type feature|fix|chore]` (title input +
            optional type), then refresh the list. Reuse the existing host
            command; do not reimplement job creation.

TASK-10: Status tracking + refresh.
     files: `tui/` source (status display + refresh hook)
     depends: TASK-4
     risk: low. Parse the `status` field from `brief.md` (values: open / done)
            and reflect it in list + detail views. Refresh on focus / on
            return from an agent run (agents edit files outside the TUI).

TASK-11: Install / launcher wiring + Makefile target.
     files: `Makefile`; `scripts/` (launcher/symlink guidance); possibly README
            install steps
     depends: TASK-2
     risk: low. Provide a Makefile target and a PATH-symlink install path
            mirroring how `safecode` / `new-job` / `finish-job` are installed,
            so the TUI is invocable as a host command. Decide distribution per
            Open Question #3.

TASK-12: Update the README.
     files: `README.md`
     depends: TASK-2 through TASK-11
     risk: low. Document the TUI: what it is, install, run command,
            keybindings, the stage→agent model, and supported platforms. If in
            scope, also add the missing `finish-job` entry (discrepancy #3).

## Suggested sequencing

TASK-1 first (hard gate), then TASK-2 → TASK-3. After TASK-3, TASK-4 / TASK-5
/ TASK-9 / TASK-10 / TASK-11 can parallelize. TASK-6 needs TASK-4 + TASK-5;
TASK-7 needs TASK-3 + TASK-6; TASK-8 needs TASK-7; TASK-12 last.
