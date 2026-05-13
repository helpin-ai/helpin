// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/lib/services/pmExternalLinkService', () => ({
  pmExternalLinkService: {
    listByEntity: vi.fn(),
    createForEntity: vi.fn(),
    remove: vi.fn(),
  },
}))

import { pmExternalLinkService } from '@/lib/services/pmExternalLinkService'
import { ExternalLinks } from '../ExternalLinks'
import { TooltipProvider } from '@/components/ui/tooltip'
import type { ExternalLink } from '@/lib/pmTypes'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root | null = null
let container: HTMLDivElement | null = null

function makeLink(id: string, url: string, title: string): ExternalLink {
  return {
    id,
    task_id: 'task-1',
    entity_type: 'task',
    entity_id: 'task-1',
    title,
    url,
    created_by_id: 'user-1',
    created_at: '2026-05-13T00:00:00Z',
    updated_at: '2026-05-13T00:00:00Z',
  }
}

async function renderExternalLinks(links: ExternalLink[]) {
  vi.mocked(pmExternalLinkService.listByEntity)
    .mockResolvedValueOnce({ data: links, error: null, status: 200 })

  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)

  await act(async () => {
    root?.render(
      <TooltipProvider>
        <ExternalLinks workspaceId="ws-1" entityType="task" entityId="task-1" />
      </TooltipProvider>,
    )
  })
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}

describe('ExternalLinks', () => {
  afterEach(() => {
    if (root) {
      act(() => root?.unmount())
    }
    container?.remove()
    root = null
    container = null
    vi.clearAllMocks()
  })

  it('renders saved links as visually grouped rows with a count', async () => {
    await renderExternalLinks([
      makeLink('link-1', 'https://github.com/helpin-ai/helpin/pull/31', 'Fix task sync'),
      makeLink('link-2', 'https://docs.helpin.ai/tasks/links', 'Task links doc'),
    ])

    expect(container?.textContent).toContain('2 links')
    expect(container?.textContent).toContain('Fix task sync')
    expect(container?.textContent).toContain('github.com')

    const row = container?.querySelector('[data-testid="external-link-row"]')
    expect(row?.className).toContain('bg-muted/35')
  })
})
