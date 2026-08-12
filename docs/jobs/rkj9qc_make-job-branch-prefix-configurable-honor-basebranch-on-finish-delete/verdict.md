# Verdict: make job branch prefix configurable; honor baseBranch on finish/delete

id: rkj9qc
status: open
reviewer: @reviewer
date: 2026-08-12

## Review

TASK-1: PASS
notes: `JobBranchPrefix string \`json:"jobBranchPrefix,omitempty"\`` added to
`tui/internal/project/settings.go` (line 39); package doc updated to "the
project conventions are … base branch … and the job branch prefix"; no
defaulting method added (empty is meaningful); round-trip test in
settings_test.go carries the new field. Exact match to the task.

TASK-2: PASS
notes: `tui/internal/ui/settings.go` — new `jobBranchPrefix` text input seeded
from `proj.JobBranchPrefix` (placeholder "jobs"), `stFieldCount` 5→6,
`stFocusJobPrefix = 2` inserted between `stFocusBranch` and `stFocusCount`
(count/profile/terminal shifted to 3/4/5), wired through `resize`,
`setFocus` (blur+focus), `update`'s text-input routing (line 217), `projectValue()`
with `strings.TrimSpace`, `render` (project-scoped label + "blank = feature/…"
hint), and `hint()`. Tab/shift+tab cycle tests updated for the 6-field cycle;
profile/recent-count/terminal tests re-tabbed correctly;
`TestSettingsJobPrefixEdits` + `TestSettingsJobPrefixSeededFromProjectSettings`
added; render string list gains "Job branch prefix". `go test -count=1
./internal/project/ ./internal/ui/` passes; `go vet` clean.

TASK-3: PASS
notes: `scripts/new-job.sh` reads `jobBranchPrefix` with the same guarded
single-key `sed` as `baseBranch` (line 103), defaults to empty; branch
composed as `${JOB_BRANCH_PREFIX:+${JOB_BRANCH_PREFIX}/}${JOB_TYPE}/${ID}_${SLUG}`
(line 128); header/usage comments updated. Verified live in a scratch repo:
with `jobBranchPrefix: "jobs"` and a plain `feature` branch present, `mg job`
creates `jobs/feature/<id>_<slug>`; empty prefix keeps `feature/<id>_<slug>`.

TASK-4: PASS
notes: `scripts/finish-job.sh` (lines 139-150) and `scripts/delete-job.sh`
(lines 190-201) both resolve `DEFAULT_BRANCH` from the `baseBranch` key in
`.manigot/manigot.json` first, falling back to the prior
`git symbolic-ref refs/remotes/origin/HEAD` → `main` detection when the key
is absent. In finish-job.sh this feeds the checkout + squash-merge target
(lines 173/201/239); in delete-job.sh the main-worktree switch target (line
214). Comments updated. Verified end-to-end in a scratch repo configured with
`baseBranch: "development"`: `mg done` on a `jobs/feature/...` branch
squash-merged it into `development` (main untouched). `bash -n` clean on all
three scripts.

TASK-5: PASS
notes: `scripts/new-job.sh` pre-flight check (lines 136-154), inside the git
case (`CURRENT_BRANCH` non-empty) and before `git worktree add`: walks every
ancestor path segment of the composed branch name except the `<id>_<slug>`
leaf via `${BRANCH%/*}` iteration, tests `git rev-parse --verify --quiet
refs/heads/<ancestor>`, and on hit exits 1 with the clear two-line error
naming the blocking branch and pointing at `jobBranchPrefix`. Non-git
fallback untouched. Verified live: plain `feature` branch + no prefix →
clear error, exit 1, no branch/worktree created; plain `jobs` branch + prefix
`jobs` → error naming `jobs`; plain `jobs/feature` branch + prefix `jobs` →
error naming `jobs/feature`; deep prefix `a/b` succeeds when no ancestor
collides. No false positives (unrelated branches don't trigger it).

TASK-6: PASS
notes: `docs/AGENTS.md` — new-job.sh bullet documents
`[<prefix>/]<type>/<id>_<slug>` naming + collision rationale + pre-check;
finish-job.sh/delete-job.sh bullets document `baseBranch`-first merge/switch
target with `origin/HEAD` → `main` fallback; `.manigot/manigot.json` bullet
documents `jobBranchPrefix` alongside `baseBranch` and names all three
readers; `mg job`/`mg done` command lines and the Job workflow section
updated. `README.md` "Branch naming:" line updated with the optional prefix.
`project-template/docs/AGENTS.md` and `agents/*.md` untouched and correctly
so — the agent files reference only `brief.md`'s `branch:` field
generically, and `reviewer.md`'s `git diff main...HEAD` is explicitly out of
scope per brief.md. `git diff main...HEAD` confirms no other files changed.

## Security

No security-relevant changes: the new `sed` extraction reads the same
project-local JSON the `baseBranch` read already did (ref-name values only,
no code execution); the collision check is a read-only `rev-parse --verify`.
No tokens or secrets touched. No findings.

## Overall

APPROVED

All six tasks are implemented as specified, scope is clean (only the files
named in tasks.md changed, plus the job's own docs), commit discipline is
correct (`[rkj9qc] TASK-N: …` per task, separate brief and implementation
commits), `go test ./...` (fresh `-count=1`) and `go vet` pass from `tui/`,
`bash -n` is clean on all three scripts, and both headline behaviors were
verified end-to-end in scratch repos: prefixed job branches with a
conflicting plain branch present, and `mg done` merging into the configured
`baseBranch` instead of `origin/HEAD`/`main`.

Non-blocking observations (not blockers, not in tasks.md):
- `jobBranchPrefix` is unvalidated: a value with a trailing/leading slash
  (e.g. `jobs/`) yields an invalid branch name that fails at `git worktree
  add` with git's own "not a valid branch name" fatal rather than a friendly
  message. Same class of unvalidated-input UX as the existing `baseBranch`
  handling; the tasks did not ask for validation.
- The collision error is split across two echo lines vs. the single sentence
  template in tasks.md — same content, cosmetic only.
- `finish-job.sh` line 222's comment references "line 190" for the checkout
  (stale line number after the diff) — cosmetic comment drift only.
