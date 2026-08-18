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
