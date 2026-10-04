import { Badge } from '@/components/ui/badge';
import { isExternalAgent } from '@/lib/externalAgents';
import type { Agent } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

export function ExternalAgentBadge({ agent, className }: { agent?: Pick<Agent, 'runtime_kind'> | null; className?: string }) {
  if (!isExternalAgent(agent)) return null;
  return (
    <Badge
      variant="outline"
      className={cn('h-5 shrink-0 px-1.5 text-[10px]', className)}
      title="Runs outside Helpin over the A2A protocol"
      data-external-agent-badge
    >
      External
    </Badge>
  );
}
