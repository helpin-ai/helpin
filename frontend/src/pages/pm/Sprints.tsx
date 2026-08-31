import { useDeferredValue, useMemo, useState } from 'react';
import type { InfiniteData } from '@tanstack/react-query';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { useTitle } from '@/hooks/useTitle';
import { SprintPlanningFilters, type SprintStatusFilter } from '@/components/pm/sprints/SprintPlanningFilters';
import { SprintPlanningWorkspace } from '@/components/pm/sprints/SprintPlanningWorkspace';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useDeleteSprint, useInfiniteSprintBacklogTasks, useSprintCloseouts, useSprintPlanningWorkspace, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { queryKeys } from '@/lib/queryKeys';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import type { LinkSprintTasksResponse, PaginatedResponse, SprintPlanningWorkspace as SprintPlanningWorkspaceData, SprintPlanningTaskPreview } from '@/lib/pmTypes';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { unwrap } from '@/lib/queryUtils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { formatSprintDateOnly } from '@/lib/pmSprintOptions';
import { Button } from '@/components/ui/button';
import { QuietPageHeader, QuietPrimaryAction } from '@/components/design-system/quiet';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { LinkTasksToSprintDialog } from '@/components/pm/sprints/LinkTasksToSprintDialog';
import {
  ArrowUpRight01Icon,
  Copy01Icon,
  Delete01Icon,
  Link01Icon,
  MoreHorizontalIcon,
  PlusSignIcon,
} from '@/lib/icons';

const TASK_PREVIEW_LIMIT = 20;
const BACKLOG_LIMIT = 50;

interface SprintsPageProps {
  teamId?: string;
}

interface SprintLinkTarget {
  id: string;
  name: string;
  teamId: string;
  teamName: string;
}

interface ArchivedSprintRowProps {
  card: {
    sprint: { id: string; name: string; start_date: string | null; end_date: string | null };
    stats: { task_count: number; done_task_count: number };
    closeout?: { committed_count: number; completed_count: number; rolled_over_count: number; committed_points: number; completed_points: number };
  };
  workspaceId: string;
  workspaceSlug: string;
  canEdit: boolean;
  onOpen: (sprintId: string) => void;
}

