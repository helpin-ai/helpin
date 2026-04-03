import { describe, expect, it } from 'vitest';

import {
  applyLinkedDateValue,
  formatCompactDateValue,
  getDatePickerPresets,
  parseNaturalLanguageDate,
  parseStoredDate,
  preloadNaturalLanguageParser,
} from '@/lib/datePicker';

describe('datePicker helpers', () => {
  it('clears the linked end date when a new start date moves past it', () => {
    expect(
      applyLinkedDateValue(
        { primary: '2026-04-10', linked: '2026-04-12' },
        'primary',
        '2026-04-15',
        true,
      ),
    ).toEqual({ primary: '2026-04-15', linked: '' });
  });

  it('clears the linked start date when a new end date moves before it', () => {
    expect(
      applyLinkedDateValue(
        { primary: '2026-04-10', linked: '2026-04-12' },
        'linked',
        '2026-04-08',
        true,
      ),
    ).toEqual({ primary: '', linked: '2026-04-08' });
  });

  it('returns due-date presets with weekend shortcuts', () => {
    const referenceDate = new Date('2026-04-03T12:00:00Z');
    const presets = getDatePickerPresets('due', referenceDate);

    expect(presets.map((preset) => preset.label)).toEqual([
      'Today',
      'Tomorrow',
      'This weekend',
      'Next week',
      'Next weekend',
      'In 2 weeks',
      'In 4 weeks',
    ]);
    expect(presets[3]?.value).toBe('2026-04-06');
    expect(presets[4]?.value).toBe('2026-04-11');
  });

  it('returns a narrower preset list for start dates', () => {
    const referenceDate = new Date('2026-04-03T12:00:00Z');
    const presets = getDatePickerPresets('start', referenceDate);

    expect(presets.map((preset) => preset.label)).toEqual([
      'Today',
      'Tomorrow',
      'Next week',
      'In 2 weeks',
      'In 4 weeks',
    ]);
  });

  it('parses chrono input and shorthand aliases', async () => {
    await preloadNaturalLanguageParser();
    const referenceDate = new Date('2026-04-03T12:00:00Z');

    expect(parseNaturalLanguageDate('tmr', { referenceDate })).toEqual({ value: '2026-04-04' });
    expect(parseNaturalLanguageDate('2w', { referenceDate })).toEqual({ value: '2026-04-17' });
  });

  it('rejects ambiguous numeric dates and out-of-range results', async () => {
    await preloadNaturalLanguageParser();
    const referenceDate = new Date('2026-04-03T12:00:00Z');

    expect(parseNaturalLanguageDate('04/05', { referenceDate }).error).toMatch(/Use words like tomorrow/);
    expect(
      parseNaturalLanguageDate('tomorrow', {
        referenceDate,
        max: new Date('2026-04-03T00:00:00Z'),
      }).error,
    ).toBe('Choose Apr 3, 2026 or earlier.');
  });

  it('formats and parses stored ISO date strings', () => {
    expect(parseStoredDate('2026-04-03')).toBeInstanceOf(Date);
    expect(formatCompactDateValue('2026-04-03')).toBe('Fri, Apr 3');
    expect(parseStoredDate('not-a-date')).toBeUndefined();
  });
});
