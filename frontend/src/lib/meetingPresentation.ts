import { format, isValid } from 'date-fns';
import type { CRMMeetingPlatform, CRMMeetingStatus } from './crmMeetingTypes';

const platformLabels: Record<CRMMeetingPlatform, string> = {
  google_meet: 'Google Meet',
  zoom: 'Zoom',
  teams: 'Microsoft Teams',
  webex: 'Webex',
};

export function getMeetingPlatformLabel(platform: CRMMeetingPlatform) {
  return platformLabels[platform];
}

export function detectMeetingPlatform(url: string): CRMMeetingPlatform | null {
  const normalized = url.trim().toLowerCase();
  if (!normalized) return null;
  if (normalized.includes('meet.google.com')) return 'google_meet';
  if (normalized.includes('zoom.us') || normalized.includes('zoom.com')) return 'zoom';
  if (normalized.includes('teams.microsoft.com') || normalized.includes('teams.live.com')) return 'teams';
  if (normalized.includes('webex.com')) return 'webex';
  return null;
}

export function getMeetingStatusLabel(status: CRMMeetingStatus) {
  return status.replace(/_/g, ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());
}

export function formatMeetingDate(
  value: string | Date | null | undefined,
  pattern: string,
  fallback = 'Date pending',
) {
  const dateOnlyMatch = typeof value === 'string'
    ? value.match(/^(\d{4})-(\d{2})-(\d{2})$/)
    : null;
  const date = dateOnlyMatch
    ? new Date(Number(dateOnlyMatch[1]), Number(dateOnlyMatch[2]) - 1, Number(dateOnlyMatch[3]))
    : value instanceof Date
      ? value
      : new Date(value ?? '');
  return isValid(date) ? format(date, pattern) : fallback;
}

export function formatMeetingActionDueDate(value?: string | null): string | null {
  const match = value?.match(/^(\d{4})-(\d{2})-(\d{2})(?:T|$)/);
  if (!match) return null;

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(year, month - 1, day);
  const isSameCalendarDate = date.getFullYear() === year
    && date.getMonth() === month - 1
    && date.getDate() === day;

  return isSameCalendarDate ? format(date, 'MMM d') : null;
}
