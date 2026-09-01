import { describe, expect, it } from 'vitest';
import {
  detectMeetingPlatform,
  formatMeetingActionDueDate,
  formatMeetingDate,
  getMeetingPlatformLabel,
  getMeetingStatusLabel,
} from './meetingPresentation';

describe('meeting presentation helpers', () => {
  it.each([
    ['https://meet.google.com/abc-defg-hij', 'google_meet'],
    ['https://acme.zoom.us/j/123', 'zoom'],
    ['https://teams.microsoft.com/l/meetup-join/123', 'teams'],
    ['https://acme.webex.com/meet/azhar', 'webex'],
  ] as const)('detects %s as %s', (url, platform) => {
    expect(detectMeetingPlatform(url)).toBe(platform);
  });

  it('returns null for unsupported meeting links', () => {
    expect(detectMeetingPlatform('https://example.com/call')).toBeNull();
  });

  it('provides human-readable platform and status labels', () => {
    expect(getMeetingPlatformLabel('google_meet')).toBe('Google Meet');
    expect(getMeetingStatusLabel('finalizing')).toBe('Finalizing');
  });

  it('formats valid meeting dates and falls back for invalid API values', () => {
    expect(formatMeetingDate('2026-08-20T19:00:00Z', 'MMM d, yyyy')).toBe('Aug 20, 2026');
    expect(formatMeetingDate('2026-09-01', 'MMM d')).toBe('Sep 1');
    expect(formatMeetingDate('', 'MMM d, yyyy')).toBe('Date pending');
    expect(formatMeetingDate('not-a-date', 'MMM d, yyyy', 'Schedule pending')).toBe('Schedule pending');
  });

  it('formats action due dates without shifting the calendar day', () => {
    expect(formatMeetingActionDueDate('2026-09-01')).toBe('Sep 1');
    expect(formatMeetingActionDueDate('2026-09-01T00:00:00Z')).toBe('Sep 1');
    expect(formatMeetingActionDueDate('2026-02-30T00:00:00Z')).toBeNull();
    expect(formatMeetingActionDueDate('not-a-date')).toBeNull();
  });
});
