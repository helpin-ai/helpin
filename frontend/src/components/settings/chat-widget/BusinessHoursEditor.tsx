import { useState } from 'react';

import { QuietIconAction, QuietPrimaryAction } from '@/components/design-system/quiet';
import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { QuietTimeSelect } from '@/components/design-system/quiet-time-select';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Switch } from '@/components/ui/switch';
import { Cancel01Icon, Copy01Icon, PlusSignIcon } from '@/lib/icons';
import type { BusinessHoursDay, BusinessHoursSlot } from '@/lib/pmTypes';
import { MINUTES_PER_DAY, clockToMinutes, formatClockLabel, minutesToClock } from '@/lib/timeOfDay';
import {
  DEFAULT_BUSINESS_HOURS_SLOT,
  addBusinessHoursSlot,
  businessHoursDayIssue,
  copyBusinessHoursDay,
  daySlots,
  withSlots,
} from './businessHours';
import { DAYS } from './constants';

const SELECT_ALL = '__all__';

interface BusinessHoursEditorProps {
  schedule: Record<string, BusinessHoursDay>;
  onChange: (schedule: Record<string, BusinessHoursDay>) => void;
}

/** Weekly open hours: each day can be closed or hold several time ranges. */
export function BusinessHoursEditor({ schedule, onChange }: BusinessHoursEditorProps) {
  return (
    <div className="@container divide-y divide-quiet-divider-light border-y border-quiet-divider-light">
      {DAYS.map(({ key, label }) => (
        <BusinessHoursDayRow
          key={key}
          dayKey={key}
          label={label}
          day={schedule[key]}
          onDayChange={(day) => onChange({ ...schedule, [key]: day })}
          onCopy={(toKeys) => onChange(copyBusinessHoursDay(schedule, key, toKeys))}
        />
      ))}
    </div>
  );
}

interface BusinessHoursDayRowProps {
  dayKey: string;
  label: string;
  day: BusinessHoursDay;
  onDayChange: (day: BusinessHoursDay) => void;
  onCopy: (toKeys: string[]) => void;
}

function BusinessHoursDayRow({ dayKey, label, day, onDayChange, onCopy }: BusinessHoursDayRowProps) {
  const slots = daySlots(day);
  const issue = businessHoursDayIssue(day);
  const withAddedSlot = addBusinessHoursSlot(day);

  const setSlots = (next: BusinessHoursSlot[]) => {
    onDayChange(next.length > 0 ? withSlots(day, next) : withSlots({ ...day, enabled: false }, [DEFAULT_BUSINESS_HOURS_SLOT]));
  };
  const setStart = (index: number, start: string) => {
    const slot = slots[index];
    const startMinutes = clockToMinutes(start) ?? 0;
    const endMinutes = clockToMinutes(slot.end) ?? 0;
    // Keep the range valid by moving the end along when the start passes it.
    const end = endMinutes > startMinutes ? slot.end : minutesToClock(Math.min(startMinutes + 60, MINUTES_PER_DAY));
    setSlots(slots.map((s, i) => (i === index ? { start, end } : s)));
  };

  return (
    // Narrow sections put the time ranges on their own line under the day.
    <div className="flex flex-wrap items-start gap-x-3 gap-y-1 py-2.5 @md:flex-nowrap">
      <label className="order-1 flex w-32 shrink-0 cursor-pointer items-center gap-2.5 pt-1.5">
        <Switch
          size="sm"
          checked={day.enabled}
          aria-label={`Open on ${label}`}
          onCheckedChange={(enabled) => onDayChange({ ...day, enabled })}
        />
        <span className="text-sm text-quiet-text-primary">{label}</span>
      </label>

      <div className="order-3 min-w-0 basis-full space-y-1 @md:order-2 @md:basis-auto @md:flex-1">
        {day.enabled ? (
          slots.map((slot, index) => (
            <div key={index} className="flex items-center gap-2">
              <QuietTimeSelect
                label={`${label} start time`}
                value={slot.start}
                invalid={issue !== null}
                onChange={(start) => setStart(index, start)}
              />
              <span className="text-quiet-muted" aria-hidden="true">–</span>
              <QuietTimeSelect
                label={`${label} end time`}
                value={slot.end}
                after={slot.start}
                endOfDay
                invalid={issue !== null}
                onChange={(end) => setSlots(slots.map((s, i) => (i === index ? { ...s, end } : s)))}
              />
              <QuietIconAction
                aria-label={`Remove ${label} ${formatClockLabel(slot.start)} to ${formatClockLabel(slot.end)}`}
                onClick={() => setSlots(slots.filter((_, i) => i !== index))}
              >
                <Cancel01Icon className="size-3.5" />
              </QuietIconAction>
            </div>
          ))
        ) : (
          <p className="pt-1.5 text-sm text-quiet-muted">Unavailable</p>
        )}
        {issue && <p role="alert" className="text-xs text-destructive">{issue}</p>}
      </div>

      <div className="order-2 ml-auto flex shrink-0 items-center gap-0.5 pt-0.5 @md:order-3">
        <QuickTooltip label={withAddedSlot ? 'Add time range' : 'No time left in this day'}>
          <span>
            <QuietIconAction
              aria-label={`Add a time range on ${label}`}
              disabled={!withAddedSlot}
              onClick={() => withAddedSlot && onDayChange(withAddedSlot)}
            >
              <PlusSignIcon className="size-3.5" />
            </QuietIconAction>
          </span>
        </QuickTooltip>
        <CopyTimesMenu sourceKey={dayKey} sourceLabel={label} onApply={onCopy} />
      </div>
    </div>
  );
}

