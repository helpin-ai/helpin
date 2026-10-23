import { describe, expect, it } from 'vitest';

import { inboxClearNames, inboxClearScopeFor, isOwnInboxClear } from '../inboxClearedScope';
import { hiddenDuringHandoff } from '../../threadHandoff';

const mailboxId = '8f0e1c2a-3b4d-4e5f-8a9b-0c1d2e3f4a5b';

describe('inboxClearScopeFor', () => {
  it('maps inbox views to the scope the server records', () => {
    expect(inboxClearScopeFor('inbox', 'all')).toEqual({ scope: 'all', mailboxId: null, scopeKey: 'all' });
    expect(inboxClearScopeFor('inbox', mailboxId)).toEqual({ scope: 'mailbox', mailboxId, scopeKey: `mailbox:${mailboxId}` });
    expect(inboxClearScopeFor('mine', 'all')).toEqual({ scope: 'mine', mailboxId: null, scopeKey: 'mine' });
    expect(inboxClearScopeFor('inbox', 'shared')).toEqual({ scope: 'all', mailboxId: null, scopeKey: 'mailbox:shared' });
  });

  it('ignores views that are not an inbox to clear', () => {
    for (const view of ['waiting', 'resolved', 'spam', 'ai_active', 'resolved_by_ai'] as const) {
      expect(inboxClearScopeFor(view, 'all')).toBeNull();
    }
  });
});

describe('inboxClearNames', () => {
  it('names team inboxes, the whole inbox and the personal queue', () => {
    expect(inboxClearNames({ scope: 'mailbox', mailboxId, scopeKey: 'x' }, 'Billing')).toEqual({ inbox: 'Billing', inboxInSentence: 'Billing' });
    expect(inboxClearNames({ scope: 'all', mailboxId: null, scopeKey: 'all' }, null)).toEqual({ inbox: 'Your inbox', inboxInSentence: 'your inbox' });
    expect(inboxClearNames({ scope: 'mine', mailboxId: null, scopeKey: 'mine' }, null)).toEqual({ inbox: 'Your queue', inboxInSentence: 'your queue' });
  });
});

describe('isOwnInboxClear', () => {
  const now = 1_000_000;

  it('fires when every conversation that left was removed by the viewer', () => {
    expect(isOwnInboxClear({ previousIds: ['a', 'b'], currentIds: [], ownRemovals: { a: now - 1000, b: now - 500 }, now })).toBe(true);
  });

  it('stays quiet when the AI or a teammate removed one of them', () => {
    expect(isOwnInboxClear({ previousIds: ['a', 'b'], currentIds: [], ownRemovals: { a: now }, now })).toBe(false);
  });

  it('stays quiet when the list is not empty, was already empty, or the action is old', () => {
    expect(isOwnInboxClear({ previousIds: ['a'], currentIds: ['c'], ownRemovals: { a: now }, now })).toBe(false);
    expect(isOwnInboxClear({ previousIds: [], currentIds: [], ownRemovals: {}, now })).toBe(false);
    expect(isOwnInboxClear({ previousIds: ['a'], currentIds: [], ownRemovals: { a: now - 31_000 }, now })).toBe(false);
  });
});

describe('hiddenDuringHandoff', () => {
  const now = Date.parse('2026-10-23T12:00:00Z');

  it('hides only the new resolved notice while the thread hands off', () => {
    const fresh = { system_event_type: 'resolved', created_at: '2026-10-23T11:59:59Z' } as const;
    const old = { system_event_type: 'resolved', created_at: '2026-10-20T09:00:00Z' } as const;
    expect(hiddenDuringHandoff(fresh, true, now)).toBe(true);
    expect(hiddenDuringHandoff(fresh, false, now)).toBe(false);
    expect(hiddenDuringHandoff(old, true, now)).toBe(false);
    expect(hiddenDuringHandoff({ system_event_type: 'assigned', created_at: '2026-10-23T11:59:59Z' }, true, now)).toBe(false);
  });
});
