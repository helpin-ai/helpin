import { describe, expect, it } from 'vitest';
import type { Notification } from '../notificationTypes';
import { getExternalMCPNotificationTarget, getNotificationDockTarget } from '../notificationNavigation';

function notification(entityType: string): Notification {
  return {
    id: 'notification-1',
    workspace_id: 'workspace-1',
    recipient_id: 'user-1',
    entity_type: entityType,
    entity_id: 'server-1',
    event_type: 'external_mcp.reauthorization_required',
    title: 'Reconnect Customer.io',
    metadata: {},
    latest_event_category: 'status_changes',
    actor_snapshot: {},
    entity_snapshot: {},
    event_count: 1,
    last_event_at: '2026-08-01T00:00:00Z',
    status: 'unread',
    priority: 'high',
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-01T00:00:00Z',
  };
}

describe('external MCP notification navigation', () => {
  it('opens External MCP workspace settings', () => {
    expect(getExternalMCPNotificationTarget(notification('external_mcp_server'), 'acme'))
      .toBe('/w/acme/settings/external-mcp');
  });

  it('ignores unrelated notification entities', () => {
    expect(getExternalMCPNotificationTarget(notification('task'), 'acme')).toBeNull();
  });
});

describe('agent attention notification navigation', () => {
  it('opens the chat instead of its task or backing run', () => {
    expect(getNotificationDockTarget({ ...notification('agent_run'), metadata: { dock_chat_id: 'chat-1', run_id: 'run-1', task_id: 'task-1' } })).toEqual({ chatId: 'chat-1' });
  });
  it('keeps task agent notifications on the task', () => {
    expect(getNotificationDockTarget({ ...notification('agent_run'), metadata: { task_id: 'task-1' } })).toBeNull();
  });
  it('opens standalone runs in the dock', () => {
    expect(getNotificationDockTarget(notification('agent_run'))).toEqual({ runId: 'server-1' });
  });
  it('ignores non-run notifications even with chat metadata', () => {
    expect(getNotificationDockTarget({ ...notification('task'), metadata: { dock_chat_id: 'chat-1' } })).toBeNull();
  });
});
