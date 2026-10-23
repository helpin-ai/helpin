import { describe, expect, it } from 'vitest';

import {
  composeInboxClearedMessage,
  shouldThrowConfetti,
  INBOX_CLEARED_RECENT_LIMIT,
  type InboxClearedContext,
  type InboxClearedMemory,
  type InboxClearedStats,
} from '../inboxClearedMessages';

const quietStats: InboxClearedStats = { streak_days: 3, clears_today: 1, resolved_today: 12, cleared_count: 5 };

function context(overrides: Partial<InboxClearedContext> = {}): InboxClearedContext {
  return { inbox: 'Billing', inboxInSentence: 'Billing', hour: 15, day: 3, stats: quietStats, ...overrides };
}

const empty = (): InboxClearedMemory => ({ recent: [], unlocked: {} });
const options = { scopeKey: 'mailbox:billing', localDate: '2026-10-23' };

describe('composeInboxClearedMessage', () => {
  it('uses an everyday clear without numbers when nothing special fits', () => {
    for (let i = 0; i < 200; i++) {
      const { message } = composeInboxClearedMessage(context(), empty(), options);
      expect(message.rarity).toBe('common');
      expect(message.title).not.toMatch(/\d/);
      expect(message.body).not.toMatch(/\d/);
      expect(message.title.toLowerCase().startsWith(message.name.toLowerCase())).toBe(false);
      expect(message.title).not.toMatch(/[{}]/);
      expect(message.body).not.toMatch(/[{}]/);
    }
  });

  it('picks the time-of-day achievement', () => {
    const { message } = composeInboxClearedMessage(context({ hour: 8 }), empty(), options);
    expect(message).toMatchObject({ achievementId: 'early-bird', rarity: 'rare', name: 'Early bird' });
  });

  it('lets the rarest achievement win and shows its number', () => {
    const { message } = composeInboxClearedMessage(
      context({ hour: 8, stats: { ...quietStats, resolved_today: 54 } }),
      empty(),
      options,
    );
    expect(message).toMatchObject({ achievementId: 'marathon', rarity: 'legendary' });
    expect(`${message.title} ${message.body}`).toContain('54');
  });

  it('unlocks an achievement once a day per inbox', () => {
    const first = composeInboxClearedMessage(context({ hour: 8 }), empty(), options);
    const again = composeInboxClearedMessage(context({ hour: 8 }), first.memory, options);
    const otherInbox = composeInboxClearedMessage(context({ hour: 8 }), first.memory, { ...options, scopeKey: 'all' });
    const nextDay = composeInboxClearedMessage(context({ hour: 8 }), first.memory, { ...options, localDate: '2026-10-24' });

    expect(again.message.rarity).toBe('common');
    expect(otherInbox.message.achievementId).toBe('early-bird');
    expect(nextDay.message.achievementId).toBe('early-bird');
  });

  it('counts the clears of the day in words', () => {
    const { message } = composeInboxClearedMessage(context({ stats: { ...quietStats, clears_today: 2 } }), empty(), options);
    expect(message.achievementId).toBe('encore');
    expect(`${message.title} ${message.body}`).toMatch(/second/i);
  });

  it('does not repeat recent wording', () => {
    let memory = empty();
    const titles = new Set<string>();
    for (let i = 0; i < 5; i++) {
      const result = composeInboxClearedMessage(context(), memory, options);
      titles.add(result.message.title);
      memory = result.memory;
    }
    expect(titles.size).toBe(5);
    expect(memory.recent.length).toBeLessThanOrEqual(INBOX_CLEARED_RECENT_LIMIT);
  });

  it('celebrates with the time of day alone when the server numbers are missing', () => {
    const { message } = composeInboxClearedMessage(context({ hour: 23, stats: null }), empty(), options);
    expect(message.achievementId).toBe('night-owl');
  });

  it('keeps confetti for clears that mean something', () => {
    const everyday = { rarity: 'common' as const };
    expect(shouldThrowConfetti(everyday, { ...quietStats, clears_today: 1 }, 1)).toBe(true);
    expect(shouldThrowConfetti({ rarity: 'rare' }, { ...quietStats, clears_today: 4 }, 1)).toBe(true);
    expect(shouldThrowConfetti(everyday, { ...quietStats, clears_today: 4 }, 5)).toBe(true);
    expect(shouldThrowConfetti(everyday, { ...quietStats, clears_today: 4 }, 2)).toBe(false);
    expect(shouldThrowConfetti(everyday, null, 2)).toBe(false);
  });

  it('forgets unlocks from other days', () => {
    const stale: InboxClearedMemory = { recent: [], unlocked: { 'all:early-bird': '2026-10-01' } };
    const { memory } = composeInboxClearedMessage(context(), stale, options);
    expect(memory.unlocked).toEqual({});
  });
});
