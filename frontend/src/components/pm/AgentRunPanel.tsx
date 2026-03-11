import { useCallback, useEffect, useMemo, useState } from 'react';
import { Bot, Loader2, Play } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { agentService } from '@/lib/services/agentService';
import type { AgentRun, AgentRunArtifact } from '@/lib/pmTypes';
import { ACTIVE_RUN_STATUSES } from './agentRunConstants';
import { AgentRunTable } from './AgentRunTable';
import { AgentRunDetail, AgentRunDetailEmpty } from './AgentRunDetail';

interface Props {
  storyId: string;
  workspaceId: string;
  assignedAgentId?: string;
}

function mergeArtifactsForDisplay(artifacts: AgentRunArtifact[]): AgentRunArtifact[] {
  const displayArtifacts = artifacts.filter(
    (artifact) => artifact.artifact_type !== 'opencode_stdout_chunk' && artifact.artifact_type !== 'opencode_stderr_chunk',
  );

  const stdoutArtifact = displayArtifacts.find((artifact) => artifact.artifact_type === 'opencode_stdout');
  const stderrArtifact = displayArtifacts.find((artifact) => artifact.artifact_type === 'opencode_stderr');

  if (!stdoutArtifact) {
    const stdoutContent = artifacts
      .filter((artifact) => artifact.artifact_type === 'opencode_stdout_chunk')
      .map((artifact) => artifact.inline_content ?? '')
      .join('');
    if (stdoutContent) {
      displayArtifacts.unshift({
        id: 'live-opencode-stdout',
        workspace_id: artifacts[0]?.workspace_id ?? '',
        run_id: artifacts[0]?.run_id ?? '',
        artifact_type: 'opencode_stdout',
        format: 'text',
        storage_mode: 'inline',
        inline_content: stdoutContent,
        metadata: {},
        sequence_no: -2,
        created_at: artifacts[0]?.created_at ?? new Date().toISOString(),
      });
    }
  }

  if (!stderrArtifact) {
    const stderrContent = artifacts
      .filter((artifact) => artifact.artifact_type === 'opencode_stderr_chunk')
      .map((artifact) => artifact.inline_content ?? '')
      .join('');
    if (stderrContent) {
      displayArtifacts.unshift({
        id: 'live-opencode-stderr',
        workspace_id: artifacts[0]?.workspace_id ?? '',
        run_id: artifacts[0]?.run_id ?? '',
        artifact_type: 'opencode_stderr',
        format: 'text',
        storage_mode: 'inline',
        inline_content: stderrContent,
        metadata: {},
        sequence_no: -1,
        created_at: artifacts[0]?.created_at ?? new Date().toISOString(),
      });
    }
  }

  return displayArtifacts;
}

export function AgentRunPanel({ storyId, workspaceId, assignedAgentId }: Props) {
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [selectedRun, setSelectedRun] = useState<AgentRun | null>(null);
  const [artifacts, setArtifacts] = useState<AgentRunArtifact[]>([]);
  const [triggering, setTriggering] = useState(false);
  const [actingOnRun, setActingOnRun] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchRuns = useCallback(async () => {
    if (!assignedAgentId) {
      setRuns([]);
      setLoading(false);
      return;
    }
    try {
      const res = await agentService.listRuns(workspaceId, assignedAgentId);
      const data = res.data?.data ?? [];
      const storyRuns = (Array.isArray(data) ? data : []).filter(
        (run: AgentRun) => run.target_type === 'story' && run.target_id === storyId
      );
      setRuns(storyRuns);
      setSelectedRun((current) => {
        if (!current) return current;
        return storyRuns.find((run: AgentRun) => run.id === current.id) ?? current;
      });
    } finally {
      setLoading(false);
    }
  }, [workspaceId, assignedAgentId, storyId]);

  const loadArtifacts = useCallback(async (runId: string) => {
    const res = await agentService.listRunArtifacts(workspaceId, runId);
    setArtifacts(res.data ?? []);
  }, [workspaceId]);

  useEffect(() => {
    fetchRuns();
  }, [fetchRuns]);

  useEffect(() => {
    const handler = (e: Event) => {
      const detail = (e as CustomEvent).detail;
      if (detail?.parent_type === 'story' && detail?.parent_id === storyId) {
        void fetchRuns();
        if (selectedRun?.id && detail?.entity_id === selectedRun.id) {
          void loadArtifacts(selectedRun.id);
        }
      }
    };
    window.addEventListener('agent_run-updated', handler);
    window.addEventListener('agent_run-created', handler);
    return () => {
      window.removeEventListener('agent_run-updated', handler);
      window.removeEventListener('agent_run-created', handler);
    };
  }, [storyId, fetchRuns, loadArtifacts, selectedRun?.id]);

  useEffect(() => {
    if (!selectedRun || !ACTIVE_RUN_STATUSES.has(selectedRun.status)) return;
    const interval = window.setInterval(() => {
      void fetchRuns();
      void loadArtifacts(selectedRun.id);
    }, 2000);
    return () => window.clearInterval(interval);
  }, [fetchRuns, loadArtifacts, selectedRun]);

  const displayArtifacts = useMemo(() => mergeArtifactsForDisplay(artifacts), [artifacts]);

  const handleRunAgent = async () => {
    setTriggering(true);
    try {
      await agentService.runAgent(workspaceId, storyId);
      await fetchRuns();
    } finally {
      setTriggering(false);
    }
  };

  const handleSelectRun = async (run: AgentRun) => {
    setSelectedRun(run);
    await loadArtifacts(run.id);
  };

  const handleCancelRun = async (runId: string) => {
    setActingOnRun(runId);
    try {
      await agentService.cancelRun(workspaceId, runId);
      await fetchRuns();
    } finally {
      setActingOnRun(null);
    }
  };

  const handleApproveRun = async (runId: string) => {
    setActingOnRun(runId);
    try {
      await agentService.approveRun(workspaceId, runId, { send_message: true });
      await fetchRuns();
      if (selectedRun?.id === runId) {
        await loadArtifacts(runId);
      }
    } finally {
      setActingOnRun(null);
    }
  };

  // Auto-select the newest run after triggering
  useEffect(() => {
    if (runs.length > 0 && !selectedRun) {
      void handleSelectRun(runs[0]);
    }
  }, [runs.length]);

  if (!assignedAgentId) return null;

  return (
    <div className="mt-6">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Bot className="h-4 w-4 text-muted-foreground" />
          <h3 className="text-sm font-semibold">Agent Runs</h3>
        </div>
        <Button
          size="sm"
          variant="outline"
          onClick={handleRunAgent}
          disabled={triggering}
          className="gap-1.5"
        >
          {triggering ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
          Run Agent
        </Button>
      </div>

      {/* Master-detail vertical split */}
      <div className="flex flex-col rounded-md border border-border/60 overflow-hidden">
        {/* Top: run table */}
        <div className="max-h-[240px] overflow-auto border-b border-border/60">
          <AgentRunTable
            runs={runs}
            selectedRunId={selectedRun?.id ?? null}
            onSelectRun={handleSelectRun}
            loading={loading}
          />
        </div>

        {/* Bottom: detail pane */}
        <div className="min-h-[200px] max-h-[400px] overflow-auto">
          {selectedRun ? (
            <AgentRunDetail
              run={selectedRun}
              artifacts={displayArtifacts}
              actingOnRun={actingOnRun}
              onCancel={handleCancelRun}
              onApprove={handleApproveRun}
            />
          ) : (
            <AgentRunDetailEmpty />
          )}
        </div>
      </div>

      <Separator className="mt-4" />
    </div>
  );
}
