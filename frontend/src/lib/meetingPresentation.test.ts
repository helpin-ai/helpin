import { describe, expect, it } from 'vitest';
import { detectMeetingPlatform, getMeetingPlatformLabel, getMeetingStatusLabel } from './meetingPresentation';

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
});
