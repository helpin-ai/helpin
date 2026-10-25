// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { SupportCoverageGapDetail } from '@/lib/supportCoverageTypes'
import { GapDataSourceNotice } from '../GapDataSourceNotice'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, params, children }: { to: string; params: { slug: string }; children: React.ReactNode }) => (
    <a href={to.replace('$slug', params.slug)}>{children}</a>
  ),
}))

let container: HTMLDivElement | null = null

function render(gap: Partial<SupportCoverageGapDetail>) {
  container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  act(() => root.render(<GapDataSourceNotice gap={gap as SupportCoverageGapDetail} wsSlug="acme" />))
  return container
}

afterEach(() => {
  container?.remove()
  container = null
})

describe('GapDataSourceNotice', () => {
  it('offers to connect a tool for open data gaps', () => {
    const view = render({ gap_kind: 'data', status: 'open' })
    const link = view.querySelector('a')
    expect(view.textContent).toContain('The AI lacked the customer or account data needed to answer this.')
    expect(link?.textContent).toBe('Connect a tool')
    expect(link?.getAttribute('href')).toBe('/w/acme/settings/external-mcp')
  })

  it.each([
    { gap_kind: 'content', status: 'open' },
    { gap_kind: 'data', status: 'rejected' },
  ])('stays hidden for %o', (gap) => {
    expect(render(gap as Partial<SupportCoverageGapDetail>).textContent).toBe('')
  })
})
