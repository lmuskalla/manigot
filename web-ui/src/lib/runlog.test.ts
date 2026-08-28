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
