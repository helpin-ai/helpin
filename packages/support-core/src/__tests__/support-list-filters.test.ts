import { describe, it, expect, beforeEach, vi } from 'vitest'
import { configureSupportApi, supportService } from '../support-service'

function makeFakeApi() {
  return { get: vi.fn().mockResolvedValue({ data: { data: [] }, error: null }), post: vi.fn(), put: vi.fn() }
}

describe('listConversations serializes the full parity param set', () => {
  let api: ReturnType<typeof makeFakeApi>
  beforeEach(() => {
    api = makeFakeApi()
    configureSupportApi(api)
  })

  it('forwards every web filter field with the web/server param name', async () => {
    await supportService.listConversations('ws-1', {
      status: 'spam',
      statuses: 'open,waiting_on_customer',
      filter: 'inbox',
      mailbox_id: 'shared',
      mailbox_ids: 'm1,m2',
      ai: 'ai_active',
      flow_state: 'x',
      search: 'hello world',
      assigned_to: 'me',
      sort: 'oldest',
      tag_ids: 't1,t2',
    })
    const path = api.get.mock.calls[0][0] as string
    for (const frag of [
      'status=spam',
      'statuses=open%2Cwaiting_on_customer',
      'filter=inbox',
      'mailbox_id=shared',
      'mailbox_ids=m1%2Cm2',
      'ai=ai_active',
      'flow_state=x',
      'search=hello%20world',
      'assigned_to=me',
      'sort=oldest',
      'tag_ids=t1%2Ct2',
    ]) {
      expect(path).toContain(frag)
    }
  })

  it('omits empty/undefined fields and drops mailbox_id="all"', async () => {
    await supportService.listConversations('ws-1', { filter: 'mine', mailbox_id: 'all', status: '' })
    const path = api.get.mock.calls[0][0] as string
    expect(path).toContain('filter=mine')
    expect(path).not.toContain('mailbox_id=')
    expect(path).not.toContain('status=')
  })
})
