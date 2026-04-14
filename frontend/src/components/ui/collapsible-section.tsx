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
        open && 'bg-muted/70 dark:bg-muted/40',
      )}
    >
      <div className="flex items-center">
        <button
          type="button"
          className={cn(
            'group flex flex-1 items-center gap-2 px-4 py-2.5 text-left transition-colors hover:bg-muted/40',
            open && 'hover:bg-muted/80 dark:hover:bg-muted/50',
          )}
          onClick={() => setOpen(!open)}
          aria-expanded={open}
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
          <ArrowRight01Icon
            className={cn(
              'h-3.5 w-3.5 shrink-0 text-muted-foreground/70 transition-transform duration-200 group-hover:text-foreground',
              open && 'rotate-90',
            )}
          />
        </button>
        {onAdd && (
          <button
            type="button"
            className="mr-2 rounded p-1 text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
            onClick={(e) => { e.stopPropagation(); onAdd(); }}
            aria-label={`Add ${title.toLowerCase()}`}
          >
            <PlusSignIcon className="h-3.5 w-3.5" />
          </button>
        )}
      </div>
      {open && (
        <div className="px-4 pb-3 pt-0.5 space-y-2">
          {children}
        </div>
      )}
    </div>
  );
}
