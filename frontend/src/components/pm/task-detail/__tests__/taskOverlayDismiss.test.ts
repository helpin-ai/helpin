import { describe, expect, it } from 'vitest';
import {
  TASK_OVERLAY_INITIAL_DISMISS_SUPPRESSION_MS,
  shouldSuppressTaskOverlayOutsideDismiss,
} from '@/components/pm/task-detail/taskOverlayDismiss';

describe('shouldSuppressTaskOverlayOutsideDismiss', () => {
  it('suppresses outside dismiss immediately after open', () => {
    expect(shouldSuppressTaskOverlayOutsideDismiss(1_000, 1_025)).toBe(true);
  });

  it('stops suppressing after the initial protection window', () => {
    expect(
      shouldSuppressTaskOverlayOutsideDismiss(
        1_000,
        1_000 + TASK_OVERLAY_INITIAL_DISMISS_SUPPRESSION_MS,
      ),
    ).toBe(false);
  });

  it('does not suppress when overlay has not been opened yet', () => {
    expect(shouldSuppressTaskOverlayOutsideDismiss(null, 1_000)).toBe(false);
  });
});
