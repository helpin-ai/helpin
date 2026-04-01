import { describe, expect, it } from 'vitest';
import {
  STORY_OVERLAY_INITIAL_DISMISS_SUPPRESSION_MS,
  shouldSuppressStoryOverlayOutsideDismiss,
} from '@/components/pm/story-detail/storyOverlayDismiss';

describe('shouldSuppressStoryOverlayOutsideDismiss', () => {
  it('suppresses outside dismiss immediately after open', () => {
    expect(shouldSuppressStoryOverlayOutsideDismiss(1_000, 1_025)).toBe(true);
  });

  it('stops suppressing after the initial protection window', () => {
    expect(
      shouldSuppressStoryOverlayOutsideDismiss(
        1_000,
        1_000 + STORY_OVERLAY_INITIAL_DISMISS_SUPPRESSION_MS,
      ),
    ).toBe(false);
  });

  it('does not suppress when overlay has not been opened yet', () => {
    expect(shouldSuppressStoryOverlayOutsideDismiss(null, 1_000)).toBe(false);
  });
});
