import { useMemo } from 'react';

import { QuietDropdown, type QuietDropdownOption } from './quiet-dropdown';
import { quietUnderlineControlClassName } from './quiet';
import { ArrowUpDownIcon } from '@/lib/icons';
import { MINUTES_PER_DAY, clockToMinutes, formatClockLabel, minutesToClock } from '@/lib/timeOfDay';
import { cn } from '@/lib/utils';

export interface QuietTimeSelectProps {
  /** Accessible name, e.g. "Monday start time". */
  label: string;
  /** Selected time as "HH:MM". */
  value: string;
  onChange: (value: string) => void;
  /** Only offer times later than this "HH:MM" value. */
  after?: string;
  /** Offer midnight at the end of the day ("24:00") instead of at its start. */
  endOfDay?: boolean;
  stepMinutes?: number;
  disabled?: boolean;
  invalid?: boolean;
  id?: string;
}

function timeKeywords(minutes: number): string[] {
  const clock = minutesToClock(minutes);
  const label = formatClockLabel(clock);
  const compact = label.replace(':', '');
  // Words a typed time can match: "9:30am", "930am", "09:30", "9:30", "21:30".
  const keywords = [label, compact, clock, clock.replace(/^0/, '')];
  if (minutes === 0 || minutes === MINUTES_PER_DAY) keywords.push('midnight');
  if (minutes === 12 * 60) keywords.push('noon');
  return keywords;
}

/** Time-of-day picker: an underline trigger with a searchable list of times. */
export function QuietTimeSelect({
  label,
  value,
  onChange,
  after,
  endOfDay = false,
  stepMinutes = 15,
  disabled,
  invalid,
  id,
}: QuietTimeSelectProps) {
  const options = useMemo<QuietDropdownOption[]>(() => {
    const earliest = after ? clockToMinutes(after) : null;
    const minutes: number[] = [];
    const first = endOfDay ? stepMinutes : 0;
    const last = endOfDay ? MINUTES_PER_DAY : MINUTES_PER_DAY - stepMinutes;
    for (let m = first; m <= last; m += stepMinutes) {
      if (earliest === null || m > earliest) minutes.push(m);
    }
    // Keep a saved time that is off the step grid selectable.
    const current = clockToMinutes(value);
    if (current !== null && !minutes.includes(current)) {
      minutes.push(current);
      minutes.sort((a, b) => a - b);
    }
    return minutes.map((m) => ({
      value: minutesToClock(m),
      label: formatClockLabel(minutesToClock(m)),
      keywords: timeKeywords(m),
    }));
  }, [after, endOfDay, stepMinutes, value]);

  const display = formatClockLabel(value);
  return (
    <QuietDropdown
      label={label}
      options={options}
      selected={[value]}
      onSelect={onChange}
      disabled={disabled}
      searchMode="always"
      searchPlaceholder="Type a time"
      empty="No matching time"
      contentClassName="w-36"
      contentProps={{
        onOpenAutoFocus: (event) => {
          const content = event.currentTarget as HTMLElement | null;
          requestAnimationFrame(() => {
            content?.querySelector<HTMLElement>('[data-checked="true"]')?.scrollIntoView({ block: 'center' });
          });
        },
      }}
      trigger={
        <button
          type="button"
          id={id}
          disabled={disabled}
          aria-label={`${label}: ${display}`}
          aria-invalid={invalid || undefined}
          className={cn(
            quietUnderlineControlClassName,
            'flex w-[5.75rem] items-center justify-between gap-1 text-left tabular-nums disabled:cursor-not-allowed disabled:opacity-50',
            invalid && 'border-destructive hover:border-destructive focus-visible:border-destructive',
          )}
        >
          <span>{display}</span>
          <ArrowUpDownIcon className="size-3 shrink-0 text-quiet-muted" />
        </button>
      }
    />
  );
}
