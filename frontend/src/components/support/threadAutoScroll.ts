export const THREAD_BOTTOM_THRESHOLD_PX = 96;
export const THREAD_TOP_THRESHOLD_PX = 64;

type ThreadViewportMetrics = Pick<HTMLElement, 'scrollHeight' | 'scrollTop' | 'clientHeight'>;

export type ThreadScrollMessage = {
  id: string;
  sender_type: string;
  message_type?: string | null;
  system_event_type?: string | null;
  is_internal?: boolean;
  created_at: string;
};

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

export function isNearThreadTop(
  viewport: Pick<HTMLElement, 'scrollTop'>,
  threshold = THREAD_TOP_THRESHOLD_PX,
) {
  return viewport.scrollTop <= threshold;
}

export function getPrependRestoredScrollTop({
  previousScrollHeight,
  nextScrollHeight,
  previousScrollTop,
}: {
  previousScrollHeight: number;
  nextScrollHeight: number;
  previousScrollTop: number;
}) {
  return previousScrollTop + Math.max(0, nextScrollHeight - previousScrollHeight);
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

export function getInitialThreadScrollTarget(
  messages: ThreadScrollMessage[],
  teamLastSeenAt?: string | null,
) {
  if (messages.length === 0) return null;

  const cursorTime = teamLastSeenAt ? Date.parse(teamLastSeenAt) : Number.NaN;
  if (Number.isFinite(cursorTime)) {
    const firstUnread = messages.find((message) => {
      if (message.sender_type !== 'customer') return false;
      if (message.is_internal) return false;
      if (message.message_type && message.message_type !== 'reply') return false;
      if (message.system_event_type) return false;
      return Date.parse(message.created_at) > cursorTime;
    });
    if (firstUnread) return firstUnread.id;
  }

  return messages[messages.length - 1]?.id ?? null;
}

export function shouldMarkOpenThreadRead({
  unreadCount,
  isNearBottom,
  isLoading,
}: {
  unreadCount?: number | null;
  isNearBottom: boolean;
  isLoading: boolean;
}) {
  return !isLoading && (unreadCount ?? 0) > 0 && isNearBottom;
}
