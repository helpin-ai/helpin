import * as React from 'react';
import { differenceInDays, format, isBefore, isSameDay, startOfDay } from 'date-fns';
import { CalendarDays, CornerDownLeft, X } from 'lucide-react';
import type { DateRange, Matcher } from 'react-day-picker';
import { cn } from '@/lib/utils';
import {
  applyLinkedDateValue,
  type DatePickerKind,
  type LinkedDateValues,
  formatDateValue,
  formatShortDateValue,
  getDatePickerLabel,
  getDatePickerPresets,
  parseNaturalLanguageDate,
  parseStoredDate,
  preloadNaturalLanguageParser,
} from '@/lib/datePicker';
import { Button } from '@/components/ui/button';
import { Calendar } from '@/components/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';

interface LinkedDatePickerField {
  label?: string;
  value?: string;
  onChange: (value: string) => void;
  placeholder?: string;
  kind?: DatePickerKind;
  disablePast?: boolean;
}

interface DatePickerProps {
  /** ISO date string (yyyy-MM-dd) or empty */
  value?: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
  /** If true, dates before today are disabled */
  disablePast?: boolean;
  /** If true, hides the calendar icon inside the trigger */
  hideIcon?: boolean;
  /** If true, color the date red when overdue, amber when approaching (within 3 days) */
  urgencyColor?: boolean;
  /** If true, the item is completed and urgency colors should not apply */
  completed?: boolean;
  /** Semantic context for presets and labels */
  kind?: DatePickerKind;
  /** Optional linked date shown as a second header field inside the popover */
  linkedDate?: LinkedDatePickerField;
  /** Which field is active when the linked popover opens */
  defaultActiveField?: 'primary' | 'linked';
  /** Optional label shown in the popover header */
  label?: string;
  /** Which field the closed trigger should represent when linked dates are enabled */
  triggerField?: 'primary' | 'linked';
}

function useDebounce<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = React.useState(value);
  React.useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(id);
  }, [value, delay]);
  return debounced;
}

