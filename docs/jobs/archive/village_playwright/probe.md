# Perception probe protocol — village_playwright

Status: open — protocol ready, sessions to be run by a human (see "How to run").

## Purpose

Determine, per profile × mode, whether the model **can see images** (the PNG
produced by `shot`) or must fall back to the render report / `--describe`
prose. The result feeds the perception matrix and the documented fallback
rule (docs/PLAYWRIGHT.md scope item 6).

## Fixture

`docs/jobs/village_playwright/fixture/index.html` — committed page with three
seeded defects:

1. **Known-failing contrast pair** — `#999999` text on white, ≈ 2.85:1, fails WCAG AA.
2. **Known overflow** — fixed-width notice box with clipped long word (`overflow-x: hidden`).
3. **Known misalignment** — a card pushed 8px off the sibling grid line.

The PNG produced by `shot` doubles as the perception-probe image: one artifact,
two purposes (verified render + probe image).

## How to run (human)

Per profile × mode — six sessions total (3 profiles × interactive/`--print`):

1. `make rebuild` (the gate — the image must carry playwright + shot).
2. Start the fixture server inside the session:
   ```
   cd /workspace/docs/jobs/village_playwright/fixture && python3 -m http.server 8322 &
   ```
3. Run the probe render (as developer — shot is gated for read-only agents):
   ```
   shot http://127.0.0.1:8322/index.html --describe
   ```
4. Answer the perception questions below, from the **PNG directly** (open
   `screenshots/*.png` if your model supports images) and from the report.
5. Copy the answers back into this job, into the matrix below.

Profiles and modes:

| # | Profile | Mode | How to launch |
|---|---|---|---|
| 1 | claude-pro | interactive | `mg --job village --profile claude-pro --agent developer` |
| 2 | claude-pro | `--print` | `mg --job village --profile claude-pro --agent developer --print --prompt "<probe prompt>"` |
| 3 | zai | interactive | `mg --job village --profile zai --agent developer` |
| 4 | zai | `--print` | `mg --job village --profile zai --agent developer --print --prompt "<probe prompt>"` |
| 5 | opencode-go | interactive | `mg --job village --profile opencode-go --agent developer` |
| 6 | opencode-go | `--print` | `mg --job village --profile opencode-go --agent developer --print --prompt "<probe prompt>"` |

Probe prompt (same text for all six):

```
You are running the perception probe for the shot render tool.
Run: shot http://127.0.0.1:8322/index.html --describe
Then answer:
1. CAN YOU SEE IMAGES? (yes/no — did you actually view the PNG pixels, or did you rely on the render report / prose?)
2. List every defect you can perceive in the rendered page.
3. Which of the three seeded defects did you catch? (low-contrast text, clipped/overflowing notice box, off-grid card)
4. One sentence: was the render report or the --describe prose the more useful signal for you?
5. Quote one concrete fact from the PNG you could only know by seeing it (e.g. a color, a size, a position).
```

## Perception matrix

Fill one row per session. "Sees PNG" = the model actually perceived the image
(not just the report). "Caught defects" = how many of the 3 seeded defects the
model identified from the image.

| Profile | Mode | Sees PNG? | Caught defects (of 3) | Fallback used | Notes |
|---|---|---|---|---|---|
| claude-pro | interactive | | | | |
| claude-pro | `--print` | | | | |
| zai | interactive | | | | |
| zai | `--print` | | | | |
| opencode-go | interactive | | | | |
| opencode-go | `--print` | | | | |

## Prose samples

Paste one representative `--describe` prose block per profile here (raw,
unedited) so the human and later agents can judge quality:

### claude-pro

<!-- paste -->

### zai

<!-- paste -->

### opencode-go

<!-- paste -->

## Fallback rule (to document in PLAYWRIGHT.md once the matrix lands)

The agent files already carry the self-selection instruction ("if you cannot
see the PNG, run `shot --describe` and reason from the prose"). The matrix
decides the *default expectation* per profile, and whether `--describe`'s key
availability should be widened (TASK-4 flip: forward `ZHIPU_API_KEY` into
opencode-go sessions via `OpenCodeKeys` + test update) — recorded, not done,
because the probes are human-gated and the change is test-pinned.
