import { describe, expect, it } from 'vitest';
import { shouldClearConversationForMailbox } from '../supportInboxSelection';

describe('shouldClearConversationForMailbox', () => {
  it('keeps conversations in aggregate inbox views', () => {
    expect(shouldClearConversationForMailbox('all', 'mailbox-other')).toBe(false);
  });

  it('keeps a conversation that belongs to the selected team inbox', () => {
    expect(shouldClearConversationForMailbox('mailbox-admin', 'mailbox-admin')).toBe(false);
  });

  it('clears a conversation from another inbox or with no mailbox', () => {
    expect(shouldClearConversationForMailbox('mailbox-admin', 'mailbox-other')).toBe(true);
    expect(shouldClearConversationForMailbox('mailbox-admin', null)).toBe(true);
  });
});
