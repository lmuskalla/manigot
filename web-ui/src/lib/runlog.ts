// run.log parsing — the per-invocation event summary mg-jdi appends. Header
// conventions:
//
//   === 2026-08-28T22:16:13+02:00 analyst invoked (attempt 1) ===
//   === 2026-08-28T22:16:40+02:00 analyst finished (attempt 1) ===
//
//   <two blank lines>
//   <the agent's full final-response text, often several paragraphs>
//   NEEDS-HUMAN-INPUT: <the question the agent stopped on>
//
// (the real shape `logInvocation` in src/cmd/mg/jdioutput.go writes — a
// "finished" header followed by the agent's whole response, not a one-line
// summary), plus the job-level terminal headers `logJobFinished`/
// `logImmediateStop` always write at the end of every run:
//
//   === 2026-08-28T22:41:00+02:00 mg jdi finished: stop-finished ===
//   verdict.md's Overall verdict is APPROVED
//
//   === 2026-08-28T22:41:00+02:00 mg jdi stopped before running any agent ===
//   brief.md is not written yet
//
// (the reason line is optional — `logJobFinished`'s `includeReason` is false
// for the stop-before-any-agent case, whose reason was already printed by
// `logImmediateStop` immediately above it). The idealized single-line
// convention some older fixtures used — "analyst finished — tasks.md written
// (3 tasks)" — is still recognized as a 'note', and the single-line
// "=== ... stopped: needs human ===" convention is still recognized as a
// 'stop', both for compatibility. The events become the activity timeline;
// the NEEDS-HUMAN-INPUT line is lifted out as the handoff banner wherever it
// appears, including inside a "finished" event's response body.

export interface RunEvent {
  kind: 'start' | 'invoke' | 'finished' | 'note' | 'stop' | 'human' | 'raw'
  agent?: string
  attempt?: number
  text: string
  timestamp?: string
}

export function parseRunLog(runLog: string | null | undefined): RunEvent[] {
  if (!runLog) return []
  const events: RunEvent[] = []
  // Both 'finished' (agent response) and 'stop' (job-level terminal) events
  // are pushed to events immediately (at their header), preserving
  // chronological order against anything found inside their body (a
  // NEEDS-HUMAN-INPUT line, in particular) — only .text is filled/appended
  // later, once the whole body has been collected, by flushOpen (called at
  // the next header line, or at the end of the log). A 'finished' event's
  // text is replaced wholly by its body (the agent's response, and nothing
  // else preceded it); a 'stop' event's text starts as its own header label
  // and gets an optional " — <reason>" suffix appended from its body, since
  // unlike 'finished' it already carries meaningful text from the header.
  let openEvent: RunEvent | null = null
  let openBody: string[] = []

  const flushOpen = () => {
    if (openEvent) {
      const body = openBody.join('\n').trim()
      if (openEvent.kind === 'finished') {
        openEvent.text = body
      } else if (body) {
        openEvent.text = openEvent.text ? `${openEvent.text} — ${body}` : body
      }
    }
    openEvent = null
    openBody = []
  }

  for (const raw of runLog.split('\n')) {
    const line = raw.trim()
    if (!line) {
      // A blank line inside an open body is a paragraph break worth keeping;
      // everywhere else it's just log formatting, always skipped.
      if (openEvent) openBody.push('')
      continue
    }

    const hdr = line.match(
      /^===\s*(.+?)\s+(mg jdi started.*|mg jdi finished:.*|mg jdi stopped before running any agent|.+? invoked \(attempt \d+\)|.+? finished \(attempt \d+\)|stopped:.*)\s*===$/,
    )
    if (hdr) {
      flushOpen()
      const stamp = hdr[1]
      const what = hdr[2]
      if (what.startsWith('mg jdi started')) {
        events.push({ kind: 'start', timestamp: stamp, text: what })
      } else if (what.includes(' invoked ')) {
        events.push({ kind: 'invoke', agent: what.split(' ')[0], timestamp: stamp, text: what })
      } else if (what.includes(' finished ')) {
        const attempt = what.match(/attempt (\d+)/)
        openEvent = {
          kind: 'finished',
          agent: what.split(' ')[0],
          attempt: attempt ? Number(attempt[1]) : undefined,
          timestamp: stamp,
          text: '',
        }
        events.push(openEvent)
        openBody = []
      } else if (what.startsWith('mg jdi finished:')) {
        // The job-level terminal header (logJobFinished): "mg jdi finished:
        // stop-finished" / "mg jdi finished: stop-needs-human" — reworded to
        // match the older "stopped: needs human" convention's label so the
        // existing 'needs human' styling keeps applying.
        const kind = what.slice('mg jdi finished:'.length).trim()
        const label =
          kind === 'stop-needs-human' ? 'needs human' : kind === 'stop-finished' ? 'finished' : kind
        openEvent = { kind: 'stop', timestamp: stamp, text: label }
        events.push(openEvent)
        openBody = []
      } else if (what === 'mg jdi stopped before running any agent') {
        openEvent = { kind: 'stop', timestamp: stamp, text: 'stopped before running any agent' }
        events.push(openEvent)
        openBody = []
      } else {
        // The older single-line "stopped: ..." convention — no separate
        // reason line follows it in practice, but still left "open" so one
        // would be appended if it ever did.
        openEvent = { kind: 'stop', timestamp: stamp, text: what }
        events.push(openEvent)
        openBody = []
      }
      continue
    }
    if (line.startsWith('NEEDS-HUMAN-INPUT:')) {
      // Extracted as its own event exactly as before, even mid-response.
      events.push({ kind: 'human', text: line.replace(/^NEEDS-HUMAN-INPUT:\s*/, '') })
      continue
    }
    if (/^stopped:/.test(line)) {
      flushOpen()
      events.push({ kind: 'stop', text: line })
      continue
    }
    if (openEvent) {
      openBody.push(line)
      continue
    }
    if (/^(analyst|developer|reviewer|owner|security|quality)\b/.test(line)) {
      events.push({ kind: 'note', agent: line.split(' ')[0], text: line })
      continue
    }
    events.push({ kind: 'raw', text: line })
  }
  flushOpen()
  return events
}
