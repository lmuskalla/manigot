// Small time helpers — the control plane shows "3 minutes ago" more often
// than timestamps, but exact RFC3339 is one title-attr away.

export function relativeTime(iso: string, nowMs = Date.now()): string {
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return ''
  const s = Math.max(0, Math.round((nowMs - t) / 1000))
  if (s < 45) return 'just now'
  const m = Math.round(s / 60)
  if (m < 60) return `${m} min ago`
  const h = Math.round(m / 60)
  if (h < 24) return `${h} h ago`
  const d = Math.round(h / 24)
  if (d < 14) return `${d} d ago`
  return new Date(t).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

/** mm:ss elapsed for a running state's `updated` timestamp. */
export function sinceLabel(iso: string, nowMs = Date.now()): string {
  const t = Date.parse(iso)
  if (Number.isNaN(t)) return ''
  const s = Math.max(0, Math.round((nowMs - t) / 1000))
  if (s < 90) return `${s}s`
  return `${Math.floor(s / 60)}m ${s % 60}s`
}
