import { useState } from 'react';
import { Check, ChevronDown, ChevronUp, ExternalLink, Loader2, ShieldCheck } from 'lucide-react';

import { Button } from '@/components/ui/button';
import type { Epic } from '@/lib/pmTypes';
import type { StepStatus } from './planningStepUtils';
import { StepCard } from './PlannerSetupStep';

interface Props {
  status: StepStatus;
  epic: Epic;
  specDocTitle: string;
  canApprove: boolean;
  pendingClarifyCount: number;
  onApproveSpec: () => void;
  onOpenSpecDoc: () => void;
  approvingSpec: boolean;
}

export function ApproveSpecStep({ status, epic, specDocTitle, canApprove, pendingClarifyCount, onApproveSpec, onOpenSpecDoc, approvingSpec }: Props) {
  const [expanded, setExpanded] = useState(status === 'current');

  if (status === 'upcoming') {
    return (
      <StepCard status={status}>
        <div className="flex items-center gap-2">
          <span className="flex h-5 w-5 items-center justify-center rounded-full bg-muted text-[10px] font-medium text-muted-foreground">4</span>
          <span className="text-sm text-muted-foreground">Approve spec</span>
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
            <span className="text-sm font-medium">Spec approved</span>
          </div>
          <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
        </button>
      </StepCard>
    );
  }

  return (
    <StepCard status={status}>
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium">Approve spec</h4>
        {status === 'completed' && (
          <button type="button" onClick={() => setExpanded(false)}>
            <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
          </button>
        )}
      </div>
      <p className="mt-1 text-xs text-muted-foreground">
        Review the spec in Docs, make any edits, then approve the current version to proceed.
      </p>

      <div className="mt-3 flex flex-wrap gap-2">
        <Button size="sm" variant="outline" onClick={onOpenSpecDoc} disabled={!epic.spec_document_id}>
          <ExternalLink className="h-3.5 w-3.5" />
          Open {specDocTitle} in Docs
        </Button>
        <Button size="sm" onClick={onApproveSpec} disabled={!epic.spec_document_id || approvingSpec || !canApprove}>
          {approvingSpec ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ShieldCheck className="h-3.5 w-3.5" />}
          Approve current version
        </Button>
      </div>

      {!canApprove && pendingClarifyCount > 0 && (
        <div className="mt-3 rounded-md border border-dashed border-amber-300 bg-amber-50 p-2.5 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-300">
          Resolve {pendingClarifyCount} remaining {pendingClarifyCount === 1 ? 'clarification' : 'clarifications'} before approving the spec.
        </div>
      )}
    </StepCard>
  );
}
