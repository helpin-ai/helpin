import { ArrowLeft02Icon, Cancel01Icon, Clock03Icon, Loading01Icon, PauseIcon, PlusSignIcon } from '@/lib/icons';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

export type DockHeaderMode = 'conversation' | 'list';

interface DockHeaderProps {
  mode: DockHeaderMode;
  runningCount: number;
  awaitingCount: number;
  onSwapMode: () => void;
  onNew: () => void;
  onClose: () => void;
}

export function DockHeader({
  mode,
  runningCount,
  awaitingCount,
  onSwapMode,
  onNew,
  onClose,
}: DockHeaderProps) {
  const hasRunning = runningCount > 0;
  const hasAwaiting = awaitingCount > 0;
  return (
    <div className="flex items-center justify-between gap-2 px-3.5 py-2">
      <div className="flex min-w-0 items-center gap-2">
        {mode === 'list' ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                onClick={onSwapMode}
                aria-label="Back to conversation"
                className="rounded-full p-1 text-muted-foreground transition hover:bg-muted hover:text-foreground"
              >
                <ArrowLeft02Icon className="h-3.5 w-3.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="bottom" sideOffset={4} className="z-[70]">Back to conversation</TooltipContent>
          </Tooltip>
        ) : null}
        <span className="text-sm font-semibold text-foreground">
          {mode === 'list' ? 'Recent runs' : 'Ask agents'}
        </span>
        {hasRunning ? (
          <span
            className="inline-flex items-center gap-1 rounded-full bg-orange-500/10 px-1.5 py-0.5 text-[10px] font-medium text-orange-700 dark:text-orange-300"
            title={`${runningCount} running`}
          >
            <Loading01Icon className="h-2.5 w-2.5 animate-spin" />
            {runningCount} running
          </span>
        ) : null}
        {hasAwaiting ? (
          <span
            className="inline-flex items-center gap-1 rounded-full bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-300"
            title={`${awaitingCount} waiting on you`}
          >
            <PauseIcon className="h-2.5 w-2.5" />
            {awaitingCount} waiting
          </span>
        ) : null}
      </div>
      <div className="flex shrink-0 items-center gap-0.5">
        {mode === 'conversation' ? (
          <HeaderButton tip="View recent runs" onClick={onSwapMode}>
            <Clock03Icon className="h-3.5 w-3.5" />
          </HeaderButton>
        ) : null}
        <HeaderButton tip="Start a new conversation" onClick={onNew}>
          <PlusSignIcon className="h-3.5 w-3.5" />
          <span className="text-[11px] font-medium">New</span>
        </HeaderButton>
        <HeaderButton tip="Hide dock" onClick={onClose}>
          <Cancel01Icon className="h-3.5 w-3.5" />
        </HeaderButton>
      </div>
    </div>
  );
}

function HeaderButton({
  children,
  onClick,
  tip,
}: {
  children: React.ReactNode;
  onClick: () => void;
  tip: string;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={onClick}
          aria-label={tip}
          className={cn(
            'inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-muted-foreground transition hover:bg-muted hover:text-foreground',
          )}
        >
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent side="bottom" sideOffset={4} className="z-[70]">{tip}</TooltipContent>
    </Tooltip>
  );
}
