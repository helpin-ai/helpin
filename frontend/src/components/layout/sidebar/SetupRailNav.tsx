import type { SetupJourney } from '@/lib/setupTypes';
import { cn } from '@/lib/utils';

type SetupRailNavProps = {
  journeys: SetupJourney[];
  activeJourneyKey?: string;
  onSelect: (journeyKey: string) => void;
};

export function SetupRailNav({ journeys, activeJourneyKey, onSelect }: SetupRailNavProps) {
  return (
    <nav aria-label="Setup journeys" className="py-2">
      <p className="px-2 pb-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">Setup areas</p>
      <div className="space-y-0.5">
        {journeys.map((journey) => {
          const active = journey.key === activeJourneyKey;
          return (
            <button
              key={journey.key}
              type="button"
              aria-current={active ? 'location' : undefined}
              onClick={() => onSelect(journey.key)}
              className={cn(
                'flex w-full items-center justify-between gap-3 rounded-md px-2 py-2 text-left text-sm transition-colors hover:bg-muted/70',
                active && 'bg-muted font-medium text-foreground',
              )}
            >
              <span className="min-w-0 truncate">{journey.title}</span>
              <span className="shrink-0 text-[10px] tabular-nums text-muted-foreground">{journey.completed_count}/{journey.total_count}</span>
            </button>
          );
        })}
      </div>
    </nav>
  );
}
