import { cn } from '@/lib/utils';

/** The date line between days in a support thread, or the floating date while scrolling. */
export function DaySeparator({
  label,
  floating = false,
  phase = 'visible',
  separatorRef,
}: {
  label: string;
  floating?: boolean;
  phase?: 'visible' | 'handoff' | 'hidden';
  separatorRef?: (node: HTMLDivElement | null) => void;
}) {
  const dateLabel = (
    <span className="shrink-0 rounded-full bg-muted px-3 py-1 text-xs font-semibold text-foreground/75">
      {label}
    </span>
  );

  if (floating) {
    return (
      <div
        data-support-floating-date
        aria-hidden="true"
        className={cn(
          'flex justify-center motion-reduce:transition-none',
          phase === 'handoff' ? 'transition-none translate-y-0 opacity-0'
            : 'transition-[opacity,transform] duration-200 ease-out',
          phase === 'visible' ? 'translate-y-0 opacity-100' : phase === 'hidden' ? '-translate-y-2 opacity-0' : null,
        )}
      >
        {dateLabel}
      </div>
    );
  }

  return (
    <div ref={separatorRef} data-support-day-separator className="my-5 flex items-center gap-3">
      <div className="h-px flex-1 bg-border" aria-hidden />
      {dateLabel}
      <div className="h-px flex-1 bg-border" aria-hidden />
    </div>
  );
}
