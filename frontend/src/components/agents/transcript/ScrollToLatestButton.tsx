import { ArrowDown01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

/**
 * Floating "jump to latest" pill for transcript scroll containers. Render it
 * inside a relatively-positioned wrapper around the scroll area; the caller
 * decides visibility (typically: user has scrolled away from the tail).
 */
export function ScrollToLatestButton({
  onClick,
  className,
}: {
  onClick: () => void;
  className?: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label="Scroll to latest"
      className={cn(
        'absolute bottom-3 left-1/2 z-10 inline-flex -translate-x-1/2 items-center gap-1 rounded-full border border-border/70 bg-background/95 px-2.5 py-1 text-[11px] font-medium text-foreground shadow-md transition hover:bg-muted',
        className,
      )}
    >
      <ArrowDown01Icon className="h-3 w-3" />
      Latest
    </button>
  );
}
