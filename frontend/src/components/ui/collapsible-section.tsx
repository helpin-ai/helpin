import { useState } from 'react';
import { ArrowRight01Icon, PlusSignIcon } from '@/lib/icons';
import { cn } from '@/lib/utils';

interface CollapsibleSectionProps {
  title: string;
  icon?: React.ElementType;
  count?: number;
  defaultOpen?: boolean;
  /** Follow item availability until the user explicitly toggles this section. */
  autoOpenWhenPopulated?: boolean;
  /** Remembers this person's open/closed choice for the section across conversations. */
  persistKey?: string;
  onAdd?: () => void;
  children: React.ReactNode;
}

const SECTION_PREFERENCE_PREFIX = 'helpin.collapsibleSection.';

function readSectionPreference(key: string): boolean | undefined {
  try {
    const value = window.localStorage.getItem(SECTION_PREFERENCE_PREFIX + key);
    return value === 'open' ? true : value === 'closed' ? false : undefined;
  } catch {
    return undefined;
  }
}

function writeSectionPreference(key: string, open: boolean) {
  try {
    window.localStorage.setItem(SECTION_PREFERENCE_PREFIX + key, open ? 'open' : 'closed');
  } catch {
    // Storage can be unavailable (private mode, quota); the choice then lasts for this view only.
  }
}

export function CollapsibleSection({
  title,
  count = 0,
  defaultOpen = false,
  autoOpenWhenPopulated = false,
  persistKey,
  onAdd,
  children,
}: CollapsibleSectionProps) {
  const [userOpen, setUserOpen] = useState<boolean | undefined>(() => {
    const remembered = persistKey ? readSectionPreference(persistKey) : undefined;
    if (remembered !== undefined) return remembered;
    return autoOpenWhenPopulated ? undefined : defaultOpen;
  });
  const open = userOpen ?? (count > 0);
  const toggle = () => {
    setUserOpen(!open);
    if (persistKey) writeSectionPreference(persistKey, !open);
  };

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
        onClick={toggle}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            toggle();
          }
        }}
        className={cn(
          'group flex cursor-pointer items-center gap-1.5 px-4 py-2.5 transition-colors hover:bg-muted/30',
          open && 'hover:bg-muted/50 dark:hover:bg-muted/35',
        )}
      >
        <span className="min-w-0 truncate text-sm font-semibold text-foreground">{title}</span>
        {count > 0 && (
          <span className="inline-flex h-[18px] min-w-5 shrink-0 items-center justify-center rounded-full bg-foreground/[0.08] px-1.5 text-[11px] font-semibold tabular-nums text-foreground/80">
            {count}
          </span>
        )}
        <span className="flex-1" />
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
