import { describe, expect, it } from 'vitest';

import {
  AUTOMATED_ROUTING_DESCRIPTION,
  CONFIGURE_ROUTING_LINK_CLASS,
  DEFAULT_EXPANDED_ROUTING_SECTIONS,
  getDisabledRoutingWarning,
  getEmailForwardingStatus,
  getRoutingTooltipLines,
  getSharedEmailRoute,
  getWorkspaceDefaultSender,
  HANDOFF_ASSIGNMENT_DESCRIPTION,
  shouldShowAutomatedRoutingContent,
} from '../ConversationRoutingTab';

describe('conversation routing status helpers', () => {
  it('warns when configured AI routing is blocked by global automated routing', () => {
    expect(getDisabledRoutingWarning('AI routing', true, false)).toBe(
      'AI routing is configured for this inbox, but it is not running because Automated routing is off globally.',
    );
  });

  it('warns when configured rule-based routing is blocked by global automated routing', () => {
    expect(getDisabledRoutingWarning('Rule-based routing', true, false)).toBe(
      'Rule-based routing is configured for this inbox, but it is not running because Automated routing is off globally.',
    );
  });

  it('does not warn when routing is not configured or global automated routing is enabled', () => {
    expect(getDisabledRoutingWarning('AI routing', false, false)).toBeNull();
    expect(getDisabledRoutingWarning('AI routing', true, true)).toBeNull();
  });

  it('styles configure links differently from primary action links', () => {
    expect(CONFIGURE_ROUTING_LINK_CLASS).toContain('text-amber');
    expect(CONFIGURE_ROUTING_LINK_CLASS).not.toContain('text-primary');
  });

  it('keeps routing and assignment section descriptions concise', () => {
    expect(AUTOMATED_ROUTING_DESCRIPTION).toBe(
      'Routing decides which inbox a conversation moves to. Automated routing checks manual rules first, then uses AI when no rule matches.',
    );
    expect(HANDOFF_ASSIGNMENT_DESCRIPTION).toBe(
      'Assignment decides which teammate owns a handoff. Choose where AI handoffs go and whether a teammate is assigned automatically.',
    );
  });

  it('opens routing sections by default', () => {
    expect(DEFAULT_EXPANDED_ROUTING_SECTIONS).toEqual(['automated-routing', 'handoff-assignment']);
  });

  it('shows only the warning in status tooltips when routing is globally disabled', () => {
    const warning = getDisabledRoutingWarning('AI routing', true, false);

    expect(getRoutingTooltipLines(warning, ['Billing prompt'])).toEqual([warning]);
  });

  it('shows normal tooltip detail when there is no disabled-routing warning', () => {
    expect(getRoutingTooltipLines(null, ['Text contains: refund'])).toEqual(['Text contains: refund']);
  });

  it('hides automated routing content while automated routing is off', () => {
    expect(shouldShowAutomatedRoutingContent(false)).toBe(false);
    expect(shouldShowAutomatedRoutingContent(true)).toBe(true);
  });

  it('selects the shared inbox forwarding route', () => {
    expect(getSharedEmailRoute([
      { id: 'team-route', mailbox_id: 'mailbox-1', active: true },
      { id: 'shared-route', mailbox_id: null, active: true },
    ])?.id).toBe('shared-route');
  });

  it('selects the workspace default sender for the shared inbox row', () => {
    expect(getWorkspaceDefaultSender([
      { id: 'mailbox-sender', default_scope: 'mailbox', active: true },
      { id: 'workspace-sender', default_scope: 'workspace', active: true },
    ])?.id).toBe('workspace-sender');
  });

  it('shows awaiting email for active forwarding before the first inbound email', () => {
    expect(getEmailForwardingStatus({ active: true, last_inbound_at: null })).toEqual({
      label: 'Awaiting email',
      tone: 'warning',
      tooltip: 'Forwarding is enabled. Send or forward a test email to finish verification.',
    });
  });

  it('shows on for forwarding after inbound email has been received', () => {
    expect(getEmailForwardingStatus({ active: true, last_inbound_at: '2026-06-16T12:00:00Z' })).toEqual({
      label: 'On',
      tone: 'success',
      tooltip: null,
    });
  });
});
