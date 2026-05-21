// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const captured = {
  agents: null as ReturnType<typeof import('@/hooks/queries/useAgents').useAgents> | null,
}

vi.mock('@/lib/services/automationService', () => ({
  automationService: {
    listAgents: vi.fn().mockResolvedValue({
      data: [],
      error: null,
      status: 200,
    }),
  },
}))

import { automationService } from '@/lib/services/automationService'
import { useAgents } from '@/hooks/queries/useAgents'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function Harness() {
  captured.agents = useAgents('ws-1')
  return null
}

describe('useAgents', () => {
  afterEach(() => {
    captured.agents = null
    vi.clearAllMocks()
  })

  it('uses the filtered automation agent service for run pickers', async () => {
    vi.mocked(automationService.listAgents).mockResolvedValue({
      data: [],
      error: null,
      status: 200,
    } as never)

    const client = new QueryClient()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      await captured.agents?.refetch()
    })

    expect(automationService.listAgents).toHaveBeenCalledWith('ws-1')

    await act(async () => {
      root.unmount()
    })
    container.remove()
  })
})
