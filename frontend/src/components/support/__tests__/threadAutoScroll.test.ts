import { describe, expect, it } from 'vitest';
import { isNearThreadBottom, shouldAutoScrollThread } from '../threadAutoScroll';

describe('thread auto-scroll helpers', () => {
  describe('isNearThreadBottom', () => {
    it('treats a viewport within the threshold as near the bottom', () => {
      expect(isNearThreadBottom({ scrollHeight: 1000, scrollTop: 620, clientHeight: 300 }, 96)).toBe(true);
    });

    it('treats a viewport beyond the threshold as away from the bottom', () => {
      expect(isNearThreadBottom({ scrollHeight: 1000, scrollTop: 560, clientHeight: 300 }, 96)).toBe(false);
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
});
