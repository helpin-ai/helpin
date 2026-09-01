import { useState } from 'react';
import { format, isValid, parseISO } from 'date-fns';

import { ObjectivePicker, type ObjectivePickerSelection } from '@/components/pm/ObjectivePicker';
import { Calendar } from '@/components/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Calendar03Icon } from '@/lib/icons';
import type { EpicWithStats, Objective, UpdateEpicRequest } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

type EpicDateField = 'deadline' | 'planned_start_date';

function parseDate(value?: string) {
  if (!value) return undefined;
  const parsed = parseISO(value);
  return isValid(parsed) ? parsed : undefined;
}

export function InlineEpicDateControl({
  epicId,
  value,
  emptyLabel,
  ariaLabel,
  onUpdate,
  patchKey,
  minDate,
  maxDate,
  disabled = false,
  className,
}: {
  epicId: string;
  value?: string;
  emptyLabel: string;
  ariaLabel?: string;
  onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void>;
  patchKey: EpicDateField;
  minDate?: Date;
  maxDate?: Date;
  disabled?: boolean;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const selected = parseDate(value);
  const disabledDates = [
    ...(minDate ? [{ before: minDate }] : []),
    ...(maxDate ? [{ after: maxDate }] : []),
  ];

  if (disabled) {
    return (
      <span className={cn('inline-flex min-w-0 items-center gap-1.5 py-1 text-sm text-quiet-text-tertiary', className)}>
        <Calendar03Icon className="h-[15px] w-[15px] shrink-0 text-quiet-muted" />
        <span className="truncate">{selected ? format(selected, 'MMM d') : '—'}</span>
      </span>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={ariaLabel ?? `${emptyLabel} for epic`}
          className={cn(
            'inline-flex min-w-0 items-center gap-1.5 border-b border-quiet-field px-0.5 py-1 text-sm text-quiet-text-secondary transition-colors hover:border-quiet-text-primary hover:text-quiet-text-primary focus-visible:border-b-2 focus-visible:border-quiet-text-primary focus-visible:outline-none disabled:cursor-wait disabled:opacity-60',
            !selected && 'text-quiet-muted',
            className,
          )}
          onClick={(event) => {
            event.stopPropagation();
            setOpen(true);
          }}
        >
          <Calendar03Icon className="h-[15px] w-[15px] shrink-0 text-quiet-muted" />
          <span className="truncate">{selected ? format(selected, 'MMM d') : emptyLabel}</span>
        </button>
      </PopoverTrigger>
      {open ? (
        <PopoverContent
          className="w-auto p-0"
          align="start"
          side="bottom"
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <Calendar
            mode="single"
            selected={selected}
            defaultMonth={selected ?? minDate ?? maxDate}
            disabled={disabledDates.length > 0 ? disabledDates : undefined}
            onSelect={(date) => {
              if (!date) return;
              void onUpdate(epicId, {
                [patchKey]: format(date, 'yyyy-MM-dd'),
              });
              setOpen(false);
            }}
          />
        </PopoverContent>
      ) : null}
    </Popover>
  );
}

function buildSelectedObjectives(
  allObjectives: Objective[],
  linkedObjectives: EpicWithStats['objectives'],
): ObjectivePickerSelection[] {
  return (linkedObjectives ?? []).map((objective) => ({
    id: objective.id,
    name: objective.name,
    archived: !allObjectives.some((candidate) => candidate.id === objective.id),
  }));
}

export function InlineEpicObjectivesControl({
  entry,
  allObjectives,
  onChange,
  disabled = false,
  className,
}: {
  entry: EpicWithStats;
  allObjectives: Objective[];
  onChange: (epicId: string, objectiveIds: string[]) => Promise<void>;
  disabled?: boolean;
  className?: string;
}) {
  const objectives = entry.objectives ?? [];
  const summary = objectives.length > 0
    ? `${objectives[0].name}${objectives.length > 1 ? ` +${objectives.length - 1}` : ''}`
    : null;

  if (disabled) {
    return (
      <span
        className={cn('block min-w-0 truncate py-1 text-sm text-quiet-text-tertiary', className)}
        title={objectives.map((objective) => objective.name).join(', ')}
      >
        {summary ?? '—'}
      </span>
    );
  }

  return (
    <div
      onClick={(event) => event.stopPropagation()}
      className={cn('flex min-w-0 items-center gap-1.5', className)}
    >
      {summary ? (
        <span
          className="min-w-0 truncate text-sm font-medium text-foreground/90"
          title={objectives.map((objective) => objective.name).join(', ')}
        >
          {summary}
        </span>
      ) : null}
      <ObjectivePicker
        objectives={allObjectives}
        selectedObjectiveIds={objectives.map((objective) => objective.id)}
        selectedObjectives={buildSelectedObjectives(allObjectives, objectives)}
        onChange={(objectiveIds) => onChange(entry.epic.id, objectiveIds)}
        addLabel="Add objective"
        triggerOnly
      />
    </div>
  );
}
