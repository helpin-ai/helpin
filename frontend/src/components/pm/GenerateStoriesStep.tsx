import { useState } from 'react';
import { Check, ChevronDown, ChevronUp, Loader2, MessageSquare, Play } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import type { AgentRun, Epic, OrchestrationProposal } from '@/lib/pmTypes';
import type { StepStatus } from './planningStepUtils';
import { humanizeError } from './planningStepUtils';
import { StepCard } from './PlannerSetupStep';

interface Props {
  status: StepStatus;
  epic: Epic;
  latestPlanRun: AgentRun | null;
  proposal: OrchestrationProposal | null;
  additionalContext: string;
  onAdditionalContextChange: (value: string) => void;
  onGenerateStories: () => void;
  triggeringPlan: boolean;
}

export function GenerateStoriesStep({
  status,
  epic,
  latestPlanRun,
  proposal,
  additionalContext,
  onAdditionalContextChange,
  onGenerateStories,
  triggeringPlan,
}: Props) {
  const [expanded, setExpanded] = useState(status === 'current');
  const [showNotes, setShowNotes] = useState(false);

  const isRunning = latestPlanRun && ['queued', 'running'].includes(latestPlanRun.status);
  const hasFailed = latestPlanRun?.status === 'failed';
  const storyCount = proposal?.proposed_stories?.length ?? 0;
  const effectiveStatus: StepStatus = isRunning || hasFailed ? 'current' : status;

  if (effectiveStatus === 'upcoming') {
    return (
      <StepCard status={effectiveStatus}>
        <div className="flex items-center gap-2">
          <span className="flex h-5 w-5 items-center justify-center rounded-full bg-muted text-[10px] font-medium text-muted-foreground">4</span>
          <span className="text-sm text-muted-foreground">Generate stories</span>
        </div>
      </StepCard>
    );
  }

  if (effectiveStatus === 'completed' && !expanded) {
    return (
      <StepCard status={effectiveStatus}>
        <button type="button" className="flex w-full items-center justify-between" onClick={() => setExpanded(true)}>
          <div className="flex items-center gap-2">
            <Check className="h-4 w-4 text-emerald-500" />
            <span className="text-sm font-medium">Stories generated</span>
            {storyCount > 0 && (
              <Badge variant="secondary" className="text-[10px]">{storyCount} stories proposed</Badge>
            )}
          </div>
          <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
        </button>
      </StepCard>
    );
  }

  return (
    <StepCard status={effectiveStatus}>
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium">Generate stories</h4>
        {effectiveStatus === 'completed' && (
          <button type="button" onClick={() => setExpanded(false)}>
            <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
          </button>
        )}
      </div>
      <p className="mt-1 text-xs text-muted-foreground">
        The AI planner will decompose the approved spec into dependency-aware stories.
      </p>

      <div className="mt-3 flex flex-wrap gap-2">
        <Button
          size="sm"
          onClick={onGenerateStories}
          disabled={!epic.approved_spec_version_id || !epic.planning_repository_id || triggeringPlan || !!isRunning}
        >
          {triggeringPlan || isRunning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
          {latestPlanRun ? 'Regenerate stories' : 'Generate stories'}
        </Button>
        <Button size="sm" variant="ghost" onClick={() => setShowNotes(!showNotes)}>
          <MessageSquare className="h-3.5 w-3.5" />
          {showNotes ? 'Hide notes' : 'Add notes for AI'}
        </Button>
      </div>

      {!epic.planning_repository_id && epic.approved_spec_version_id && (
        <div className="mt-3 rounded-md border border-dashed border-amber-300 bg-amber-50 p-2.5 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-300">
          Configure a planning repository in epic settings to enable code-aware story generation.
        </div>
      )}

      {showNotes && (
        <Textarea
          value={additionalContext}
          onChange={(event) => onAdditionalContextChange(event.target.value)}
          placeholder="Architecture preferences, scope constraints, or areas to focus on..."
          className="mt-3 min-h-[72px] text-xs"
        />
      )}

      {isRunning && (
        <div className="mt-3 flex items-center gap-2 rounded-md bg-primary/5 p-2.5 text-xs text-primary">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          AI is generating stories...
        </div>
      )}

      {hasFailed && (
        <div className="mt-3 rounded-md border border-destructive/30 bg-destructive/5 p-2.5 text-xs text-destructive">
          {humanizeError(latestPlanRun?.error_message)}
        </div>
      )}
    </StepCard>
  );
}
