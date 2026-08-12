import { AiMagicIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

export function SupportAskAgentsButton({ open, onOpen }: { open: boolean; onOpen: () => void }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          aria-label="Ask agents about this conversation"
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
          <AiMagicIcon className="h-3.5 w-3.5" />
          <span>Ask agents</span>
        </Button>
      </TooltipTrigger>
      <TooltipContent side="top">Open Ask agents with this conversation selected</TooltipContent>
    </Tooltip>
  );
}
