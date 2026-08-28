import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import Pipeline from '$lib/components/Pipeline.svelte'

describe('Pipeline', () => {
  it('renders all five stations', () => {
    render(Pipeline, { stage: 'implement', variant: 'full' })
    const labels = screen.getByRole('img').textContent
    for (const s of ['define', 'plan', 'implement', 'review', 'finished']) {
      expect(labels).toContain(s)
    }
  })

  it('exposes the stage in the accessible label', () => {
    render(Pipeline, { stage: 'review', variant: 'mini' })
    expect(screen.getByRole('img').getAttribute('aria-label')).toContain('review')
  })
})
