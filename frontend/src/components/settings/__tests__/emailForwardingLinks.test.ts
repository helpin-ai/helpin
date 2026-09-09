import { describe, expect, it } from 'vitest';
import { emailForwardingInboxHref } from '../emailForwardingLinks';
import { normalizeSupportInboxRouteSearch } from '@/lib/supportInboxRouting';

describe('forwarding inbox links', () => {
  it('selects the Team Inbox while opening the received confirmation', () => {
    const url = new URL(emailForwardingInboxHref('contentstudio', 'team-123', 'confirmation-456')!, 'https://app.helpin.ai');
    expect(url.pathname).toBe('/w/contentstudio/support/confirmation-456');
    const search = normalizeSupportInboxRouteSearch(Object.fromEntries(url.searchParams));
    expect(search.team_inbox).toBe('team-123');
    expect(search.mailbox_ids).toBeUndefined();
  });
  it('opens the same Team Inbox when no confirmation is available', () => {
    expect(emailForwardingInboxHref('workspace', 'team')).toBe('/w/workspace/support?view=team&team_inbox=team');
  });
  it('opens Shared Inbox explicitly and avoids stale saved filters for a confirmation', () => {
    const url = new URL(emailForwardingInboxHref('workspace', null, 'confirmation')!, 'https://app.helpin.ai');
    expect(url.searchParams.get('view')).toBe('inbox');
    expect(url.searchParams.get('states')).toBe('open,waiting_on_customer,resolved,spam');
    expect(url.searchParams.has('team_inbox')).toBe(false);
    expect(emailForwardingInboxHref(undefined, 'team')).toBeNull();
  });
});
