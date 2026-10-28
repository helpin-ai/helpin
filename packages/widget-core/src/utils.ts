import type { WidgetConfig } from './types';

/**
 * Format a date string as a relative time ago label.
 * "Just now", "5m ago", "2h ago", "3d ago", or locale date for 7d+.
 */
export function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffMs = Math.max(0, now - then);
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return 'Just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 7) return `${diffDay}d ago`;
  return new Date(dateStr).toLocaleDateString();
}

/**
 * The availability status line on the customer's own clock. The server's
 * statusText names the next opening time in the business timezone, so while
 * the team is offline the widget rebuilds it from nextOnlineAt.
 */
export function availabilityStatusText(
  availability: WidgetConfig['availability'],
  options: { locale?: string; timeZone?: string } = {},
): string | undefined {
  if (!availability || availability.isOnline || !availability.nextOnlineAt) {
    return availability?.statusText;
  }
  const nextOnline = new Date(availability.nextOnlineAt);
  if (Number.isNaN(nextOnline.getTime())) {
    return availability.statusText;
  }
  const when = nextOnline.toLocaleString(options.locale, {
    weekday: 'short',
    hour: 'numeric',
    minute: '2-digit',
    timeZone: options.timeZone,
  });
  return `Offline now - Back ${when}`;
}
