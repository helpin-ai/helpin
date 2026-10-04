import { AiMagicIcon } from '@/lib/icons';
import { AskAgentWorkAnimation } from '@/components/agents/AskAgentWorkAnimation';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

export function SupportAskAgentsButton({
  open,
  onOpen,
  chatExists = false,
  runStatus = null,
}: {
  open: boolean;
  onOpen: () => void;
  chatExists?: boolean;
  runStatus?: 'queued' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled' | null;
}) {
  const working = runStatus === 'queued' || runStatus === 'running';
  const waiting = runStatus === 'paused';
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          aria-label="Ask Agent about this conversation"
          aria-expanded={open}
          aria-controls="support-agent-sidebar"
          onClick={onOpen}
          className={cn(
            'hidden h-7 items-center gap-1.5 rounded-full px-2.5 text-xs transition-[background-color,color,transform] duration-200 xl:inline-flex motion-reduce:transition-none',
            open
              ? 'bg-primary/10 text-primary'
              : 'text-muted-foreground hover:bg-primary/10 hover:text-primary',
          )}
        >
          {working ? <AskAgentWorkAnimation /> : <AiMagicIcon className="h-3.5 w-3.5" />}
          <span>Ask Agent</span>
          {!working && waiting && <span className="agent-paused-dot-pulse h-1.5 w-1.5 rounded-full bg-amber-500" aria-label="Waiting for input" />}
          {!working && !waiting && chatExists && <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" aria-label="Ask Agent chat available" />}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="top">Open Ask agents with this conversation selected</TooltipContent>
    </Tooltip>
  );
}
