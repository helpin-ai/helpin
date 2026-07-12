import { describe, it, expect, beforeEach, vi } from 'vitest'
import { configureSupportApi, supportService } from '../support-service'

function makeFakeApi() {
  return { get: vi.fn().mockResolvedValue({ data: [], error: null }), post: vi.fn(), put: vi.fn(), del: vi.fn() }
}

describe('support inbox view endpoints', () => {
  let api: ReturnType<typeof makeFakeApi>
  beforeEach(() => {
    api = makeFakeApi()
    configureSupportApi(api)
  })

  it('hits the right paths for counts, builtin views and custom views', async () => {
    await supportService.listInboxViewCounts('ws-1')
    await supportService.listBuiltinInboxViews('ws-1')
    await supportService.listInboxViews('ws-1')
    const paths = api.get.mock.calls.map((c) => c[0] as string)
    expect(paths[0]).toBe('/support/inbox/views/counts?workspace_id=ws-1')
    expect(paths[1]).toBe('/support/inbox/views/builtin?workspace_id=ws-1')
    expect(paths[2]).toBe('/support/inbox/views?workspace_id=ws-1')
  })
})