interface CopyTimesMenuProps {
  sourceKey: string;
  sourceLabel: string;
  onApply: (toKeys: string[]) => void;
}

/** Copies one day's hours to the days picked in the menu. */
function CopyTimesMenu({ sourceKey, sourceLabel, onApply }: CopyTimesMenuProps) {
  const [open, setOpen] = useState(false);
  const [targets, setTargets] = useState<string[]>([]);
  const otherKeys = DAYS.map((day) => day.key).filter((key) => key !== sourceKey);
  const allSelected = otherKeys.every((key) => targets.includes(key));

  const close = () => {
    setOpen(false);
    setTargets([]);
  };
  const toggle = (value: string) => {
    if (value === SELECT_ALL) {
      setTargets(allSelected ? [] : otherKeys);
      return;
    }
    setTargets((prev) => (prev.includes(value) ? prev.filter((key) => key !== value) : [...prev, value]));
  };

  return (
    <QuietDropdown
      label={`Copy ${sourceLabel} times`}
      open={open}
      onOpenChange={(next) => (next ? setOpen(true) : close())}
      multiple
      searchMode="off"
      selected={[sourceKey, ...targets, ...(allSelected ? [SELECT_ALL] : [])]}
      onSelect={toggle}
      groups={[{
        id: 'days',
        label: 'Copy times to',
        options: [
          { value: SELECT_ALL, label: 'Select all' },
          ...DAYS.map((day) => ({ value: day.key, label: day.label, disabled: day.key === sourceKey })),
        ],
      }]}
      contentClassName="w-48"
      contentProps={{ align: 'end' }}
      triggerWrapper={(trigger) => <QuickTooltip label="Copy times to other days">{trigger}</QuickTooltip>}
      trigger={
        <QuietIconAction aria-label={`Copy ${sourceLabel} times to other days`}>
          <Copy01Icon className="size-3.5" />
        </QuietIconAction>
      }
      footer={
        <QuietPrimaryAction
          className="w-full"
          disabled={targets.length === 0}
          onClick={() => {
            onApply(targets);
            close();
          }}
        >
          Apply
        </QuietPrimaryAction>
      }
    />
  );
}
