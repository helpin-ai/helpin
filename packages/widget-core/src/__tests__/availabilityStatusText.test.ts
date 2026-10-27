import { describe, expect, it } from 'vitest';
import { availabilityStatusText } from '../utils';

const offline = {
  isOnline: false,
  statusText: 'Offline now - Back Mon 9:00 AM',
  replyTimeText: "We're currently offline.",
  // Monday 9:00 AM in New York.
  nextOnlineAt: '2026-10-26T09:00:00-04:00',
};

describe('availabilityStatusText', () => {
  it("shows the next opening time on the customer's own clock", () => {
    expect(availabilityStatusText(offline, { locale: 'en-US', timeZone: 'Asia/Karachi' })).toBe(
      'Offline now - Back Mon 6:00 PM',
    );
  });

  it('keeps the server text while the team is online', () => {
    expect(availabilityStatusText({ ...offline, isOnline: true, statusText: 'Online now' })).toBe('Online now');
  });

  it('keeps the server text without a next opening time', () => {
    expect(availabilityStatusText({ ...offline, nextOnlineAt: undefined })).toBe(offline.statusText);
  });

  it('keeps the server text when the next opening time is invalid', () => {
    expect(availabilityStatusText({ ...offline, nextOnlineAt: 'not a date' })).toBe(offline.statusText);
  });
});
