export const STORY_OVERLAY_INITIAL_DISMISS_SUPPRESSION_MS = 150;

export function shouldSuppressStoryOverlayOutsideDismiss(
  openedAt: number | null,
  now: number,
) {
  if (openedAt === null) return false;
  return now - openedAt < STORY_OVERLAY_INITIAL_DISMISS_SUPPRESSION_MS;
}
