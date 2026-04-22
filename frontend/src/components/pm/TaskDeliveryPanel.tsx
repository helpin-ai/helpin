import { useEffect, useMemo, useState } from 'react';
import { ArrowRight01Icon, GitBranchIcon, GitPullRequestIcon, Loading01Icon, FloppyDiskIcon, LinkSquare01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { RepositoryBranchPicker } from '@/components/git/RepositoryBranchPicker';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { repositoryDefaultBranchLabel, taskBranchOptionLabel } from '@/lib/branchLabels';
import { gitService } from '@/lib/services/gitService';
import type {
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
  pushed: { label: 'Branch Pushed', className: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
  pr_open: { label: 'PR Open', className: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' },
  pr_merged: { label: 'PR Merged', className: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-400' },
  pr_failed: { label: 'PR Failed', className: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  completed: { label: 'Completed', className: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' },
  failed: { label: 'Failed', className: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' },
};

interface Props {
  workspaceId: string;
  taskDetail: TaskDetail;
  onTaskUpdated: (task: TaskDetail) => void;
}

// ── Hook ──────────────────────────────────────────────────────────

export function useTaskDelivery(workspaceId: string, taskDetail: TaskDetail, _onTaskUpdated: (task: TaskDetail) => void) {
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [target, setTarget] = useState<TaskDeliveryTarget | null>(null);
  const [repositoryId, setRepositoryId] = useState('');
  const [baseBranch, setBaseBranch] = useState('');
  const [loading, setLoading] = useState(true);
  const [savingTarget, setSavingTarget] = useState(false);

  useEffect(() => {
    let mounted = true;

    const load = async () => {
      setLoading(true);
      const [reposRes, targetRes] = await Promise.all([
        gitService.listRepositories(workspaceId),
        gitService.getTaskDeliveryTarget(workspaceId, taskDetail.task.id),
      ]);
      if (!mounted) {
        return;
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

  const selectedRepository = useMemo(
    () => repositories.find((repository) => repository.id === repositoryId) ?? null,
    [repositories, repositoryId],
  );

  const resolvedBaseBranch = baseBranch.trim() || selectedRepository?.default_branch || 'main';
  const branchPreview = target?.working_branch || buildBranchPreview(taskDetail.task.task_key, taskDetail.task.name);
  const isConfigured = Boolean(target?.repository_id);
  const deliveryTargetSaved =
    repositoryId === (target?.repository_id ?? '') &&
    (!repositoryId || resolvedBaseBranch === (target?.base_branch ?? ''));

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

  const handleSaveDelivery = async () => {
    await ensureDeliveryTargetSaved(true);
  };

  const handleBaseBranchChange = async (nextBaseBranch: string) => {
    setBaseBranch(nextBaseBranch);
    if (!repositoryId) {
      toast.error('Choose a repository first');
      return false;
    }

    const resolved = nextBaseBranch.trim() || selectedRepository?.default_branch || 'main';
    if (resolved === resolvedBaseBranch && deliveryTargetSaved) {
      return true;
    }

    setSavingTarget(true);
    const { data, error } = await gitService.updateTaskDeliveryTarget(workspaceId, taskDetail.task.id, {
      repository_id: repositoryId,
      base_branch: resolved,
    });
    setSavingTarget(false);
    if (error) {
      toast.error(error);
      return false;
    }

    setTarget(data ?? null);
    if (data) {
      setRepositoryId(data.repository_id ?? repositoryId);
      setBaseBranch(data.base_branch ?? resolved);
    } else {
      setBaseBranch(resolved);
    }
    toast.success('Delivery target updated');
    return true;
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

  const deliveryStateCfg = target?.delivery_state
    ? (DELIVERY_STATE_CONFIG[target.delivery_state] ?? { label: target.delivery_state.replace(/_/g, ' '), className: 'bg-muted text-muted-foreground' })
    : null;

  return {
    repositories,
    target,
    repositoryId,
    setRepositoryId,
    baseBranch,
    setBaseBranch,
    loading,
    hidden,
    savingTarget,
    resolvedBaseBranch,
    branchPreview,
    isConfigured,
    deliveryTargetSaved,
    deliveryStateCfg,
    selectedRepository,
    handleSaveDelivery,
    handleBaseBranchChange,
    handleRepoChange,
    ensureDeliveryTargetSaved,
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
              Configure the repository and base branch this task should use when code or review agents run.
            </p>

            <div className="space-y-2">
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
                    Repository changes are not saved yet. Save will persist them.
                  </p>
                )}
              </div>

              <div className="space-y-1">
                <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                  Base branch
                </label>
                <RepositoryBranchPicker
                  workspaceId={workspaceId}
                  repositoryId={d.repositoryId || undefined}
                  value={d.baseBranch}
                  onChange={d.setBaseBranch}
                  placeholder={d.selectedRepository?.default_branch || 'main'}
                  emptyLabel={repositoryDefaultBranchLabel(d.selectedRepository?.default_branch)}
                  extraOptions={
                    d.branchPreview
                      ? [{ value: d.branchPreview, label: taskBranchOptionLabel(d.branchPreview) }]
                      : []
                  }
                  disabled={d.savingTarget}
                />
              </div>

              <div className="space-y-1">
                <label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                  Task branch
                </label>
                <div className="flex h-8 items-center rounded-md border border-border/70 bg-muted/30 px-2.5 text-sm">
                  <span className="truncate font-mono">{d.branchPreview}</span>
                </div>
              </div>
            </div>
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
          </div>

          {/* Summary */}
          {(d.target?.active_pr_url || d.target?.last_commit_sha || d.target?.repo_full_name) && (
            <>
              <Separator />
              <div className="space-y-1 px-2 py-2 text-xs">
                <div>
                  <p className="mb-0.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Repository</p>
                  {d.target?.repo_full_name || d.selectedRepository?.full_name ? (
                    <a
                      href={`https://github.com/${d.target?.repo_full_name || d.selectedRepository?.full_name}`}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1 text-xs font-medium text-foreground transition-colors hover:text-primary"
                    >
                      <span>{d.target?.repo_full_name || d.selectedRepository?.full_name}</span>
                      <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0" />
                    </a>
                  ) : (
                    <p className="text-xs font-medium">Unconfigured</p>
                  )}
                </div>
                <div>
                  <p className="mb-0.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Last Commit</p>
                  {d.target?.last_commit_sha && d.target?.repo_full_name ? (
                    <a
                      href={`https://github.com/${d.target.repo_full_name}/commit/${d.target.last_commit_sha}`}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1 font-mono text-xs text-foreground transition-colors hover:text-primary"
                    >
                      <span>{d.target.last_commit_sha.slice(0, 7)}</span>
                      <LinkSquare01Icon className="h-2.5 w-2.5 shrink-0" />
                    </a>
                  ) : (
                    <p className="font-mono text-xs">{d.target?.last_commit_sha ? d.target.last_commit_sha.slice(0, 7) : <span className="text-muted-foreground">None</span>}</p>
                  )}
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
