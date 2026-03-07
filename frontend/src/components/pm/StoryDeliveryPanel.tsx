import { useEffect, useMemo, useRef, useState } from 'react';
import { AlertCircle, Bot, ChevronRight, GitBranch, GitPullRequest, Loader2, Play, Save, UserRoundCog, UserPlus } from 'lucide-react';
import { toast } from 'sonner';
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
import { pmStoryService } from '@/lib/services/pmStoryService';
import type {
  Agent,
  GitRepository,
  StoryDeliveryTarget,
  StoryDetail,
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
  storyDetail: StoryDetail;
  onStoryUpdated: (story: StoryDetail) => void;
}

export function StoryDeliveryPanel({ workspaceId, storyDetail, onStoryUpdated }: Props) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [target, setTarget] = useState<StoryDeliveryTarget | null>(null);
  const assignedAgentId = storyDetail.story.assigned_agent_id ?? '';
  const [selectedAgentId, setSelectedAgentId] = useState(assignedAgentId);
  const [repositoryId, setRepositoryId] = useState('');
  const [baseBranch, setBaseBranch] = useState('');
  const [loading, setLoading] = useState(true);
  const [savingAssignment, setSavingAssignment] = useState(false);
  const [savingTarget, setSavingTarget] = useState(false);
  const [triggeringRun, setTriggeringRun] = useState(false);
  const [expanded, setExpanded] = useState<boolean | null>(null);
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
        gitService.getStoryDeliveryTarget(workspaceId, storyDetail.story.id),
      ]);
      if (!mounted) {
        return;
      }

      if (agentsRes.error) {
        toast.error(agentsRes.error);
      } else {
        setAgents(agentsRes.data ?? []);
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
  }, [storyDetail.story.id, workspaceId]);

  const selectedAgent = useMemo(
    () => agents.find((agent) => agent.id === selectedAgentId),
    [agents, selectedAgentId],
  );

  const selectedRepository = useMemo(
    () => repositories.find((repository) => repository.id === repositoryId) ?? null,
    [repositories, repositoryId],
  );

  const resolvedBaseBranch = baseBranch.trim() || selectedRepository?.default_branch || 'main';
  const requiresRepo = Boolean(selectedAgent && selectedAgent.agent_kind === 'llm' && requiresRepoProfile(selectedAgent.capability_profile));
  const hasDeliveryTarget = Boolean(repositoryId && resolvedBaseBranch);
  const branchPreview = target?.working_branch || buildBranchPreview(storyDetail.story.display_id, storyDetail.story.name);
  const isConfigured = Boolean(assignedAgentId || target?.repository_id);
  const agentSelectionSaved = selectedAgentId === assignedAgentId;
  const deliveryTargetSaved =
    repositoryId === (target?.repository_id ?? '') &&
    (!repositoryId || resolvedBaseBranch === (target?.base_branch ?? ''));

  // Auto-expand when configured, collapse when not — but only on initial load
  const isExpanded = expanded ?? isConfigured;

  const refreshStory = async () => {
    const { data, error } = await pmStoryService.get(workspaceId, storyDetail.story.id);
    if (error) {
      toast.error(error);
      return;
    }
    if (data) {
      onStoryUpdated(data);
    }
  };

  const refreshDeliveryTarget = async (syncInputs = false) => {
    const targetRes = await gitService.getStoryDeliveryTarget(workspaceId, storyDetail.story.id);
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
    const { data, error } = await gitService.updateStoryDeliveryTarget(workspaceId, storyDetail.story.id, {
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
      const { error } = await agentService.assignToStory(workspaceId, storyDetail.story.id, selectedAgentId);
      setSavingAssignment(false);
      if (error) {
        toast.error(error);
        return false;
      }
      if (showSuccessToast) {
        toast.success('Agent assigned');
      }
      await refreshStory();
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
    const { error } = await agentService.runAgent(workspaceId, storyDetail.story.id);
    setTriggeringRun(false);
    if (error) {
      toast.error(error);
      return;
    }
    toast.success('Agent run started');
  };

  if (loading) {
    return (
      <div className="mt-6 flex items-center gap-2 py-3 text-xs text-muted-foreground">
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
        Loading delivery...
      </div>
    );
  }

  const deliveryStateCfg = target?.delivery_state
    ? (DELIVERY_STATE_CONFIG[target.delivery_state] ?? { label: target.delivery_state.replace(/_/g, ' '), className: 'bg-muted text-muted-foreground' })
    : null;

  return (
    <div className="mt-6 rounded-lg border border-border/70 bg-card">
      {/* Header — always visible, clickable to toggle */}
      <button
        type="button"
        className="flex w-full items-center gap-2 px-3 py-2.5 text-left transition-colors hover:bg-muted/30 cursor-pointer"
        onClick={() => setExpanded(!isExpanded)}
      >
        <ChevronRight className={`h-3.5 w-3.5 text-muted-foreground transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
        <GitBranch className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="text-sm font-semibold">Delivery</span>
        {!isConfigured && !isExpanded && (
          <span className="text-xs text-muted-foreground">Not configured</span>
        )}
        <div className="ml-auto flex items-center gap-1.5">
          {deliveryStateCfg && (
            <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium leading-none ${deliveryStateCfg.className}`}>
              {deliveryStateCfg.label}
            </span>
          )}
        </div>
      </button>

      {isExpanded && (
        <>
          <Separator />

          {/* Form */}
          <div className="space-y-3 px-3 py-3">
            <p className="text-xs text-muted-foreground">
              Pick the execution agent and the repository lane this story should ship through.
            </p>

            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-1">
                <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                  Agent
                </label>
                <Select value={selectedAgentId || undefined} onValueChange={setSelectedAgentId}>
                  <SelectTrigger className="h-8 text-sm">
                    <SelectValue placeholder="Choose agent" />
                  </SelectTrigger>
                  <SelectContent>
                    {agents.map((agent) => (
                      <SelectItem key={agent.id} value={agent.id}>
                        <div className="flex items-center gap-2">
                          {agent.agent_kind === 'human' ? (
                            <UserRoundCog className="h-3 w-3 text-muted-foreground" />
                          ) : (
                            <Bot className="h-3 w-3 text-muted-foreground" />
                          )}
                          <span>{agent.name}</span>
                          <span className="text-muted-foreground">· {agent.capability_profile}</span>
                        </div>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {selectedAgent && (
                  <p className="text-[11px] text-muted-foreground">
                    {selectedAgent.agent_kind === 'human'
                      ? 'Human agents are assignment-only and do not execute code.'
                      : selectedAgent.trigger_mode === 'auto_on_assignment'
                        ? 'Starts automatically after assignment.'
                        : 'Runs manually after assignment.'}
                  </p>
                )}
                {selectedAgentId && !agentSelectionSaved && (
                  <p className="text-[11px] text-amber-700 dark:text-amber-400">
                    This agent change is not saved yet. Assign or Run Now will persist it.
                  </p>
                )}
              </div>

              <div className="space-y-1">
                <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                  Repository
                </label>
                <Select value={repositoryId || undefined} onValueChange={setRepositoryId}>
                  <SelectTrigger className="h-8 text-sm">
                    <SelectValue placeholder="Choose repository" />
                  </SelectTrigger>
                  <SelectContent>
                    {repositories.map((repository) => (
                      <SelectItem key={repository.id} value={repository.id}>
                        {repository.full_name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-[11px] text-muted-foreground">
                  Team defaults prefill this. Story delivery can override it.
                </p>
                {repositoryId && !deliveryTargetSaved && (
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
                  value={baseBranch}
                  onChange={(event) => setBaseBranch(event.target.value)}
                  placeholder={selectedRepository?.default_branch || 'main'}
                  className="h-8 text-sm"
                />
              </div>

              <div className="space-y-1">
                <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                  Working Branch
                </label>
                <div className="flex h-8 items-center rounded-md border border-border/70 bg-muted/30 px-2.5 text-sm">
                  <span className="truncate font-mono">{branchPreview}</span>
                </div>
              </div>
            </div>

            {selectedAgent && selectedAgent.agent_kind === 'llm' && requiresRepo && !hasDeliveryTarget && (
              <Alert variant="destructive" className="border-amber-500/30 bg-amber-50 text-amber-800 dark:bg-amber-900/10 dark:text-amber-400 [&>svg]:text-amber-600 dark:[&>svg]:text-amber-400">
                <AlertCircle className="h-4 w-4" />
                <AlertTitle className="text-xs">Repository required</AlertTitle>
                <AlertDescription className="text-[11px] text-amber-700 dark:text-amber-400/80">
                  Choose a repository and base branch, then save.
                </AlertDescription>
              </Alert>
            )}

            {selectedAgent?.agent_kind === 'human' && (
              <Alert>
                <UserRoundCog className="h-4 w-4" />
                <AlertTitle className="text-xs">Human handoff</AlertTitle>
                <AlertDescription className="text-xs">
                  Human agents are tracked as assignees only. No run will be started.
                </AlertDescription>
              </Alert>
            )}
          </div>

          {/* Actions */}
          <Separator />
          <div className="flex flex-wrap items-center gap-1.5 px-3 py-2">
            <Button
              size="xs"
              variant="outline"
              onClick={handleSaveDelivery}
              disabled={savingTarget || !repositoryId}
            >
              {savingTarget ? <Loader2 className="h-3 w-3 animate-spin" /> : <Save className="h-3 w-3" />}
              Save
            </Button>
            <Button
              size="xs"
              variant="outline"
              onClick={handleAssignAgent}
              disabled={savingAssignment || !selectedAgentId}
            >
              {savingAssignment ? <Loader2 className="h-3 w-3 animate-spin" /> : <UserPlus className="h-3 w-3" />}
              Assign
            </Button>
            {selectedAgent?.agent_kind === 'llm' && selectedAgent.trigger_mode === 'manual' && (
              <Button
                size="xs"
                onClick={handleRunNow}
                disabled={triggeringRun || savingAssignment || savingTarget || (requiresRepo && !hasDeliveryTarget)}
              >
                {triggeringRun ? <Loader2 className="h-3 w-3 animate-spin" /> : <Play className="h-3 w-3" />}
                Run Now
              </Button>
            )}
          </div>

          {/* Summary */}
          {(target?.active_pr_url || target?.last_commit_sha || target?.repo_full_name) && (
            <>
              <Separator />
              <div className="grid gap-3 px-3 py-2.5 text-xs md:grid-cols-3">
                <div>
                  <p className="mb-0.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Repository</p>
                  <p className="text-xs font-medium">{target?.repo_full_name || selectedRepository?.full_name || 'Unconfigured'}</p>
                </div>
                <div>
                  <p className="mb-0.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Last Commit</p>
                  <p className="font-mono text-xs">{target?.last_commit_sha ? target.last_commit_sha.slice(0, 7) : <span className="text-muted-foreground">None</span>}</p>
                </div>
                <div>
                  <p className="mb-0.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Pull Request</p>
                  {target?.active_pr_url ? (
                    <a
                      href={target.active_pr_url}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1.5 text-xs text-primary hover:underline"
                    >
                      <GitPullRequest className="h-3 w-3" />
                      <span>#{target.active_pr_number}</span>
                      {target.active_pr_status && (
                        <Badge variant="outline" className={`text-[9px] ${PR_STATUS_COLORS[target.active_pr_status] ?? ''}`}>
                          {target.active_pr_status}
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

function requiresRepoProfile(profile: string) {
  return profile === 'engineer' || profile === 'reviewer_tester';
}

function buildBranchPreview(displayId: number, storyName: string) {
  return `tp-${displayId}-${slugify(storyName)}`;
}

function slugify(value: string) {
  const slug = value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return slug || 'story';
}
