import { Bot, Check, ChevronDown, ChevronUp, Loader2 } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type { Agent, Epic } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { useState } from 'react';
import type { StepStatus } from './planningStepUtils';

interface Props {
  status: StepStatus;
  epic: Epic;
  agents: Agent[];
  selectedAgentId: string;
  onSelectAgent: (id: string) => void;
  onAssign: () => void;
  assigning: boolean;
}

export function PlannerSetupStep({ status, epic, agents, selectedAgentId, onSelectAgent, onAssign, assigning }: Props) {
  const [expanded, setExpanded] = useState(status === 'current');
  const assignedAgent = agents.find((a) => a.id === epic.orchestrator_agent_id);

  if (status === 'completed' && !expanded) {
    return (
      <StepCard status={status}>
        <button type="button" className="flex w-full items-center justify-between" onClick={() => setExpanded(true)}>
          <div className="flex items-center gap-2">
            <Check className="h-4 w-4 text-emerald-500" />
            <span className="text-sm font-medium">Planner assigned</span>
            {assignedAgent && (
              <Badge variant="secondary" className="text-[10px]">{assignedAgent.name}</Badge>
            )}
          </div>
          <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
        </button>
      </StepCard>
    );
  }

  return (
    <StepCard status={status}>
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium">Set up planner</h4>
        {status === 'completed' && (
          <button type="button" onClick={() => setExpanded(false)}>
            <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
          </button>
        )}
      </div>
      <p className="mt-1 text-xs text-muted-foreground">
        Choose an AI planner to draft specs and generate stories for this epic.
      </p>

      <div className="mt-3 flex flex-col gap-2 sm:flex-row">
        <select
          value={selectedAgentId}
          onChange={(event) => onSelectAgent(event.target.value)}
          className="h-9 rounded-md border border-border bg-background px-3 text-xs sm:flex-1"
        >
          <option value="">Choose an AI planner...</option>
          {agents.map((agent) => (
            <option key={agent.id} value={agent.id}>
              {agent.name} ({agent.capability_profile || agent.role || 'agent'})
            </option>
          ))}
        </select>
        <Button size="sm" variant="outline" onClick={onAssign} disabled={!selectedAgentId || assigning}>
          {assigning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Bot className="h-3.5 w-3.5" />}
          Assign planner
        </Button>
      </div>

      {!epic.planning_repository_id && (
        <div className="mt-3 rounded-md border border-dashed border-amber-300 bg-amber-50 p-2.5 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-300">
          A planning repository is required for both PRD drafting and code-aware story planning.
        </div>
      )}
    </StepCard>
  );
}

function StepCard({ status, children }: { status: StepStatus; children: React.ReactNode }) {
  return (
    <div
      className={cn(
        'rounded-md border p-4 transition-colors',
        status === 'completed' && 'border-emerald-200 bg-emerald-50/30 dark:border-emerald-900 dark:bg-emerald-950/10',
        status === 'current' && 'border-primary/40 bg-background shadow-sm',
        status === 'upcoming' && 'border-border/40 bg-muted/10 opacity-60',
      )}
    >
      {children}
    </div>
  );
}

export { StepCard };
