import { useCallback, useEffect, useMemo, useState } from 'react';
import { Bot, Loader2, Play } from 'lucide-react';
import { toast } from 'sonner';

import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { AgentRunTable } from '@/components/pm/AgentRunTable';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { agentService } from '@/lib/services/agentService';
import type { Agent, AgentRun } from '@/lib/pmTypes';

interface Props {
  taskId: string;
  workspaceId: string;
  assignedAgentId?: string;
}

export function AgentRunPanel({ taskId, workspaceId, assignedAgentId }: Props) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [selectedAgentId, setSelectedAgentId] = useState('');
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [triggering, setTriggering] = useState(false);
  const [loadingAgents, setLoadingAgents] = useState(true);
  const [loading, setLoading] = useState(true);

  const fetchAgents = useCallback(async () => {
    setLoadingAgents(true);
    try {
      const res = await agentService.list(workspaceId);
      if (res.error) {
        toast.error(res.error);
        return;
      }
      setAgents((res.data ?? []).filter(isTaskRunnableAgent));
    } finally {
      setLoadingAgents(false);
    }
  }, [workspaceId]);

  const fetchRuns = useCallback(async () => {
    try {
      const res = await agentService.listTargetRuns(workspaceId, 'task', taskId);
      setRuns(res.data ?? []);
      setSelectedRunId((current) => current && (res.data ?? []).some((run) => run.id === current) ? current : (res.data?.[0]?.id ?? null));
    } finally {
      setLoading(false);
    }
  }, [assignedAgentId, taskId, workspaceId]);

  useEffect(() => {
    void fetchAgents();
  }, [fetchAgents]);

  useEffect(() => {
    void fetchRuns();
  }, [fetchRuns]);

  const taskRunnableAgents = useMemo(() => agents.filter(isTaskRunnableAgent), [agents]);
  const preferredAgent = useMemo(() => {
    if (assignedAgentId) {
      return taskRunnableAgents.find((agent) => agent.id === assignedAgentId) ?? null;
    }
    return taskRunnableAgents.find((agent) => agent.preset_key === 'task_planner')
      ?? taskRunnableAgents.find((agent) => agent.preset_key === 'story_planner')
      ?? taskRunnableAgents.find((agent) => agent.preset_key === 'code_builder')
      ?? taskRunnableAgents.find((agent) => agent.preset_key === 'review_agent')
      ?? taskRunnableAgents[0]
      ?? null;
  }, [assignedAgentId, taskRunnableAgents]);
  const selectedAgent = useMemo(
    () => taskRunnableAgents.find((agent) => agent.id === selectedAgentId) ?? null,
    [selectedAgentId, taskRunnableAgents],
  );

  useEffect(() => {
    if (!selectedAgentId && preferredAgent) {
      setSelectedAgentId(preferredAgent.id);
      return;
    }
    if (selectedAgentId && !taskRunnableAgents.some((agent) => agent.id === selectedAgentId)) {
      setSelectedAgentId(preferredAgent?.id ?? '');
    }
  }, [preferredAgent, selectedAgentId, taskRunnableAgents]);

  useEffect(() => {
    const handler = (event: Event) => {
      const detail = (event as CustomEvent).detail as { parent_type?: string; parent_id?: string } | undefined;
      if (detail?.parent_type === 'task' && detail.parent_id === taskId) {
        void fetchRuns();
      }
    };
    window.addEventListener('agent_run-updated', handler);
    window.addEventListener('agent_run-created', handler);
    return () => {
      window.removeEventListener('agent_run-updated', handler);
      window.removeEventListener('agent_run-created', handler);
    };
  }, [fetchRuns, taskId]);

  const handleRunAgent = async () => {
    if (!selectedAgentId) return;
    setTriggering(true);
    try {
      const res = await agentService.runTask(workspaceId, taskId, { agent_id: selectedAgentId });
      if (res.error) {
        toast.error(res.error);
        return;
      }
      await fetchRuns();
      if (res.data?.id) {
        setSelectedRunId(res.data.id);
        setDrawerOpen(true);
      }
    } finally {
      setTriggering(false);
    }
  };

  if (taskRunnableAgents.length === 0 && runs.length === 0 && !loading && !loadingAgents) return null;

  return (
    <div className="mt-6">
      <div className="mb-3 space-y-3">
        <div className="flex items-center gap-2">
          {selectedAgent ? <AgentAvatar agent={selectedAgent} className="h-6 w-6" /> : <Bot className="h-4 w-4 text-muted-foreground" />}
          <h3 className="text-sm font-semibold">Agent Runs</h3>
        </div>

        <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
          <div className="space-y-1.5">
            <Label className="text-xs">Start with</Label>
            <Select value={selectedAgentId || '__none__'} onValueChange={(value) => setSelectedAgentId(value === '__none__' ? '' : value)}>
              <SelectTrigger>
                <SelectValue placeholder={loadingAgents ? 'Loading agents...' : 'Select a planner or coder'} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__">No agent selected</SelectItem>
                {taskRunnableAgents.map((agent) => (
                  <SelectItem key={agent.id} value={agent.id}>
                    <div className="flex items-center gap-2">
                      <AgentAvatar agent={agent} className="h-5 w-5" />
                      <span>{agent.name}{agent.role ? ` · ${agent.role}` : ''}</span>
                    </div>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={handleRunAgent}
            disabled={triggering || loadingAgents || !selectedAgentId}
            className="gap-1.5 md:min-w-40"
          >
            {triggering ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
            Start Run
          </Button>
        </div>
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

      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={setDrawerOpen}
        title="Task Agent Run"
      />

      <Separator className="mt-4" />
    </div>
  );
}

function isTaskRunnableAgent(agent: Agent) {
  return agent.allowed_targets.includes('task');
}
