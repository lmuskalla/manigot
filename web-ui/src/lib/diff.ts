// Parsers for git plain text — the shapes `mg diff` produces.
//
// The quick eyeball is `git log --oneline` + `git diff --stat` as raw text;
// the full patch is a unified diff. These parsers turn them into renderable
// structures (per-file hunks with +/- lines) so the web view can do what a
// TUI cannot: color by change type, group by file, collapse context.

export interface DiffLine {
  kind: 'add' | 'del' | 'ctx' | 'hunk' | 'meta'
  text: string
  oldNo?: number
  newNo?: number
}

export interface DiffFile {
  path: string
  binary?: boolean
  lines: DiffLine[]
  adds: number
  dels: number
}

export interface StatRow {
  file: string
  adds: number
  dels: number
  graph: string
}

/** Parse a unified diff patch into per-file groups. */
export function parsePatch(patch: string): DiffFile[] {
  const files: DiffFile[] = []
  let cur: DiffFile | null = null
  let oldNo = 0
  let newNo = 0

  for (const raw of patch.split('\n')) {
    if (raw.startsWith('diff --git ')) {
      const m = raw.match(/^diff --git a\/(.+) b\/(.+)$/)
      cur = { path: m ? m[2] : raw.slice(11), lines: [], adds: 0, dels: 0 }
      files.push(cur)
      continue
    }
    if (!cur) continue
    if (raw.startsWith('--- ') || raw.startsWith('+++ ') || raw.startsWith('index ') || raw.startsWith('new file') || raw.startsWith('deleted file') || raw.startsWith('similarity')) {
      cur.lines.push({ kind: 'meta', text: raw })
      continue
    }
    if (raw.startsWith('@@')) {
      const m = raw.match(/^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/)
      if (m) {
        oldNo = parseInt(m[1], 10)
        newNo = parseInt(m[2], 10)
      }
      cur.lines.push({ kind: 'hunk', text: raw })
      continue
    }
    if (raw.startsWith('+')) {
      cur.adds++
      cur.lines.push({ kind: 'add', text: raw.slice(1), newNo: newNo++ })
    } else if (raw.startsWith('-')) {
      cur.dels++
      cur.lines.push({ kind: 'del', text: raw.slice(1), oldNo: oldNo++ })
    } else if (raw.startsWith(' ')) {
      cur.lines.push({ kind: 'ctx', text: raw.slice(1), oldNo: oldNo++, newNo: newNo++ })
    } else if (raw === '\\ No newline at end of file') {
      cur.lines.push({ kind: 'meta', text: raw })
    }
  }
  return files
}

/** Parse `git diff --stat` output into rows with add/del bar graphs. */
export function parseStat(stat: string): StatRow[] {
  const rows: StatRow[] = []
  for (const line of stat.split('\n')) {
    const m = line.match(/^(.+?)\s+\|\s+(\d+)\s+([+-]*)$/)
    if (m) {
      rows.push({ file: m[1].trim(), adds: (m[3].match(/\+/g) ?? []).length, dels: (m[3].match(/-/g) ?? []).length, graph: m[3] })
      continue
    }
    const total = line.match(/^ (\d+) files? changed(?:.*?(\d+) insertions?\(\+\))?(?:.*?(\d+) deletions?\(-\))?/)
    if (total) {
      rows.push({ file: '__total__', adds: parseInt(total[2] ?? '0', 10), dels: parseInt(total[3] ?? '0', 10), graph: '' })
    }
  }
  return rows
}

/** Parse `git log --oneline` into {hash, subject} rows. */
export function parseLogOneline(log: string): { hash: string; subject: string }[] {
  const rows: { hash: string; subject: string }[] = []
  for (const line of log.split('\n')) {
    const m = line.match(/^([0-9a-f]{7,})\s+(.*)$/)
    if (m) rows.push({ hash: m[1], subject: m[2] })
  }
  return rows
}
