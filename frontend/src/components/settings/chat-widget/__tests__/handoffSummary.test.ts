import { describe, expect, it } from 'vitest';
import {
  buildChatWidgetHandoffSummary,
  CHAT_WIDGET_ROUTING_ASSIGNMENT_DESCRIPTION,
} from '../handoffSummary';

describe('buildChatWidgetHandoffSummary', () => {
  it('summarizes leave-unassigned handoffs', () => {
    expect(buildChatWidgetHandoffSummary({
      mailboxName: 'Shared Inbox',
      behavior: 'unassigned',
    })).toBe('When AI hands off to a human, the conversation moves to Shared Inbox and remains unassigned.');
  });

  it('summarizes round-robin handoffs', () => {
    expect(buildChatWidgetHandoffSummary({
      mailboxName: 'Support',
      behavior: 'round_robin',
    })).toBe('When AI hands off to a human, the conversation moves to Support and is assigned by round robin.');
  });

  it('summarizes team handoffs', () => {
    expect(buildChatWidgetHandoffSummary({
      mailboxName: 'Support',
      behavior: 'assign_to_team',
      teamName: 'Billing',
    })).toBe('When AI hands off to a human, the conversation moves to Support and is assigned to Billing.');
  });

  it('explains that new widget conversations start shared', () => {
    expect(CHAT_WIDGET_ROUTING_ASSIGNMENT_DESCRIPTION).toBe(
      'New widget conversations start in Shared Inbox. Configure human handoff ownership in Routing & Assignment.',
    );
  });
});
