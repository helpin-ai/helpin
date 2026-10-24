import { describe, expect, it } from 'vitest';

import type { SupportEmailRoute, SupportEmailSender } from '@/lib/pmTypes';
import { getInboxForwardingRoutes, getInboxSenderAddress, isSenderVerified } from '../teamInboxEmailSetup';

function route(overrides: Partial<SupportEmailRoute>): SupportEmailRoute {
  return {
    id: 'route-1',
    workspace_id: 'ws-1',
    mailbox_id: 'mb-1',
    route_key: 'key',
    inbound_address: 'key@in.helpin.ai',
    provider_type: 'forwarding',
    active: true,
    created_by_id: 'user-1',
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    ...overrides,
  };
}

function sender(overrides: Partial<SupportEmailSender>): SupportEmailSender {
  return {
    id: 'sender-1',
    workspace_id: 'ws-1',
    email: 'support@acme.com',
    local_part: 'support',
    domain: 'acme.com',
    display_name: 'Acme Support',
    return_path_domain: '',
    return_path_domain_cname_value: '',
    return_path_domain_verified: true,
    dkim_host: '',
    dkim_text_value: '',
    dkim_pending_host: '',
    dkim_pending_text_value: '',
    dkim_verified: true,
    dkim_update_status: '',
    dmarc_host: '',
    dmarc_policy: '',
    dmarc_record_present: false,
    domain_status: 'verified',
    forwarding_status: 'not_started',
    forwarding_address: '',
    verification_status: 'verified',
    default_scope: 'none',
    active: true,
    created_by_id: 'user-1',
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    ...overrides,
  };
}

describe('team inbox email setup', () => {
  it('lists only active forwarding routes bound to the inbox', () => {
    const routes = [
      route({ id: 'inbox' }),
      route({ id: 'inactive', active: false }),
      route({ id: 'shared', mailbox_id: null }),
      route({ id: 'other', mailbox_id: 'mb-2' }),
    ];

    expect(getInboxForwardingRoutes(routes, 'mb-1').map((item) => item.id)).toEqual(['inbox']);
    expect(getInboxForwardingRoutes(routes, undefined)).toEqual([]);
  });

  it('prefers the inbox sender and falls back to the workspace default', () => {
    const workspaceDefault = sender({ id: 'workspace', default_scope: 'workspace' });
    const inboxSender = sender({ id: 'inbox', default_scope: 'mailbox', mailbox_ids: ['mb-1'] });

    expect(getInboxSenderAddress([workspaceDefault, inboxSender], 'mb-1')).toEqual({ sender: inboxSender, source: 'inbox' });
    expect(getInboxSenderAddress([workspaceDefault, inboxSender], 'mb-2')).toEqual({ sender: workspaceDefault, source: 'workspace' });
    expect(getInboxSenderAddress([workspaceDefault], undefined)).toEqual({ sender: workspaceDefault, source: 'workspace' });
    expect(getInboxSenderAddress([sender({ default_scope: 'workspace', active: false })], 'mb-1')).toBeNull();
  });

  it('treats a sender as verified only when DKIM and return path both pass', () => {
    expect(isSenderVerified({ dkim_verified: true, return_path_domain_verified: true })).toBe(true);
    expect(isSenderVerified({ dkim_verified: true, return_path_domain_verified: false })).toBe(false);
  });
});
