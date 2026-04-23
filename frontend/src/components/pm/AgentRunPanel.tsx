import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { BotIcon, Loading01Icon, PlayIcon } from '@/lib/icons';
import { toast } from 'sonner';

import { AgentAvatar, resolveAgentPersonaKey, type AgentPersonaKey } from '@/components/agents/AgentAvatar';
import { NextAgentHint } from '@/components/agents/NextAgentHint';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { AgentRunTable } from '@/components/pm/AgentRunTable';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { agentService } from '@/lib/services/agentService';
import type { Agent, AgentRun } from '@/lib/pmTypes';

interface Props {
  taskId: string;
  workspaceId: string;
  latestRunAgentId?: string | null;
}

export function AgentRunPanel({ taskId, workspaceId, latestRunAgentId }: Props) {
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as { run?: string };
  const urlRunId = search.run ?? null;

  const [agents, setAgents] = useState<Agent[]>([]);
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(urlRunId);
  const [selectedAgentId, setSelectedAgentId] = useState('');
  const [drawerOpen, setDrawerOpen] = useState<boolean>(Boolean(urlRunId));
  const [triggering, setTriggering] = useState(false);
  const [loadingAgents, setLoadingAgents] = useState(true);
  const [loading, setLoading] = useState(true);

  const setRunInUrl = useCallback(
    (runId: string | null) => {
      navigate({
        to: '.',
        search: (prev) => {
          const next = { ...(prev as Record<string, unknown>) };
          delete next.task;
          return { ...next, run: runId ?? undefined };
        },
        replace: true,
      });
    },
    [navigate],
  );

  useEffect(() => {
    if (urlRunId) {
      setSelectedRunId(urlRunId);
      setDrawerOpen(true);
    } else {
      setDrawerOpen(false);
    }
  }, [urlRunId]);

  const fetchAgents = useCallback(async () => {
    setLoadingAgents(true);
    try {
      const res = await agentService.list(workspaceId);
      if (res.error) {
        toast.error(res.error);
        return;
      }
      setAgents(res.data ?? []);
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
  }, [taskId, workspaceId]);

  useEffect(() => {
    void fetchAgents();
  }, [fetchAgents]);

  useEffect(() => {
    void fetchRuns();
  }, [fetchRuns]);

  const taskRunnableAgents = useMemo(() => agents.filter(isTaskRunnableAgent), [agents]);
  const preferredAgent = useMemo(() => {
    if (latestRunAgentId) {
      return taskRunnableAgents.find((agent) => agent.id === latestRunAgentId) ?? null;
    }
    return taskRunnableAgents.find((agent) => agent.preset_key === 'task_planner')
      ?? taskRunnableAgents.find((agent) => agent.preset_key === 'story_planner')
      ?? taskRunnableAgents.find((agent) => agent.preset_key === 'code_builder')
      ?? taskRunnableAgents.find((agent) => agent.preset_key === 'review_agent')
      ?? taskRunnableAgents[0]
      ?? null;
  }, [latestRunAgentId, taskRunnableAgents]);

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

  const startRun = useCallback(async (agentId: string) => {
    const res = await agentService.runTask(workspaceId, taskId, { agent_id: agentId });
    if (res.error) {
      toast.error(res.error);
      return;
    }
    await fetchRuns();
    if (res.data?.id) {
      setRunInUrl(res.data.id);
    }
  }, [fetchRuns, setRunInUrl, taskId, workspaceId]);

  const handleRunAgent = async () => {
    if (!selectedAgentId) return;
    setTriggering(true);
    try {
      await startRun(selectedAgentId);
    } finally {
      setTriggering(false);
    }
  };

  const latestRun = runs[0];
  const latestCompletedAgent = useMemo(() => {
    if (!latestRun || latestRun.status !== 'completed') return null;
    return agents.find((agent) => agent.id === latestRun.agent_id) ?? null;
  }, [agents, latestRun]);
  const completedPersonaKeys = useMemo(() => {
    const agentById = new Map(agents.map((agent) => [agent.id, agent]));
    const keys = new Set<AgentPersonaKey>();
    for (const run of runs) {
      const agent = agentById.get(run.agent_id);
      if (agent) {
        keys.add(resolveAgentPersonaKey({ agent }));
      }
    }
    return keys;
  }, [agents, runs]);

  if (taskRunnableAgents.length === 0 && runs.length === 0 && !loading && !loadingAgents) return null;

  return (
    <div className="mt-6">
      <div className="overflow-hidden rounded-md border border-border/60 bg-card">
        <div className="flex items-center gap-2 px-3 py-2">
          <BotIcon className="h-3.5 w-3.5 text-muted-foreground" />
          <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Agent Runs
          </span>
          {runs.length > 0 && (
            <span className="inline-flex h-5 min-w-5 items-center justify-center rounded border border-border/60 bg-muted px-1.5 text-[11px] font-medium text-muted-foreground">
              {runs.length}
            </span>
          )}
        </div>

        <div className="flex items-center justify-between gap-2 border-t border-border/60 px-3 py-2">
          <div className="flex min-w-0 items-center gap-2">
            <span className="shrink-0 text-xs text-muted-foreground">Start with</span>
            <Select
              value={selectedAgentId || '__none__'}
              onValueChange={(value) => setSelectedAgentId(value === '__none__' ? '' : value)}
            >
              <SelectTrigger size="sm" className="h-7 w-auto min-w-0 gap-1.5 text-xs">
                <SelectValue placeholder={loadingAgents ? 'Loading agents...' : 'Select agent'} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__" className="text-xs">No agent selected</SelectItem>
                {taskRunnableAgents.map((agent) => (
                  <SelectItem key={agent.id} value={agent.id} className="text-xs">
                    <div className="flex items-center gap-1.5">
                      <AgentAvatar agent={agent} className="h-5 w-5 rounded-none border-0 bg-transparent shadow-none" genericBare />
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
            className="h-7 gap-1 px-2.5 text-xs"
          >
            {triggering ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <PlayIcon className="h-3 w-3" />}
            Run
          </Button>
        </div>

        {latestCompletedAgent ? (
          <NextAgentHint
            completedAgent={latestCompletedAgent}
            candidates={taskRunnableAgents}
            completedPersonaKeys={completedPersonaKeys}
            onRun={(agent) => startRun(agent.id)}
          />
        ) : null}

        <div className="border-t border-border/60">
          <AgentRunTable
            runs={runs}
            agents={agents}
            selectedRunId={selectedRunId}
            onSelectRun={(run) => setRunInUrl(run.id)}
            loading={loading}
          />
        </div>
      </div>

      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={(open) => {
          if (!open) setRunInUrl(null);
        }}
        title="Task Agent Run"
      />
    </div>
  );
}

function isTaskRunnableAgent(agent: Agent) {
  return agent.allowed_targets.includes('task');
}
