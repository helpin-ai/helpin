/** Times of day as "HH:MM" strings, where "24:00" is midnight at the end of the day. */

export const MINUTES_PER_DAY = 24 * 60;

/** Reads "HH:MM" as minutes after midnight; "24:00" is midnight at the end of the day. */
export function clockToMinutes(value: string): number | null {
  const match = /^(\d{1,2}):(\d{2})$/.exec(value.trim());
  if (!match) return null;
  const minutes = Number(match[1]) * 60 + Number(match[2]);
  if (Number(match[2]) > 59 || minutes > MINUTES_PER_DAY) return null;
  return minutes;
}

export function minutesToClock(minutes: number): string {
  const hours = Math.floor(minutes / 60);
  return `${String(hours).padStart(2, '0')}:${String(minutes % 60).padStart(2, '0')}`;
}

/** "09:00" → "9:00am", "17:30" → "5:30pm", "24:00" → "12:00am". */
export function formatClockLabel(value: string): string {
  const minutes = clockToMinutes(value);
  if (minutes === null) return value;
  const hours24 = Math.floor(minutes / 60) % 24;
  const suffix = hours24 < 12 ? 'am' : 'pm';
  const hours12 = hours24 % 12 === 0 ? 12 : hours24 % 12;
  return `${hours12}:${String(minutes % 60).padStart(2, '0')}${suffix}`;
}
