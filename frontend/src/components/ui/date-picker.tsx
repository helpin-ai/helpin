import * as React from 'react';
import { differenceInDays, format, isBefore, parseISO, startOfDay } from 'date-fns';
import { CalendarDays } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Calendar } from '@/components/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';

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
}

export function DatePicker({ value, onChange, placeholder = 'Pick a date', className, disablePast, hideIcon, urgencyColor, completed }: DatePickerProps) {
  const [open, setOpen] = React.useState(false);

  const selected = React.useMemo(() => {
    if (!value) return undefined;
    try {
      return parseISO(value);
    } catch {
      return undefined;
    }
  }, [value]);

  const today = React.useMemo(() => {
    const d = new Date();
    d.setHours(0, 0, 0, 0);
    return d;
  }, []);

  const urgency = React.useMemo(() => {
    if (!urgencyColor || !selected || completed) return null;
    const now = startOfDay(new Date());
    if (isBefore(selected, now)) return 'overdue';
    if (differenceInDays(selected, now) <= 3) return 'approaching';
    return null;
  }, [urgencyColor, selected, completed]);

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
          {selected ? format(selected, 'MMM d, yyyy') : placeholder}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0" align="start">
        <Calendar
          mode="single"
          selected={selected}
          onSelect={(date) => {
            onChange(date ? format(date, 'yyyy-MM-dd') : '');
            setOpen(false);
          }}
          defaultMonth={selected}
          {...(disablePast ? { disabled: { before: today } } : {})}
        />
      </PopoverContent>
    </Popover>
  );
}
