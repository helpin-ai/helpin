import { useMemo } from 'react';
import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { Button } from '@/components/ui/button';
import { ArrowDown01Icon, GlobeIcon } from '@/lib/icons';
import { cn } from '@/lib/utils';

function timezoneOption(id: string) {
  let offset = '';
  try {
    const parts = new Intl.DateTimeFormat('en-US', { timeZone: id, timeZoneName: 'shortOffset' }).formatToParts(new Date());
    const gmt = parts.find((part) => part.type === 'timeZoneName')?.value ?? '';
    offset = gmt === 'GMT' ? 'UTC+00:00' : gmt.replace('GMT', 'UTC');
  } catch {
    // Keep a saved value visible even if this browser cannot format its offset.
  }
  return {
    value: id,
    label: id,
    offset,
    keywords: [`${id} ${offset}`, `${id.replace(/_/g, ' ')} ${offset}`],
    trailing: offset ? <span className="shrink-0 text-muted-foreground">{offset}</span> : undefined,
  };
}

// Intl's list omits UTC and can omit aliases used by saved settings.
const TIMEZONE_LIST = ['UTC', ...Intl.supportedValuesOf('timeZone')].map(timezoneOption);

export function TimezoneSelector({ id, value, onChange, disabled = false, className }: {
  id: string;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  className?: string;
}) {
  const options = useMemo(() => TIMEZONE_LIST.some((timezone) => timezone.value === value)
    ? TIMEZONE_LIST
    : [...TIMEZONE_LIST, timezoneOption(value)], [value]);
  const selectedTimezone = options.find((timezone) => timezone.value === value);

  return (
    <QuietDropdown
      label="Timezone"
      options={options}
      selected={[value]}
      onSelect={onChange}
      disabled={disabled}
      searchPlaceholder="Search timezones..."
      empty="No timezones found"
      contentClassName="w-80"
      trigger={
        <Button id={id} type="button" variant="outline" disabled={disabled} className={cn('w-full justify-between font-normal', className)}>
          <span className="flex min-w-0 items-center gap-2">
            <GlobeIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate">{value}</span>
            {selectedTimezone?.offset && <span className="shrink-0 text-muted-foreground">({selectedTimezone.offset})</span>}
          </span>
          <ArrowDown01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
        </Button>
      }
    />
  );
}
