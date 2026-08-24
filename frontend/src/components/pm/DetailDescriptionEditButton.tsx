import { PencilEdit01Icon } from '@/lib/icons';
import { QuickTooltip } from '@/components/ui/quick-tooltip';

interface DetailDescriptionEditButtonProps {
  onClick: () => void;
  label?: string;
}

export function DetailDescriptionEditButton({
  onClick,
  label = 'Edit description',
}: DetailDescriptionEditButtonProps) {
  return (
    <div className="pointer-events-none absolute inset-y-0 right-0 z-10">
      <div className="sticky top-3">
        <QuickTooltip label={label} side="left">
          <button
            type="button"
            aria-label={label}
            className="pointer-events-auto inline-flex h-9 w-9 cursor-pointer items-center justify-center rounded-lg border border-border/70 bg-background/95 text-muted-foreground opacity-100 shadow-sm backdrop-blur-sm transition-[opacity,color,background-color,border-color,box-shadow] hover:border-border hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 md:opacity-0 md:group-hover/desc:opacity-100 md:group-focus-within/desc:opacity-100"
            onClick={onClick}
          >
            <PencilEdit01Icon className="h-4 w-4" />
          </button>
        </QuickTooltip>
      </div>
    </div>
  );
}
