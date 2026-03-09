import { useState } from 'react';
import { Check, ChevronDown, ChevronUp, ExternalLink, Loader2, MessageSquare, Play } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import type { AgentRun, Epic } from '@/lib/pmTypes';
import type { StepStatus } from './planningStepUtils';
import { humanizeError } from './planningStepUtils';
import { StepCard } from './PlannerSetupStep';

interface ProductSpecDraft {
  title: string;
  summary: string;
  spec_markdown: string;
  risks?: string[];
  open_questions?: string[];
  sources?: {
    title: string;
    url: string;
    note?: string;
    published_at?: string;
  }[];
}

interface Props {
  status: StepStatus;
  epic: Epic;
  specDocTitle: string;
  latestDraftRun: AgentRun | null;
  specDraft: ProductSpecDraft | null;
  additionalContext: string;
  onAdditionalContextChange: (value: string) => void;
  onDraftSpec: () => void;
  onOpenSpecDoc: () => void;
  triggeringDraft: boolean;
}

export function DraftSpecStep({
  status,
  epic,
  specDocTitle,
  latestDraftRun,
  specDraft,
  additionalContext,
  onAdditionalContextChange,
  onDraftSpec,
  onOpenSpecDoc,
  triggeringDraft,
}: Props) {
  const [expanded, setExpanded] = useState(status === 'current');
  const [showNotes, setShowNotes] = useState(false);
  const [showPreview, setShowPreview] = useState(false);

  const isRunning = latestDraftRun && ['queued', 'running'].includes(latestDraftRun.status);
  const hasFailed = latestDraftRun?.status === 'failed';

  if (status === 'upcoming') {
    return (
      <StepCard status={status}>
        <div className="flex items-center gap-2">
          <span className="flex h-5 w-5 items-center justify-center rounded-full bg-muted text-[10px] font-medium text-muted-foreground">2</span>
          <span className="text-sm text-muted-foreground">Draft spec</span>
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
            <span className="text-sm font-medium">Spec drafted</span>
            {epic.spec_document_id && (
              <Badge variant="secondary" className="text-[10px]">{specDocTitle}</Badge>
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
        <h4 className="text-sm font-medium">Draft spec</h4>
        <div className="flex items-center gap-2">
          {status === 'completed' && (
            <button type="button" onClick={() => setExpanded(false)}>
              <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
            </button>
          )}
        </div>
      </div>
      <p className="mt-1 text-xs text-muted-foreground">
        The AI planner will draft a product spec into Docs using the epic planning repository as live context. You can edit it there before approving.
      </p>

      <div className="mt-3 flex flex-wrap gap-2">
        <Button size="sm" onClick={onDraftSpec} disabled={!epic.planning_repository_id || triggeringDraft || !!isRunning}>
          {triggeringDraft || isRunning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
          {latestDraftRun ? 'Redraft spec' : 'Draft spec'}
        </Button>
        {epic.spec_document_id && (
          <Button size="sm" variant="outline" onClick={onOpenSpecDoc}>
            <ExternalLink className="h-3.5 w-3.5" />
            Open in Docs
          </Button>
        )}
        <Button size="sm" variant="ghost" onClick={() => setShowNotes(!showNotes)}>
          <MessageSquare className="h-3.5 w-3.5" />
          {showNotes ? 'Hide notes' : 'Add notes for AI'}
        </Button>
      </div>

      {!epic.planning_repository_id && (
        <div className="mt-3 rounded-md border border-dashed border-amber-300 bg-amber-50 p-2.5 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-300">
          Configure a planning repository in epic settings before drafting the PRD. The planner now uses live repo context during `draft_spec`.
        </div>
      )}

      {showNotes && (
        <Textarea
          value={additionalContext}
          onChange={(event) => onAdditionalContextChange(event.target.value)}
          placeholder="Context, constraints, or specific areas to focus on..."
          className="mt-3 min-h-[72px] text-xs"
        />
      )}

      {isRunning && (
        <div className="mt-3 flex items-center gap-2 rounded-md bg-primary/5 p-2.5 text-xs text-primary">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          AI is drafting your spec...
        </div>
      )}

      {hasFailed && (
        <div className="mt-3 rounded-md border border-destructive/30 bg-destructive/5 p-2.5 text-xs text-destructive">
          {humanizeError(latestDraftRun?.error_message)}
        </div>
      )}

      {specDraft && (
        <div className="mt-3">
          <button
            type="button"
            className="text-xs font-medium text-muted-foreground hover:text-foreground"
            onClick={() => setShowPreview(!showPreview)}
          >
            {showPreview ? 'Hide draft preview' : 'Show draft preview'}
          </button>
          {showPreview && (
            <div className="mt-2 space-y-3 rounded-md border border-border/60 p-3">
              <div>
                <h5 className="text-sm font-medium">{specDraft.title || 'Draft output'}</h5>
                {specDraft.summary && (
                  <p className="mt-1 text-xs text-muted-foreground">{specDraft.summary}</p>
                )}
              </div>
              {specDraft.risks && specDraft.risks.length > 0 && (
                <div>
                  <div className="mb-1 text-xs font-medium">Risks</div>
                  <div className="space-y-1 text-xs text-muted-foreground">
                    {specDraft.risks.map((risk) => <div key={risk}>- {risk}</div>)}
                  </div>
                </div>
              )}
              {specDraft.sources && specDraft.sources.length > 0 && (
                <div>
                  <div className="mb-1 text-xs font-medium">Research sources</div>
                  <div className="space-y-1 text-xs text-muted-foreground">
                    {specDraft.sources.map((source) => (
                      <div key={`${source.url}-${source.title}`}>
                        - {source.title} ({source.url})
                        {source.note ? ` - ${source.note}` : ''}
                      </div>
                    ))}
                  </div>
                </div>
              )}
              <pre className="max-h-72 overflow-auto whitespace-pre-wrap rounded-md bg-muted/30 p-3 text-[11px] text-muted-foreground">
                {specDraft.spec_markdown}
              </pre>
            </div>
          )}
        </div>
      )}
    </StepCard>
  );
}
