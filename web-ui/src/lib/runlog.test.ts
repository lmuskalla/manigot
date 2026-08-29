import { describe, expect, it } from 'vitest'
import { parseRunLog } from '$lib/runlog'

const LOG = [
  '=== 2026-08-28T22:16:13+02:00 mg jdi started: job farmer_x, profile claude-pro ===',
  '',
  '=== 2026-08-28T22:16:13+02:00 analyst invoked (attempt 1) ===',
  'analyst finished — tasks.md written (3 tasks)',
  '=== 2026-08-28T22:40:00+02:00 developer invoked (attempt 1) ===',
  'NEEDS-HUMAN-INPUT: which base branch should this cut from?',
  '=== 2026-08-28T22:41:00+02:00 stopped: needs human ===',
].join('\n')

describe('parseRunLog', () => {
  it('parses the event kinds', () => {
    const events = parseRunLog(LOG)
    expect(events.map((e) => e.kind)).toEqual([
      'start',
      'invoke',
      'note',
      'invoke',
      'human',
      'stop',
    ])
  })

  it('extracts the agent from invoke lines', () => {
    const events = parseRunLog(LOG)
    const invokes = events.filter((e) => e.kind === 'invoke')
    expect(invokes[0].agent).toBe('analyst')
    expect(invokes[0].text).toContain('attempt 1')
  })

  it('strips the marker from human lines', () => {
    const human = parseRunLog(LOG).find((e) => e.kind === 'human')!
    expect(human.text).toBe('which base branch should this cut from?')
  })

  it('handles null and empty', () => {
    expect(parseRunLog(null)).toEqual([])
    expect(parseRunLog('')).toEqual([])
  })
})

// Mirrors the real header shape src/cmd/mg/jdioutput.go's logInvocation
// writes: a "finished (attempt N)" header, two blank lines, then the agent's
// full (often multi-paragraph) final-response text — not the idealized
// single-line "analyst finished — ..." convention above.
const REAL_LOG = [
  '=== 2026-08-29T10:00:00+02:00 mg jdi started: job seven_md-rendering, profile claude-pro ===',
  '',
  '=== 2026-08-29T10:00:00+02:00 analyst invoked (attempt 1) ===',
  '=== 2026-08-29T10:05:12+02:00 analyst finished (attempt 1) ===',
  '',
  '',
  'Wrote tasks.md with 4 tasks covering the markdown pipeline audit and the',
  'run-log parser fix.',
  '',
  'No blockers found; ready for the developer.',
  '=== 2026-08-29T10:06:00+02:00 developer invoked (attempt 1) ===',
  '=== 2026-08-29T10:20:45+02:00 developer finished (attempt 1) ===',
  '',
  '',
  "I'm not sure which base branch this should target.",
  'NEEDS-HUMAN-INPUT: which base branch should this cut from?',
].join('\n')

describe('parseRunLog — real finished-header shape', () => {
  it('groups the finished header and its multi-line body into one event', () => {
    const events = parseRunLog(REAL_LOG)
    expect(events.map((e) => e.kind)).toEqual([
      'start',
      'invoke',
      'finished',
      'invoke',
      'finished',
      'human',
    ])
  })

  it('captures agent and attempt on the finished event', () => {
    const finished = parseRunLog(REAL_LOG).filter((e) => e.kind === 'finished')
    expect(finished[0].agent).toBe('analyst')
    expect(finished[0].attempt).toBe(1)
    expect(finished[1].agent).toBe('developer')
  })

  it('joins the multi-line response into a single text block, preserving paragraph breaks', () => {
    const finished = parseRunLog(REAL_LOG).filter((e) => e.kind === 'finished')
    expect(finished[0].text).toBe(
      'Wrote tasks.md with 4 tasks covering the markdown pipeline audit and the\nrun-log parser fix.\n\nNo blockers found; ready for the developer.',
    )
  })

  it('still extracts NEEDS-HUMAN-INPUT out of a finished event body as its own event', () => {
    const events = parseRunLog(REAL_LOG)
    const human = events.find((e) => e.kind === 'human')!
    expect(human.text).toBe('which base branch should this cut from?')
    const lastFinished = events.filter((e) => e.kind === 'finished').at(-1)!
    expect(lastFinished.text).toBe("I'm not sure which base branch this should target.")
  })
})

