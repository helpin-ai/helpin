import { useState, type ReactNode } from 'react';
import { cn } from '@/lib/utils';

export interface TranscriptRowProps {
  icon: ReactNode;
  /** Tint for the icon glyph. */
  iconClassName?: string;
  label: ReactNode;
  /** Right-aligned trailing content (duration, "Live" badge, …). */
  meta?: ReactNode;
  tone?: 'default' | 'muted' | 'failed';
  /** When true and `children` are present, the row toggles a disclosure. */
  expandable?: boolean;
  defaultOpen?: boolean;
  /** Controlled disclosure state; omit to let the row manage its own. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** Avoid mounting expensive disclosure bodies until they are opened. */
  lazyMount?: boolean;
  children?: ReactNode;
}

/**
 * The single one-line transcript primitive: `[icon] label … [meta] [chevron]`.
 * Flat — no avatar, no connector rail. Tool calls use the static form; rows
 * that still benefit from depth (reasoning and run context) opt into disclosure.
 */
export function TranscriptRow({
  icon,
  iconClassName,
  label,
  meta,
  tone = 'muted',
  expandable = false,
  defaultOpen = false,
  open: controlledOpen,
  onOpenChange,
  lazyMount = false,
  children,
}: TranscriptRowProps) {
  const [internalOpen, setInternalOpen] = useState(defaultOpen);
  const open = controlledOpen ?? internalOpen;
  const setOpen = (next: boolean) => {
    if (controlledOpen === undefined) setInternalOpen(next);
    onOpenChange?.(next);
  };
  const canExpand = expandable && children != null;

  const header = (
    <div className="flex items-center gap-1.5 text-[11px]">
      <span className={cn('flex h-3.5 w-3.5 shrink-0 items-center justify-center', iconClassName)}>{icon}</span>
      <span
        className={cn(
          'min-w-0 flex-1 truncate',
          tone === 'failed' ? 'text-destructive' : tone === 'muted' ? 'text-muted-foreground' : 'text-foreground',
        )}
      >
        {label}
      </span>
      {meta ? <span className="shrink-0 text-[10px] text-muted-foreground">{meta}</span> : null}
      {canExpand ? (
        <span aria-hidden className="shrink-0 text-[10px] text-muted-foreground">
          {open ? '▾' : '▸'}
        </span>
      ) : null}
    </div>
  );

  if (!canExpand) {
    return <div>{header}</div>;
  }

  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="w-full rounded-sm text-left transition-colors hover:bg-muted/30"
        aria-expanded={open}
      >
        {header}
      </button>
      {!lazyMount || open ? (
        <div className={cn('mt-1.5 space-y-1.5 pl-5', !open && 'hidden')}>{children}</div>
      ) : null}
    </div>
  );
}
