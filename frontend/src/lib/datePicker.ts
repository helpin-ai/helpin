import { parseDate } from 'chrono-node';
import {
  addDays,
  format,
  isAfter,
  isBefore,
  isValid,
  parseISO,
  startOfDay,
} from 'date-fns';

export type DatePickerKind = 'generic' | 'start' | 'due' | 'target' | 'end';

export interface DatePickerPreset {
  id: string;
  label: string;
  value: string;
  helper: string;
}

interface ParseNaturalLanguageDateOptions {
  referenceDate?: Date;
  min?: Date;
  max?: Date;
}

const AMBIGUOUS_NUMERIC_DATE_RE = /^\d{1,2}[/-]\d{1,2}([/-]\d{2,4})?$/;

function formatISODate(date: Date): string {
  return format(startOfDay(date), 'yyyy-MM-dd');
}

export function parseStoredDate(value?: string): Date | undefined {
  if (!value) return undefined;
  const parsed = parseISO(value);
  return isValid(parsed) ? parsed : undefined;
}

export function formatDateValue(value?: string, fallback = 'Pick a date'): string {
  const parsed = parseStoredDate(value);
  return parsed ? format(parsed, 'MMM d, yyyy') : fallback;
}

export function formatCompactDateValue(value?: string, fallback = 'None'): string {
  const parsed = parseStoredDate(value);
  return parsed ? format(parsed, 'EEE, MMM d') : fallback;
}

export function formatShortDateValue(value?: string, fallback = ''): string {
  const parsed = parseStoredDate(value);
  return parsed ? format(parsed, 'M/d/yy') : fallback;
}

export function getDatePickerLabel(kind: DatePickerKind): string {
  switch (kind) {
    case 'start':
      return 'Start date';
    case 'due':
      return 'Due date';
    case 'target':
      return 'Target date';
    case 'end':
      return 'End date';
    default:
      return 'Date';
  }
}

function formatPresetHelper(date: Date, referenceDate: Date): string {
  const refYear = referenceDate.getFullYear();
  const dateYear = date.getFullYear();
  return format(date, refYear === dateYear ? 'EEE, MMM d' : 'MMM d, yyyy');
}

function nextWeekday(referenceDate: Date, weekday: number, includeToday = false): Date {
  const base = startOfDay(referenceDate);
  const current = base.getDay();
  let delta = (weekday - current + 7) % 7;
  if (delta === 0 && !includeToday) delta = 7;
  return addDays(base, delta);
}

function getThisWeekend(referenceDate: Date): Date {
  return nextWeekday(referenceDate, 6, true);
}

function getNextWeekend(referenceDate: Date): Date {
  const thisWeekend = getThisWeekend(referenceDate);
  return addDays(thisWeekend, 7);
}

export function getDatePickerPresets(kind: DatePickerKind, referenceDate = new Date()): DatePickerPreset[] {
  const today = startOfDay(referenceDate);

  const presets = {
    today: today,
    tomorrow: addDays(today, 1),
    thisWeekend: getThisWeekend(today),
    nextWeek: nextWeekday(today, 1, false),
    nextWeekend: getNextWeekend(today),
    in2Weeks: addDays(today, 14),
    in4Weeks: addDays(today, 28),
  } as const;

  const build = (id: string, label: string, date: Date): DatePickerPreset => ({
    id,
    label,
    value: formatISODate(date),
    helper: formatPresetHelper(date, today),
  });

  switch (kind) {
    case 'start':
      return [
        build('today', 'Today', presets.today),
        build('tomorrow', 'Tomorrow', presets.tomorrow),
        build('next-week', 'Next week', presets.nextWeek),
        build('in-2-weeks', 'In 2 weeks', presets.in2Weeks),
        build('in-4-weeks', 'In 4 weeks', presets.in4Weeks),
      ];
    case 'due':
    case 'target':
    case 'end':
      return [
        build('today', 'Today', presets.today),
        build('tomorrow', 'Tomorrow', presets.tomorrow),
        build('this-weekend', 'This weekend', presets.thisWeekend),
        build('next-week', 'Next week', presets.nextWeek),
        build('next-weekend', 'Next weekend', presets.nextWeekend),
        build('in-2-weeks', 'In 2 weeks', presets.in2Weeks),
        build('in-4-weeks', 'In 4 weeks', presets.in4Weeks),
      ];
    default:
      return [
        build('today', 'Today', presets.today),
        build('tomorrow', 'Tomorrow', presets.tomorrow),
        build('next-week', 'Next week', presets.nextWeek),
        build('in-2-weeks', 'In 2 weeks', presets.in2Weeks),
        build('in-4-weeks', 'In 4 weeks', presets.in4Weeks),
      ];
  }
}

function normalizeNaturalLanguageInput(input: string): string {
  const normalized = input.trim().toLowerCase().replace(/\s+/g, ' ');

  if (!normalized) return normalized;

  const exactAliases: Record<string, string> = {
    tmr: 'tomorrow',
    tmrw: 'tomorrow',
    tod: 'today',
  };

  if (exactAliases[normalized]) return exactAliases[normalized];

  const shortWeeks = normalized.match(/^(?:in )?(\d+)\s*w(?:k|ks)?$/);
  if (shortWeeks) return `in ${shortWeeks[1]} weeks`;

  const shortDays = normalized.match(/^(?:in )?(\d+)\s*d$/);
  if (shortDays) return `in ${shortDays[1]} days`;

  return normalized;
}

export function parseNaturalLanguageDate(
  input: string,
  options: ParseNaturalLanguageDateOptions = {},
): { value?: string; error?: string } {
  const normalized = normalizeNaturalLanguageInput(input);
  if (!normalized) {
    return { error: 'Enter a date like tomorrow or next Friday.' };
  }

  if (AMBIGUOUS_NUMERIC_DATE_RE.test(normalized)) {
    return { error: 'Use words like tomorrow or a full date like Apr 12 2026.' };
  }

  const referenceDate = startOfDay(options.referenceDate ?? new Date());
  const parsed = parseDate(normalized, referenceDate, { forwardDate: true });
  if (!parsed || !isValid(parsed)) {
    return { error: 'Could not understand that date.' };
  }

  const resolved = startOfDay(parsed);
  if (options.min && isBefore(resolved, startOfDay(options.min))) {
    return { error: `Choose ${format(startOfDay(options.min), 'MMM d, yyyy')} or later.` };
  }

  if (options.max && isAfter(resolved, startOfDay(options.max))) {
    return { error: `Choose ${format(startOfDay(options.max), 'MMM d, yyyy')} or earlier.` };
  }

  return { value: formatISODate(resolved) };
}
