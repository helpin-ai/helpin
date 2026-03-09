import { useState } from 'react';
import { ChevronDown, ChevronUp, Clock, FileText, Loader2, StopCircle } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type { AgentRun, AgentRunArtifact } from '@/lib/pmTypes';

const STATUS_CONFIG: Record<string, { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
  queued: { label: 'Queued', variant: 'secondary' },
  running: { label: 'Running', variant: 'default' },
  awaiting_approval: { label: 'Review required', variant: 'secondary' },
  completed: { label: 'Completed', variant: 'outline' },
  failed: { label: 'Failed', variant: 'destructive' },
  cancelled: { label: 'Cancelled', variant: 'secondary' },
};

function getRunStage(run: AgentRun): string {
  const stage = run.input?.stage;
  return typeof stage === 'string' ? stage : '';
}

interface PlanningRunSummary {
  stage?: string;
  spec_document_id?: string;
  spec_version_id?: string;
  summary?: string;
}

function parseJSONValue<T>(value: unknown): T | null {
  if (!value) return null;
  if (typeof value === 'string') {
    try { return JSON.parse(value) as T; } catch { return null; }
  }
  if (typeof value === 'object') return value as T;
  return null;
}

interface Props {
  runs: AgentRun[];
  loadingRuns: boolean;
  selectedRunId: string | null;
  onSelectRun: (id: string) => void;
  selectedRun: AgentRun | null;
  artifacts: AgentRunArtifact[];
  actingOnRun: string | null;
  onCancelRun: (id: string) => void;
}

export function PlanningActivityLog({ runs, loadingRuns, selectedRunId, onSelectRun, selectedRun, artifacts, actingOnRun, onCancelRun }: Props) {
  const [open, setOpen] = useState(false);

  const selectedRunStatus = selectedRun ? (STATUS_CONFIG[selectedRun.status] ?? STATUS_CONFIG.queued) : null;
  const selectedSummary = parseJSONValue<PlanningRunSummary>(selectedRun?.output_summary);
  const otherArtifacts = artifacts.filter(
    (a) => !['story_plan_proposal', 'orchestration_proposal', 'product_spec_draft'].includes(a.artifact_type),
  );

  return (
    <div className="rounded-md border border-border/60">
      <button
        type="button"
        className="flex w-full items-center justify-between p-3"
        onClick={() => setOpen(!open)}
      >
        <div className="flex items-center gap-2">
          <Clock className="h-3.5 w-3.5 text-muted-foreground" />
          <span className="text-xs font-medium">Activity log</span>
          {runs.length > 0 && (
            <Badge variant="secondary" className="text-[10px]">{runs.length} runs</Badge>
          )}
        </div>
        {open ? (
          <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
        ) : (
          <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
        )}
      </button>

      {open && (
        <div className="border-t border-border/60 p-4 space-y-4">
          <div className="space-y-2">
            {loadingRuns ? (
              <div className="flex items-center gap-2 py-4 text-xs text-muted-foreground">
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                Loading runs...
              </div>
            ) : runs.length === 0 ? (
              <p className="py-3 text-xs text-muted-foreground">No planning runs yet.</p>
            ) : (
              runs.map((run) => {
                const config = STATUS_CONFIG[run.status] ?? STATUS_CONFIG.queued;
                const stage = getRunStage(run) || 'legacy';
                const selected = selectedRunId === run.id;
                return (
                  <div key={run.id} className={`rounded-md border p-3 ${selected ? 'border-primary bg-accent/30' : 'border-border/60'}`}>
                    <button type="button" className="w-full text-left" onClick={() => onSelectRun(run.id)}>
                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge variant={config.variant} className="text-[10px]">{config.label}</Badge>
                          <Badge variant="outline" className="text-[10px]">{stage}</Badge>
                          <span className="text-[10px] text-muted-foreground">approval: {run.approval_state}</span>
                        </div>
                        <span className="text-[10px] text-muted-foreground">
                          {run.tokens_used > 0 ? `${run.tokens_used.toLocaleString()} tokens` : 'no tokens yet'}
                        </span>
                      </div>
                      <div className="mt-1 flex flex-wrap gap-3 text-[10px] text-muted-foreground">
                        {run.execution_stage && <span>stage: {run.execution_stage}</span>}
                        {run.error_message && <span className="text-destructive">{run.error_message}</span>}
                      </div>
                    </button>

                    {selected && ['queued', 'running', 'awaiting_approval'].includes(run.status) && (
                      <div className="mt-3 flex flex-wrap gap-2">
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 gap-1 text-[11px]"
                          disabled={actingOnRun === run.id}
                          onClick={() => onCancelRun(run.id)}
                        >
                          {actingOnRun === run.id ? <Loader2 className="h-3 w-3 animate-spin" /> : <StopCircle className="h-3 w-3" />}
                          Cancel
                        </Button>
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>

          {selectedRun && (
            <div className="rounded-md border border-border/60 p-4">
              <div className="flex flex-wrap items-center gap-2">
                <FileText className="h-4 w-4 text-muted-foreground" />
                <h4 className="text-sm font-medium">Selected Run</h4>
                {selectedRunStatus && <Badge variant={selectedRunStatus.variant} className="text-[10px]">{selectedRunStatus.label}</Badge>}
              </div>

              {selectedSummary?.spec_version_id && (
                <p className="mt-2 text-xs text-muted-foreground break-all">
                  Spec version: {selectedSummary.spec_version_id}
                </p>
              )}

              {selectedSummary?.summary && (
                <p className="mt-2 rounded bg-muted/30 p-2 text-xs text-muted-foreground">{selectedSummary.summary}</p>
              )}

              {otherArtifacts.length > 0 && (
                <div className="mt-4 space-y-2">
                  {otherArtifacts.map((artifact) => (
                    <div key={artifact.id} className="rounded border border-border/60 bg-muted/20 p-2">
                      <div className="mb-1 text-[11px] font-medium">
                        {artifact.artifact_type.replace(/_/g, ' ')} <span className="text-muted-foreground">({artifact.format})</span>
                      </div>
                      {artifact.inline_content && (
                        <pre className="max-h-32 overflow-auto whitespace-pre-wrap break-all text-[10px] text-muted-foreground">
                          {artifact.inline_content.slice(0, 2000)}
                          {artifact.inline_content.length > 2000 ? '...' : ''}
                        </pre>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
