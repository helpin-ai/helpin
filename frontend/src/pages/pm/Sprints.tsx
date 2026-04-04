import { useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { useTitle } from '@/hooks/useTitle';
import { SprintPlanningFilters, type SprintStatusFilter } from '@/components/pm/sprints/SprintPlanningFilters';
import { SprintPlanningWorkspace } from '@/components/pm/sprints/SprintPlanningWorkspace';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useSprintPlanningWorkspace, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { queryKeys } from '@/lib/queryKeys';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import type { SprintPlanningWorkspace as SprintPlanningWorkspaceData, SprintPlanningTaskPreview } from '@/lib/pmTypes';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { unwrap } from '@/lib/queryUtils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';

const TASK_PREVIEW_LIMIT = 20;
const BACKLOG_LIMIT = 50;

interface SprintsPageProps {
  teamId?: string;
}

function clonePlanningWorkspace(workspace: SprintPlanningWorkspaceData): SprintPlanningWorkspaceData {
  const buckets = workspace.buckets ?? [];
  const backlogTasks = workspace.backlog_tasks ?? [];
  return {
    ...workspace,
    buckets: buckets.map((bucket) => ({
      ...bucket,
      sprints: (bucket.sprints ?? []).map((card) => ({
        ...card,
        stats: { ...card.stats },
        preview_tasks: [...(card.preview_tasks ?? [])],
      })),
    })),
    backlog_tasks: [...backlogTasks],
  };
}

function removeTaskFromCards(workspace: SprintPlanningWorkspaceData, taskId: string) {
  let found: SprintPlanningTaskPreview | null = null;
  let sourceSprintId: string | null = null;
  const backlogTasks = workspace.backlog_tasks ?? [];

  const backlogIndex = backlogTasks.findIndex((t) => t.id === taskId);
  if (backlogIndex >= 0) {
    found = backlogTasks[backlogIndex];
    backlogTasks.splice(backlogIndex, 1);
    workspace.backlog_tasks = backlogTasks;
    workspace.backlog_total = Math.max(0, workspace.backlog_total - 1);
  }

  for (const bucket of (workspace.buckets ?? [])) {
    for (const card of (bucket.sprints ?? [])) {
      const tasks = card.preview_tasks ?? [];
      const index = tasks.findIndex((t) => t.id === taskId);
      if (index >= 0) {
        found = tasks[index];
        sourceSprintId = card.sprint.id;
        tasks.splice(index, 1);
        card.preview_tasks = tasks;
        card.stats.task_count = Math.max(0, card.stats.task_count - 1);
        card.stats.total_points = Math.max(0, card.stats.total_points - (found.estimate ?? 0));
        if (found.state_type === 'done') {
          card.stats.done_task_count = Math.max(0, card.stats.done_task_count - 1);
          card.stats.done_points = Math.max(0, card.stats.done_points - (found.estimate ?? 0));
        }
        const hiddenCount = Math.max(card.stats.task_count - card.preview_tasks.length, 0);
        card.task_preview_overflow = hiddenCount;
        break;
      }
    }
    if (found) break;
  }

  return { found, sourceSprintId };
}

function addTaskToSprint(card: SprintPlanningWorkspaceData['buckets'][number]['sprints'][number], task: SprintPlanningTaskPreview, sprintId: string) {
  const nextTask = { ...task, sprint_id: sprintId };
  card.preview_tasks = [nextTask, ...(card.preview_tasks ?? []).filter((item) => item.id !== task.id)].slice(0, TASK_PREVIEW_LIMIT);
  card.stats.task_count += 1;
  card.stats.total_points += task.estimate ?? 0;
  if (task.state_type === 'done') {
    card.stats.done_task_count += 1;
    card.stats.done_points += task.estimate ?? 0;
  }
  card.task_preview_overflow = Math.max(card.stats.task_count - card.preview_tasks.length, 0);
}

function addTaskToBacklog(workspace: SprintPlanningWorkspaceData, task: SprintPlanningTaskPreview) {
  const nextTask = { ...task, sprint_id: undefined };
  const backlogTasks = workspace.backlog_tasks ?? [];
  workspace.backlog_tasks = [nextTask, ...backlogTasks.filter((item) => item.id !== task.id)].slice(0, BACKLOG_LIMIT);
  workspace.backlog_total += 1;
}

function applyTaskAssignment(
  workspace: SprintPlanningWorkspaceData,
  task: SprintPlanningTaskPreview,
  targetSprintId: string | null,
): SprintPlanningWorkspaceData {
  const next = clonePlanningWorkspace(workspace);
  const { found, sourceSprintId } = removeTaskFromCards(next, task.id);
  const movingTask = found ?? task;
  if (sourceSprintId === targetSprintId) return workspace;

  if (!targetSprintId) {
    addTaskToBacklog(next, movingTask);
    return next;
  }

  for (const bucket of (next.buckets ?? [])) {
    for (const card of (bucket.sprints ?? [])) {
      if (card.sprint.id === targetSprintId) {
        addTaskToSprint(card, movingTask, targetSprintId);
        return next;
      }
    }
  }
  return workspace;
}

export function SprintsPage({ teamId }: SprintsPageProps) {
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const workspaceSlug = workspace?.slug ?? '';
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  const openCreate = useGlobalCreateStore((state) => state.openCreate);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { canEdit } = usePermissions(access);
  const { teams } = useAccessibleTeams(workspaceId);
  const { members } = useAssignableWorkspaceMembers(workspaceId);
  const [backlogOpen, setBacklogOpen] = useState(false);
  const [statusFilter, setStatusFilter] = useState<SprintStatusFilter>('all');

  const teamName = teamId ? teams.find((t) => t.id === teamId)?.name : undefined;
  useTitle(teamName ? `Sprints — ${teamName}` : 'Sprints');

  const isArchived = statusFilter === 'archived';

  const filters = useMemo(
    () => ({
      team_id: teamId || undefined,
      include_completed: true,
    }),
    [teamId],
  );

  const planningQuery = useSprintPlanningWorkspace(workspaceId, filters);
  const planningQueryKey = useMemo(
    () => queryKeys.pm.sprintPlanning(workspaceId, filters as Record<string, unknown> | undefined),
    [workspaceId, filters],
  );

  const handleOpenTask = (taskId: string) => {
    if (!workspaceSlug) return;
    openTaskRoute(navigate as never, location as never, workspaceSlug, taskId);
  };

  // Separate query for archived sprints — only enabled when filter is "archived"
  const archivedQuery = useQuery({
    queryKey: ['pm', workspaceId, 'sprints', 'archived', teamId],
    queryFn: async () => unwrap(await pmSprintService.list(workspaceId, { team_id: teamId, archived: true })),
    enabled: !!workspaceId && isArchived,
  });

  // Filter the planning workspace buckets based on status filter
  const filteredWorkspace = useMemo(() => {
    if (!planningQuery.data || isArchived) return planningQuery.data ?? null;
    if (statusFilter === 'all') return planningQuery.data;

    const bucketKey = statusFilter === 'upcoming' ? 'upcoming' : statusFilter === 'active' ? 'active' : 'completed';
    return {
      ...planningQuery.data,
      buckets: planningQuery.data.buckets.filter((b) => b.key === bucketKey),
    };
  }, [planningQuery.data, statusFilter, isArchived]);

  const handleAssignTask = async (task: SprintPlanningTaskPreview, sprintId: string | null) => {
    if (!planningQuery.data || !canEdit) return;
    const previous = queryClient.getQueryData<SprintPlanningWorkspaceData>(planningQueryKey) ?? planningQuery.data;
    const optimistic = applyTaskAssignment(previous, task, sprintId);
    if (optimistic === previous) return;

    queryClient.setQueryData(planningQueryKey, optimistic);
    try {
      const { data, error } = await pmTaskService.update(workspaceId, task.id, { sprint_id: sprintId ?? '' });
      if (error) throw new Error(error);
      if (data) {
        queryClient.setQueryData(queryKeys.pm.task(workspaceId, task.id), data);
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.task(workspaceId, task.id) });
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.tasks(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.board(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.sprintPreviewTasksRoot(workspaceId) });
      queryClient.invalidateQueries({ queryKey: planningQueryKey });
    } catch (error) {
      queryClient.setQueryData(planningQueryKey, previous);
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.task(workspaceId, task.id) });
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.tasks(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.board(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.sprintPreviewTasksRoot(workspaceId) });
      queryClient.invalidateQueries({ queryKey: planningQueryKey });
      toast.error(error instanceof Error ? error.message : 'Failed to update task sprint');
    }
  };

  const handleCreateTask = (sprintId?: string) => {
    openCreate('task', {
      teamId: teamId || undefined,
      ownerMemberId: undefined,
      sprintId,
    });
  };

  // Check unfiltered data for any sprints (to distinguish "no sprints ever" from "no sprints matching filter")
  const hasAnySprintUnfiltered = Boolean(planningQuery.data?.buckets.some((bucket) => (bucket.sprints?.length ?? 0) > 0));
  const hasAnySprintFiltered = Boolean(filteredWorkspace?.buckets.some((bucket) => (bucket.sprints?.length ?? 0) > 0));
  const isFiltered = statusFilter !== 'all';

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="mx-auto flex max-w-[1600px] flex-col gap-4">
      {(hasAnySprintUnfiltered || isFiltered) && (
        <SprintPlanningFilters
          teamName={teamName}
          statusFilter={statusFilter}
          canEdit={canEdit}
          onStatusFilterChange={setStatusFilter}
          onCreateSprint={() => openCreate('sprint', { teamId: teamId || undefined })}
        />
      )}

      {planningQuery.error && !isArchived ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {planningQuery.error instanceof Error ? planningQuery.error.message : 'Failed to load sprint planning'}
        </div>
      ) : null}

      {isArchived ? (
        archivedQuery.isLoading ? (
          <div className="rounded-xl border border-border/60 bg-card/60 px-4 py-8 text-sm text-muted-foreground">
            Loading archived sprints…
          </div>
        ) : archivedQuery.data && archivedQuery.data.length > 0 ? (
          <div className="space-y-2">
            {archivedQuery.data.map((s) => (
              <button
                key={s.sprint.id}
                type="button"
                className="flex w-full items-center justify-between rounded-lg border border-border/60 bg-card px-4 py-3 text-left transition-colors hover:bg-muted/40"
                onClick={() => navigate({ to: '/w/$slug/pm/sprints/$sprintId', params: { slug: workspace.slug, sprintId: s.sprint.id } })}
              >
                <div>
                  <p className="text-sm font-medium">{s.sprint.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {s.sprint.start_date && s.sprint.end_date
                      ? `${new Date(s.sprint.start_date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })} – ${new Date(s.sprint.end_date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}`
                      : 'No dates set'}
                  </p>
                </div>
                <span className="text-xs text-muted-foreground">
                  {s.stats.done_task_count}/{s.stats.task_count} tasks
                </span>
              </button>
            ))}
          </div>
        ) : (
          <div className="rounded-xl border border-border/60 bg-card/60 px-4 py-8 text-center text-sm text-muted-foreground">
            No archived sprints.
          </div>
        )
      ) : planningQuery.isLoading && !planningQuery.data ? (
        <div className="rounded-xl border border-border/60 bg-card/60 px-4 py-8 text-sm text-muted-foreground">
          Loading sprints…
        </div>
      ) : isFiltered && !hasAnySprintFiltered && hasAnySprintUnfiltered ? (
        <div className="rounded-xl border border-border/60 bg-card/60 px-4 py-8 text-center text-sm text-muted-foreground">
          No {statusFilter} sprints found. Try a different filter.
        </div>
      ) : (
        <SprintPlanningWorkspace
          workspace={filteredWorkspace}
          workspaceId={workspaceId}
          backlogOpen={backlogOpen}
          onBacklogToggle={() => setBacklogOpen((prev) => !prev)}
          canEdit={canEdit}
          members={members}
          onOpenSprint={(sprintId) => navigate({ to: '/w/$slug/pm/sprints/$sprintId', params: { slug: workspace.slug, sprintId } })}
          onOpenTask={handleOpenTask}
          onCreateSprint={() => openCreate('sprint', { teamId: teamId || undefined })}
          onCreateTask={handleCreateTask}
          onAssignTask={handleAssignTask}
        />
      )}
    </div>
  );
}
