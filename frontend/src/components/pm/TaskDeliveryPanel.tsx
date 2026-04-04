import { useEffect, useMemo, useRef, useState } from 'react';
import { AlertCircleIcon, ArrowRight01Icon, GitBranchIcon, GitPullRequestIcon, Loading01Icon, PlayIcon, FloppyDiskIcon, UserAdd01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Separator } from '@/components/ui/separator';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { agentService } from '@/lib/services/agentService';
import { gitService } from '@/lib/services/gitService';
import { pmTaskService } from '@/lib/services/pmTaskService';
import type {
  Agent,
  GitRepository,
  TaskDeliveryTarget,
  TaskDetail,
} from '@/lib/pmTypes';

const PR_STATUS_COLORS: Record<string, string> = {
  open: 'bg-green-100 text-green-700 border-green-500/30 dark:bg-green-900/30 dark:text-green-400',
  merged: 'bg-sky-100 text-sky-700 border-sky-500/30 dark:bg-sky-900/30 dark:text-sky-400',
  closed: 'bg-zinc-100 text-zinc-600 border-zinc-500/30 dark:bg-zinc-800/30 dark:text-zinc-400',
};

const DELIVERY_STATE_CONFIG: Record<string, { label: string; className: string }> = {
  idle: { label: 'Idle', className: 'bg-muted text-muted-foreground' },
  queued: { label: 'Queued', className: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  running: { label: 'Running', className: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
  awaiting_approval: { label: 'Awaiting Approval', className: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  pr_open: { label: 'PR Open', className: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' },
  pr_merged: { label: 'PR Merged', className: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-400' },
  completed: { label: 'Completed', className: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' },
  failed: { label: 'Failed', className: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' },
};

interface Props {
  workspaceId: string;
  taskDetail: TaskDetail;
  onTaskUpdated: (task: TaskDetail) => void;
}

// ── Hook ──────────────────────────────────────────────────────────

export function useTaskDelivery(workspaceId: string, taskDetail: TaskDetail, onTaskUpdated: (task: TaskDetail) => void) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [target, setTarget] = useState<TaskDeliveryTarget | null>(null);
  const assignedAgentId = taskDetail.task.assigned_agent_id ?? '';
  const [selectedAgentId, setSelectedAgentId] = useState(assignedAgentId);
  const [repositoryId, setRepositoryId] = useState('');
  const [baseBranch, setBaseBranch] = useState('');
  const [loading, setLoading] = useState(true);
  const [savingAssignment, setSavingAssignment] = useState(false);
  const [savingTarget, setSavingTarget] = useState(false);
  const [triggeringRun, setTriggeringRun] = useState(false);
  const assignmentPromiseRef = useRef<Promise<boolean> | null>(null);

  useEffect(() => {
    setSelectedAgentId(assignedAgentId);
  }, [assignedAgentId]);

  useEffect(() => {
    let mounted = true;

    const load = async () => {
      setLoading(true);
      const [agentsRes, reposRes, targetRes] = await Promise.all([
        agentService.list(workspaceId),
        gitService.listRepositories(workspaceId),
        gitService.getTaskDeliveryTarget(workspaceId, taskDetail.task.id),
      ]);
      if (!mounted) {
        return;
      }

      if (agentsRes.error) {
        toast.error(agentsRes.error);
      } else {
        setAgents((agentsRes.data ?? []).filter(isTaskDeliveryAgent));
      }

      if (reposRes.error) {
        toast.error(reposRes.error);
      } else {
        setRepositories(reposRes.data ?? []);
      }

      if (targetRes.error) {
        toast.error(targetRes.error);
        setTarget(null);
      } else {
        const nextTarget = targetRes.data ?? null;
        setTarget(nextTarget);
        setRepositoryId(nextTarget?.repository_id ?? '');
        setBaseBranch(nextTarget?.base_branch ?? '');
      }
      setLoading(false);
    };

    load();
    return () => {
      mounted = false;
    };
  }, [taskDetail.task.id, workspaceId]);

  const hidden = !loading && repositories.length === 0 && !target;

  const selectedAgent = useMemo(
    () => agents.find((agent) => agent.id === selectedAgentId),
    [agents, selectedAgentId],
  );

  const selectedRepository = useMemo(
    () => repositories.find((repository) => repository.id === repositoryId) ?? null,
    [repositories, repositoryId],
  );

  const resolvedBaseBranch = baseBranch.trim() || selectedRepository?.default_branch || 'main';
  const requiresRepo = Boolean(selectedAgent && requiresRepoProfile(selectedAgent));
  const hasDeliveryTarget = Boolean(repositoryId && resolvedBaseBranch);
  const branchPreview = target?.working_branch || buildBranchPreview(taskDetail.task.task_key, taskDetail.task.name);
  const isConfigured = Boolean(assignedAgentId || target?.repository_id);
  const agentSelectionSaved = selectedAgentId === assignedAgentId;
  const deliveryTargetSaved =
    repositoryId === (target?.repository_id ?? '') &&
    (!repositoryId || resolvedBaseBranch === (target?.base_branch ?? ''));

  const refreshTask = async () => {
    const { data, error } = await pmTaskService.get(workspaceId, taskDetail.task.id);
    if (error) {
      toast.error(error);
      return;
    }
    if (data) {
      onTaskUpdated(data);
    }
  };

  const refreshDeliveryTarget = async (syncInputs = false) => {
    const targetRes = await gitService.getTaskDeliveryTarget(workspaceId, taskDetail.task.id);
    if (targetRes.error) {
      toast.error(targetRes.error);
      return false;
    }
    const nextTarget = targetRes.data ?? null;
    setTarget(nextTarget);
    if (syncInputs) {
      setRepositoryId(nextTarget?.repository_id ?? '');
      setBaseBranch(nextTarget?.base_branch ?? '');
    }
    return true;
  };

  const persistDeliveryTarget = async (showSuccessToast: boolean) => {
    if (!repositoryId) {
      toast.error('Choose a repository first');
      return false;
    }
    setSavingTarget(true);
    const { data, error } = await gitService.updateTaskDeliveryTarget(workspaceId, taskDetail.task.id, {
      repository_id: repositoryId,
      base_branch: resolvedBaseBranch,
    });
    setSavingTarget(false);
    if (error) {
      toast.error(error);
      return false;
    }
    setTarget(data ?? null);
    if (data) {
      setRepositoryId(data.repository_id ?? repositoryId);
      setBaseBranch(data.base_branch ?? resolvedBaseBranch);
    } else {
      setBaseBranch(resolvedBaseBranch);
    }
    if (showSuccessToast) {
      toast.success('Delivery target updated');
    }
    return true;
  };

  const ensureDeliveryTargetSaved = async (showSuccessToast: boolean) => {
    if (!repositoryId) {
      toast.error('Choose a repository first');
      return false;
    }
    if (deliveryTargetSaved) {
      return true;
    }
    return persistDeliveryTarget(showSuccessToast);
  };

  const ensureAgentAssigned = async (showSuccessToast: boolean) => {
    if (!selectedAgentId) {
      toast.error('Choose an agent first');
      return false;
    }
    if (agentSelectionSaved) {
      return true;
    }
    if (assignmentPromiseRef.current) {
      return assignmentPromiseRef.current;
    }

    const assignmentPromise = (async () => {
      setSavingAssignment(true);
      const { error } = await agentService.assignToTask(workspaceId, taskDetail.task.id, selectedAgentId);
      setSavingAssignment(false);
      if (error) {
        toast.error(error);
        return false;
      }
      if (showSuccessToast) {
        toast.success('Agent assigned');
      }
      await refreshTask();
      await refreshDeliveryTarget();
      return true;
    })();

    assignmentPromiseRef.current = assignmentPromise;
    try {
      return await assignmentPromise;
    } finally {
      assignmentPromiseRef.current = null;
    }
  };

  const handleSaveDelivery = async () => {
    await ensureDeliveryTargetSaved(true);
  };

  const handleAssignAgent = async () => {
    await ensureAgentAssigned(true);
  };

  const handleAgentChange = async (agentId: string) => {
    setSelectedAgentId(agentId);
    // Auto-assign immediately
    if (assignmentPromiseRef.current) return;
    const assignmentPromise = (async () => {
      setSavingAssignment(true);
      const { error } = await agentService.assignToTask(workspaceId, taskDetail.task.id, agentId);
      setSavingAssignment(false);
      if (error) {
        toast.error(error);
        return false;
      }
      toast.success('Agent assigned');
      await refreshTask();
      await refreshDeliveryTarget(true);
      return true;
    })();
    assignmentPromiseRef.current = assignmentPromise;
    try {
      await assignmentPromise;
    } finally {
      assignmentPromiseRef.current = null;
    }
  };

  const handleRepoChange = async (repoId: string) => {
    setRepositoryId(repoId);
    if (!repoId) return;
    // Auto-save delivery target when repo changes
    setSavingTarget(true);
    const resolved = baseBranch.trim() || repositories.find((r) => r.id === repoId)?.default_branch || 'main';
    const { data, error } = await gitService.updateTaskDeliveryTarget(workspaceId, taskDetail.task.id, {
      repository_id: repoId,
      base_branch: resolved,
    });
    setSavingTarget(false);
    if (error) {
      toast.error(error);
      return;
    }
    setTarget(data ?? null);
    if (data) {
      setBaseBranch(data.base_branch ?? resolved);
    }
    toast.success('Delivery target updated');
  };

  const handleRunNow = async () => {
    if (requiresRepo) {
      const savedDelivery = await ensureDeliveryTargetSaved(false);
      if (!savedDelivery) {
        return;
      }
    }
    const assigned = await ensureAgentAssigned(false);
    if (!assigned) {
      return;
    }

    setTriggeringRun(true);
    const { error } = await agentService.runTask(workspaceId, taskDetail.task.id);
    setTriggeringRun(false);
    if (error) {
      toast.error(error);
      return;
    }
    toast.success('Agent run started');
  };

  const deliveryStateCfg = target?.delivery_state
    ? (DELIVERY_STATE_CONFIG[target.delivery_state] ?? { label: target.delivery_state.replace(/_/g, ' '), className: 'bg-muted text-muted-foreground' })
    : null;

  return {
    agents,
    repositories,
    target,
    selectedAgent,
    selectedAgentId,
    setSelectedAgentId,
    repositoryId,
    setRepositoryId,
    baseBranch,
    setBaseBranch,
    loading,
    hidden,
    savingAssignment,
    savingTarget,
    triggeringRun,
    resolvedBaseBranch,
    requiresRepo,
    hasDeliveryTarget,
    branchPreview,
    isConfigured,
    agentSelectionSaved,
    deliveryTargetSaved,
    deliveryStateCfg,
    selectedRepository,
    handleSaveDelivery,
    handleAssignAgent,
    handleAgentChange,
    handleRepoChange,
    handleRunNow,
    ensureDeliveryTargetSaved,
    ensureAgentAssigned,
  };
}

// ── Card Component (full-page usage) ──────────────────────────────

export function TaskDeliveryPanel({ workspaceId, taskDetail, onTaskUpdated }: Props) {
  const d = useTaskDelivery(workspaceId, taskDetail, onTaskUpdated);
  const [expanded, setExpanded] = useState<boolean | null>(null);

  if (d.hidden) return null;

  if (d.loading) {
    return (
      <div className="mt-6 flex items-center gap-2 py-3 text-xs text-muted-foreground">
        <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
        Loading delivery...
      </div>
    );
  }

  const isExpanded = expanded ?? d.isConfigured;

  return (
    <div className="mt-6 rounded-lg border border-border/70 bg-card">
      {/* Header — always visible, clickable to toggle */}
      <button
        type="button"
        className="flex w-full items-center gap-2 px-3 py-2.5 text-left transition-colors hover:bg-muted/30 cursor-pointer"
        onClick={() => setExpanded(!isExpanded)}
      >
        <ArrowRight01Icon className={`h-3.5 w-3.5 text-muted-foreground transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
        <GitBranchIcon className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="text-sm font-semibold">Delivery</span>
        {!d.isConfigured && !isExpanded && (
          <span className="text-xs text-muted-foreground">Not configured</span>
        )}
        <div className="ml-auto flex items-center gap-1.5">
          {d.deliveryStateCfg && (
            <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium leading-none ${d.deliveryStateCfg.className}`}>
              {d.deliveryStateCfg.label}
            </span>
          )}
        </div>
      </button>

      {isExpanded && (
        <>
          <Separator />

          {/* Form */}
          <div className="space-y-3 px-2 py-2">
            <p className="text-xs text-muted-foreground">
              Pick the agent and repo for this task.
            </p>

            <div className="space-y-2">
              <div className="space-y-1">
                <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                  Agent
                </label>
                <Select value={d.selectedAgentId || undefined} onValueChange={d.setSelectedAgentId}>
                  <SelectTrigger className="h-8 text-sm">
                    {d.selectedAgent ? (
                      <div className="flex items-center gap-2">
                        <AgentAvatar agent={d.selectedAgent} className="h-5 w-5" />
                        <span>{d.selectedAgent.name}</span>
                      </div>
                    ) : (
                      <SelectValue placeholder="Choose agent" />
                    )}
                  </SelectTrigger>
                  <SelectContent>
                    {d.agents.map((agent) => (
                      <SelectItem key={agent.id} value={agent.id}>
                        <div className="flex items-center gap-2">
                          <AgentAvatar agent={agent} className="h-5 w-5" />
                          <span>{agent.name}</span>
                          <span className="text-muted-foreground">· {agentSummaryLabel(agent)}</span>
                        </div>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {d.selectedAgent && (
                  <p className="text-[11px] text-muted-foreground">
                    Starts automatically after assignment.
                  </p>
                )}
                {d.selectedAgentId && !d.agentSelectionSaved && (
                  <p className="text-[11px] text-amber-700 dark:text-amber-400">
                    This agent change is not saved yet. Assign or Run Now will persist it.
                  </p>
                )}
              </div>

              <div className="space-y-1">
                <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                  Repository
                </label>
                <Select value={d.repositoryId || undefined} onValueChange={d.setRepositoryId}>
                  <SelectTrigger className="h-8 text-sm">
                    <SelectValue placeholder="Choose repository" />
                  </SelectTrigger>
                  <SelectContent>
                    {d.repositories.map((repository) => (
                      <SelectItem key={repository.id} value={repository.id}>
                        {repository.full_name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-[11px] text-muted-foreground">
                  Team defaults prefill this. Task delivery can override it.
                </p>
                {d.repositoryId && !d.deliveryTargetSaved && (
                  <p className="text-[11px] text-amber-700 dark:text-amber-400">
                    Repository changes are not saved yet. Save or Run Now will persist them.
                  </p>
                )}
              </div>

              <div className="space-y-1">
                <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                  Base Branch
                </label>
                <Input
                  value={d.baseBranch}
                  onChange={(event) => d.setBaseBranch(event.target.value)}
                  placeholder={d.selectedRepository?.default_branch || 'main'}
                  className="h-8 text-sm"
                />
              </div>

              <div className="space-y-1">
                <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                  Working Branch
                </label>
                <div className="flex h-8 items-center rounded-md border border-border/70 bg-muted/30 px-2.5 text-sm">
                  <span className="truncate font-mono">{d.branchPreview}</span>
                </div>
              </div>
            </div>

            {d.selectedAgent && d.requiresRepo && !d.hasDeliveryTarget && (
              <Alert variant="destructive" className="border-amber-500/30 bg-amber-50 text-amber-800 dark:bg-amber-900/10 dark:text-amber-400 [&>svg]:text-amber-600 dark:[&>svg]:text-amber-400">
                <AlertCircleIcon className="h-4 w-4" />
                <AlertTitle className="text-xs">Repository required</AlertTitle>
                <AlertDescription className="text-[11px] text-amber-700 dark:text-amber-400/80">
                  Choose a repository and base branch, then save.
                </AlertDescription>
              </Alert>
            )}
          </div>

          {/* Actions */}
          <Separator />
          <div className="flex flex-wrap items-center gap-1.5 px-2 py-2">
            <Button
              size="xs"
              variant="outline"
              onClick={d.handleSaveDelivery}
              disabled={d.savingTarget || !d.repositoryId}
            >
              {d.savingTarget ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <FloppyDiskIcon className="h-3 w-3" />}
              Save
            </Button>
            <Button
              size="xs"
              variant="outline"
              onClick={d.handleAssignAgent}
              disabled={d.savingAssignment || !d.selectedAgentId}
            >
              {d.savingAssignment ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <UserAdd01Icon className="h-3 w-3" />}
              Assign
            </Button>
            <Button
              size="xs"
              onClick={d.handleRunNow}
              disabled={d.triggeringRun || d.savingAssignment || d.savingTarget || (d.requiresRepo && !d.hasDeliveryTarget)}
            >
              {d.triggeringRun ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <PlayIcon className="h-3 w-3" />}
              Run Now
            </Button>
          </div>

          {/* Summary */}
          {(d.target?.active_pr_url || d.target?.last_commit_sha || d.target?.repo_full_name) && (
            <>
              <Separator />
              <div className="space-y-1 px-2 py-2 text-xs">
                <div>
                  <p className="mb-0.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Repository</p>
                  <p className="text-xs font-medium">{d.target?.repo_full_name || d.selectedRepository?.full_name || 'Unconfigured'}</p>
                </div>
                <div>
                  <p className="mb-0.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Last Commit</p>
                  <p className="font-mono text-xs">{d.target?.last_commit_sha ? d.target.last_commit_sha.slice(0, 7) : <span className="text-muted-foreground">None</span>}</p>
                </div>
                <div>
                  <p className="mb-0.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Pull Request</p>
                  {d.target?.active_pr_url ? (
                    <a
                      href={d.target.active_pr_url}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1.5 text-xs text-primary hover:underline"
                    >
                      <GitPullRequestIcon className="h-3 w-3" />
                      <span>#{d.target.active_pr_number}</span>
                      {d.target.active_pr_status && (
                        <Badge variant="outline" className={`text-[9px] ${PR_STATUS_COLORS[d.target.active_pr_status] ?? ''}`}>
                          {d.target.active_pr_status}
                        </Badge>
                      )}
                    </a>
                  ) : (
                    <span className="text-xs text-muted-foreground">No PR yet</span>
                  )}
                </div>
              </div>
            </>
          )}
        </>
      )}
    </div>
  );
}

function isTaskDeliveryAgent(agent: { preset_key?: string; allowed_targets?: string[] }) {
  return agent.preset_key === 'task_planner' ||
    agent.preset_key === 'story_planner' ||
    agent.preset_key === 'code_builder' ||
    agent.preset_key === 'review_agent';
}

function agentSummaryLabel(agent: { preset_key?: string; runtime_kind?: string; role?: string }) {
  if (agent.role) return agent.role;
  switch (agent.preset_key) {
    case 'code_builder':
      return 'Code Builder';
    case 'review_agent':
      return 'Review Agent';
    case 'task_planner':
      return 'Task Planner';
    case 'story_planner':
      return 'Task Planner';
    case 'epic_planner':
      return 'Epic Planner';
    case 'support_agent':
      return 'Support Agent';
    case 'crm_operator':
      return 'CRM Operator';
    default:
      return agent.runtime_kind === 'native_sdk' ? 'Interactive Agent' : 'Autonomous Agent';
  }
}

function requiresRepoProfile(agent: { runtime_kind?: string; allowed_tools?: string[] }) {
  if (agent.runtime_kind === 'opencode' || agent.runtime_kind === 'codex') {
    return true;
  }
  return Boolean(
    agent.allowed_tools?.some((tool) =>
      tool === 'write_file' ||
      tool === 'create_branch' ||
      tool === 'commit_and_push' ||
      tool === 'open_pr',
    ),
  );
}

function buildBranchPreview(taskKey: string, taskName: string) {
  return `${taskKey}-${slugify(taskName)}`;
}

function slugify(value: string) {
  const slug = value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return slug || 'task';
}