function ArchivedSprintRow({ card, workspaceId, workspaceSlug, canEdit, onOpen }: ArchivedSprintRowProps) {
  const deleteSprint = useDeleteSprint(workspaceId);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const sprintPath = `/w/${workspaceSlug}/pm/sprints/${card.sprint.id}`;
  const fullUrl = typeof window !== 'undefined' ? `${window.location.origin}${sprintPath}` : sprintPath;

  const dateLabel = card.sprint.start_date && card.sprint.end_date ? `${formatSprintDateOnly(card.sprint.start_date)} – ${formatSprintDateOnly(card.sprint.end_date)}` : 'No dates set';

  const handleCopyLink = () => {
    void navigator.clipboard.writeText(fullUrl).then(
      () => toast.success('Link copied'),
      () => toast.error('Failed to copy link'),
    );
  };

  const handleConfirmDelete = () => {
    deleteSprint.mutate(card.sprint.id, {
      onSuccess: () => {
        toast.success('Sprint deleted');
        setDeleteOpen(false);
      },
      onError: (err) => {
        toast.error(err instanceof Error ? err.message : 'Failed to delete sprint');
      },
    });
  };

  const committedTasks = card.closeout?.committed_count ?? card.stats.task_count;
  const completedTasks = card.closeout?.completed_count ?? card.stats.done_task_count;
  const rolledOverTasks = card.closeout?.rolled_over_count ?? 0;
  return (
    <div
      role="button"
      tabIndex={0}
      className="group flex w-full items-center justify-between rounded-lg border border-border/60 bg-card px-4 py-3 text-left transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      onClick={() => onOpen(card.sprint.id)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onOpen(card.sprint.id);
        }
      }}
    >
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{card.sprint.name}</p>
        <p className="text-xs text-muted-foreground">{dateLabel}</p>
      </div>
      <div className="flex shrink-0 items-center gap-3">
        <span className="text-xs text-muted-foreground">
          {completedTasks}/{committedTasks} tasks {card.closeout ? 'completed' : 'done'}
          {card.closeout && rolledOverTasks > 0 ? ` · ${rolledOverTasks} rolled over` : ''}
        </span>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground" onClick={(e) => e.stopPropagation()} aria-label="Sprint actions">
              <MoreHorizontalIcon className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
            <DropdownMenuItem onClick={() => onOpen(card.sprint.id)}>
              <ArrowUpRight01Icon className="mr-2 h-4 w-4" />
              Open sprint
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => window.open(sprintPath, '_blank', 'noopener,noreferrer')}>
              <Link01Icon className="mr-2 h-4 w-4" />
              Open in new tab
            </DropdownMenuItem>
            <DropdownMenuItem onClick={handleCopyLink}>
              <Copy01Icon className="mr-2 h-4 w-4" />
              Copy link
            </DropdownMenuItem>
            {canEdit && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem variant="destructive" onClick={() => setDeleteOpen(true)}>
                  <Delete01Icon className="mr-2 h-4 w-4" />
                  Delete sprint
                </DropdownMenuItem>
              </>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="Delete sprint"
        description={
          <>
            This will permanently delete <span className="font-medium">{card.sprint.name}</span>. Tasks in this sprint will be moved back to the backlog. This action cannot be undone.
          </>
        }
        confirmLabel={deleteSprint.isPending ? 'Deleting…' : 'Delete'}
        variant="destructive"
        onConfirm={handleConfirmDelete}
      />
    </div>
  );
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

  for (const bucket of workspace.buckets ?? []) {
    for (const card of bucket.sprints ?? []) {
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

function applyTaskAssignment(workspace: SprintPlanningWorkspaceData, task: SprintPlanningTaskPreview, targetSprintId: string | null): SprintPlanningWorkspaceData {
  const next = clonePlanningWorkspace(workspace);
  const { found, sourceSprintId } = removeTaskFromCards(next, task.id);
  const movingTask = found ?? task;
  if (sourceSprintId === targetSprintId) return workspace;

  if (!found && !task.sprint_id && targetSprintId) {
    next.backlog_total = Math.max(0, next.backlog_total - 1);
  }

  if (!targetSprintId) {
    addTaskToBacklog(next, movingTask);
    return next;
  }

  for (const bucket of next.buckets ?? []) {
    for (const card of bucket.sprints ?? []) {
      if (card.sprint.id === targetSprintId) {
        addTaskToSprint(card, movingTask, targetSprintId);
        return next;
      }
    }
  }
  return workspace;
}

type SprintBacklogInfiniteData = InfiniteData<PaginatedResponse<SprintPlanningTaskPreview[]>>;

function applyBacklogTaskAssignment(data: SprintBacklogInfiniteData | undefined, task: SprintPlanningTaskPreview, targetSprintId: string | null) {
  if (!data) return data;
  const wasBacklog = !task.sprint_id;
  const willBeBacklog = !targetSprintId;
  if (wasBacklog === willBeBacklog) return data;

  const pages = data.pages.map((page) => ({
    ...page,
    data: page.data.filter((item) => item.id !== task.id),
  }));
  if (willBeBacklog && pages[0]) {
    pages[0].data = [{ ...task, sprint_id: undefined }, ...pages[0].data];
  }
  const nextTotal = Math.max(0, (pages[0]?.total ?? 0) + (willBeBacklog ? 1 : -1));
  for (const page of pages) {
    page.total = nextTotal;
    page.total_pages = nextTotal > 0 ? Math.ceil(nextTotal / page.per_page) : 0;
  }
  return { ...data, pages };
}

function normalizeSearchText(value: string | number | null | undefined) {
  return String(value ?? '').toLowerCase();
}

function sprintMatchesSearch(card: Pick<SprintPlanningWorkspaceData['buckets'][number]['sprints'][number], 'sprint' | 'stats'>, query: string) {
  if (!query) return true;
  const { sprint, stats } = card;
  const haystack = [sprint.name, sprint.description, sprint.status, sprint.start_date, sprint.end_date, `${stats.task_count} tasks`, `${stats.done_task_count}/${stats.task_count} tasks`, `${stats.total_points} points`, `${stats.done_points}/${stats.total_points} points`].map(normalizeSearchText).join(' ');

  return haystack.includes(query);
}

function syncPreviewQueryCaches(queryClient: ReturnType<typeof useQueryClient>, workspaceId: string, workspace: SprintPlanningWorkspaceData) {
  for (const bucket of workspace.buckets ?? []) {
    for (const card of bucket.sprints ?? []) {
      const previewTasks = card.preview_tasks ?? [];
      const total = card.stats.task_count ?? 0;
      queryClient.setQueryData<
        InfiniteData<{
          data: SprintPlanningTaskPreview[];
          total: number;
          page: number;
          per_page: number;
          total_pages: number;
        }>
      >(queryKeys.pm.sprintPreviewTasks(workspaceId, card.sprint.id), {
        pageParams: [1],
        pages: [
          {
            data: previewTasks,
            total,
            page: 1,
            per_page: TASK_PREVIEW_LIMIT,
            total_pages: total > 0 ? Math.ceil(total / TASK_PREVIEW_LIMIT) : 0,
          },
        ],
      });
    }
  }
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
  const { canEdit, isAdmin, isTeamManager, teamMemberships } = usePermissions(access);
  const { teams } = useAccessibleTeams(workspaceId);
  const { members } = useAssignableWorkspaceMembers(workspaceId);
  const [backlogOpen, setBacklogOpen] = useState(false);
  const [statusFilter, setStatusFilter] = useState<SprintStatusFilter>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [linkTarget, setLinkTarget] = useState<SprintLinkTarget | null>(null);
  const deferredSearchQuery = useDeferredValue(searchQuery);
  const normalizedSearchQuery = deferredSearchQuery.trim().toLowerCase();

  const teamName = teamId ? teams.find((t) => t.id === teamId)?.name : undefined;
  useTitle(teamName ? `Sprints — ${teamName}` : 'Sprints');
  const canCreateSprint = canEdit && (isAdmin || (teamId ? isTeamManager(teamId) : teamMemberships.some((tm) => tm.role === 'owner')));

  const isArchived = statusFilter === 'archived';

  const filters = useMemo(
    () => ({
      team_id: teamId || undefined,
      include_completed: true,
    }),
    [teamId],
  );

  const planningQuery = useSprintPlanningWorkspace(workspaceId, filters);
  const backlogQuery = useInfiniteSprintBacklogTasks(workspaceId, teamId, BACKLOG_LIMIT);
  const backlogQueryKey = useMemo(() => queryKeys.pm.sprintBacklogTasks(workspaceId, teamId), [teamId, workspaceId]);
  const backlogTasks = useMemo(() => backlogQuery.data?.pages.flatMap((page) => page.data) ?? planningQuery.data?.backlog_tasks ?? [], [backlogQuery.data?.pages, planningQuery.data?.backlog_tasks]);
  const backlogTotal = backlogQuery.data?.pages[0]?.total ?? planningQuery.data?.backlog_total ?? 0;
  const closeoutsQuery = useSprintCloseouts(workspaceId, {
    team_id: teamId || undefined,
  });
  const closeoutBySprintId = useMemo(() => new Map((closeoutsQuery.data?.items ?? []).map((item) => [item.sprint_id, item] as const)), [closeoutsQuery.data?.items]);
  const planningQueryKey = useMemo(() => queryKeys.pm.sprintPlanning(workspaceId, filters as Record<string, unknown> | undefined), [workspaceId, filters]);

  const handleOpenTask = (taskId: string) => {
    if (!workspaceSlug) return;
    openTaskRoute(navigate as never, location as never, workspaceSlug, taskId);
  };

  // Separate query for archived sprints — only enabled when filter is "archived"
  const archivedQuery = useQuery({
    queryKey: ['pm', workspaceId, 'sprints', 'archived', teamId],
    queryFn: async () =>
      unwrap(
        await pmSprintService.list(workspaceId, {
          team_id: teamId,
          archived: true,
        }),
      ),
    enabled: !!workspaceId && isArchived,
  });

  // Filter the planning workspace buckets based on status and search filters
  const filteredWorkspace = useMemo(() => {
    if (!planningQuery.data || isArchived) return planningQuery.data ?? null;

    const workspaceWithCloseouts: SprintPlanningWorkspaceData = {
      ...planningQuery.data,
      buckets: (planningQuery.data.buckets ?? []).map((bucket) => ({
        ...bucket,
        sprints: (bucket.sprints ?? []).map((card) => ({
          ...card,
          closeout: closeoutBySprintId.get(card.sprint.id),
        })),
      })),
    };

    const bucketKey = statusFilter === 'upcoming' ? 'upcoming' : statusFilter === 'active' ? 'active' : 'completed';
    const statusBuckets = statusFilter === 'all' ? workspaceWithCloseouts.buckets : workspaceWithCloseouts.buckets.filter((b) => b.key === bucketKey);

    if (!normalizedSearchQuery) {
      return statusFilter === 'all'
        ? workspaceWithCloseouts
        : {
            ...workspaceWithCloseouts,
            buckets: statusBuckets,
          };
    }

    return {
      ...workspaceWithCloseouts,
      buckets: statusBuckets.map((bucket) => ({
        ...bucket,
        sprints: (bucket.sprints ?? []).filter((card) => sprintMatchesSearch(card, normalizedSearchQuery)),
      })),
    };
  }, [planningQuery.data, closeoutBySprintId, statusFilter, isArchived, normalizedSearchQuery]);

  const filteredArchivedSprints = useMemo(() => {
    const archivedSprints = (archivedQuery.data ?? []).map((card) => ({
      ...card,
      closeout: closeoutBySprintId.get(card.sprint.id),
    }));
    if (!normalizedSearchQuery) return archivedSprints;
    return archivedSprints.filter((card) => sprintMatchesSearch(card, normalizedSearchQuery));
  }, [archivedQuery.data, closeoutBySprintId, normalizedSearchQuery]);

  const handleAssignTask = async (task: SprintPlanningTaskPreview, sprintId: string | null) => {
    if (!planningQuery.data || !canEdit) return;
    const previous = queryClient.getQueryData<SprintPlanningWorkspaceData>(planningQueryKey) ?? planningQuery.data;
    const previousBacklog = queryClient.getQueryData<SprintBacklogInfiniteData>(backlogQueryKey);
    const optimistic = applyTaskAssignment(previous, task, sprintId);
    if (optimistic === previous) return;

    queryClient.setQueryData(planningQueryKey, optimistic);
    queryClient.setQueryData<SprintBacklogInfiniteData>(backlogQueryKey, (current) => applyBacklogTaskAssignment(current, task, sprintId));
    syncPreviewQueryCaches(queryClient, workspaceId, optimistic);
    try {
      const { data, error } = await pmTaskService.update(workspaceId, task.id, {
        sprint_id: sprintId ?? '',
      });
      if (error) throw new Error(error);
      if (data) {
        queryClient.setQueryData(queryKeys.pm.task(workspaceId, task.id), data);
      }
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.task(workspaceId, task.id),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.tasks(workspaceId),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.board(workspaceId),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.sprintPreviewTasksRoot(workspaceId),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.sprintBacklogTasksRoot(workspaceId),
      });
      queryClient.invalidateQueries({ queryKey: planningQueryKey });
    } catch (error) {
      queryClient.setQueryData(planningQueryKey, previous);
      queryClient.setQueryData(backlogQueryKey, previousBacklog);
      syncPreviewQueryCaches(queryClient, workspaceId, previous);
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.task(workspaceId, task.id),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.tasks(workspaceId),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.board(workspaceId),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.sprintPreviewTasksRoot(workspaceId),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.sprintBacklogTasksRoot(workspaceId),
      });
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

  const handleOpenLinkTasks = (sprintId: string) => {
    const card = (planningQuery.data?.buckets ?? []).flatMap((bucket) => bucket.sprints ?? []).find((entry) => entry.sprint.id === sprintId);
    const sprintTeamId = card?.sprint.team_id ?? '';
    if (!card || !sprintTeamId) {
      toast.error('Assign this sprint to a team before linking tasks.');
      return;
    }
    setLinkTarget({
      id: card.sprint.id,
      name: card.sprint.name,
      teamId: sprintTeamId,
      teamName: teams.find((team) => team.id === sprintTeamId)?.name ?? 'this team',
    });
  };

  const handleTasksLinked = (result: LinkSprintTasksResponse) => {
    void Promise.all([
      queryClient.invalidateQueries({ queryKey: planningQueryKey }),
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.tasks(workspaceId),
      }),
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.board(workspaceId),
      }),
      queryClient.invalidateQueries({
        queryKey: queryKeys.pm.sprintPreviewTasksRoot(workspaceId),
      }),
    ]);
    const linkedLabel = `${result.linked_count} task${result.linked_count === 1 ? '' : 's'}`;
    if (result.moved_count > 0) {
      toast.success(`Linked ${linkedLabel}; ${result.moved_count} moved from another sprint.`);
      return;
    }
    toast.success(`Linked ${linkedLabel} to this sprint.`);
  };

  // Check unfiltered data for any sprints (to distinguish "no sprints ever" from "no sprints matching filter")
  const hasAnySprintUnfiltered = Boolean(planningQuery.data?.buckets?.some((bucket) => (bucket.sprints?.length ?? 0) > 0));
  const hasAnySprintFiltered = Boolean(filteredWorkspace?.buckets?.some((bucket) => (bucket.sprints?.length ?? 0) > 0));
  const isSearchFiltered = normalizedSearchQuery.length > 0;
  const isFiltered = statusFilter !== 'all' || isSearchFiltered;

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <QuietPageHeader
        variant="shell"
        title="Sprints"
        context={teamName}
        description="Plan and track work across time-boxed cycles."
        actions={
          canEdit ? (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="inline-flex">
                    <QuietPrimaryAction className="gap-1.5" disabled={!canCreateSprint} onClick={() => openCreate('sprint', { teamId: teamId || undefined })}>
                      <PlusSignIcon className="h-4 w-4" />
                      Add sprint
                    </QuietPrimaryAction>
                  </span>
                </TooltipTrigger>
                {!canCreateSprint ? (
                  <TooltipContent side="top" className="max-w-[260px] text-xs">
                    Only team managers can create sprints. Ask your team manager for access.
                  </TooltipContent>
                ) : null}
              </Tooltip>
            </TooltipProvider>
          ) : null
        }
      />

      <div className="min-h-0 flex-1 overflow-auto">
        <div className="mx-auto flex max-w-[1600px] flex-col gap-4 p-4 md:p-6 2xl:max-w-[1920px] min-[2560px]:max-w-[2400px]">
          {(hasAnySprintUnfiltered || isFiltered) && <SprintPlanningFilters statusFilter={statusFilter} searchQuery={searchQuery} onStatusFilterChange={setStatusFilter} onSearchQueryChange={setSearchQuery} />}

          {planningQuery.error && !isArchived ? <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">{planningQuery.error instanceof Error ? planningQuery.error.message : 'Failed to load sprint planning'}</div> : null}

          {isArchived ? (
            archivedQuery.isLoading ? (
              <div className="rounded-xl border border-border/60 bg-card/60 px-4 py-8 text-sm text-muted-foreground">Loading archived sprints…</div>
            ) : archivedQuery.error ? (
              <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">{archivedQuery.error instanceof Error ? archivedQuery.error.message : 'Failed to load archived sprints'}</div>
            ) : filteredArchivedSprints.length > 0 ? (
              <div className="space-y-2">
                {filteredArchivedSprints.map((s) => (
                  <ArchivedSprintRow
                    key={s.sprint.id}
                    card={s}
                    workspaceId={workspaceId}
                    workspaceSlug={workspace.slug}
                    canEdit={canEdit}
                    onOpen={(sprintId) =>
                      navigate({
                        to: '/w/$slug/pm/sprints/$sprintId',
                        params: { slug: workspace.slug, sprintId },
                      })
                    }
                  />
                ))}
              </div>
            ) : isSearchFiltered && archivedQuery.data && archivedQuery.data.length > 0 ? (
              <div className="rounded-xl border border-border/60 bg-card/60 px-4 py-8 text-center text-sm text-muted-foreground">No archived sprints match your search.</div>
            ) : (
              <div className="rounded-xl border border-border/60 bg-card/60 px-4 py-8 text-center text-sm text-muted-foreground">No archived sprints.</div>
            )
          ) : planningQuery.isLoading && !planningQuery.data ? (
            <div className="rounded-xl border border-border/60 bg-card/60 px-4 py-8 text-sm text-muted-foreground">Loading sprints…</div>
          ) : isSearchFiltered && !hasAnySprintFiltered && hasAnySprintUnfiltered ? (
            <div className="rounded-xl border border-border/60 bg-card/60 px-4 py-8 text-center text-sm text-muted-foreground">No sprints match your search.</div>
          ) : isFiltered && !hasAnySprintFiltered && hasAnySprintUnfiltered ? (
            <div className="rounded-xl border border-border/60 bg-card/60 px-4 py-8 text-center text-sm text-muted-foreground">No {statusFilter} sprints found. Try a different filter.</div>
          ) : (
            <SprintPlanningWorkspace
              workspace={filteredWorkspace}
              workspaceId={workspaceId}
              workspaceSlug={workspaceSlug}
              backlogOpen={backlogOpen}
              onBacklogToggle={() => setBacklogOpen((prev) => !prev)}
              canEdit={canEdit}
              members={members}
              backlogTasks={backlogTasks}
              backlogTotal={backlogTotal}
              backlogHasMore={Boolean(backlogQuery.hasNextPage)}
              backlogLoadingMore={backlogQuery.isFetchingNextPage}
              onLoadMoreBacklog={() => void backlogQuery.fetchNextPage()}
              onOpenSprint={(sprintId) =>
                navigate({
                  to: '/w/$slug/pm/sprints/$sprintId',
                  params: { slug: workspace.slug, sprintId },
                })
              }
              onOpenTask={handleOpenTask}
              onLinkTasks={handleOpenLinkTasks}
              onCreateTask={handleCreateTask}
              onAssignTask={handleAssignTask}
            />
          )}

          {linkTarget ? (
            <LinkTasksToSprintDialog
              open
              onOpenChange={(open) => {
                if (!open) setLinkTarget(null);
              }}
              workspaceId={workspaceId}
              sprintId={linkTarget.id}
              sprintName={linkTarget.name}
              teamId={linkTarget.teamId}
              teamName={linkTarget.teamName}
              onLinked={handleTasksLinked}
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}
