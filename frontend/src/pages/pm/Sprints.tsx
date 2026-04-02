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
import type { SprintPlanningWorkspace as SprintPlanningWorkspaceData, SprintPlanningStoryPreview } from '@/lib/pmTypes';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { unwrap } from '@/lib/queryUtils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';

const STORY_PREVIEW_LIMIT = 5;
const BACKLOG_LIMIT = 50;

interface SprintsPageProps {
  teamId?: string;
}

function clonePlanningWorkspace(workspace: SprintPlanningWorkspaceData): SprintPlanningWorkspaceData {
  return {
    ...workspace,
    buckets: workspace.buckets.map((bucket) => ({
      ...bucket,
      sprints: (bucket.sprints ?? []).map((card) => ({
        ...card,
        preview_stories: [...(card.preview_stories ?? [])],
      })),
    })),
    backlog_stories: [...workspace.backlog_stories],
  };
}

function removeStoryFromCards(workspace: SprintPlanningWorkspaceData, storyId: string) {
  let found: SprintPlanningStoryPreview | null = null;
  let sourceSprintId: string | null = null;

  const backlogIndex = workspace.backlog_stories.findIndex((story) => story.id === storyId);
  if (backlogIndex >= 0) {
    found = workspace.backlog_stories[backlogIndex];
    workspace.backlog_stories.splice(backlogIndex, 1);
    workspace.backlog_total = Math.max(0, workspace.backlog_total - 1);
  }

  for (const bucket of workspace.buckets) {
    for (const card of (bucket.sprints ?? [])) {
      const stories = card.preview_stories ?? [];
      const index = stories.findIndex((story) => story.id === storyId);
      if (index >= 0) {
        found = stories[index];
        sourceSprintId = card.sprint.id;
        stories.splice(index, 1);
        card.preview_stories = stories;
        card.stats.story_count = Math.max(0, card.stats.story_count - 1);
        card.stats.total_points = Math.max(0, card.stats.total_points - (found.estimate ?? 0));
        if (found.state_type === 'done') {
          card.stats.done_story_count = Math.max(0, card.stats.done_story_count - 1);
          card.stats.done_points = Math.max(0, card.stats.done_points - (found.estimate ?? 0));
        }
        const hiddenCount = Math.max(card.stats.story_count - card.preview_stories.length, 0);
        card.task_preview_overflow = hiddenCount;
      }
    }
  }

  return { found, sourceSprintId };
}

function addStoryToSprint(card: SprintPlanningWorkspaceData['buckets'][number]['sprints'][number], story: SprintPlanningStoryPreview, sprintId: string) {
  const nextStory = { ...story, sprint_id: sprintId };
  card.preview_stories = [nextStory, ...(card.preview_stories ?? []).filter((item) => item.id !== story.id)].slice(0, STORY_PREVIEW_LIMIT);
  card.stats.story_count += 1;
  card.stats.total_points += story.estimate ?? 0;
  if (story.state_type === 'done') {
    card.stats.done_story_count += 1;
    card.stats.done_points += story.estimate ?? 0;
  }
  card.task_preview_overflow = Math.max(card.stats.story_count - card.preview_stories.length, 0);
}

function addStoryToBacklog(workspace: SprintPlanningWorkspaceData, story: SprintPlanningStoryPreview) {
  const nextStory = { ...story, sprint_id: undefined };
  workspace.backlog_stories = [nextStory, ...workspace.backlog_stories.filter((item) => item.id !== story.id)].slice(0, BACKLOG_LIMIT);
  workspace.backlog_total += 1;
}

function applyStoryAssignment(
  workspace: SprintPlanningWorkspaceData,
  story: SprintPlanningStoryPreview,
  targetSprintId: string | null,
): SprintPlanningWorkspaceData {
  const next = clonePlanningWorkspace(workspace);
  const { found, sourceSprintId } = removeStoryFromCards(next, story.id);
  const movingStory = found ?? story;
  if (sourceSprintId === targetSprintId) return workspace;

  if (!targetSprintId) {
    addStoryToBacklog(next, movingStory);
    return next;
  }

  for (const bucket of next.buckets) {
    for (const card of (bucket.sprints ?? [])) {
      if (card.sprint.id === targetSprintId) {
        addStoryToSprint(card, movingStory, targetSprintId);
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

  const handleOpenStory = (storyId: string) => {
    if (!workspaceSlug) return;
    openTaskRoute(navigate as never, location as never, workspaceSlug, storyId);
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

  const handleAssignStory = async (story: SprintPlanningStoryPreview, sprintId: string | null) => {
    if (!planningQuery.data || !canEdit) return;
    const previous = queryClient.getQueryData<SprintPlanningWorkspaceData>(planningQueryKey) ?? planningQuery.data;
    const optimistic = applyStoryAssignment(previous, story, sprintId);
    if (optimistic === previous) return;

    queryClient.setQueryData(planningQueryKey, optimistic);
    try {
      const { error } = await pmTaskService.update(workspaceId, story.id, { sprint_id: sprintId ?? '' });
      if (error) throw new Error(error);
    } catch (error) {
      queryClient.setQueryData(planningQueryKey, previous);
      toast.error(error instanceof Error ? error.message : 'Failed to update story sprint');
    }
  };

  const handleCreateStory = (sprintId?: string) => {
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
                  {s.stats.done_story_count}/{s.stats.story_count} stories
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
          backlogOpen={backlogOpen}
          onBacklogToggle={() => setBacklogOpen((prev) => !prev)}
          canEdit={canEdit}
          members={members}
          onOpenSprint={(sprintId) => navigate({ to: '/w/$slug/pm/sprints/$sprintId', params: { slug: workspace.slug, sprintId } })}
          onOpenStory={handleOpenStory}
          onCreateSprint={() => openCreate('sprint', { teamId: teamId || undefined })}
          onCreateStory={handleCreateStory}
          onAssignStory={handleAssignStory}
        />
      )}
    </div>
  );
}
