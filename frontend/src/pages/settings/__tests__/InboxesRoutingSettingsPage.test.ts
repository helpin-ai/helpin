import { describe, expect, it } from 'vitest';
import { INBOXES_ROUTING_TABS, normalizeInboxesRoutingTab } from '../InboxesRoutingSettingsPage';

describe('InboxesRoutingSettingsPage tabs', () => {
  it('splits inbox management from routing and assignment', () => {
    expect(INBOXES_ROUTING_TABS.map((tab) => tab.label)).toEqual([
      'Inboxes',
      'Routing & Assignment',
      'Email Forwarding',
      'Sender Addresses',
    ]);
  });

  it('keeps existing tab links valid and maps routing intent to the new tab', () => {
    expect(normalizeInboxesRoutingTab('inboxes')).toBe('inboxes');
    expect(normalizeInboxesRoutingTab('routing')).toBe('routing');
    expect(normalizeInboxesRoutingTab('routing-assignment')).toBe('routing');
    expect(normalizeInboxesRoutingTab('email')).toBe('email');
    expect(normalizeInboxesRoutingTab('senders')).toBe('senders');
    expect(normalizeInboxesRoutingTab('unknown')).toBe('inboxes');
    expect(normalizeInboxesRoutingTab(undefined)).toBe('inboxes');
  });
});
