// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const captured = {
  agents: null as ReturnType<typeof import('@/hooks/queries/useAgents').useAgents> | null,
}

vi.mock('@/lib/services/agentService', () => ({
  agentService: {
    list: vi.fn(),
  },
}))

vi.mock('@/lib/services/automationService', () => ({
  automationService: {
    listAgents: vi.fn().mockResolvedValue({
      data: [],
      error: null,
      status: 200,
    }),
  },
}))

import { agentService } from '@/lib/services/agentService'
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

  it('uses the PM agent service for PM agent metadata', async () => {
    vi.mocked(agentService.list).mockResolvedValue({
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

    expect(agentService.list).toHaveBeenCalledWith('ws-1')

    root.unmount()
    container.remove()
  })
})
