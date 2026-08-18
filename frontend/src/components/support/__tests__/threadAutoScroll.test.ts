import { describe, expect, it } from 'vitest';
import { getInitialThreadScrollTarget, getPrependRestoredScrollTop, isNearThreadBottom, isNearThreadTop, shouldAutoScrollThread, shouldMarkOpenThreadRead } from '../threadAutoScroll';

describe('thread auto-scroll helpers', () => {
  describe('isNearThreadBottom', () => {
    it('treats a viewport within the threshold as near the bottom', () => {
      expect(isNearThreadBottom({ scrollHeight: 1000, scrollTop: 620, clientHeight: 300 }, 96)).toBe(true);
    });

    it('treats a viewport beyond the threshold as away from the bottom', () => {
      expect(isNearThreadBottom({ scrollHeight: 1000, scrollTop: 560, clientHeight: 300 }, 96)).toBe(false);
    });
  });

  describe('older history loading', () => {
    it('detects the top loading threshold', () => {
      expect(isNearThreadTop({ scrollTop: 40 }, 64)).toBe(true);
      expect(isNearThreadTop({ scrollTop: 65 }, 64)).toBe(false);
    });

    it('preserves the visible position after messages are prepended', () => {
      expect(getPrependRestoredScrollTop({ previousScrollHeight: 800, nextScrollHeight: 1250, previousScrollTop: 20 })).toBe(470);
    });
  });

  describe('shouldAutoScrollThread', () => {
    it('scrolls when opening a different conversation', () => {
      expect(
        shouldAutoScrollThread({
          conversationChanged: true,
          initialLoad: false,
          messageCountIncreased: false,
          wasNearBottom: false,
          pendingInitialScroll: false,
        }),
      ).toBe(true);
    });

    it('scrolls when messages first load for the selected conversation', () => {
      expect(
        shouldAutoScrollThread({
          conversationChanged: false,
          initialLoad: true,
          messageCountIncreased: false,
          wasNearBottom: false,
          pendingInitialScroll: false,
        }),
      ).toBe(true);
    });

    it('keeps following new messages when the agent is already near the bottom', () => {
      expect(
        shouldAutoScrollThread({
          conversationChanged: false,
          initialLoad: false,
          messageCountIncreased: true,
          wasNearBottom: true,
          pendingInitialScroll: false,
        }),
      ).toBe(true);
    });

    it('does not yank the thread down when the agent is reading older messages', () => {
      expect(
        shouldAutoScrollThread({
          conversationChanged: false,
          initialLoad: false,
          messageCountIncreased: true,
          wasNearBottom: false,
          pendingInitialScroll: false,
        }),
      ).toBe(false);
    });

    it('continues the initial scroll while delayed history is hydrating', () => {
      expect(
        shouldAutoScrollThread({
          conversationChanged: false,
          initialLoad: false,
          messageCountIncreased: false,
          wasNearBottom: false,
          pendingInitialScroll: true,
        }),
      ).toBe(true);
    });
  });

  describe('getInitialThreadScrollTarget', () => {
    const base = '2026-06-03T10:00:00.000Z';

    it('targets the first unread customer reply when the team has a read cursor', () => {
      expect(
        getInitialThreadScrollTarget(
          [
            { id: 'read-customer', sender_type: 'customer', message_type: 'reply', is_internal: false, created_at: base },
            { id: 'agent', sender_type: 'user', message_type: 'reply', is_internal: false, created_at: '2026-06-03T10:01:00.000Z' },
            { id: 'first-unread', sender_type: 'customer', message_type: 'reply', is_internal: false, created_at: '2026-06-03T10:02:00.000Z' },
            { id: 'second-unread', sender_type: 'customer', message_type: 'reply', is_internal: false, created_at: '2026-06-03T10:03:00.000Z' },
          ],
          '2026-06-03T10:01:30.000Z',
        ),
      ).toBe('first-unread');
    });

    it('falls back to the end when there is no unread customer reply', () => {
      expect(
        getInitialThreadScrollTarget(
          [
            { id: 'customer', sender_type: 'customer', message_type: 'reply', is_internal: false, created_at: base },
            { id: 'agent', sender_type: 'user', message_type: 'reply', is_internal: false, created_at: '2026-06-03T10:01:00.000Z' },
          ],
          '2026-06-03T10:02:00.000Z',
        ),
      ).toBe('agent');
    });

    it('ignores system events when finding the unread target', () => {
      expect(
        getInitialThreadScrollTarget(
          [
            { id: 'resolved-event', sender_type: 'system', message_type: 'system', system_event_type: 'resolved', is_internal: true, created_at: '2026-06-03T10:02:00.000Z' },
            { id: 'customer', sender_type: 'customer', message_type: 'reply', is_internal: false, created_at: '2026-06-03T10:03:00.000Z' },
          ],
          '2026-06-03T10:01:00.000Z',
        ),
      ).toBe('customer');
    });
  });

  describe('shouldMarkOpenThreadRead', () => {
    it('marks the open thread read when unread messages are visible at the bottom', () => {
      expect(shouldMarkOpenThreadRead({ unreadCount: 1, isNearBottom: true, isLoading: false })).toBe(true);
    });

    it('keeps unread when the agent is scrolled away from the new message', () => {
      expect(shouldMarkOpenThreadRead({ unreadCount: 1, isNearBottom: false, isLoading: false })).toBe(false);
    });

    it('does not mark read while the thread is still loading', () => {
      expect(shouldMarkOpenThreadRead({ unreadCount: 1, isNearBottom: true, isLoading: true })).toBe(false);
    });
  });
});
