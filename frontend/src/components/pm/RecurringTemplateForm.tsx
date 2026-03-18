import { useMemo, useState } from 'react';
import { CalendarDays, CheckCircle2, Clock3, RotateCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import type {
  RecurringCompletionEvent,
  RecurringDueDateMode,
  RecurringFrequency,
  RecurringScheduleType,
  RecurringSprintAssignmentMode,
  RecurringTemplateConfig,
} from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

export interface RecurringTemplateFormValue {
  title: string;
  description: string;
  config: RecurringTemplateConfig;
}

interface RecurringTemplateFormProps {
  initialValue?: Partial<RecurringTemplateFormValue>;
  submitLabel?: string;
  saving?: boolean;
  onSubmit: (value: RecurringTemplateFormValue) => void;
  onCancel?: () => void;
}

const weekdayOptions = [
  { value: 1, label: 'Mon' },
  { value: 2, label: 'Tue' },
  { value: 3, label: 'Wed' },
  { value: 4, label: 'Thu' },
  { value: 5, label: 'Fri' },
  { value: 6, label: 'Sat' },
  { value: 0, label: 'Sun' },
] as const;

function toDateInput(value?: string) {
  return value ? value.slice(0, 10) : '';
}

function buttonClass(active: boolean) {
  return cn(
    'rounded-md border px-2.5 py-1.5 text-xs font-medium transition-colors',
    active
      ? 'border-primary/40 bg-primary/10 text-primary'
      : 'border-border text-muted-foreground hover:bg-accent hover:text-foreground',
  );
}

export function RecurringTemplateForm({
  initialValue,
  submitLabel = 'Save recurrence',
  saving = false,
  onSubmit,
  onCancel,
}: RecurringTemplateFormProps) {
  const initialConfig = initialValue?.config;
  const [title, setTitle] = useState(initialValue?.title ?? '');
  const [description, setDescription] = useState(initialValue?.description ?? '');
  const [scheduleType, setScheduleType] = useState<RecurringScheduleType>(initialConfig?.schedule_type ?? 'time');
  const [frequency, setFrequency] = useState<RecurringFrequency>(initialConfig?.frequency ?? 'weekly');
  const [interval, setInterval] = useState(initialConfig?.interval ?? 1);
  const [weekdays, setWeekdays] = useState<number[]>(initialConfig?.weekdays ?? [1]);
  const [dayOfMonth, setDayOfMonth] = useState(initialConfig?.day_of_month ?? 1);
  const [completionEvent, setCompletionEvent] = useState<RecurringCompletionEvent>(initialConfig?.completion_event ?? 'done_state');
  const [dueDateMode, setDueDateMode] = useState<RecurringDueDateMode>(initialConfig?.due_date_mode ?? 'scheduled_date');
  const [dueOffsetDays, setDueOffsetDays] = useState(initialConfig?.due_offset_days ?? 1);
  const [startsOn, setStartsOn] = useState(toDateInput(initialConfig?.starts_on));
  const [endsOn, setEndsOn] = useState(toDateInput(initialConfig?.ends_on));
  const [endsAfterOccurrences, setEndsAfterOccurrences] = useState(initialConfig?.ends_after_occurrences ?? 5);
  const [endMode, setEndMode] = useState<'never' | 'date' | 'count'>(
    initialConfig?.ends_on ? 'date' : initialConfig?.ends_after_occurrences ? 'count' : 'never',
  );
  const [sprintAssignmentMode, setSprintAssignmentMode] = useState<RecurringSprintAssignmentMode>(
    initialConfig?.sprint_assignment_mode ?? 'none',
  );

  const normalizedWeekdays = useMemo(
    () => [...weekdays].sort((left, right) => weekdayOptions.findIndex((option) => option.value === left) - weekdayOptions.findIndex((option) => option.value === right)),
    [weekdays],
  );

  const handleWeekdayToggle = (weekday: number) => {
    setWeekdays((current) => {
      if (current.includes(weekday)) {
        if (current.length === 1) return current;
        return current.filter((value) => value !== weekday);
      }
      return [...current, weekday];
    });
  };

  const handleSubmit = () => {
    const trimmedTitle = title.trim();
    if (!trimmedTitle) return;

    const config: RecurringTemplateConfig = {
      schedule_type: scheduleType,
      due_date_mode: dueDateMode,
      sprint_assignment_mode: sprintAssignmentMode,
    };

    if (scheduleType === 'time') {
      config.frequency = frequency;
      config.interval = Math.max(1, interval || 1);
      if (frequency === 'weekly') {
        config.weekdays = normalizedWeekdays;
      }
      if (frequency === 'monthly' || frequency === 'yearly') {
        config.day_of_month = Math.min(31, Math.max(1, dayOfMonth || 1));
      }
    } else {
      config.completion_event = completionEvent;
    }

    if (dueDateMode === 'offset_days') {
      config.due_offset_days = Math.max(1, dueOffsetDays || 1);
    }
    if (startsOn) {
      config.starts_on = `${startsOn}T00:00:00Z`;
    }
    if (endMode === 'date' && endsOn) {
      config.ends_on = `${endsOn}T00:00:00Z`;
    }
    if (endMode === 'count') {
      config.ends_after_occurrences = Math.max(1, endsAfterOccurrences || 1);
    }

    onSubmit({
      title: trimmedTitle,
      description: description.trim(),
      config,
    });
  };

  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <label className="text-xs font-medium text-muted-foreground">Template title</label>
        <Input
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="Weekly planning review"
        />
      </div>

      <div className="space-y-2">
        <label className="text-xs font-medium text-muted-foreground">Internal note</label>
        <Textarea
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          placeholder="Optional note for operators"
          className="min-h-20 resize-none"
        />
      </div>

      <div className="space-y-2">
        <div className="text-xs font-medium text-muted-foreground">Trigger</div>
        <div className="flex flex-wrap gap-2">
          <button type="button" className={buttonClass(scheduleType === 'time')} onClick={() => setScheduleType('time')}>
            <Clock3 className="mr-1 inline h-3 w-3" />
            Time-based
          </button>
          <button type="button" className={buttonClass(scheduleType === 'completion')} onClick={() => setScheduleType('completion')}>
            <CheckCircle2 className="mr-1 inline h-3 w-3" />
            On completion
          </button>
        </div>
      </div>

      {scheduleType === 'time' ? (
        <div className="space-y-4 rounded-lg border border-border/60 bg-muted/20 p-3">
          <div className="space-y-2">
            <div className="text-xs font-medium text-muted-foreground">Frequency</div>
            <div className="flex flex-wrap gap-2">
              {(['daily', 'weekly', 'monthly', 'yearly'] as const).map((value) => (
                <button
                  key={value}
                  type="button"
                  className={buttonClass(frequency === value)}
                  onClick={() => setFrequency(value)}
                >
                  {value[0].toUpperCase() + value.slice(1)}
                </button>
              ))}
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-2">
              <label className="text-xs font-medium text-muted-foreground">Every</label>
              <Input
                type="number"
                min={1}
                value={interval}
                onChange={(event) => setInterval(Number(event.target.value) || 1)}
              />
            </div>

            {(frequency === 'monthly' || frequency === 'yearly') && (
              <div className="space-y-2">
                <label className="text-xs font-medium text-muted-foreground">Day of month</label>
                <Input
                  type="number"
                  min={1}
                  max={31}
                  value={dayOfMonth}
                  onChange={(event) => setDayOfMonth(Number(event.target.value) || 1)}
                />
              </div>
            )}
          </div>

          {frequency === 'weekly' && (
            <div className="space-y-2">
              <div className="text-xs font-medium text-muted-foreground">Weekdays</div>
              <div className="flex flex-wrap gap-2">
                {weekdayOptions.map((option) => (
                  <button
                    key={option.label}
                    type="button"
                    className={buttonClass(weekdays.includes(option.value))}
                    onClick={() => handleWeekdayToggle(option.value)}
                  >
                    {option.label}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      ) : (
        <div className="space-y-2 rounded-lg border border-border/60 bg-muted/20 p-3">
          <div className="text-xs font-medium text-muted-foreground">Completion event</div>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              className={buttonClass(completionEvent === 'done_state')}
              onClick={() => setCompletionEvent('done_state')}
            >
              Done state
            </button>
            <button
              type="button"
              className={buttonClass(completionEvent === 'completed')}
              onClick={() => setCompletionEvent('completed')}
            >
              Completed flag
            </button>
          </div>
        </div>
      )}

      <div className="space-y-2">
        <div className="text-xs font-medium text-muted-foreground">Due date</div>
        <div className="flex flex-wrap gap-2">
          <button type="button" className={buttonClass(dueDateMode === 'none')} onClick={() => setDueDateMode('none')}>
            No due date
          </button>
          <button
            type="button"
            className={buttonClass(dueDateMode === 'scheduled_date')}
            onClick={() => setDueDateMode('scheduled_date')}
          >
            On schedule
          </button>
          <button
            type="button"
            className={buttonClass(dueDateMode === 'offset_days')}
            onClick={() => setDueDateMode('offset_days')}
          >
            Offset days
          </button>
        </div>
        {dueDateMode === 'offset_days' && (
          <Input
            type="number"
            min={1}
            value={dueOffsetDays}
            onChange={(event) => setDueOffsetDays(Number(event.target.value) || 1)}
          />
        )}
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <label className="text-xs font-medium text-muted-foreground">Starts on</label>
          <div className="relative">
            <CalendarDays className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input type="date" value={startsOn} onChange={(event) => setStartsOn(event.target.value)} className="pl-9" />
          </div>
        </div>

        <div className="space-y-2">
          <div className="text-xs font-medium text-muted-foreground">End condition</div>
          <div className="flex flex-wrap gap-2">
            <button type="button" className={buttonClass(endMode === 'never')} onClick={() => setEndMode('never')}>
              Never
            </button>
            <button type="button" className={buttonClass(endMode === 'date')} onClick={() => setEndMode('date')}>
              On date
            </button>
            <button type="button" className={buttonClass(endMode === 'count')} onClick={() => setEndMode('count')}>
              After count
            </button>
          </div>
          {endMode === 'date' && (
            <Input type="date" value={endsOn} onChange={(event) => setEndsOn(event.target.value)} />
          )}
          {endMode === 'count' && (
            <Input
              type="number"
              min={1}
              value={endsAfterOccurrences}
              onChange={(event) => setEndsAfterOccurrences(Number(event.target.value) || 1)}
            />
          )}
        </div>
      </div>

      <div className="space-y-2">
        <div className="text-xs font-medium text-muted-foreground">Sprint assignment</div>
        <div className="flex flex-wrap gap-2">
          {([
            { value: 'none', label: 'No sprint' },
            { value: 'current_sprint', label: 'Current sprint' },
            { value: 'by_due_date', label: 'By due date' },
          ] as const).map((option) => (
            <button
              key={option.value}
              type="button"
              className={buttonClass(sprintAssignmentMode === option.value)}
              onClick={() => setSprintAssignmentMode(option.value)}
            >
              <RotateCw className="mr-1 inline h-3 w-3" />
              {option.label}
            </button>
          ))}
        </div>
      </div>

      <div className="flex items-center justify-end gap-2">
        {onCancel ? (
          <Button type="button" variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
        ) : null}
        <Button type="button" disabled={saving || !title.trim()} onClick={handleSubmit}>
          {submitLabel}
        </Button>
      </div>
    </div>
  );
}
