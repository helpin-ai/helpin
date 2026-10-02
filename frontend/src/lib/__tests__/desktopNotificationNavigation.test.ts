import { describe, expect, it } from 'vitest'
import { desktopNotificationTarget } from '../desktopNotificationNavigation'
import type { WSEvent } from '@/hooks/useWebSocket'
const event = (data: Record<string, unknown>) => ({ data } as WSEvent)
describe('desktop alert navigation', () => {
  it('opens the exact conversation, task, document, and integration settings', () => {
    expect(desktopNotificationTarget(event({ entity_type: 'support_conversation', entity_id: 'c1' }), 'acme')).toEqual({ path: '/w/acme/support/c1' })
    expect(desktopNotificationTarget(event({ entity_type: 'task', entity_id: 't1', run_id: 'r1' }), 'acme')).toEqual({ path: '/w/acme/pm/tasks/t1', search: { run: 'r1' } })
    expect(desktopNotificationTarget(event({ entity_type: 'doc', entity_id: 'd1' }), 'acme')).toEqual({ path: '/w/acme/docs/documents/d1' })
    expect(desktopNotificationTarget(event({ entity_type: 'external_mcp_server', entity_id: 's1' }), 'acme')).toEqual({ path: '/w/acme/settings/external-mcp' })
  })
  it('preserves dock and task-run routing', () => {
    expect(desktopNotificationTarget(event({ entity_type: 'agent_run', entity_id: 'r1', dock_chat_id: 'chat1', parent_task_id: 't1' }), 'acme')).toEqual({ dock: { chatId: 'chat1' } })
    expect(desktopNotificationTarget(event({ entity_type: 'agent_run', entity_id: 'r1', parent_task_id: 't1' }), 'acme')).toEqual({ path: '/w/acme/pm/tasks/t1', search: { run: 'r1' } })
    expect(desktopNotificationTarget(event({ entity_type: 'agent_run', entity_id: 'r1' }), 'acme')).toEqual({ dock: { runId: 'r1' } })
  })
  it('falls back to the inbox without using untrusted URLs', () => {
    expect(desktopNotificationTarget(event({ entity_type: 'unknown', url: 'https://example.com' }), 'acme')).toEqual({ path: '/w/acme/notifications' })
  })
})