export function DatePicker({
  value,
  onChange,
  placeholder = 'Pick a date',
  className,
  disablePast,
  hideIcon,
  urgencyColor,
  completed,
  kind = 'generic',
  linkedDate,
  defaultActiveField = 'primary',
  label,
  triggerField = 'primary',
}: DatePickerProps) {
  const [open, setOpen] = React.useState(false);
  const [activeField, setActiveField] = React.useState<'primary' | 'linked'>(defaultActiveField);
  const [drafts, setDrafts] = React.useState<{ primary: string; linked: string }>({ primary: '', linked: '' });
  const [parseError, setParseError] = React.useState<string | null>(null);
  const [calendarMonth, setCalendarMonth] = React.useState<Date | undefined>(undefined);
  const externalValues = React.useMemo<LinkedDateValues>(() => ({
    primary: value ?? '',
    linked: linkedDate?.value ?? '',
  }), [linkedDate?.value, value]);
  const [sessionValues, setSessionValues] = React.useState<LinkedDateValues>(externalValues);

  const primaryIsStart = kind === 'start' || kind === 'generic';
  const visualFirstField: 'primary' | 'linked' = primaryIsStart ? 'primary' : 'linked';
  const visualSecondField: 'primary' | 'linked' = primaryIsStart ? 'linked' : 'primary';
  const currentValues = open ? sessionValues : externalValues;
  const triggerKey: 'primary' | 'linked' = linkedDate ? triggerField : 'primary';

  const selected = React.useMemo(() => parseStoredDate(currentValues.primary), [currentValues.primary]);
  const linkedSelected = React.useMemo(() => parseStoredDate(currentValues.linked), [currentValues.linked]);
  const triggerSelected = triggerKey === 'linked' ? linkedSelected : selected;
  const triggerPlaceholder = triggerKey === 'linked' ? (linkedDate?.placeholder ?? placeholder) : placeholder;
  const inputRefs = React.useRef<{ primary: HTMLInputElement | null; linked: HTMLInputElement | null }>({
    primary: null,
    linked: null,
  });

  const advanceToRef = React.useRef<'primary' | 'linked' | null>(null);

  const focusInputField = React.useCallback((field: 'primary' | 'linked', selectText = false) => {
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        const input = inputRefs.current[field];
        if (!input) return;
        input.focus();
        if (selectText) input.select();
      });
    });
  }, []);

  const today = React.useMemo(() => {
    const d = new Date();
    d.setHours(0, 0, 0, 0);
    return d;
  }, []);

  const urgency = React.useMemo(() => {
    if (!urgencyColor || !triggerSelected || completed) return null;
    const now = startOfDay(new Date());
    if (isBefore(triggerSelected, now)) return 'overdue';
    if (differenceInDays(triggerSelected, now) <= 3) return 'approaching';
    return null;
  }, [urgencyColor, triggerSelected, completed]);

  const activeKey = activeField === 'linked' && linkedDate ? 'linked' : 'primary';
  const activeKind = activeKey === 'linked' ? (linkedDate?.kind ?? 'generic') : kind;
  const activeSelected = activeKey === 'linked' ? linkedSelected : selected;
  const activeDisablePast = activeKey === 'linked' ? linkedDate?.disablePast : disablePast;

  const minDate = React.useMemo(() => {
    const candidates: Date[] = [];
    if (activeDisablePast) candidates.push(today);
    if ((activeKind === 'due' || activeKind === 'target' || activeKind === 'end') && selected && activeKey === 'linked') {
      candidates.push(selected);
    }
    if ((activeKind === 'due' || activeKind === 'target' || activeKind === 'end') && linkedSelected && activeKey === 'primary') {
      candidates.push(linkedSelected);
    }
    if (candidates.length === 0) return undefined;
    return candidates.reduce((latest, current) => (current > latest ? current : latest));
  }, [activeDisablePast, activeKey, activeKind, linkedSelected, selected, today]);

  const maxDate = React.useMemo(() => {
    if (activeKind !== 'start') return undefined;
    if (activeKey === 'primary') return linkedSelected;
    return selected;
  }, [activeKey, activeKind, linkedSelected, selected]);

  const presets = React.useMemo(() => getDatePickerPresets(activeKind), [activeKind]);

  // Fix #1: Atomic apply — when setting start past end (or vice-versa), clear the opposing bound
  React.useEffect(() => {
    if (!open) {
      setSessionValues(externalValues);
    }
  }, [externalValues, open]);

  const applyValue = React.useCallback((field: 'primary' | 'linked', nextValue: string): LinkedDateValues => {
    const baseValues = open ? sessionValues : externalValues;
    const nextValues = linkedDate
      ? applyLinkedDateValue(baseValues, field, nextValue, primaryIsStart)
      : { ...baseValues, [field]: nextValue };

    setSessionValues(nextValues);

    if (nextValues.primary !== baseValues.primary) {
      onChange(nextValues.primary);
    }

    if (linkedDate && nextValues.linked !== baseValues.linked) {
      linkedDate.onChange(nextValues.linked);
    }

    return nextValues;
  }, [externalValues, linkedDate, onChange, open, primaryIsStart, sessionValues]);

  const applyAndAdvance = React.useCallback((nextValue: string) => {
    const nextValues = applyValue(activeKey, nextValue);
    const nextDrafts = {
      primary: formatDateValue(nextValues.primary, ''),
      linked: formatDateValue(nextValues.linked, ''),
    };
    setDrafts(nextDrafts);
    setParseError(null);

    if (linkedDate && activeKey === visualFirstField) {
      advanceToRef.current = visualSecondField;
      setActiveField(visualSecondField);
      if (!nextValues[visualSecondField]) {
        setDrafts((current) => ({ ...current, [visualSecondField]: '' }));
      }
      focusInputField(visualSecondField, true);
    } else if (!linkedDate) {
      setOpen(false);
    }
  }, [activeKey, applyValue, focusInputField, linkedDate, visualFirstField, visualSecondField]);

  React.useEffect(() => {
    if (!open) return;
    setActiveField(linkedDate && defaultActiveField === 'linked' ? 'linked' : 'primary');
    setSessionValues(externalValues);
    setDrafts({
      primary: externalValues.primary ? formatDateValue(externalValues.primary, '') : '',
      linked: externalValues.linked ? formatDateValue(externalValues.linked, '') : '',
    });
    setParseError(null);
    setCalendarMonth(undefined);
    preloadNaturalLanguageParser();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const handlePresetSelect = (nextValue: string) => {
    applyAndAdvance(nextValue);
  };

  const handleClear = () => {
    const nextValues = applyValue(activeKey, '');
    setDrafts({
      primary: formatDateValue(nextValues.primary, ''),
      linked: formatDateValue(nextValues.linked, ''),
    });
    setParseError(null);
  };

  const handleDraftApply = () => {
    const result = parseNaturalLanguageDate(drafts[activeKey], {
      min: minDate,
      max: maxDate,
    });

    if (!result.value) {
      setParseError(result.error ?? 'Could not understand that date.');
      return;
    }

    applyAndAdvance(result.value);
  };

  const handleTodayClick = () => {
    setCalendarMonth(today);
  };

  const disabledDays = React.useMemo(() => {
    const matchers: Matcher[] = [];
    if (minDate) matchers.push({ before: minDate });
    if (maxDate) matchers.push({ after: maxDate });
    return matchers.length > 0 ? matchers : undefined;
  }, [maxDate, minDate]);

  // Compute range for visual highlighting
  const calendarRange = React.useMemo((): DateRange | undefined => {
    if (!linkedDate) return selected ? { from: selected, to: selected } : undefined;
    const isStartKind = kind === 'start' || kind === 'generic';
    const startDate = isStartKind ? selected : linkedSelected;
    const endDate = isStartKind ? linkedSelected : selected;
    if (!startDate && !endDate) return undefined;
    if (startDate && !endDate) return { from: startDate, to: undefined };
    if (!startDate && endDate) return { from: endDate, to: undefined };
    return { from: startDate, to: endDate };
  }, [kind, linkedDate, linkedSelected, selected]);

  // Check if a preset matches the active date
  const activePresetId = React.useMemo(() => {
    if (!activeSelected) return null;
    for (const preset of presets) {
      const presetDate = parseStoredDate(preset.value);
      if (presetDate && isSameDay(presetDate, activeSelected)) return preset.id;
    }
    return null;
  }, [activeSelected, presets]);

  // Fix #3: Debounce NLP suggestion to avoid parsing on every keystroke
  const debouncedDraft = useDebounce(drafts[activeKey], 200);
  const nlpSuggestion = React.useMemo(() => {
    const draft = debouncedDraft.trim();
    if (!draft) return null;
    if (/^[A-Z][a-z]{2}\s\d{1,2},\s\d{4}$/.test(draft)) return null;
    const result = parseNaturalLanguageDate(draft, { min: minDate, max: maxDate });
    if (!result.value) return null;
    const parsed = parseStoredDate(result.value);
    if (!parsed) return null;
    return { value: result.value, label: format(parsed, 'EEEE, MMM d') };
  }, [debouncedDraft, maxDate, minDate]);

  React.useEffect(() => {
    if (open) {
      if (advanceToRef.current === activeKey) {
        advanceToRef.current = null;
        return;
      }
      focusInputField(activeKey);
    }
  }, [activeKey, focusInputField, open]);

  const primaryLabel = label ?? getDatePickerLabel(kind);
  const linkedLabel = linkedDate?.label ?? getDatePickerLabel(linkedDate?.kind ?? 'generic');

  // Fix #2: Only intercept Tab; never intercept ArrowLeft/ArrowRight
  const handleTabKeyDown = (e: React.KeyboardEvent<HTMLInputElement>, field: 'primary' | 'linked') => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleDraftApply();
      return;
    }
    if (!linkedDate) return;
    if (e.key === 'Tab') {
      const otherField = field === 'primary' ? 'linked' : 'primary';
      e.preventDefault();
      setActiveField(otherField);
      setParseError(null);
      focusInputField(otherField);
    }
  };

  // Fix #4: Semantic button for tab fields, button for NLP suggestion
  const renderTabField = (
    field: 'primary' | 'linked',
    fieldLabel: string,
    fieldValue?: string,
  ) => {
    const isActive = activeKey === field;

    return (
      <div className="relative" role="tabpanel">
        {isActive ? (
          <div
            className="flex h-8 w-[13rem] items-center gap-1.5 rounded-md border border-ring bg-background px-2.5 text-sm text-foreground shadow-sm"
          >
            <CalendarDays className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <input
              ref={(el) => { inputRefs.current[field] = el; }}
              type="text"
              value={drafts[field]}
              onChange={(event) => {
                setDrafts((current) => ({ ...current, [field]: event.target.value }));
                if (parseError) setParseError(null);
              }}
              onKeyDown={(e) => handleTabKeyDown(e, field)}
              placeholder={fieldLabel}
              aria-label={fieldLabel}
              className="h-full min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground/60"
            />
            {fieldValue && (
              <button
                type="button"
                aria-label={`Clear ${fieldLabel}`}
                className="shrink-0 rounded p-0.5 text-muted-foreground hover:text-foreground"
                onClick={() => {
                  applyValue(field, '');
                  setDrafts((current) => ({ ...current, [field]: '' }));
                  setParseError(null);
                }}
              >
                <X className="h-3 w-3" />
              </button>
            )}
          </div>
        ) : (
          <button
            type="button"
            role="tab"
            aria-selected={false}
            className="flex h-8 w-[13rem] cursor-pointer items-center gap-1.5 rounded-md px-2.5 text-sm text-muted-foreground transition-all hover:text-foreground"
            onClick={() => {
              setActiveField(field);
              setParseError(null);
              focusInputField(field);
            }}
          >
            <CalendarDays className="h-3.5 w-3.5 shrink-0" />
            <span className="min-w-0 flex-1 truncate text-left">
              {fieldValue ? formatShortDateValue(fieldValue, '') : fieldLabel}
            </span>
          </button>
        )}
        {/* NLP suggestion dropdown */}
        {isActive && nlpSuggestion && (
          <button
            type="button"
            className="absolute left-0 top-full z-50 mt-1 flex w-max items-center gap-2 rounded-md border bg-popover px-2.5 py-1.5 text-left shadow-md transition-colors hover:bg-accent"
            onClick={() => handleDraftApply()}
          >
            <span className="text-sm">{nlpSuggestion.label}</span>
            <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
              Return <CornerDownLeft className="h-3 w-3" />
            </span>
          </button>
        )}
      </div>
    );
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          className={cn(
            'justify-start text-left font-normal',
            !value && 'text-muted-foreground',
            urgency === 'overdue' && 'text-red-600 dark:text-red-400',
            urgency === 'approaching' && 'text-amber-600 dark:text-amber-400',
            className,
          )}
        >
          {!hideIcon && <CalendarDays className="mr-2 h-3.5 w-3.5" />}
          {triggerSelected ? format(triggerSelected, 'MMM d, yyyy') : triggerPlaceholder}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-auto gap-0 overflow-hidden p-0" align="start" side="bottom" collisionPadding={8}>
        {/* Tab header */}
        <div className="flex gap-1.5 p-2 pb-1.5" role="tablist">
          {linkedDate ? (
            primaryIsStart ? (
              <>
                {renderTabField('primary', primaryLabel, currentValues.primary)}
                {renderTabField('linked', linkedLabel, currentValues.linked)}
              </>
            ) : (
              <>
                {renderTabField('linked', linkedLabel, currentValues.linked)}
                {renderTabField('primary', primaryLabel, currentValues.primary)}
              </>
            )
          ) : (
            renderTabField('primary', primaryLabel, currentValues.primary)
          )}
        </div>
        {parseError && (
          <p className="px-3 pb-1 text-xs text-destructive" role="alert">{parseError}</p>
        )}
        <Separator />
        {/* Presets + Calendar */}
        <div className="flex flex-col md:flex-row">
          <ScrollArea className="max-h-64 md:max-h-none md:w-52">
            <div className="flex flex-col gap-0.5 p-1.5" role="listbox" aria-label="Date presets">
              {presets.map((preset) => {
                const isActivePreset = activePresetId === preset.id;
                return (
                  <button
                    key={`${activeKind}-${preset.id}`}
                    type="button"
                    role="option"
                    aria-selected={isActivePreset}
                    className={cn(
                      'flex items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-left transition-colors',
                      isActivePreset
                        ? 'bg-primary/10 text-primary font-medium'
                        : 'hover:bg-accent',
                    )}
                    onClick={() => handlePresetSelect(preset.value)}
                  >
                    <span className="text-sm">{preset.label}</span>
                    <span className={cn('text-xs', isActivePreset ? 'text-primary/70' : 'text-muted-foreground')}>
                      {preset.helper}
                    </span>
                  </button>
                );
              })}
              <button
                type="button"
                className="flex items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-left text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                onClick={handleClear}
              >
                <span>Clear</span>
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          </ScrollArea>
          <Separator orientation="vertical" className="hidden md:block" />
          <Separator className="md:hidden" />
          <div className="flex flex-1 flex-col items-start">
            {/* Today button */}
            <div className="flex w-full justify-end px-3 pt-2">
              <button
                type="button"
                className="text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
                onClick={handleTodayClick}
              >
                Today
              </button>
            </div>
            <Calendar
              mode="range"
              selected={calendarRange}
              onSelect={(_range, selectedDay) => {
                if (selectedDay) {
                  applyAndAdvance(format(selectedDay, 'yyyy-MM-dd'));
                }
              }}
              month={calendarMonth}
              onMonthChange={setCalendarMonth}
              defaultMonth={activeSelected ?? calendarRange?.from ?? today}
              {...(disabledDays ? { disabled: disabledDays } : {})}
            />
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
