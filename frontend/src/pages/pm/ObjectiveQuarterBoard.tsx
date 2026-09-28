import { Fragment, useRef, useState, type ReactNode } from 'react';
import { endOfQuarter, format, isValid, parseISO } from 'date-fns';
import { QuietDropdown, QuietEmptyState, QuietIconAction, QuietTextAction } from '@/components/design-system/quiet';
import { ArrowDown01Icon, ArrowLeft01Icon, ArrowRight01Icon, PlusSignIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import type { ObjectiveWithDetails } from '@/lib/pmTypes';
import type { ObjectiveCreateDates } from '@/stores/globalCreateStore';

// PM dates are calendar dates, even when the API serializes them with a timezone.
function objectiveEndDate(deadline?: string | null): string | undefined {
  const day = deadline?.slice(0, 10);
  return day && /^\d{4}-\d{2}-\d{2}$/.test(day) && isValid(parseISO(day)) ? day : undefined;
}

function objectiveIsOverdue({ objective }: ObjectiveWithDetails, today: string) {
  const end = objectiveEndDate(objective.deadline);
  return objective.state !== 'closed' && !!end && end < today;
}

type Column = { key: string; label: string; startDate?: string; endDate?: string };

export function ObjectiveQuarterBoard({
  objectives, allObjectives, renderCard, canCreate, onCreate, emptyState,
}: {
  objectives: ObjectiveWithDetails[];
  allObjectives: ObjectiveWithDetails[];
  renderCard: (objective: ObjectiveWithDetails) => ReactNode;
  canCreate: boolean;
  onCreate: (dates?: ObjectiveCreateDates) => void;
  emptyState?: ReactNode;
}) {
  const now = new Date();
  const today = format(now, 'yyyy-MM-dd');
  const currentYear = now.getFullYear();
  const currentQuarter = Math.floor(now.getMonth() / 3) + 1;
  const [year, setYear] = useState(currentYear);
  const [overdueOnly, setOverdueOnly] = useState(false);
  const scroller = useRef<HTMLDivElement>(null);
  const currentColumn = useRef<HTMLElement>(null);
  const years = [...new Set([currentYear - 1, currentYear, currentYear + 1, year,
    ...allObjectives.flatMap(({ objective }) => {
      const end = objectiveEndDate(objective.deadline);
      return end ? [Number(end.slice(0, 4))] : [];
    }),
  ])].sort((a, b) => b - a);
  const columns: Column[] = [
    { key: 'unscheduled', label: 'Unscheduled' },
    ...[4, 3, 2, 1].map(quarter => {
      const startDate = `${String(year).padStart(4, '0')}-${String(quarter * 3 - 2).padStart(2, '0')}-01`;
      return { key: `${year}-${quarter}`, label: `Q${quarter}, ${year}`, startDate,
        endDate: format(endOfQuarter(parseISO(startDate)), 'yyyy-MM-dd') };
    }),
  ];
  const visibleColumns = columns.map(column => {
    const isPast = !!column.endDate && column.endDate < today;
    const items = objectives.filter(item => {
      const end = objectiveEndDate(item.objective.deadline);
      return (column.startDate ? !!end && end >= column.startDate && end <= column.endDate! : !end)
        && (!overdueOnly || objectiveIsOverdue(item, today));
    });
    return { ...column, isPast, items };
  }).filter(column => column.items.length > 0 || (!!column.endDate && !column.isPast));
  const overdueCount = objectives.filter(item => objectiveEndDate(item.objective.deadline)?.startsWith(`${String(year).padStart(4, '0')}-`) && objectiveIsOverdue(item, today)).length;
  const changeYear = (next: number) => {
    setYear(next);
    scroller.current?.scrollTo({ left: 0 });
  };
  const showCurrent = () => {
    setYear(currentYear);
    setOverdueOnly(false);
    requestAnimationFrame(() => {
      const column = currentColumn.current;
      const first = scroller.current?.querySelector<HTMLElement>('[data-objective-column]');
      if (column && first) scroller.current?.scrollTo({
        left: column.offsetLeft - first.offsetLeft,
        behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth',
      });
    });
  };

  return <div className="flex min-h-0 flex-1 flex-col">
    <div className="flex shrink-0 flex-wrap items-center gap-1 border-b border-quiet-divider-strong px-4 py-2 md:px-6">
      <QuietIconAction aria-label="Previous year" disabled={year <= 1} onClick={() => changeYear(year - 1)}><ArrowLeft01Icon className="h-4 w-4" /></QuietIconAction>
      <QuietDropdown label="Year" selected={[String(year)]} options={years.map(value => ({ value: String(value), label: String(value) }))}
        onSelect={value => changeYear(Number(value))}
        trigger={<QuietTextAction aria-label={`Year: ${year}`} className="min-h-8 gap-1 px-2 font-semibold focus-visible:underline">{year}<ArrowDown01Icon className="h-3.5 w-3.5" /></QuietTextAction>} />
      <QuietIconAction aria-label="Next year" disabled={year >= 9999} onClick={() => changeYear(year + 1)}><ArrowRight01Icon className="h-4 w-4" /></QuietIconAction>
      <QuietTextAction className="min-h-8 px-2 focus-visible:underline" onClick={showCurrent}>This quarter</QuietTextAction>
      <span className="mx-2 h-4 border-l border-quiet-divider-strong" aria-hidden="true" />
      <QuietTextAction aria-label="Show overdue objectives" aria-pressed={overdueOnly} onClick={() => setOverdueOnly(value => !value)}
        className={cn('min-h-8 gap-1.5 px-2 focus-visible:underline', overdueOnly && 'bg-quiet-hover text-quiet-accent')}>
        Overdue <span className="text-xs tabular-nums">{overdueCount}</span>
      </QuietTextAction>
    </div>
    <div ref={scroller} role="region" aria-label="Objectives by quarter" tabIndex={0}
      className="min-h-0 flex-1 overflow-auto p-4 pb-20 focus-visible:outline-2 focus-visible:outline-quiet-field md:p-6 md:pb-24">
      {emptyState ?? (visibleColumns.length === 0 ? <QuietEmptyState title={`No objectives for ${year}`} description="Choose another year to see objectives." /> : <div className="relative flex min-w-full items-start gap-4">
        {visibleColumns.map(column => {
          const isCurrent = column.key === `${currentYear}-${currentQuarter}`;
          return <section key={column.key} ref={isCurrent ? currentColumn : undefined} data-objective-column="" aria-label={column.label}
            className={cn('flex min-h-[420px] w-[calc(100vw-3rem)] shrink-0 flex-col overflow-hidden rounded-lg border border-quiet-divider-strong bg-quiet-surface sm:w-[320px]', isCurrent && 'border-t-2 border-t-quiet-lifecycle')}>
            <div className="border-b border-quiet-divider-strong p-3.5">
              <div className="flex items-center justify-between gap-3">
                <h2 className="text-sm font-semibold text-quiet-text-primary">{column.label}</h2>
                {column.endDate && <span className={cn('text-[11px]', isCurrent ? 'text-quiet-lifecycle' : 'text-quiet-text-tertiary')}>
                  {isCurrent ? 'Current quarter' : column.endDate < today ? 'Past' : 'Upcoming'}
                </span>}
              </div>
              <p className="mt-1.5 text-xs text-quiet-text-tertiary">
                {column.startDate ? `${format(parseISO(column.startDate), 'MMM d')} – ${format(parseISO(column.endDate!), 'MMM d')}` : 'No end date'}
              </p>
            </div>
            <div className="space-y-3 p-3">
              {column.items.map(item => <Fragment key={item.objective.id}>{renderCard(item)}</Fragment>)}
              {column.items.length === 0 && <p className="px-1 py-5 text-xs text-quiet-text-tertiary">
                {overdueOnly ? 'No overdue objectives.' : 'No objectives.'}
              </p>}
            </div>
            {canCreate && !column.isPast && <div className="mx-3 mb-3">
              <Button variant="ghost" size="sm" aria-label={`Add objective to ${column.label}`} className="w-full gap-1.5 px-2 text-muted-foreground"
                onClick={() => onCreate(column.startDate ? { startDate: column.startDate, endDate: column.endDate! } : undefined)}>
                <PlusSignIcon className="h-4 w-4" />Add objective
              </Button>
            </div>}
          </section>;
        })}
      </div>)}
    </div>
  </div>;
}
