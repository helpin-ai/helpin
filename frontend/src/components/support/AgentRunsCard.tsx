import { useState, useCallback } from 'react';
import { Bot, Loader2, ShieldCheck } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { AGENT_RUNTIME_LABELS } from '@/lib/agentRuntime';
import { agentService } from '@/lib/services/agentService';
import type { AgentRun, AgentRunArtifact } from '@/lib/pmTypes';
import { timeAgo } from './helpers';

interface AgentRunsCardProps {
  workspaceId: string;
  agentRuns: AgentRun[];
  onApprove: (runId: string) => Promise<void>;
}

export function AgentRunsCard({ workspaceId, agentRuns, onApprove }: AgentRunsCardProps) {
  const [selectedRunId, setSelectedRunId] = useState<string | null>(agentRuns[0]?.id ?? null);
  const [runArtifacts, setRunArtifacts] = useState<AgentRunArtifact[]>([]);
  const [approvingRun, setApprovingRun] = useState<string | null>(null);
  const [loadingArtifacts, setLoadingArtifacts] = useState(false);

  const loadArtifacts = useCallback(async (runId: string) => {
    setLoadingArtifacts(true);
    const res = await agentService.listRunArtifacts(workspaceId, runId);
    if (!res.error) setRunArtifacts(res.data ?? []);
    setLoadingArtifacts(false);
  }, [workspaceId]);

  const handleApprove = async (runId: string) => {
    setApprovingRun(runId);
    try {
      await onApprove(runId);
    } finally {
      setApprovingRun(null);
    }
  };

  if (agentRuns.length === 0) return null;

  return (
    <Card className="space-y-2 p-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Bot className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm font-medium">Agent Runs</span>
        </div>
        {agentRuns[0]?.approval_state === 'pending' && (
          <Button
            size="sm"
            variant="outline"
            className="h-7 gap-1 text-xs"
            disabled={approvingRun === agentRuns[0].id}
            onClick={() => handleApprove(agentRuns[0].id)}
          >
            {approvingRun === agentRuns[0].id ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : (
              <ShieldCheck className="h-3 w-3" />
            )}
            Approve Draft
          </Button>
        )}
      </div>
      <div className="space-y-2">
        {agentRuns.map((run) => (
          <button
            key={run.id}
            type="button"
            className={`w-full rounded border px-2 py-2 text-left text-xs ${
              selectedRunId === run.id ? 'border-primary bg-muted' : 'border-border/60'
            }`}
            onClick={() => {
              setSelectedRunId(run.id);
              loadArtifacts(run.id);
            }}
          >
            <div className="flex items-center justify-between">
              <span>{run.status}</span>
              <span className="text-muted-foreground">{run.approval_state}</span>
            </div>
            <div className="mt-1 text-[10px] text-muted-foreground">
              {timeAgo(run.created_at)} · {AGENT_RUNTIME_LABELS[run.runtime_kind] ?? run.runtime_kind}
            </div>
          </button>
        ))}
      </div>
      {loadingArtifacts && <p className="text-xs text-muted-foreground">Loading artifacts...</p>}
      {runArtifacts.length > 0 && (
        <div className="space-y-2 border-t pt-2">
          {runArtifacts.map((artifact) => (
            <div key={artifact.id} className="rounded border border-border/60 bg-muted/40 p-2">
              <div className="text-[11px] font-medium">
                {artifact.artifact_type}{' '}
                <span className="text-muted-foreground">({artifact.format})</span>
              </div>
              {artifact.inline_content && (
                <pre className="mt-1 max-h-28 overflow-auto whitespace-pre-wrap break-all text-[10px] text-muted-foreground">
                  {artifact.inline_content.slice(0, 1200)}
                  {artifact.inline_content.length > 1200 && '...'}
                </pre>
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}
