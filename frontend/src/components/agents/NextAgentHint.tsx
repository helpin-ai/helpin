import { useMemo, useState } from 'react';
import { Loading01Icon, PlayIcon, SparklesIcon } from '@/lib/icons';
import { AgentAvatar, resolveAgentPersonaKey, type AgentPersonaKey } from '@/components/agents/AgentAvatar';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import type { Agent } from '@/lib/pmTypes';

const NEXT_PERSONA: Partial<Record<AgentPersonaKey, AgentPersonaKey>> = {
  scribe: 'forge',
  forge: 'lens',
};

const HINT_COPY: Partial<Record<AgentPersonaKey, { lead: string; cta: string }>> = {
  forge: {
    lead: 'Plan is ready. Hand off to Forge to start building.',
    cta: 'Start Forge',
  },
  lens: {
    lead: 'Build is in. Send it to Lens for review.',
    cta: 'Start Lens',
  },
};

interface Props {
  completedAgent: Pick<Agent, 'name' | 'preset_key'> | null | undefined;
  candidates: Agent[];
  onRun: (agent: Agent) => Promise<void> | void;
  className?: string;
  density?: 'compact' | 'comfortable';
}

export function NextAgentHint({ completedAgent, candidates, onRun, className, density = 'compact' }: Props) {
  const [running, setRunning] = useState(false);

  const nextAgent = useMemo(() => {
    if (!completedAgent) return null;
    const fromPersona = resolveAgentPersonaKey({ agent: completedAgent });
    const targetPersona = NEXT_PERSONA[fromPersona];
    if (!targetPersona) return null;
    return (
      candidates.find((agent) => resolveAgentPersonaKey({ agent }) === targetPersona)
      ?? null
    );
  }, [completedAgent, candidates]);

  if (!nextAgent) return null;

  const personaKey = resolveAgentPersonaKey({ agent: nextAgent });
  const copy = HINT_COPY[personaKey];
  if (!copy) return null;

  const handleClick = async () => {
    if (running) return;
    setRunning(true);
    try {
      await onRun(nextAgent);
    } finally {
      setRunning(false);
    }
  };

  const isComfortable = density === 'comfortable';

  return (
    <div
      className={cn(
        'flex items-center justify-between gap-3 bg-primary/[0.04] text-xs text-foreground',
        isComfortable
          ? 'rounded-lg border border-primary/20 px-3.5 py-2.5'
          : 'border-t border-primary/15 px-3 py-2',
        className,
      )}
    >
      <div className="flex min-w-0 items-center gap-2">
        <SparklesIcon className="h-3.5 w-3.5 shrink-0 text-primary/70" />
        <AgentAvatar agent={nextAgent} className="h-4 w-4 shrink-0" />
        <span className="truncate text-muted-foreground">
          <span className="font-medium text-foreground">Next up · {nextAgent.name}.</span>
          <span className="ml-1">{copy.lead}</span>
        </span>
      </div>
      <Button
        size="sm"
        variant="outline"
        onClick={handleClick}
        disabled={running}
        className="h-7 shrink-0 gap-1 px-2.5 text-xs"
      >
        {running ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <PlayIcon className="h-3 w-3" />}
        {copy.cta}
      </Button>
    </div>
  );
}
