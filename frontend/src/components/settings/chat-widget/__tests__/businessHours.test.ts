import { describe, expect, it } from 'vitest';
import {
  addBusinessHoursSlot,
  businessHoursDayIssue,
  copyBusinessHoursDay,
  daySlots,
  isOpenAt,
} from '../businessHours';
import { normalizeBusinessHoursDay, normalizeBusinessHoursSchedule } from '../utils';
import { clockToMinutes, formatClockLabel } from '@/lib/timeOfDay';
import type { BusinessHoursDay } from '@/lib/pmTypes';

const open = (...slots: Array<[string, string]>): BusinessHoursDay => ({
  start: slots[0]?.[0] ?? '09:00',
  end: slots[0]?.[1] ?? '17:00',
  enabled: true,
  slots: slots.map(([start, end]) => ({ start, end })),
});

describe('time of day', () => {
  it('formats times the way the picker shows them', () => {
    expect(formatClockLabel('09:00')).toBe('9:00am');
    expect(formatClockLabel('12:00')).toBe('12:00pm');
    expect(formatClockLabel('17:30')).toBe('5:30pm');
    expect(formatClockLabel('00:00')).toBe('12:00am');
    expect(formatClockLabel('24:00')).toBe('12:00am');
  });

  it('reads valid times only', () => {
    expect(clockToMinutes('09:15')).toBe(555);
    expect(clockToMinutes('24:00')).toBe(1440);
    expect(clockToMinutes('24:30')).toBeNull();
    expect(clockToMinutes('9am')).toBeNull();
  });
});

describe('business hours days', () => {
  it('reads settings saved before time ranges existed', () => {
    expect(daySlots({ start: '08:00', end: '16:00', enabled: true })).toEqual([{ start: '08:00', end: '16:00' }]);
    expect(normalizeBusinessHoursDay({ start: '08:00', end: '16:00', enabled: true }).slots).toEqual([{ start: '08:00', end: '16:00' }]);
  });

  it('fills every day of the week', () => {
    const schedule = normalizeBusinessHoursSchedule({ mon: open(['09:00', '12:00'], ['13:00', '17:00']) });
    expect(Object.keys(schedule)).toHaveLength(7);
    expect(schedule.mon.start).toBe('09:00');
    expect(schedule.mon.slots).toHaveLength(2);
    expect(schedule.sun.enabled).toBe(false);
  });

  it('adds a one-hour range after the last one', () => {
    expect(addBusinessHoursSlot(open(['09:00', '17:00']))?.slots).toEqual([
      { start: '09:00', end: '17:00' },
      { start: '17:00', end: '18:00' },
    ]);
  });

  it('adds a range before the first one when the day is full at the end', () => {
    expect(addBusinessHoursSlot(open(['20:00', '24:00']))?.slots?.[0]).toEqual({ start: '19:00', end: '20:00' });
    expect(addBusinessHoursSlot(open(['00:00', '24:00']))).toBeNull();
  });

  it('opens a closed day when adding a range', () => {
    const added = addBusinessHoursSlot({ ...open(['10:00', '14:00']), enabled: false });
    expect(added?.enabled).toBe(true);
    expect(added?.slots).toEqual([{ start: '10:00', end: '14:00' }]);
  });

  it('explains ranges that cannot be saved', () => {
    expect(businessHoursDayIssue(open(['09:00', '12:00'], ['13:00', '17:00']))).toBeNull();
    expect(businessHoursDayIssue(open(['09:00', '12:00'], ['11:00', '13:00']))).toBe('Times overlap.');
    expect(businessHoursDayIssue(open(['17:00', '09:00']))).toBe('End time must be after the start time.');
    expect(businessHoursDayIssue({ ...open(['17:00', '09:00']), enabled: false })).toBeNull();
  });

  it('copies open state and ranges to other days without sharing them', () => {
    const schedule = normalizeBusinessHoursSchedule({ mon: open(['09:00', '12:00'], ['13:00', '17:00']) });
    const copied = copyBusinessHoursDay(schedule, 'mon', ['tue', 'sat']);
    expect(copied.tue).toEqual(copied.mon);
    expect(copied.sat.enabled).toBe(true);
    expect(copied.tue.slots).not.toBe(copied.mon.slots);
    expect(copied.wed).toBe(schedule.wed);

    const closed = copyBusinessHoursDay(copied, 'sun', ['tue']);
    expect(closed.tue.enabled).toBe(false);
  });

  it('checks whether a day is open at a given minute', () => {
    const day = open(['09:00', '12:00'], ['13:00', '17:00']);
    expect(isOpenAt(day, 10 * 60)).toBe(true);
    expect(isOpenAt(day, 12 * 60 + 30)).toBe(false);
    expect(isOpenAt(day, 13 * 60)).toBe(true);
    expect(isOpenAt({ ...day, enabled: false }, 10 * 60)).toBe(false);
  });
});
