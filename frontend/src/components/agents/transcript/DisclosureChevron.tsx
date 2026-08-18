import { ArrowRight01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

export function DisclosureChevron({ open, className }: { open: boolean; className?: string }) {
  return (
    <span
      aria-hidden
      data-disclosure-chevron
      className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90', className)}
    >
      <ArrowRight01Icon className="h-full w-full" />
    </span>
  );
}