// A complete, realistic run.log as mg-jdi actually writes one end to end:
// started -> invoked -> finished (agent response) -> the job-level terminal
// header logJobFinished always appends at Run's loop exit
// (src/cmd/mg/jdioutput.go). This is the shape every normal completed run
// ends with — the primary regression case for the bug where the trailer got
// silently absorbed into the preceding agent's response text instead of
// becoming its own event.
const FULL_APPROVED_LOG = [
  '=== 2026-08-29T10:00:00+02:00 mg jdi started: job seven_md-rendering, profile claude-pro ===',
  '',
  '=== 2026-08-29T10:06:00+02:00 reviewer invoked (attempt 1) ===',
  '=== 2026-08-29T10:20:45+02:00 reviewer finished (attempt 1) ===',
  '',
  '',
  'APPROVED — everything looks good.',
  '',
  '=== 2026-08-29T10:20:45+02:00 mg jdi finished: stop-finished ===',
  "verdict.md's Overall verdict is APPROVED",
].join('\n')

describe('parseRunLog — real job-finished trailer', () => {
  it('keeps the job-level "finished" trailer as its own stop event, not absorbed into the preceding agent response', () => {
    const events = parseRunLog(FULL_APPROVED_LOG)
    expect(events.map((e) => e.kind)).toEqual(['start', 'invoke', 'finished', 'stop'])
  })

  it('does not leak the trailer header or reason into the agent finished event text', () => {
    const events = parseRunLog(FULL_APPROVED_LOG)
    const finished = events.find((e) => e.kind === 'finished')!
    expect(finished.text).toBe('APPROVED — everything looks good.')
    expect(finished.text).not.toContain('mg jdi finished')
    expect(finished.text).not.toContain("verdict.md's Overall verdict")
  })

  it('formats the stop event with a friendly label and the reason', () => {
    const events = parseRunLog(FULL_APPROVED_LOG)
    const stop = events.find((e) => e.kind === 'stop')!
    expect(stop.text).toBe("finished — verdict.md's Overall verdict is APPROVED")
  })

  it('handles the stop-needs-human trailer the same way, preserving "needs human" for styling', () => {
    const log = [
      '=== 2026-08-29T10:00:00+02:00 mg jdi started: job x, profile claude-pro ===',
      '',
      '=== 2026-08-29T10:06:00+02:00 developer invoked (attempt 1) ===',
      '=== 2026-08-29T10:20:45+02:00 developer finished (attempt 1) ===',
      '',
      '',
      "I'm not sure which base branch this should target.",
      'NEEDS-HUMAN-INPUT: which base branch should this cut from?',
      '',
      '=== 2026-08-29T10:20:45+02:00 mg jdi finished: stop-needs-human ===',
      'needs human: which base branch should this cut from?',
    ].join('\n')
    const events = parseRunLog(log)
    expect(events.map((e) => e.kind)).toEqual(['start', 'invoke', 'finished', 'human', 'stop'])
    const stop = events.find((e) => e.kind === 'stop')!
    expect(stop.text).toBe('needs human — needs human: which base branch should this cut from?')
    expect(stop.text).toContain('needs human')
  })

  it('handles the stop-before-any-agent-ran case, with its reason line and no duplicate at the trailer', () => {
    const log = [
      '=== 2026-08-29T10:00:00+02:00 mg jdi started: job x, profile claude-pro ===',
      '',
      '=== 2026-08-29T10:00:00+02:00 mg jdi stopped before running any agent ===',
      'brief.md is not written yet',
      '',
      '=== 2026-08-29T10:00:00+02:00 mg jdi finished: stop-needs-human ===',
    ].join('\n')
    const events = parseRunLog(log)
    expect(events.map((e) => e.kind)).toEqual(['start', 'stop', 'stop'])
    expect(events[1].text).toBe('stopped before running any agent — brief.md is not written yet')
    expect(events[2].text).toBe('needs human')
  })
})
