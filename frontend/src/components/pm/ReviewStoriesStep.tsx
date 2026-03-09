import { useState } from 'react';
import { Check, ChevronDown, ChevronUp, Loader2, ShieldCheck } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import type { OrchestrationProposal, PlanningSourceRef, ProposedStory } from '@/lib/pmTypes';
import type { StepStatus } from './planningStepUtils';
import { StepCard } from './PlannerSetupStep';

function splitLines(value: string): string[] {
  return value.split('\n').map((item) => item.trim()).filter(Boolean);
}

interface Props {
  status: StepStatus;
  proposal: OrchestrationProposal | null;
  editedStories: ProposedStory[];
  onUpdateStory: <K extends keyof ProposedStory>(index: number, field: K, value: ProposedStory[K]) => void;
  onConfirmPlan: () => void;
  confirmingPlan: boolean;
  canConfirm: boolean;
}

export function ReviewStoriesStep({ status, proposal, editedStories, onUpdateStory, onConfirmPlan, confirmingPlan, canConfirm }: Props) {
  const [expanded, setExpanded] = useState(status === 'current');
  const [expandedCards, setExpandedCards] = useState<Set<number>>(new Set());

  const toggleCard = (index: number) => {
    setExpandedCards((current) => {
      const next = new Set(current);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  };

  if (status === 'upcoming') {
    return (
      <StepCard status={status}>
        <div className="flex items-center gap-2">
          <span className="flex h-5 w-5 items-center justify-center rounded-full bg-muted text-[10px] font-medium text-muted-foreground">5</span>
          <span className="text-sm text-muted-foreground">Review & confirm stories</span>
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
            <span className="text-sm font-medium">Stories confirmed</span>
            <Badge variant="secondary" className="text-[10px]">{editedStories.length} stories</Badge>
          </div>
          <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
        </button>
      </StepCard>
    );
  }

  return (
    <StepCard status={status}>
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium">Review & confirm stories</h4>
        {status === 'completed' && (
          <button type="button" onClick={() => setExpanded(false)}>
            <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
          </button>
        )}
      </div>
      <p className="mt-1 text-xs text-muted-foreground">
        Review the proposed stories, edit as needed, then confirm to create them.
      </p>

      {proposal && (
        <div className="mt-3 space-y-3">
          {proposal.summary && (
            <p className="rounded-md bg-muted/30 p-3 text-xs text-muted-foreground">{proposal.summary}</p>
          )}

          {proposal.risks && proposal.risks.length > 0 && (
            <div className="rounded-md border border-amber-200 bg-amber-50/50 p-3 dark:border-amber-800 dark:bg-amber-950/20">
              <div className="mb-1.5 text-xs font-medium text-amber-800 dark:text-amber-300">Risks</div>
              <div className="space-y-1 text-xs text-amber-700 dark:text-amber-400">
                {proposal.risks.map((risk) => <div key={risk}>- {risk}</div>)}
              </div>
            </div>
          )}

          {proposal.open_questions && proposal.open_questions.length > 0 && (
            <div className="rounded-md border border-blue-200 bg-blue-50/50 p-3 dark:border-blue-800 dark:bg-blue-950/20">
              <div className="mb-1.5 text-xs font-medium text-blue-800 dark:text-blue-300">Open Questions</div>
              <div className="space-y-1 text-xs text-blue-700 dark:text-blue-400">
                {proposal.open_questions.map((item) => <div key={item}>- {item}</div>)}
              </div>
            </div>
          )}
        </div>
      )}

      {editedStories.length > 0 && (
        <div className="mt-4 space-y-2">
          {editedStories.map((story, index) => (
            <StoryCard
              key={`${story.ref ?? index}-${story.name}`}
              story={story}
              index={index}
              expanded={expandedCards.has(index)}
              onToggle={() => toggleCard(index)}
              onUpdate={onUpdateStory}
            />
          ))}

          <div className="pt-2">
            <Button size="sm" onClick={onConfirmPlan} disabled={!canConfirm || confirmingPlan} className="w-full sm:w-auto">
              {confirmingPlan ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ShieldCheck className="h-3.5 w-3.5" />}
              Create {editedStories.length} {editedStories.length === 1 ? 'story' : 'stories'}
            </Button>
          </div>
        </div>
      )}
    </StepCard>
  );
}

function StoryCard({
  story,
  index,
  expanded,
  onToggle,
  onUpdate,
}: {
  story: ProposedStory;
  index: number;
  expanded: boolean;
  onToggle: () => void;
  onUpdate: <K extends keyof ProposedStory>(index: number, field: K, value: ProposedStory[K]) => void;
}) {
  return (
    <div className="rounded-md border border-border/60 p-3">
      <div className="flex items-start gap-3">
        <div className="min-w-0 flex-1">
          <input
            value={story.name}
            onChange={(event) => onUpdate(index, 'name', event.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm font-medium"
          />
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <select
            value={story.story_type}
            onChange={(event) => onUpdate(index, 'story_type', event.target.value)}
            className="rounded-md border border-border bg-background px-2 py-2 text-xs"
          >
            <option value="feature">feature</option>
            <option value="bug">bug</option>
            <option value="chore">chore</option>
          </select>
          <input
            value={story.estimate ?? ''}
            onChange={(event) => onUpdate(index, 'estimate', event.target.value ? Number(event.target.value) : undefined)}
            placeholder="Est."
            className="w-14 rounded-md border border-border bg-background px-2 py-2 text-xs text-center"
          />
        </div>
      </div>

      <Textarea
        value={story.description}
        onChange={(event) => onUpdate(index, 'description', event.target.value)}
        className="mt-2 min-h-[60px] text-xs"
        placeholder="Story description..."
      />

      <button
        type="button"
        className="mt-2 text-[11px] font-medium text-muted-foreground hover:text-foreground"
        onClick={onToggle}
      >
        {expanded ? 'Hide details' : 'Show details (criteria, dependencies, traceability)'}
      </button>

      {expanded && (
        <div className="mt-2 grid gap-3 md:grid-cols-3">
          <div>
            <div className="mb-1 text-[11px] font-medium text-muted-foreground">Acceptance Criteria</div>
            <Textarea
              value={(story.acceptance_criteria ?? []).join('\n')}
              onChange={(event) => onUpdate(index, 'acceptance_criteria', splitLines(event.target.value))}
              className="min-h-[88px] text-xs"
              placeholder="One criterion per line"
            />
          </div>
          <div>
            <div className="mb-1 text-[11px] font-medium text-muted-foreground">Dependencies</div>
            <Textarea
              value={(story.dependency_refs ?? []).join('\n')}
              onChange={(event) => onUpdate(index, 'dependency_refs', splitLines(event.target.value))}
              className="min-h-[88px] text-xs"
              placeholder="One story ref per line"
            />
          </div>
          <div>
            <div className="mb-1 text-[11px] font-medium text-muted-foreground">Ref & Traceability</div>
            <input
              value={story.ref ?? ''}
              onChange={(event) => onUpdate(index, 'ref', event.target.value)}
              placeholder="story_ref"
              className="mb-2 w-full rounded-md border border-border bg-background px-3 py-1.5 text-xs"
            />
            <div className="min-h-[56px] rounded-md border border-border/60 bg-muted/20 p-2 text-xs text-muted-foreground">
              {story.source_refs && story.source_refs.length > 0 ? (
                story.source_refs.map((ref: PlanningSourceRef) => (
                  <div key={`${ref.type}-${ref.id ?? ref.title ?? 'source'}`} className="mb-1">
                    - {ref.title || ref.type}{ref.id ? ` (${ref.id})` : ''}
                  </div>
                ))
              ) : (
                <div>No source refs.</div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
