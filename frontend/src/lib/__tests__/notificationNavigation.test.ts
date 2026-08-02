import { describe, expect, it } from 'vitest';
import type { Notification } from '../notificationTypes';
import { getExternalMCPNotificationTarget } from '../notificationNavigation';

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
  it('opens Automation tool connections', () => {
    expect(getExternalMCPNotificationTarget(notification('external_mcp_server'), 'acme'))
      .toBe('/w/acme/automation/tools/connections');
  });

  it('ignores unrelated notification entities', () => {
    expect(getExternalMCPNotificationTarget(notification('task'), 'acme')).toBeNull();
  });
});
