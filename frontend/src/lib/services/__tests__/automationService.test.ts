import { afterEach, describe, expect, it, vi } from 'vitest'

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  api: {
    get: getMock,
    post: vi.fn(),
    put: vi.fn(),
    patch: vi.fn(),
    del: vi.fn(),
  },
  API_BASE: 'http://localhost:8080/api',
}))

import { automationService } from '@/lib/services/automationService'

describe('automationService.getAgentFleet', () => {
  afterEach(() => {
    getMock.mockReset()
  })

  it('uses the non-UUID fleet route and forwards cancellation', async () => {
    getMock.mockResolvedValueOnce({
      data: { generated_at: '', window_started_at: '', agents: [] },
      error: null,
      status: 200,
    })
    const controller = new AbortController()

    await automationService.getAgentFleet('ws-1', controller.signal)

    expect(getMock).toHaveBeenCalledWith(
      '/automation/agent-fleet?workspace_id=ws-1',
      { signal: controller.signal },
    )
  })
})
