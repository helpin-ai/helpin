import type { SupportMessage } from '@/lib/pmTypes';

const RECENT_RESOLVED_NOTICE_MS = 30_000;

/**
 * While a resolved conversation hands off to the next one, the server's new
 * "resolved" notice would pop into the thread for a moment and vanish with it.
 * Older resolved notices in the history stay visible.
 */
export function hiddenDuringHandoff(
  message: Pick<SupportMessage, 'system_event_type' | 'created_at'>,
  isHandingOff: boolean,
  now: number = Date.now(),
): boolean {
  return isHandingOff
    && message.system_event_type === 'resolved'
    && Date.parse(message.created_at) > now - RECENT_RESOLVED_NOTICE_MS;
}
