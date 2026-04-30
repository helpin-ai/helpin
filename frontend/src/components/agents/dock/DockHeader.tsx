import { ArrowLeft02Icon, Cancel01Icon, Clock03Icon, Loading01Icon, PlusSignIcon } from '@/lib/icons';
import { cn } from '@/lib/utils';

export type DockHeaderMode = 'conversation' | 'list';

interface DockHeaderProps {
  mode: DockHeaderMode;
  activeCount: number;
  onSwapMode: () => void;
  onNew: () => void;
  onClose: () => void;
}

export function DockHeader({ mode, activeCount, onSwapMode, onNew, onClose }: DockHeaderProps) {
  return (
    <div className="flex items-center justify-between gap-2 px-3.5 py-2">
      <div className="flex min-w-0 items-center gap-2">
        {mode === 'list' ? (
          <button
            type="button"
            onClick={onSwapMode}
            className="rounded-full p-1 text-muted-foreground transition hover:bg-muted hover:text-foreground"
            title="Back to conversation"
          >
            <ArrowLeft02Icon className="h-3.5 w-3.5" />
          </button>
        ) : (
          <span aria-hidden className="inline-block h-1.5 w-1.5 rounded-full bg-orange-500" />
        )}
        <span className="text-sm font-semibold text-foreground">
          {mode === 'list' ? 'Recent runs' : 'Ask agents'}
        </span>
        {activeCount > 0 ? (
          <span className="inline-flex items-center gap-1 rounded-full bg-orange-500/10 px-1.5 py-0.5 text-[10px] font-medium text-orange-700 dark:text-orange-300">
            <Loading01Icon className="h-2.5 w-2.5 animate-spin" />
            {activeCount} active
          </span>
        ) : null}
      </div>
      <div className="flex shrink-0 items-center gap-0.5">
        {mode === 'conversation' ? (
          <HeaderButton title="Recent runs" onClick={onSwapMode}>
            <Clock03Icon className="h-3.5 w-3.5" />
          </HeaderButton>
        ) : null}
        <HeaderButton title="New" onClick={onNew}>
          <PlusSignIcon className="h-3.5 w-3.5" />
          <span className="text-[11px] font-medium">New</span>
        </HeaderButton>
        <HeaderButton title="Hide" onClick={onClose}>
          <Cancel01Icon className="h-3.5 w-3.5" />
        </HeaderButton>
      </div>
    </div>
  );
}

function HeaderButton({
  children,
  onClick,
  title,
}: {
  children: React.ReactNode;
  onClick: () => void;
  title: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      title={title}
      className={cn(
        'inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-muted-foreground transition hover:bg-muted hover:text-foreground',
      )}
    >
      {children}
    </button>
  );
}
