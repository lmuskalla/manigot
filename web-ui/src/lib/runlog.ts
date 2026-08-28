// run.log parsing — the per-invocation event summary mg-jdi appends:
//   === 2026-08-28T22:16:13+02:00 analyst invoked (attempt 1) ===
//   analyst finished — tasks.md written (3 tasks)
//   NEEDS-HUMAN-INPUT: <the question the agent stopped on>
// The events become the activity timeline; the NEEDS-HUMAN-INPUT line is
// lifted out as the handoff banner.

export interface RunEvent {
  kind: 'start' | 'invoke' | 'note' | 'stop' | 'human' | 'raw'
  agent?: string
  text: string
  timestamp?: string
}

export function parseRunLog(runLog: string | null | undefined): RunEvent[] {
  if (!runLog) return []
  const events: RunEvent[] = []
  for (const raw of runLog.split('\n')) {
    const line = raw.trim()
    if (!line) continue
    const hdr = line.match(/^===\s*(.+?)\s+(mg jdi started.*|.+? invoked \(attempt \d+\)|stopped:.*)\s*===$/)
    if (hdr) {
      const stamp = hdr[1]
      const what = hdr[2]
      if (what.startsWith('mg jdi started')) {
        events.push({ kind: 'start', timestamp: stamp, text: what })
      } else if (what.includes(' invoked ')) {
        events.push({ kind: 'invoke', agent: what.split(' ')[0], timestamp: stamp, text: what })
      } else {
        events.push({ kind: 'stop', timestamp: stamp, text: what })
      }
      continue
    }
    if (line.startsWith('NEEDS-HUMAN-INPUT:')) {
      events.push({ kind: 'human', text: line.replace(/^NEEDS-HUMAN-INPUT:\s*/, '') })
      continue
    }
    if (/^stopped:/.test(line)) {
      events.push({ kind: 'stop', text: line })
      continue
    }
    if (/^(analyst|developer|reviewer|owner|security|quality)\b/.test(line)) {
      events.push({ kind: 'note', agent: line.split(' ')[0], text: line })
      continue
    }
    events.push({ kind: 'raw', text: line })
  }
  return events
}
