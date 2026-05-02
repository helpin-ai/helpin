export const THREAD_BOTTOM_THRESHOLD_PX = 96;

type ThreadViewportMetrics = Pick<HTMLElement, 'scrollHeight' | 'scrollTop' | 'clientHeight'>;

interface ShouldAutoScrollThreadInput {
  conversationChanged: boolean;
  initialLoad: boolean;
  messageCountIncreased: boolean;
  wasNearBottom: boolean;
  pendingInitialScroll: boolean;
}

export function isNearThreadBottom(
  viewport: ThreadViewportMetrics,
  threshold = THREAD_BOTTOM_THRESHOLD_PX,
) {
  return viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight <= threshold;
}

export function shouldAutoScrollThread({
  conversationChanged,
  initialLoad,
  messageCountIncreased,
  wasNearBottom,
  pendingInitialScroll,
}: ShouldAutoScrollThreadInput) {
  if (conversationChanged || initialLoad || pendingInitialScroll) {
    return true;
  }

  return messageCountIncreased && wasNearBottom;
}
