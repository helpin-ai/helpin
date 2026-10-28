import type { BusinessHoursDay, BusinessHoursSlot } from '@/lib/pmTypes';
import { MINUTES_PER_DAY, clockToMinutes, minutesToClock } from '@/lib/timeOfDay';

export const BUSINESS_HOURS_STEP_MINUTES = 15;
export const DEFAULT_BUSINESS_HOURS_SLOT: BusinessHoursSlot = { start: '09:00', end: '17:00' };

/** The day's time ranges, reading settings saved before ranges existed from start and end. */
export function daySlots(day?: Partial<BusinessHoursDay> | null): BusinessHoursSlot[] {
  if (day?.slots && day.slots.length > 0) return day.slots;
  if (day?.start || day?.end) return [{ start: day.start ?? '', end: day.end ?? '' }];
  return [];
}

/** Sets the day's ranges and mirrors the first into start and end for older readers. */
export function withSlots(day: BusinessHoursDay, slots: BusinessHoursSlot[]): BusinessHoursDay {
  const first = slots[0] ?? DEFAULT_BUSINESS_HOURS_SLOT;
  return { ...day, start: first.start, end: first.end, slots };
}

/** Adds a one-hour range after the day's last range, or before its first when the day is full at the end. */
export function addBusinessHoursSlot(day: BusinessHoursDay): BusinessHoursDay | null {
  const slots = daySlots(day);
  if (!day.enabled) {
    return withSlots({ ...day, enabled: true }, slots.length > 0 ? slots : [DEFAULT_BUSINESS_HOURS_SLOT]);
  }
  const ranges = slots
    .map((slot) => [clockToMinutes(slot.start), clockToMinutes(slot.end)] as const)
    .filter((range): range is readonly [number, number] => range[0] !== null && range[1] !== null);
  if (ranges.length === 0) {
    return withSlots(day, [...slots, DEFAULT_BUSINESS_HOURS_SLOT]);
  }
  const lastEnd = Math.max(...ranges.map(([, end]) => end));
  if (lastEnd + BUSINESS_HOURS_STEP_MINUTES <= MINUTES_PER_DAY) {
    const next = { start: minutesToClock(lastEnd), end: minutesToClock(Math.min(lastEnd + 60, MINUTES_PER_DAY)) };
    return withSlots(day, [...slots, next]);
  }
  const firstStart = Math.min(...ranges.map(([start]) => start));
  if (firstStart >= BUSINESS_HOURS_STEP_MINUTES) {
    const earlier = { start: minutesToClock(Math.max(firstStart - 60, 0)), end: minutesToClock(firstStart) };
    return withSlots(day, [earlier, ...slots]);
  }
  return null;
}

/** Explains why an open day's ranges cannot be saved, or null when they can. */
export function businessHoursDayIssue(day: BusinessHoursDay): string | null {
  if (!day.enabled) return null;
  const ranges: Array<[number, number]> = [];
  for (const slot of daySlots(day)) {
    const start = clockToMinutes(slot.start);
    const end = clockToMinutes(slot.end);
    if (start === null || end === null || start >= MINUTES_PER_DAY) return 'Choose a valid time.';
    if (end <= start) return 'End time must be after the start time.';
    ranges.push([start, end]);
  }
  ranges.sort((a, b) => a[0] - b[0]);
  for (let i = 1; i < ranges.length; i += 1) {
    if (ranges[i][0] < ranges[i - 1][1]) return 'Times overlap.';
  }
  return null;
}

/** Copies one day's open state and ranges to other days. */
export function copyBusinessHoursDay(
  schedule: Record<string, BusinessHoursDay>,
  fromKey: string,
  toKeys: string[],
): Record<string, BusinessHoursDay> {
  const source = schedule[fromKey];
  if (!source) return schedule;
  const next = { ...schedule };
  for (const key of toKeys) {
    if (key === fromKey) continue;
    next[key] = withSlots({ ...source }, daySlots(source).map((slot) => ({ ...slot })));
  }
  return next;
}

/** Whether a day is open at the given minute after midnight. */
export function isOpenAt(day: BusinessHoursDay | undefined, minutes: number): boolean {
  if (!day?.enabled) return false;
  return daySlots(day).some((slot) => {
    const start = clockToMinutes(slot.start);
    const end = clockToMinutes(slot.end);
    return start !== null && end !== null && minutes >= start && minutes < end;
  });
}
