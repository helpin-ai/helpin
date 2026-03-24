import { useCallback, useEffect, useState } from 'react';
import { Bot, Loader2, Play } from 'lucide-react';

import { AgentRunDrawer } from '@/components/pm/AgentRunDrawer';
import { AgentRunTable } from '@/components/pm/AgentRunTable';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { agentService } from '@/lib/services/agentService';
import type { AgentRun } from '@/lib/pmTypes';

interface Props {
  storyId: string;
  workspaceId: string;
  assignedAgentId?: string;
}

export function AgentRunPanel({ storyId, workspaceId, assignedAgentId }: Props) {
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [triggering, setTriggering] = useState(false);
  const [loading, setLoading] = useState(true);

  const fetchRuns = useCallback(async () => {
    try {
      const res = await agentService.listTargetRuns(workspaceId, 'story', storyId);
      setRuns(res.data ?? []);
      setSelectedRunId((current) => current && (res.data ?? []).some((run) => run.id === current) ? current : (res.data?.[0]?.id ?? null));
    } finally {
      setLoading(false);
    }
  }, [assignedAgentId, storyId, workspaceId]);

  useEffect(() => {
    void fetchRuns();
  }, [fetchRuns]);

  useEffect(() => {
    const handler = (event: Event) => {
      const detail = (event as CustomEvent).detail as { parent_type?: string; parent_id?: string } | undefined;
      if (detail?.parent_type === 'story' && detail.parent_id === storyId) {
        void fetchRuns();
      }
    };
    window.addEventListener('agent_run-updated', handler);
    window.addEventListener('agent_run-created', handler);
    return () => {
      window.removeEventListener('agent_run-updated', handler);
      window.removeEventListener('agent_run-created', handler);
    };
  }, [fetchRuns, storyId]);

  const handleRunAgent = async () => {
    if (!assignedAgentId) return;
    setTriggering(true);
    try {
      const res = await agentService.runStory(workspaceId, storyId);
      await fetchRuns();
      if (res.data?.id) {
        setSelectedRunId(res.data.id);
        setDrawerOpen(true);
      }
    } finally {
      setTriggering(false);
    }
  };

  if (!assignedAgentId && runs.length === 0 && !loading) return null;

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
          disabled={triggering || !assignedAgentId}
          className="gap-1.5"
        >
          {triggering ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
          Run Agent
        </Button>
      </div>

      <div className="overflow-hidden rounded-md border border-border/60">
        <AgentRunTable
          runs={runs}
          selectedRunId={selectedRunId}
          onSelectRun={(run) => {
            setSelectedRunId(run.id);
            setDrawerOpen(true);
          }}
          loading={loading}
        />
      </div>

      <AgentRunDrawer
        workspaceId={workspaceId}
        runId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={setDrawerOpen}
        canEdit
        title="Story Agent Run"
      />

      <Separator className="mt-4" />
    </div>
  );
}
