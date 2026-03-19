import { useMemo, useState } from 'react';
import { CalendarDays, CheckCircle2, Clock3 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import type {
  RecurringDueDateMode,
  RecurringFrequency,
  RecurringScheduleType,
  RecurringSprintAssignmentMode,
  RecurringTemplateConfig,
  WorkflowState,
} from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

export interface RecurringTemplateFormValue {
  title: string;
  description: string;
  config: RecurringTemplateConfig;
}

interface RecurringTemplateFormProps {
  initialValue?: Partial<RecurringTemplateFormValue>;
  workflowStates?: WorkflowState[];
  submitLabel?: string;
  saving?: boolean;
  onSubmit: (value: RecurringTemplateFormValue) => void;
  onCancel?: () => void;
  onRemove?: () => void;
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

function chip(active: boolean) {
  return cn(
    'rounded-md border px-2.5 py-1.5 text-xs font-medium transition-colors',
    active
      ? 'border-primary/40 bg-primary/10 text-primary'
      : 'border-border text-muted-foreground hover:bg-accent hover:text-foreground',
  );
}

export function RecurringTemplateForm({
  initialValue,
  workflowStates = [],
  submitLabel = 'Save recurrence',
  saving = false,
  onSubmit,
  onCancel,
  onRemove,
}: RecurringTemplateFormProps) {
  const initialConfig = initialValue?.config;
  const [title, setTitle] = useState(initialValue?.title ?? '');
  const [scheduleType, setScheduleType] = useState<RecurringScheduleType>(initialConfig?.schedule_type ?? 'time');
  const [frequency, setFrequency] = useState<RecurringFrequency>(initialConfig?.frequency ?? 'weekly');
  const [interval, setInterval] = useState(initialConfig?.interval ?? 1);
  const [weekdays, setWeekdays] = useState<number[]>(initialConfig?.weekdays ?? [1]);
  const [dayOfMonth, setDayOfMonth] = useState(initialConfig?.day_of_month ?? 1);
  const [completionStateIDs, setCompletionStateIDs] = useState<string[]>(initialConfig?.completion_state_ids ?? []);
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
    () => [...weekdays].sort((left, right) => weekdayOptions.findIndex((o) => o.value === left) - weekdayOptions.findIndex((o) => o.value === right)),
    [weekdays],
  );

  const handleWeekdayToggle = (weekday: number) => {
    setWeekdays((current) => {
      if (current.includes(weekday)) {
        if (current.length === 1) return current;
        return current.filter((v) => v !== weekday);
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
      if (frequency === 'weekly') config.weekdays = normalizedWeekdays;
      if (frequency === 'monthly' || frequency === 'yearly') config.day_of_month = Math.min(31, Math.max(1, dayOfMonth || 1));
    } else {
      if (completionStateIDs.length > 0) {
        config.completion_state_ids = completionStateIDs;
      } else {
        config.completion_event = 'done_state';
      }
    }

    if (dueDateMode === 'offset_days') config.due_offset_days = Math.max(1, dueOffsetDays || 1);
    if (startsOn) config.starts_on = `${startsOn}T00:00:00Z`;
    if (endMode === 'date' && endsOn) config.ends_on = `${endsOn}T00:00:00Z`;
    if (endMode === 'count') config.ends_after_occurrences = Math.max(1, endsAfterOccurrences || 1);

    onSubmit({ title: trimmedTitle, description: '', config });
  };

  return (
    <div className="space-y-4">
      {/* Template name */}
      <div className="space-y-1.5">
        <Label>Name</Label>
        <Input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="e.g. Weekly planning review"
        />
      </div>

      {/* Trigger type */}
      <div className="space-y-1.5">
        <Label>Trigger</Label>
        <div className="flex flex-wrap gap-2">
          <button type="button" className={chip(scheduleType === 'time')} onClick={() => setScheduleType('time')}>
            <Clock3 className="mr-1 inline h-3 w-3" /> Time-based
          </button>
          <button type="button" className={chip(scheduleType === 'completion')} onClick={() => setScheduleType('completion')}>
            <CheckCircle2 className="mr-1 inline h-3 w-3" /> On completion
          </button>
        </div>
        <p className="text-[11px] text-muted-foreground">
          {scheduleType === 'time'
            ? 'Creates a new story on a fixed schedule (daily, weekly, etc.)'
            : 'Creates a new story when the current one is marked done'}
        </p>
      </div>

      {/* Schedule config */}
      {scheduleType === 'time' ? (
        <div className="space-y-3 rounded-lg border border-border/60 bg-muted/20 p-3">
          <div className="space-y-1.5">
            <Label>Frequency</Label>
            <div className="flex flex-wrap gap-2">
              {(['daily', 'weekly', 'monthly', 'yearly'] as const).map((v) => (
                <button key={v} type="button" className={chip(frequency === v)} onClick={() => setFrequency(v)}>
                  {v[0].toUpperCase() + v.slice(1)}
                </button>
              ))}
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label>Repeat every</Label>
              <div className="flex items-center gap-2">
                <Input
                  type="number"
                  min={1}
                  className="w-20"
                  value={interval}
                  onChange={(e) => setInterval(Number(e.target.value) || 1)}
                />
                <span className="text-xs text-muted-foreground">{frequency === 'daily' ? 'day(s)' : frequency === 'weekly' ? 'week(s)' : frequency === 'monthly' ? 'month(s)' : 'year(s)'}</span>
              </div>
            </div>

            {(frequency === 'monthly' || frequency === 'yearly') && (
              <div className="space-y-1.5">
                <Label>Day of month</Label>
                <Input type="number" min={1} max={31} className="w-20" value={dayOfMonth} onChange={(e) => setDayOfMonth(Number(e.target.value) || 1)} />
              </div>
            )}
          </div>

          {frequency === 'weekly' && (
            <div className="space-y-1.5">
              <Label>On days</Label>
              <div className="flex flex-wrap gap-1.5">
                {weekdayOptions.map((o) => (
                  <button key={o.label} type="button" className={chip(weekdays.includes(o.value))} onClick={() => handleWeekdayToggle(o.value)}>
                    {o.label}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      ) : (
        <div className="space-y-2 rounded-lg border border-border/60 bg-muted/20 p-3">
          <Label>Trigger when story moves to</Label>
          {workflowStates.length > 0 ? (
            <div className="flex flex-wrap gap-1.5">
              {workflowStates.map((state) => {
                const selected = completionStateIDs.includes(state.id);
                return (
                  <button
                    key={state.id}
                    type="button"
                    onClick={() => {
                      setCompletionStateIDs((prev) =>
                        selected ? prev.filter((id) => id !== state.id) : [...prev, state.id]
                      );
                    }}
                    className={cn(
                      'inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-xs font-medium transition-colors',
                      selected
                        ? 'border-primary/40 bg-primary/10 text-primary'
                        : 'border-border text-muted-foreground hover:bg-accent hover:text-foreground',
                    )}
                  >
                    <span
                      className="h-2.5 w-2.5 rounded-full shrink-0"
                      style={{ backgroundColor: state.color || '#9ca3af' }}
                    />
                    {state.name}
                  </button>
                );
              })}
            </div>
          ) : (
            <p className="text-xs text-muted-foreground">No workflow states available. Select a team first.</p>
          )}
          <p className="text-[11px] text-muted-foreground">
            {completionStateIDs.length === 0
              ? 'Select one or more states. A new story is created when the current one enters any selected state.'
              : `Triggers on: ${completionStateIDs.length} state${completionStateIDs.length === 1 ? '' : 's'} selected`}
          </p>
        </div>
      )}

      {/* Due date */}
      <div className="space-y-1.5">
        <Label>Due date</Label>
        <div className="flex flex-wrap gap-2">
          <button type="button" className={chip(dueDateMode === 'none')} onClick={() => setDueDateMode('none')}>None</button>
          <button type="button" className={chip(dueDateMode === 'scheduled_date')} onClick={() => setDueDateMode('scheduled_date')}>Same as schedule</button>
          <button type="button" className={chip(dueDateMode === 'offset_days')} onClick={() => setDueDateMode('offset_days')}>Days after creation</button>
        </div>
        {dueDateMode === 'offset_days' && (
          <div className="flex items-center gap-2 pt-1">
            <Input type="number" min={1} className="w-20" value={dueOffsetDays} onChange={(e) => setDueOffsetDays(Number(e.target.value) || 1)} />
            <span className="text-xs text-muted-foreground">days after story is created</span>
          </div>
        )}
      </div>

      {/* Start / End */}
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label>Starts on</Label>
          <div className="relative">
            <CalendarDays className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input type="date" value={startsOn} onChange={(e) => setStartsOn(e.target.value)} className="pl-9" />
          </div>
          <p className="text-[11px] text-muted-foreground">Leave empty to start immediately</p>
        </div>

        <div className="space-y-1.5">
          <Label>Ends</Label>
          <div className="flex flex-wrap gap-2">
            <button type="button" className={chip(endMode === 'never')} onClick={() => setEndMode('never')}>Never</button>
            <button type="button" className={chip(endMode === 'date')} onClick={() => setEndMode('date')}>On date</button>
            <button type="button" className={chip(endMode === 'count')} onClick={() => setEndMode('count')}>After</button>
          </div>
          {endMode === 'date' && <Input type="date" value={endsOn} onChange={(e) => setEndsOn(e.target.value)} className="mt-1" />}
          {endMode === 'count' && (
            <div className="flex items-center gap-2 mt-1">
              <Input type="number" min={1} className="w-20" value={endsAfterOccurrences} onChange={(e) => setEndsAfterOccurrences(Number(e.target.value) || 1)} />
              <span className="text-xs text-muted-foreground">occurrences</span>
            </div>
          )}
        </div>
      </div>

      {/* Sprint assignment */}
      <div className="space-y-1.5">
        <Label>Sprint assignment</Label>
        <div className="flex flex-wrap gap-2">
          {([
            { value: 'none', label: 'No sprint' },
            { value: 'current_sprint', label: 'Current sprint' },
            { value: 'by_due_date', label: 'By due date' },
          ] as const).map((o) => (
            <button key={o.value} type="button" className={chip(sprintAssignmentMode === o.value)} onClick={() => setSprintAssignmentMode(o.value)}>
              {o.label}
            </button>
          ))}
        </div>
        <p className="text-[11px] text-muted-foreground">
          {sprintAssignmentMode === 'none' ? 'Story will not be assigned to any sprint' : sprintAssignmentMode === 'current_sprint' ? 'Assigned to whichever sprint is active when created' : 'Assigned to the sprint that contains the due date'}
        </p>
      </div>

      {/* Actions */}
      <div className="flex items-center gap-2 pt-1">
        {onRemove && (
          <Button type="button" variant="ghost" size="sm" className="text-destructive hover:text-destructive" onClick={onRemove}>
            Remove
          </Button>
        )}
        <div className="flex-1" />
        {onCancel && <Button type="button" variant="ghost" size="sm" onClick={onCancel}>Cancel</Button>}
        <Button type="button" size="sm" disabled={saving || !title.trim()} onClick={handleSubmit}>
          {submitLabel}
        </Button>
      </div>
    </div>
  );
}
