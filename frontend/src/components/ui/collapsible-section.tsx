import { useState } from 'react';
import { ArrowDown01Icon, ArrowRight01Icon, PlusSignIcon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';

interface CollapsibleSectionProps {
  title: string;
  icon: React.ElementType;
  count: number;
  defaultOpen?: boolean;
  onAdd?: () => void;
  children: React.ReactNode;
}

export function CollapsibleSection({
  title,
  icon: Icon,
  count,
  defaultOpen = false,
  onAdd,
  children,
}: CollapsibleSectionProps) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div className="border-b last:border-b-0">
      <div className="flex items-center">
        <button
          type="button"
          className="flex flex-1 items-center gap-2 px-3 py-2.5 text-xs font-medium text-muted-foreground hover:bg-muted/50 transition-colors"
          onClick={() => setOpen(!open)}
        >
          {open ? <ArrowDown01Icon className="h-3.5 w-3.5 shrink-0" /> : <ArrowRight01Icon className="h-3.5 w-3.5 shrink-0" />}
          <Icon className="h-3.5 w-3.5 shrink-0" />
          <span className="flex-1 text-left">{title}</span>
          {count > 0 && (
            <Badge variant="secondary" className="h-4 px-1 text-[10px]">{count}</Badge>
          )}
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
        <div className="px-3 pb-2.5 space-y-1.5">
          {children}
        </div>
      )}
    </div>
  );
}
