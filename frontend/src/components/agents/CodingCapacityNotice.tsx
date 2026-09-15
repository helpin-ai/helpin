import type { Agent } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

export function CodingCapacityNotice({ agent, className }: {
  agent: Pick<Agent, 'preset_key'> | null | undefined;
  className?: string;
}) {
  if (agent?.preset_key !== 'code_builder' && agent?.preset_key !== 'review_agent') return null;

  return (
    <p className={cn('text-xs text-quiet-text-secondary', className)}>
      Coding runs share one execution slot across all workspaces. Additional runs wait in the queue.
    </p>
  );
}
