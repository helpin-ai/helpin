// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/lib/services/automationService', () => ({
  automationService: {
    getAgentFleet: vi.fn().mockResolvedValue({
      data: { generated_at: '', window_started_at: '', agents: [] },
      error: null,
      status: 200,
    }),
  },
}))

import { automationService } from '@/lib/services/automationService'
import { useAutomationAgentFleet } from '@/hooks/queries/useAutomation'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function Harness() {
  const fleet = useAutomationAgentFleet('ws-1')
  return <span>{fleet.status}</span>
}

describe('useAutomationAgentFleet', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('loads the fleet through one abortable request', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
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
      await vi.waitFor(() => {
        expect(container.textContent).toBe('success')
      })
    })

    expect(automationService.getAgentFleet).toHaveBeenCalledTimes(1)
    expect(automationService.getAgentFleet).toHaveBeenCalledWith('ws-1', expect.any(AbortSignal))

    await act(async () => {
      root.unmount()
    })
    container.remove()
  })
})
