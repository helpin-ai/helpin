import { AskAgentWorkAnimation } from '@/components/agents/AskAgentWorkAnimation';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useDockStore } from '@/stores/dockStore';

export function SupportAskAgentActivity({ workspaceId, conversationId }: {
  workspaceId: string;
  conversationId: string;
}) {
  const activity = useDockStore((state) => {
    let waiting = false;
    for (const chat of state.chats) {
      if (chat.workspace_id !== workspaceId || chat.support_conversation_id !== conversationId || chat.archived_at) continue;
      if (chat.active_run_status === 'running' || chat.active_run_status === 'queued') return 'working';
      if (chat.active_run_status === 'paused') waiting = true;
    }
    return waiting ? 'waiting' : null;
  });
  if (!activity) return null;

  const label = activity === 'working' ? 'Ask Agent is running' : 'Ask Agent is waiting for input';
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span aria-label={label} className="inline-flex h-3.5 w-3.5 shrink-0 items-center justify-center">
          {activity === 'working' ? (
            <AskAgentWorkAnimation />
          ) : (
            <span className="h-1.5 w-1.5 rounded-full bg-quiet-accent motion-safe:animate-pulse" />
          )}
        </span>
      </TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  );
}
