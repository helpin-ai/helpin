import { useState } from 'react';
import { ArrowRight01Icon, PlusSignIcon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

interface CollapsibleSectionProps {
  title: string;
  icon?: React.ElementType;
  count?: number;
  defaultOpen?: boolean;
  onAdd?: () => void;
  children: React.ReactNode;
}

export function CollapsibleSection({
  title,
  count = 0,
  defaultOpen = false,
  onAdd,
  children,
}: CollapsibleSectionProps) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div
      className={cn(
        'border-b border-border/50 last:border-b-0 transition-colors',
        open && 'bg-muted/40 dark:bg-muted/25',
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
        {count > 0 && (
          <Badge variant="secondary" className="h-4 rounded-full px-1.5 text-[10px] font-medium">
            {count}
          </Badge>
        )}
        {onAdd && (
          <button
            type="button"
            className="rounded p-1 text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground"
            onClick={(event) => {
              event.stopPropagation();
              onAdd();
            }}
            aria-label={`Add ${title.toLowerCase()}`}
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
          </button>
        )}
        <ArrowRight01Icon
          className={cn(
            'h-3.5 w-3.5 shrink-0 text-muted-foreground/70 transition-transform duration-200 group-hover:text-foreground',
            open && 'rotate-90',
          )}
        />
      </div>
      {open && (
        <div className="px-4 pb-3 pt-0.5 space-y-2">
          {children}
        </div>
      )}
    </div>
  );
}
