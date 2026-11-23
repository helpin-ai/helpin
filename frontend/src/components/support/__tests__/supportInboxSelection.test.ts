import { describe, expect, it } from 'vitest';
import { mailboxContextForRoutedConversation, shouldClearConversationForMailbox } from '../supportInboxSelection';

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

describe('mailboxContextForRoutedConversation', () => {
  const picker = [
    { id: 'billing', active: true },
    { id: 'sales', active: true },
    { id: 'archived', active: false },
  ];

  it('leaves the context alone when it already contains the conversation', () => {
    expect(mailboxContextForRoutedConversation('all', 'sales', picker)).toBeNull();
    expect(mailboxContextForRoutedConversation('sales', 'sales', picker)).toBeNull();
  });

  it('follows a conversation that moved to another selectable inbox', () => {
    expect(mailboxContextForRoutedConversation('billing', 'sales', picker)).toBe('sales');
  });

  it('falls back to the general inbox when the destination is not selectable', () => {
    expect(mailboxContextForRoutedConversation('billing', 'hidden', picker)).toBe('all');
    expect(mailboxContextForRoutedConversation('billing', 'archived', picker)).toBe('all');
    expect(mailboxContextForRoutedConversation('billing', null, picker)).toBe('all');
    expect(mailboxContextForRoutedConversation('billing', 'sales', [])).toBe('all');
  });
});
