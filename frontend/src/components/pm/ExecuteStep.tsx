import { useState } from 'react';
import { Check, ChevronDown, ChevronUp, GitBranch, Loader2 } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import type { KickoffExecutionResult } from '@/lib/pmTypes';
import type { StepStatus } from './planningStepUtils';
import { StepCard } from './PlannerSetupStep';

interface CreatedPlanningStory {
  story_id: string;
  ref?: string;
  name: string;
  story_type: string;
  estimate?: number;
  priority?: string;
  acceptance_criteria?: string[];
  dependency_refs?: string[];
}

interface Props {
  status: StepStatus;
  createdStories: CreatedPlanningStory[];
  selectedStoryIds: string[];
  onToggleStory: (storyId: string, checked: boolean) => void;
  onKickoff: () => void;
  kickingOff: boolean;
  executionResult: KickoffExecutionResult | null;
}

export function ExecuteStep({ status, createdStories, selectedStoryIds, onToggleStory, onKickoff, kickingOff, executionResult }: Props) {
  const [expanded, setExpanded] = useState(status === 'current');

  if (status === 'upcoming') {
    return (
      <StepCard status={status}>
        <div className="flex items-center gap-2">
          <span className="flex h-5 w-5 items-center justify-center rounded-full bg-muted text-[10px] font-medium text-muted-foreground">7</span>
          <span className="text-sm text-muted-foreground">Start execution</span>
        </div>
      </StepCard>
    );
  }

  if (status === 'completed' && !expanded) {
    return (
      <StepCard status={status}>
        <button type="button" className="flex w-full items-center justify-between" onClick={() => setExpanded(true)}>
          <div className="flex items-center gap-2">
            <Check className="h-4 w-4 text-emerald-500" />
            <span className="text-sm font-medium">Execution started</span>
            {executionResult && (
              <Badge variant="secondary" className="text-[10px]">
                {executionResult.started.length} started, {executionResult.skipped.length} skipped
              </Badge>
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
        <h4 className="text-sm font-medium">Start execution</h4>
        {status === 'completed' && (
          <button type="button" onClick={() => setExpanded(false)}>
            <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
          </button>
        )}
      </div>
      <p className="mt-1 text-xs text-muted-foreground">
        Select stories to hand off to agents. Only stories with assigned agents and delivery targets will start.
      </p>

      {createdStories.length === 0 ? (
        <div className="mt-3 rounded-md border border-dashed border-border/60 p-3 text-xs text-muted-foreground">
          Confirm a story plan first. Stories will appear here once created.
        </div>
      ) : (
        <>
          <div className="mt-3 flex items-center justify-between">
            <span className="text-[11px] text-muted-foreground">
              {selectedStoryIds.length} of {createdStories.length} selected
            </span>
            <div className="flex gap-2">
              <button
                type="button"
                className="text-[11px] text-primary hover:underline"
                onClick={() => createdStories.forEach((s) => onToggleStory(s.story_id, true))}
              >
                Select all
              </button>
              <button
                type="button"
                className="text-[11px] text-muted-foreground hover:underline"
                onClick={() => createdStories.forEach((s) => onToggleStory(s.story_id, false))}
              >
                Deselect all
              </button>
            </div>
          </div>

          <div className="mt-2 space-y-1.5">
            {createdStories.map((story) => {
              const checked = selectedStoryIds.includes(story.story_id);
              return (
                <label key={story.story_id} className="flex items-start gap-3 rounded-md border border-border/60 p-2.5">
                  <Checkbox checked={checked} onCheckedChange={(value) => onToggleStory(story.story_id, value === true)} className="mt-0.5" />
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-1.5">
                      <span className="text-sm font-medium">{story.name}</span>
                      {story.ref && <Badge variant="outline" className="text-[10px]">{story.ref}</Badge>}
                      <Badge variant="secondary" className="text-[10px]">{story.story_type}</Badge>
                    </div>
                    {story.dependency_refs && story.dependency_refs.length > 0 && (
                      <div className="mt-1 text-[11px] text-muted-foreground">
                        Depends on: {story.dependency_refs.join(', ')}
                      </div>
                    )}
                  </div>
                </label>
              );
            })}
          </div>

          <div className="mt-3">
            <Button size="sm" onClick={onKickoff} disabled={selectedStoryIds.length === 0 || kickingOff}>
              {kickingOff ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <GitBranch className="h-3.5 w-3.5" />}
              Start {selectedStoryIds.length} selected {selectedStoryIds.length === 1 ? 'story' : 'stories'}
            </Button>
          </div>
        </>
      )}

      {executionResult && (
        <div className="mt-3 grid gap-3 md:grid-cols-2">
          <div className="rounded-md border border-border/60 p-3">
            <div className="mb-1.5 text-xs font-medium">Started ({executionResult.started.length})</div>
            {executionResult.started.length > 0 ? (
              <div className="space-y-1 text-xs text-muted-foreground">
                {executionResult.started.map((item) => (
                  <div key={item.run_id}>Story {item.story_id.slice(0, 8)}... started</div>
                ))}
              </div>
            ) : (
              <div className="text-xs text-muted-foreground">No runs started.</div>
            )}
          </div>
          <div className="rounded-md border border-border/60 p-3">
            <div className="mb-1.5 text-xs font-medium">Skipped ({executionResult.skipped.length})</div>
            {executionResult.skipped.length > 0 ? (
              <div className="space-y-1 text-xs text-muted-foreground">
                {executionResult.skipped.map((item) => (
                  <div key={`${item.story_id}-${item.reason}`}>{item.reason}</div>
                ))}
              </div>
            ) : (
              <div className="text-xs text-muted-foreground">No stories skipped.</div>
            )}
          </div>
        </div>
      )}
    </StepCard>
  );
}
