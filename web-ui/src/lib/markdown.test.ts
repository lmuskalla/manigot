import { describe, expect, it } from 'vitest'
import { preprocessJobMarkdown, renderMarkdown, verdictStatus } from '$lib/markdown'

describe('renderMarkdown', () => {
  it('renders basic markdown', () => {
    const html = renderMarkdown('# Title\n\nSome **bold** text.')
    expect(html).toContain('<h1>Title</h1>')
    expect(html).toContain('<strong>bold</strong>')
  })

  it('sanitizes script tags and event handlers', () => {
    const html = renderMarkdown('# hi\n\n<script>alert(1)</script>\n\n<img src=x onerror="alert(1)">')
    expect(html).not.toContain('<script')
    expect(html).not.toContain('onerror')
  })

  it('sanitifies javascript: links', () => {
    const html = renderMarkdown('[click](javascript:alert(1))')
    expect(html).not.toContain('javascript:')
  })
})

describe('verdictStatus', () => {
  it('finds the status line under ## Overall', () => {
    expect(verdictStatus('# V\n\n## Overall\n\nAPPROVED — clean.\n')).toBe('APPROVED — clean.')
    expect(verdictStatus('## Overall\n\nNEEDS WORK — profile check missing.')).toContain('NEEDS WORK')
  })

  it('stops at the next heading', () => {
    const src = '## Overall\n\nAPPROVED\n\n## Findings\nREJECTED somewhere else'
    expect(verdictStatus(src)).toBe('APPROVED')
  })

  it('returns empty without the section', () => {
    expect(verdictStatus('# no verdict here')).toBe('')
  })
})

describe('preprocessJobMarkdown', () => {
  it('turns the frontmatter run into a table', () => {
    const out = preprocessJobMarkdown('# Brief: thing\n\nstatus: open\ntype: fix\nid: ember\n\n## What\n\nBody.')
    expect(out).toContain('| `status` | open |')
    expect(out).toContain('| `type` | fix |')
    expect(out).toContain('## What')
  })

  it('does not eat body key-value-looking lines', () => {
    const out = preprocessJobMarkdown('# B\n\nintro\n\nstatus: is a word in prose\n')
    expect(out).toContain('status: is a word in prose')
    expect(out).not.toContain('| `status` |')
  })

  it('emphasises TASK markers', () => {
    const out = preprocessJobMarkdown('TASK-1: do the thing')
    expect(out).toBe('**TASK-1**: do the thing')
  })
})
