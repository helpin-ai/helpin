import { useState, type ReactNode } from 'react';
import { ArrowRight01Icon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

interface RailSectionProps {
  title: string;
  count?: number;
  defaultOpen?: boolean;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
}

export function RailSection({
  title,
  count,
  defaultOpen = true,
  action,
  children,
  className,
}: RailSectionProps) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div
      className={cn(
        'border-b border-border/50 last:border-b-0 transition-colors',
        open && 'bg-muted/40 dark:bg-muted/25',
        className,
      )}
    >
      <div
        role="button"
        tabIndex={0}
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            setOpen(!open);
          }
        }}
        className={cn(
          'group flex cursor-pointer items-center gap-2 px-4 py-2.5 transition-colors hover:bg-muted/30',
          open && 'hover:bg-muted/50 dark:hover:bg-muted/35',
        )}
      >
        <span
          className={cn(
            'flex-1 text-[11px] font-medium uppercase tracking-tight transition-colors',
            open ? 'text-foreground' : 'text-foreground/75',
          )}
        >
          {title}
        </span>
        {typeof count === 'number' && count > 0 && (
          <Badge variant="secondary" className="h-4 rounded-full px-1.5 text-[10px] font-medium">
            {count}
          </Badge>
        )}
        {action && (
          <span
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => event.stopPropagation()}
            className="flex items-center"
          >
            {action}
          </span>
        )}
        <ArrowRight01Icon
          className={cn(
            'h-3.5 w-3.5 shrink-0 text-muted-foreground/70 transition-transform duration-200 group-hover:text-foreground',
            open && 'rotate-90',
          )}
        />
      </div>
      {open && <div className="px-4 pb-3 pt-0.5 space-y-2">{children}</div>}
    </div>
  );
}
