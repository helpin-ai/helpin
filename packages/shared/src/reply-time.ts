import type { ReplyTimePreset } from './types/widget-config';

/**
 * formatReplyTimeCopy — TypeScript mirror of the Go helper in
 * server/internal/service/support_reply_expectations.go#FormatReplyTimeCopy.
 *
 * The backend is the source of truth at runtime (the widget just renders
 * `availability.replyTimeText` straight from the payload). This mirror
 * exists so the admin settings preview can render without a server round
 * trip and so tests can assert that FE and BE copy do not drift.
 *
 * Drift is prevented by importing the JSON fixture generated from the Go
 * test (shared/test-data/reply-time-cases.json) and asserting that this
 * function returns the same strings.
 */
export function formatReplyTimeCopy(preset: ReplyTimePreset | string, customMinutes = 0): string {
  switch (preset) {
    case 'few_minutes':
      return 'Usually replies in a few minutes';
    case 'few_hours':
      return 'Usually replies in a few hours';
    case 'same_day':
      return 'Usually replies within a day';
    case 'custom':
      return formatCustomReplyTimeCopy(customMinutes);
  }
  return 'Usually replies in a few minutes';
}

function formatCustomReplyTimeCopy(minutes: number): string {
  if (!Number.isFinite(minutes) || minutes < 1) {
    return 'Usually replies in a few minutes';
  }
  if (minutes < 60) {
    return `Usually replies in about ${minutes} ${pluralize(minutes, 'minute', 'minutes')}`;
  }
  if (minutes < 1440) {
    let hours = Math.floor(minutes / 60);
    if (minutes % 60 >= 30) {
      hours += 1;
    }
    return `Usually replies in about ${hours} ${pluralize(hours, 'hour', 'hours')}`;
  }
  let days = Math.floor(minutes / 1440);
  if (minutes % 1440 >= 720) {
    days += 1;
  }
  return `Usually replies within ${days} ${pluralize(days, 'day', 'days')}`;
}

function pluralize(n: number, singular: string, plural: string): string {
  return n === 1 ? singular : plural;
}
