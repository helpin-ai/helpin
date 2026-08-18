import { AskAgentAvatar } from '@/components/agents/AskAgentAvatar';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

export function SidebarRunsButton() {
  const title = 'Ask Agents';

  const onClick = () => {
    window.dispatchEvent(
      new CustomEvent('helpin:ask-agents', { detail: { mode: 'compose', intent: 'new_chat' } }),
    );
  };

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={title}
          onClick={onClick}
          className="relative flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground"
        >
          <AskAgentAvatar plateStyle="feather" className="h-6 w-6" />
        </button>
      </TooltipTrigger>
      <TooltipContent side="right">{title}</TooltipContent>
    </Tooltip>
  );
}
