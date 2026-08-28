import { describe, expect, it } from 'vitest'
import { parsePatch, parseStat, parseLogOneline } from '$lib/diff'

const STAT = ` src/cmd/mg/serve.go         |  38 ++++++++--
 src/internal/serve/api.go   | 212 +++++++++++++++++++++++++++++++++++++++++++--
 src/internal/serve/locks.go |   2 +-
 3 files changed, 250 insertions(+), 5 deletions(-)`

describe('parseStat', () => {
  it('parses file rows with add/del counts', () => {
    const rows = parseStat(STAT)
    expect(rows).toHaveLength(4)
    expect(rows[0]).toMatchObject({ file: 'src/cmd/mg/serve.go', adds: 8, dels: 2 })
    expect(rows[1]).toMatchObject({ file: 'src/internal/serve/api.go', adds: 43, dels: 2 })
  })

  it('parses the total row', () => {
    const rows = parseStat(STAT)
    const total = rows.find((r) => r.file === '__total__')
    expect(total).toMatchObject({ adds: 250, dels: 5 })
  })
})

describe('parseLogOneline', () => {
  it('splits hash from subject', () => {
    const rows = parseLogOneline('77f2ab1 (HEAD -> feature/x) TASK-7: SSE stream\n5b31c9e TASK-5: done/delete/push')
    expect(rows[0]).toMatchObject({ hash: '77f2ab1', subject: '(HEAD -> feature/x) TASK-7: SSE stream' })
    expect(rows).toHaveLength(2)
  })

  it('ignores non-commit lines', () => {
    expect(parseLogOneline('some warning\n')).toHaveLength(0)
  })
})

describe('parsePatch', () => {
  const PATCH = [
    'diff --git a/src/serve/create.go b/src/serve/create.go',
    'new file mode 100644',
    'index 0000000..8c1f2ab',
    '--- /dev/null',
    '+++ b/src/serve/create.go',
    '@@ -0,0 +1,3 @@',
    '+package serve',
    '+',
    '+// handleCreateJob scaffolds a job.',
    'diff --git a/src/serve/server.go b/src/serve/server.go',
    'index 2f9d1c1..a01b3ee 100644',
    '--- a/src/serve/server.go',
    '+++ b/src/serve/server.go',
    '@@ -47,6 +47,8 @@',
    ' context: existing',
    '+\tadded line one',
    '-removed line',
    ' more context',
  ].join('\n')

  it('groups by file', () => {
    const files = parsePatch(PATCH)
    expect(files).toHaveLength(2)
    expect(files[0].path).toBe('src/serve/create.go')
    expect(files[1].path).toBe('src/serve/server.go')
  })

  it('counts adds and dels', () => {
    const files = parsePatch(PATCH)
    expect(files[0]).toMatchObject({ adds: 3, dels: 0 })
    expect(files[1]).toMatchObject({ adds: 1, dels: 1 })
  })

  it('tracks line numbers through hunks', () => {
    const files = parsePatch(PATCH)
    const ctx = files[1].lines.find((l) => l.text === 'context: existing')!
    expect(ctx).toMatchObject({ oldNo: 47, newNo: 47 })
    const add = files[1].lines.find((l) => l.kind === 'add')!
    expect(add.newNo).toBe(48)
  })

  it('marks hunk and meta lines', () => {
    const files = parsePatch(PATCH)
    expect(files[0].lines.some((l) => l.kind === 'hunk')).toBe(true)
    expect(files[0].lines.some((l) => l.kind === 'meta')).toBe(true)
  })
})
