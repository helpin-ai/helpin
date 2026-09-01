import { useState, type ReactNode } from 'react';
import { QuietSectionHeader } from '@/components/design-system/quiet';
import { ArrowRight01Icon } from '@/lib/icons';
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
        'border-b border-quiet-divider-light last:border-b-0',
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
          'group flex cursor-pointer items-center gap-2 px-4 py-2.5 transition-colors hover:bg-quiet-row-hover focus-visible:bg-quiet-row-hover focus-visible:outline-none',
        )}
      >
        <QuietSectionHeader title={title} count={typeof count === 'number' && count > 0 ? count : undefined} className="min-h-0 flex-1" />
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
            'h-3.5 w-3.5 shrink-0 text-quiet-muted transition-transform duration-200 group-hover:text-quiet-text-primary',
            open && 'rotate-90',
          )}
        />
      </div>
      {open && <div className="space-y-2 px-4 pb-3 pt-0.5">{children}</div>}
    </div>
  );
}
