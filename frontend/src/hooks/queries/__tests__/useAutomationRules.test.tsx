// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const captured = {
  rules: null as ReturnType<typeof import('@/hooks/queries/useAutomationRules').useAutomationRulesByWorkflow> | null,
}

vi.mock('@/lib/services/automationRuleService', () => ({
  automationRuleService: {
    listByWorkflow: vi.fn(),
  },
}))

vi.mock('@/lib/services/automationService', () => ({
  automationService: {
    listFlowsByWorkflow: vi.fn().mockResolvedValue({
      data: [],
      error: null,
      status: 200,
    }),
  },
}))

import { automationRuleService } from '@/lib/services/automationRuleService'
import { useAutomationRulesByWorkflow } from '@/hooks/queries/useAutomationRules'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function Harness() {
  captured.rules = useAutomationRulesByWorkflow('ws-1', 'wf-1')
  return null
}

describe('useAutomationRulesByWorkflow', () => {
  afterEach(() => {
    captured.rules = null
    vi.clearAllMocks()
  })

  it('uses the PM automation-rule service for PM workflow metadata', async () => {
    vi.mocked(automationRuleService.listByWorkflow).mockResolvedValue({
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
      await captured.rules?.refetch()
    })

    expect(automationRuleService.listByWorkflow).toHaveBeenCalledWith('ws-1', 'wf-1')

    await act(async () => {
      root.unmount()
    })
    container.remove()
  })
})
