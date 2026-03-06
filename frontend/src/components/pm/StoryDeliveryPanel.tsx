import { useEffect, useMemo, useState } from 'react';
import { AlertCircle, Bot, GitBranch, GitPullRequest, Loader2, Play, UserRoundCog } from 'lucide-react';
import { toast } from 'sonner';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
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
  open: 'bg-green-500/15 text-green-700 border-green-500/30',
  merged: 'bg-sky-500/15 text-sky-700 border-sky-500/30',
  closed: 'bg-zinc-500/15 text-zinc-700 border-zinc-500/30',
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
  const [selectedAgentId, setSelectedAgentId] = useState(storyDetail.story.assigned_agent_id ?? '');
  const [repositoryId, setRepositoryId] = useState('');
  const [baseBranch, setBaseBranch] = useState('');
  const [loading, setLoading] = useState(true);
  const [savingAssignment, setSavingAssignment] = useState(false);
  const [savingTarget, setSavingTarget] = useState(false);
  const [triggeringRun, setTriggeringRun] = useState(false);

  useEffect(() => {
    setSelectedAgentId(storyDetail.story.assigned_agent_id ?? '');
  }, [storyDetail.story.assigned_agent_id]);

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

  const requiresRepo = Boolean(selectedAgent && selectedAgent.agent_kind === 'llm' && requiresRepoProfile(selectedAgent.capability_profile));
  const hasDeliveryTarget = Boolean(repositoryId && baseBranch.trim());
  const branchPreview = target?.working_branch || buildBranchPreview(storyDetail.story.display_id, storyDetail.story.name);

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

  const handleSaveDelivery = async () => {
    if (!repositoryId) {
      toast.error('Choose a repository first');
      return;
    }
    setSavingTarget(true);
    const { data, error } = await gitService.updateStoryDeliveryTarget(workspaceId, storyDetail.story.id, {
      repository_id: repositoryId,
      base_branch: baseBranch.trim() || selectedRepository?.default_branch || 'main',
    });
    setSavingTarget(false);
    if (error) {
      toast.error(error);
      return;
    }
    setTarget(data ?? null);
    if (data) {
      setRepositoryId(data.repository_id ?? repositoryId);
      setBaseBranch(data.base_branch ?? baseBranch);
    }
    toast.success('Delivery target updated');
  };

  const handleAssignAgent = async () => {
    if (!selectedAgentId) {
      toast.error('Choose an agent first');
      return;
    }
    setSavingAssignment(true);
    const { error } = await agentService.assignToStory(workspaceId, storyDetail.story.id, selectedAgentId);
    setSavingAssignment(false);
    if (error) {
      toast.error(error);
      return;
    }
    toast.success('Agent assigned');
    await refreshStory();
    const targetRes = await gitService.getStoryDeliveryTarget(workspaceId, storyDetail.story.id);
    if (!targetRes.error) {
      setTarget(targetRes.data ?? null);
    }
  };

  const handleRunNow = async () => {
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
      <div className="mt-6 flex items-center gap-2 text-xs text-muted-foreground">
        <Loader2 className="h-3.5 w-3.5 animate-spin" />
        Loading delivery setup...
      </div>
    );
  }

  return (
    <div className="mt-6 rounded-xl border border-border/70 bg-card/70 p-4">
      <div className="mb-4 flex items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <GitBranch className="h-4 w-4 text-muted-foreground" />
            <h3 className="text-sm font-semibold">Delivery</h3>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            Pick the execution agent and the repository lane this story should ship through.
          </p>
        </div>
        {target?.delivery_state && (
          <Badge variant="outline" className="text-[10px] uppercase tracking-wide">
            {target.delivery_state.replace(/_/g, ' ')}
          </Badge>
        )}
      </div>

      <div className="grid gap-3 md:grid-cols-2">
        <div className="space-y-1.5">
          <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            Agent
          </label>
          <Select value={selectedAgentId || undefined} onValueChange={setSelectedAgentId}>
            <SelectTrigger className="h-9">
              <SelectValue placeholder="Choose agent" />
            </SelectTrigger>
            <SelectContent>
              {agents.map((agent) => (
                <SelectItem key={agent.id} value={agent.id}>
                  <div className="flex items-center gap-2">
                    {agent.agent_kind === 'human' ? (
                      <UserRoundCog className="h-3.5 w-3.5 text-muted-foreground" />
                    ) : (
                      <Bot className="h-3.5 w-3.5 text-muted-foreground" />
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
                  ? 'This agent starts automatically after assignment once the delivery target is valid.'
                  : 'This agent runs manually after assignment.'}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            Repository
          </label>
          <Select value={repositoryId || undefined} onValueChange={setRepositoryId}>
            <SelectTrigger className="h-9">
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
        </div>

        <div className="space-y-1.5">
          <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            Base Branch
          </label>
          <Input
            value={baseBranch}
            onChange={(event) => setBaseBranch(event.target.value)}
            placeholder={selectedRepository?.default_branch || 'main'}
            className="h-9"
          />
        </div>

        <div className="space-y-1.5">
          <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            Working Branch
          </label>
          <div className="flex h-9 items-center rounded-md border border-border/70 bg-muted/30 px-3 text-sm">
            <span className="truncate font-mono text-xs">{branchPreview}</span>
          </div>
        </div>
      </div>

      {selectedAgent && selectedAgent.agent_kind === 'llm' && requiresRepo && !hasDeliveryTarget && (
        <Alert className="mt-4">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>Repository required</AlertTitle>
          <AlertDescription>
            This agent profile needs a delivery target before it can run. Choose a repository and base branch, then save delivery.
          </AlertDescription>
        </Alert>
      )}

      {selectedAgent?.agent_kind === 'human' && (
        <Alert className="mt-4">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>Human handoff</AlertTitle>
          <AlertDescription>
            Human agents are tracked as assignees only. No run will be started from this panel.
          </AlertDescription>
        </Alert>
      )}

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Button
          size="sm"
          variant="outline"
          onClick={handleSaveDelivery}
          disabled={savingTarget || !repositoryId}
        >
          {savingTarget ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : null}
          Save Delivery
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={handleAssignAgent}
          disabled={savingAssignment || !selectedAgentId}
        >
          {savingAssignment ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : null}
          Assign Agent
        </Button>
        {selectedAgent?.agent_kind === 'llm' && selectedAgent.trigger_mode === 'manual' && (
          <Button
            size="sm"
            onClick={handleRunNow}
            disabled={triggeringRun || (requiresRepo && !hasDeliveryTarget)}
          >
            {triggeringRun ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Play className="mr-1.5 h-3.5 w-3.5" />}
            Run Now
          </Button>
        )}
      </div>

      {(target?.active_pr_url || target?.last_commit_sha || target?.repo_full_name) && (
        <div className="mt-4 grid gap-2 rounded-lg border border-border/60 bg-muted/20 p-3 text-xs md:grid-cols-3">
          <div>
            <p className="mb-1 text-[10px] uppercase tracking-wide text-muted-foreground">Repository</p>
            <p className="font-medium">{target?.repo_full_name || selectedRepository?.full_name || 'Unconfigured'}</p>
          </div>
          <div>
            <p className="mb-1 text-[10px] uppercase tracking-wide text-muted-foreground">Last Commit</p>
            <p className="font-mono">{target?.last_commit_sha ? target.last_commit_sha.slice(0, 7) : 'No commits yet'}</p>
          </div>
          <div>
            <p className="mb-1 text-[10px] uppercase tracking-wide text-muted-foreground">Pull Request</p>
            {target?.active_pr_url ? (
              <a
                href={target.active_pr_url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-2 text-primary hover:underline"
              >
                <GitPullRequest className="h-3.5 w-3.5" />
                <span>#{target.active_pr_number}</span>
                {target.active_pr_status && (
                  <Badge variant="outline" className={`text-[10px] ${PR_STATUS_COLORS[target.active_pr_status] ?? ''}`}>
                    {target.active_pr_status}
                  </Badge>
                )}
              </a>
            ) : (
              <p className="text-muted-foreground">No PR yet</p>
            )}
          </div>
        </div>
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
