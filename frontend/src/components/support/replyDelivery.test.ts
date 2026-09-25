import { describe, expect, it } from 'vitest';
import { nextAutomaticReplyDelivery } from './replyDelivery';

describe('automatic reply delivery', () => {
  it('starts with chat while presence is unknown or online', () => {
    for (const presence of ['unknown', 'online'] as const) {
      expect(nextAutomaticReplyDelivery(undefined, { source: 'widget', presence, emailEligible: true })).toBe('chat_only');
    }
  });
  it('adds email when an eligible widget visitor is confirmed offline', () => {
    expect(nextAutomaticReplyDelivery('chat_only', { source: 'widget', presence: 'offline', emailEligible: true })).toBe('chat_and_email');
  });
  it('keeps promised email when presence or recipient eligibility changes', () => {
    expect(nextAutomaticReplyDelivery('chat_and_email', { source: 'widget', presence: 'online', emailEligible: false })).toBe('chat_and_email');
  });
  it('does not add unavailable email', () => {
    expect(nextAutomaticReplyDelivery(undefined, { source: 'widget', presence: 'offline', emailEligible: false })).toBe('chat_only');
  });
  it('starts email-origin conversations with email even when sending needs recipient confirmation', () => {
    expect(nextAutomaticReplyDelivery(undefined, { source: 'email', presence: 'unknown', emailEligible: false })).toBe('email_only');
  });
  it('restarts an automatic reply from current presence after clearing its draft choice', () => {
    expect(nextAutomaticReplyDelivery(undefined, { source: 'widget', presence: 'online', emailEligible: true })).toBe('chat_only');
  });
});
