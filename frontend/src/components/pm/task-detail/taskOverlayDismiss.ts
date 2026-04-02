export const TASK_OVERLAY_INITIAL_DISMISS_SUPPRESSION_MS = 150;

export function shouldSuppressTaskOverlayOutsideDismiss(
  openedAt: number | null,
  now: number,
) {
  if (openedAt === null) return false;
  return now - openedAt < TASK_OVERLAY_INITIAL_DISMISS_SUPPRESSION_MS;
}
