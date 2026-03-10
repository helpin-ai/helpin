import { useState } from 'react';
import { Check, ChevronDown, ChevronUp, ExternalLink, Loader2, MessageSquare } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import type { SpecClarification } from '@/lib/pmTypes';
import type { StepStatus } from './planningStepUtils';
import { StepCard } from './PlannerSetupStep';

interface Props {
  status: StepStatus;
  clarifications: SpecClarification[];
  saving: boolean;
  specDocTitle: string;
  onUpdateClarification: (id: string, patch: Partial<SpecClarification>) => void;
  onSaveClarifications: () => void;
  onOpenSpecDoc: () => void;
}

function isResolved(item: SpecClarification): boolean {
  if (item.kind === 'open_question') return item.disposition === 'answered' && !!item.response?.trim();
  if (item.kind === 'assumption') {
    if (item.disposition === 'accepted') return true;
    if (item.disposition === 'rejected') return !!item.response?.trim();
  }
  return false;
}

export function ClarifySpecStep({
  status,
  clarifications,
  saving,
  specDocTitle,
  onUpdateClarification,
  onSaveClarifications,
  onOpenSpecDoc,
}: Props) {
  const [expanded, setExpanded] = useState(status === 'current');

  const resolvedCount = clarifications.filter(isResolved).length;
  const pendingCount = clarifications.length - resolvedCount;

  if (status === 'upcoming') {
    return (
      <StepCard status={status}>
        <div className="flex items-center gap-2">
          <span className="flex h-5 w-5 items-center justify-center rounded-full bg-muted text-[10px] font-medium text-muted-foreground">3</span>
          <span className="text-sm text-muted-foreground">Clarify spec</span>
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
            <span className="text-sm font-medium">Clarifications resolved</span>
            <Badge variant="secondary" className="text-[10px]">
              {clarifications.length === 0 ? 'No clarifications needed' : `${resolvedCount}/${clarifications.length} resolved`}
            </Badge>
          </div>
          <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
        </button>
      </StepCard>
    );
  }

  return (
    <StepCard status={status}>
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium">Clarify spec</h4>
        {status === 'completed' && (
          <button type="button" onClick={() => setExpanded(false)}>
            <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
          </button>
        )}
      </div>

      <p className="mt-1 text-xs text-muted-foreground">
        Resolve the planner&apos;s open questions and assumptions before approving the spec. These decisions become part of the canonical spec.
      </p>

      <div className="mt-3 flex flex-wrap gap-2">
        <Button size="sm" variant="outline" onClick={onOpenSpecDoc}>
          <ExternalLink className="h-3.5 w-3.5" />
          Open {specDocTitle} in Docs
        </Button>
        <Button size="sm" onClick={onSaveClarifications} disabled={clarifications.length === 0 || saving}>
          {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <MessageSquare className="h-3.5 w-3.5" />}
          {pendingCount === 0 ? 'Save resolved clarifications' : 'Save clarification responses'}
        </Button>
      </div>

      {clarifications.length === 0 ? (
        <div className="mt-3 rounded-md border border-dashed border-border/60 p-3 text-xs text-muted-foreground">
          The planner did not raise any explicit open questions or assumptions for this draft.
        </div>
      ) : (
        <div className="mt-3 space-y-3">
          <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
            <Badge variant="secondary" className="text-[10px]">{resolvedCount} resolved</Badge>
            <Badge variant={pendingCount === 0 ? 'secondary' : 'outline'} className="text-[10px]">{pendingCount} pending</Badge>
          </div>

          {clarifications.map((item, index) => {
            const resolved = isResolved(item);
            return (
              <div key={item.id} className="rounded-md border border-border/60 p-3">
                <div className="mb-2 flex items-center gap-2">
                  <Badge variant="outline" className="text-[10px]">
                    {item.kind === 'open_question' ? 'Open question' : 'Assumption'}
                  </Badge>
                  <span className="text-[11px] text-muted-foreground">Item {index + 1}</span>
                  {resolved && <Badge variant="secondary" className="text-[10px]">Resolved</Badge>}
                </div>
                <div className="text-sm font-medium">{item.prompt}</div>

                <div className="mt-3">
                  <select
                    value={item.disposition ?? 'pending'}
                    onChange={(event) => onUpdateClarification(item.id, { disposition: event.target.value as SpecClarification['disposition'] })}
                    className="h-9 rounded-md border border-border bg-background px-3 text-xs"
                  >
                    {item.kind === 'open_question' ? (
                      <>
                        <option value="pending">Needs answer</option>
                        <option value="answered">Answered</option>
                      </>
                    ) : (
                      <>
                        <option value="pending">Needs review</option>
                        <option value="accepted">Accept assumption</option>
                        <option value="rejected">Reject assumption</option>
                      </>
                    )}
                  </select>
                </div>

                <Textarea
                  value={item.response ?? ''}
                  onChange={(event) => onUpdateClarification(item.id, { response: event.target.value })}
                  className="mt-2 min-h-[84px] text-xs"
                  placeholder={item.kind === 'open_question' ? 'Add the answer or decision...' : 'Optional note if accepted, required explanation if rejected...'}
                />
              </div>
            );
          })}
        </div>
      )}
    </StepCard>
  );
}
