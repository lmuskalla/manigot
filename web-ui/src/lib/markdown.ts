// Markdown → sanitized HTML. Job files are agent-authored markdown; they are
// rendered (never executed) with DOMPurify scrubbing everything. The renderer
// is tuned for the four-file contract: headings, lists, task lines, code
// fences, tables.

import { marked } from 'marked'
import DOMPurify from 'dompurify'

marked.setOptions({
  gfm: true,
  breaks: false,
})

export function renderMarkdown(src: string): string {
  const html = marked.parse(src ?? '', { async: false })
  return DOMPurify.sanitize(html, {
    FORBID_TAGS: ['style', 'form', 'input', 'iframe'],
    FORBID_ATTR: ['onerror', 'onclick', 'onload'],
    ALLOW_DATA_ATTR: false,
  })
}

/** Extract the `## Overall` verdict status line from a verdict document. */
export function verdictStatus(src: string): string {
  const m = src.match(/^##\s*overall\s*$/im)
  if (!m) return ''
  const rest = src.slice(m.index! + m[0].length)
  const section = rest.split(/\n(?=##\s)/)[0]
  for (const line of section.split('\n')) {
    if (/APPROVED|REJECTED|NEEDS WORK/i.test(line)) return line.trim()
  }
  return ''
}

const FRONT_KEY = /^(status|type|id|branch|date|author):/

/**
 * Pre-render transform for job markdown:
 *  · the leading run of loose frontmatter (`key: value` lines under the
 *    title, blank lines inside the run swallowed) becomes a table — the
 *    brief's metadata is data, not prose;
 *  · `TASK-n:` line prefixes are emphasised as the task markers they are.
 */
export function preprocessJobMarkdown(src: string): string {
  const lines = (src ?? '').split('\n')
  const out: string[] = []
  let i = 0

  if (lines[0]?.startsWith('# ')) {
    out.push(lines[0])
    i = 1
  }
  // Skip blank lines between the title and the frontmatter run.
  while (i < lines.length && lines[i].trim() === '') i++

  const fm: [string, string][] = []
  while (i < lines.length) {
    const line = lines[i]
    const m = line.match(/^([a-z]+):\s*(.*)$/)
    if (m && FRONT_KEY.test(line)) {
      fm.push([m[1], m[2]])
      i++
    } else if (line.trim() === '' && fm.length > 0 && FRONT_KEY.test(lines[i + 1] ?? '')) {
      i++ // blank between frontmatter lines
    } else {
      break
    }
  }
  if (fm.length > 0) {
    out.push('| key | value |', '| --- | ----- |')
    for (const [k, v] of fm) out.push(`| \`${k}\` | ${v || '—'} |`)
    out.push('')
  }

  for (; i < lines.length; i++) {
    const t = lines[i].match(/^(TASK-\d+):(.*)$/)
    out.push(t ? `**${t[1]}**:${t[2]}` : lines[i])
  }
  return out.join('\n')
}
