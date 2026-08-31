import { useEffect, useMemo, useState } from 'react';
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query';
import { DndContext, DragOverlay, MeasuringStrategy, PointerSensor, useDroppable, useSensor, useSensors, type DragEndEvent, type DragStartEvent } from '@dnd-kit/core';
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { toast } from 'sonner';

import { Cancel01Icon, LayoutTable01Icon, LayoutTwoColumnIcon, Loading01Icon, Search01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { BoardDisplayMenu } from '@/components/pm/BoardDisplayMenu';
import { ListDisplayMenu } from '@/components/pm/ListDisplayMenu';
import { TaskCard } from '@/components/pm/TaskCard';
import { TaskListView } from '@/components/pm/TaskListView';
import { TaskFilterBar, TaskFilterProvider, TaskFilterTrigger, TaskOwnerAvatarFilterRow } from '@/components/pm/TaskFilters';
import { StateTypeIcon } from '@/lib/pmConstants';
import { getVisibleTaskListGroupOptions, type TaskListGroupByOption } from '@/components/pm/task-detail/taskListGrouping';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useWorkflows } from '@/hooks/queries';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { unwrap } from '@/lib/queryUtils';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { useBoardDisplayStore, type DisplayPropertyKey } from '@/stores/boardDisplayStore';
import type { BoardFilters } from '@/stores/pmBoardStore';
import type { EpicWithStats, Label, SprintWithStats, StateType, Task, WorkflowWithStates } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { COMPANY_TASK_STATE_GROUPS, companyTaskDropStates, companyTaskStateMap, groupCompanyTasksByStateType } from './companyTaskBoard';

const STATE_DROP_PREFIX = 'company-task-state:';
const DND_MEASURING = { droppable: { strategy: MeasuringStrategy.Always } };

type CRMTasksWorkspaceObjectType = 'company' | 'contact';

interface CRMTasksWorkspaceProps {
  workspaceId: string;
  objectType: CRMTasksWorkspaceObjectType;
  objectId: string;
  onOpenTask: (task: Task) => void;
  onTaskActivityChange?: () => void;
  fullHeight?: boolean;
}

interface CompanyTaskQueryOptions {
  workspaceId: string;
  objectType: CRMTasksWorkspaceObjectType;
  objectId: string;
  userFilters: BoardFilters;
  search: string;
  teamId: string;
  stateType?: StateType;
  enabled: boolean;
  pageSize: number;
}

function useCompanyTaskQuery({ workspaceId, objectType, objectId, userFilters, search, teamId, stateType, enabled, pageSize }: CompanyTaskQueryOptions) {
  return useInfiniteQuery({
    queryKey: ['pm', workspaceId, 'tasks', 'crm-workspace', objectType, objectId, stateType ?? 'all', teamId || 'all-teams', search, userFilters],
    queryFn: async ({ pageParam }) => {
      const associationFilter = objectType === 'company' ? { company_rollup_id: objectId } : { contact_id: objectId };
      return unwrap(
        await pmTaskService.list(workspaceId, {
          ...userFilters,
          ...associationFilter,
          archived: userFilters.archived ? userFilters.archived === 'true' : false,
          page: pageParam,
          per_page: pageSize,
          search: search || undefined,
          team_id: teamId || undefined,
          state_type: stateType,
        } as Parameters<typeof pmTaskService.list>[1]),
      );
    },
    initialPageParam: 1,
    getNextPageParam: (lastPage) => (lastPage.page < lastPage.total_pages ? lastPage.page + 1 : undefined),
    enabled: enabled && !!workspaceId && !!objectId,
  });
}

function queryTasks(query: ReturnType<typeof useCompanyTaskQuery>) {
  return query.data?.pages.flatMap((page) => page.data) ?? [];
}

function StateDropBlock({ state, currentStateId }: { state: WorkflowWithStates['states'][number]; currentStateId: string }) {
  const isCurrent = state.id === currentStateId;
  const { setNodeRef, isOver } = useDroppable({
    id: `${STATE_DROP_PREFIX}${state.id}`,
    disabled: isCurrent,
  });

  return (
    <div
      ref={setNodeRef}
      className={cn(
        'flex min-h-12 flex-1 items-center justify-center rounded-md border border-dashed px-3 text-center text-xs font-medium transition-[background-color,border-color,color,opacity] duration-150',
        isCurrent ? 'border-border/50 bg-muted/45 text-muted-foreground/60' : 'border-border bg-background/90 text-foreground/80',
        isOver && 'border-primary bg-primary/10 text-primary ring-1 ring-primary/20',
      )}
    >
      <span className="truncate">{state.name}</span>
      {isCurrent && <span className="ml-1 text-[10px] font-normal">Current</span>}
    </div>
  );
}

interface BoardColumnData {
  type: StateType;
  label: string;
  tasks: Task[];
  total: number;
  isLoading: boolean;
  isFetchingNextPage: boolean;
  hasNextPage: boolean;
  fetchNextPage: () => void;
}

function CompanyTaskBoard({
  columns,
  workflows,
  workspaceId,
  teams,
  assignableMembers,
  onOpenTask,
  onMoveComplete,
  fullHeight,
}: {
  columns: BoardColumnData[];
  workflows: WorkflowWithStates[];
  workspaceId: string;
  teams: Array<{ id: string; name: string }>;
  assignableMembers: ReturnType<typeof useAssignableWorkspaceMembers>['members'];
  onOpenTask: (task: Task) => void;
  onMoveComplete: () => void;
  fullHeight: boolean;
}) {
  const queryClient = useQueryClient();
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }));
  const [activeTask, setActiveTask] = useState<Task | null>(null);
  const [optimisticState, setOptimisticState] = useState<Record<string, WorkflowWithStates['states'][number]>>({});
  const ownerNameMap = useMemo(() => buildAssignableMemberNameMap(assignableMembers), [assignableMembers]);
  const teamNameMap = useMemo(() => new Map(teams.map((team) => [team.id, team.name])), [teams]);
  const stateById = useMemo(() => {
    return companyTaskStateMap(workflows);
  }, [workflows]);
  const allRawTasks = useMemo(() => {
    const seen = new Set<string>();
    const tasks: Task[] = [];
    for (const column of columns) {
      for (const task of column.tasks) {
        if (seen.has(task.id)) continue;
        seen.add(task.id);
        tasks.push(task);
      }
    }
    return tasks;
  }, [columns]);
  const displayedTasks = useMemo(
    () =>
      allRawTasks.map((task) => {
        const state = optimisticState[task.id];
        if (!state) return task;
        return {
          ...task,
          workflow_state_id: state.id,
          state_name: state.name,
          state_type: state.state_type as StateType,
          state_color: state.color ?? undefined,
          completed: state.state_type === 'done',
        };
      }),
    [allRawTasks, optimisticState],
  );
  const displayedByType = useMemo(() => {
    return groupCompanyTasksByStateType(displayedTasks, workflows);
  }, [displayedTasks, workflows]);

  const invalidateWorkspace = async () => {
    await queryClient.invalidateQueries({
      queryKey: ['pm', workspaceId, 'tasks', 'crm-workspace'],
    });
  };

  const handleDragStart = (event: DragStartEvent) => {
    setActiveTask(displayedTasks.find((task) => task.id === String(event.active.id)) ?? null);
  };

  const handleDragEnd = async (event: DragEndEvent) => {
    const moving = activeTask;
    setActiveTask(null);
    const overID = event.over ? String(event.over.id) : '';
    if (!moving || !overID.startsWith(STATE_DROP_PREFIX)) return;

    const stateID = overID.slice(STATE_DROP_PREFIX.length);
    const targetState = stateById.get(stateID);
    if (!targetState || targetState.workflow_id !== moving.workflow_id || targetState.id === moving.workflow_state_id) return;

    setOptimisticState((current) => ({ ...current, [moving.id]: targetState }));
    try {
      const response = await pmTaskService.move(workspaceId, moving.id, {
        state_id: targetState.id,
        position: 0,
      });
      if (response.error) throw new Error(response.error);
      await invalidateWorkspace();
      onMoveComplete();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to move task');
    } finally {
      setOptimisticState((current) => {
        const next = { ...current };
        delete next[moving.id];
        return next;
      });
    }
  };

  const handleCardPatched = () => {
    void invalidateWorkspace();
  };

  return (
    <DndContext
      sensors={sensors}
      measuring={DND_MEASURING}
      onDragStart={handleDragStart}
      onDragEnd={(event) => {
        void handleDragEnd(event);
      }}
      onDragCancel={() => setActiveTask(null)}
    >
      <div
        className={cn(
          'min-h-0 overflow-x-auto overflow-y-hidden px-3 pb-3',
          fullHeight ? 'flex-1' : 'max-h-[410px]',
        )}
      >
        <div className={cn('flex min-w-max items-stretch gap-1.5', fullHeight && 'h-full')}>
          {columns.map((column) => {
            const tasks = displayedByType.get(column.type) ?? [];
            const targetStates = companyTaskDropStates(activeTask, workflows, column.type);
            return (
              <section
                key={column.type}
                className={cn(
                  'relative flex w-[276px] shrink-0 flex-col overflow-hidden',
                  fullHeight ? 'h-full min-h-0' : 'max-h-[398px]',
                )}
              >
                <header className="flex h-11 shrink-0 items-center gap-2 px-3">
                  <StateTypeIcon stateType={column.type} className="h-4 w-4" />
                  <span className="text-sm font-semibold">{column.label}</span>
                  <span className="ml-auto text-xs tabular-nums text-muted-foreground">{column.total}</span>
                </header>

                <SortableContext items={tasks.map((task) => task.id)} strategy={verticalListSortingStrategy}>
                  <div
                    className={cn(
                      'min-h-0 space-y-2 overflow-y-auto p-2 [scrollbar-gutter:stable]',
                      fullHeight ? 'flex-1' : 'max-h-[342px]',
                    )}
                  >
                    {column.isLoading ? (
                      <div className="flex h-24 items-center justify-center text-xs text-muted-foreground">
                        <Loading01Icon className="mr-2 h-3.5 w-3.5 animate-spin" />
                        Loading…
                      </div>
                    ) : tasks.length > 0 ? (
                      tasks.map((task) => (
                        <TaskCard
                          key={task.id}
                          task={task}
                          workspaceId={workspaceId}
                          assignableMembers={assignableMembers}
                          ownerNameMap={ownerNameMap}
                          teamName={task.team_id ? teamNameMap.get(task.team_id) : undefined}
                          onOpen={onOpenTask}
                          onOwnerChanged={handleCardPatched}
                          onPriorityChanged={handleCardPatched}
                          onSeverityChanged={handleCardPatched}
                          onEstimateChanged={handleCardPatched}
                          showStateBadge
                        />
                      ))
                    ) : (
                      <div className="flex h-24 items-center justify-center text-xs text-muted-foreground/70">No tasks</div>
                    )}
                    {column.hasNextPage && (
                      <Button type="button" variant="ghost" size="sm" className="w-full text-xs text-muted-foreground" disabled={column.isFetchingNextPage} onClick={column.fetchNextPage}>
                        {column.isFetchingNextPage && <Loading01Icon className="h-3.5 w-3.5 animate-spin" />}
                        Load more
                      </Button>
                    )}
                  </div>
                </SortableContext>

                {activeTask && (
                  <div className="absolute inset-x-1 bottom-2 top-[44px] z-20 flex flex-col gap-2 bg-background/94 p-2 backdrop-blur-[1px]">
                    {targetStates.length > 0 ? (
                      targetStates.map((state) => <StateDropBlock key={state.id} state={state} currentStateId={activeTask.workflow_state_id} />)
                    ) : (
                      <div className="flex flex-1 items-center justify-center rounded-md border border-dashed border-border/60 px-4 text-center text-xs text-muted-foreground">
                        No {column.label.toLowerCase()} states in this workflow
                      </div>
                    )}
                  </div>
                )}
              </section>
            );
          })}
        </div>
      </div>

      <DragOverlay>
        {activeTask ? (
          <div className="w-[260px] rotate-[1deg] opacity-95 shadow-xl">
            <TaskCard
              task={activeTask}
              isOverlay
              workspaceId={workspaceId}
              assignableMembers={assignableMembers}
              ownerNameMap={ownerNameMap}
              teamName={activeTask.team_id ? teamNameMap.get(activeTask.team_id) : undefined}
              onOpen={onOpenTask}
              showStateBadge
            />
          </div>
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}

export function CRMTasksWorkspace({ workspaceId, objectType, objectId, onOpenTask, onTaskActivityChange, fullHeight = false }: CRMTasksWorkspaceProps) {
  const queryClient = useQueryClient();
  const scopedListDisabledKeys = useMemo(() => new Set<DisplayPropertyKey>([objectType === 'company' ? 'companies' : 'contacts']), [objectType]);
  const [viewMode, setViewMode] = useState<'list' | 'board'>('board');
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [teamId, setTeamId] = useState('');
  const [userFilters, setUserFilters] = useState<BoardFilters>({});
  const [listGroupBy, setListGroupBy] = useState<TaskListGroupByOption>('workflow_state');
  const [labels, setLabels] = useState<Label[]>([]);
  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [sprints, setSprints] = useState<SprintWithStats[]>([]);
  const { teams } = useAccessibleTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const { userMemberships } = useWorkspaceTeams(workspaceId);
  const { data: workflows = [] } = useWorkflows(workspaceId);
  const displayInit = useBoardDisplayStore((state) => state.init);

  useEffect(() => {
    displayInit(workspaceId);
  }, [displayInit, workspaceId]);
  useEffect(() => {
    const timer = window.setTimeout(() => setSearch(searchInput.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [searchInput]);
  useEffect(() => {
    void Promise.all([pmLabelService.list(workspaceId), pmEpicService.list(workspaceId, { archived: false }), pmSprintService.list(workspaceId)]).then(
      ([labelResponse, epicResponse, sprintResponse]) => {
        setLabels(labelResponse.data ?? []);
        setEpics(epicResponse.data ?? []);
        setSprints(sprintResponse.data ?? []);
      },
    );
  }, [workspaceId]);

  const tableQuery = useCompanyTaskQuery({
    workspaceId,
    objectType,
    objectId,
    userFilters,
    search,
    teamId,
    enabled: viewMode === 'list',
    pageSize: 50,
  });
  const backlogQuery = useCompanyTaskQuery({
    workspaceId,
    objectType,
    objectId,
    userFilters,
    search,
    teamId,
    stateType: 'backlog',
    enabled: viewMode === 'board',
    pageSize: 25,
  });
  const unstartedQuery = useCompanyTaskQuery({
    workspaceId,
    objectType,
    objectId,
    userFilters,
    search,
    teamId,
    stateType: 'unstarted',
    enabled: viewMode === 'board',
    pageSize: 25,
  });
  const startedQuery = useCompanyTaskQuery({
    workspaceId,
    objectType,
    objectId,
    userFilters,
    search,
    teamId,
    stateType: 'started',
    enabled: viewMode === 'board',
    pageSize: 25,
  });
  const doneQuery = useCompanyTaskQuery({
    workspaceId,
    objectType,
    objectId,
    userFilters,
    search,
    teamId,
    stateType: 'done',
    enabled: viewMode === 'board',
    pageSize: 25,
  });

  const boardQueries = [backlogQuery, unstartedQuery, startedQuery, doneQuery];
  const workflowStates = useMemo(() => companyTaskStateMap(workflows), [workflows]);
  const columns: BoardColumnData[] = COMPANY_TASK_STATE_GROUPS.map((group, index) => {
    const query = boardQueries[index]!;
    const rawTasks = queryTasks(query);
    const tasks = rawTasks.filter((task) => (workflowStates.get(task.workflow_state_id)?.state_type ?? task.state_type) === group.type);
    // Older servers ignore state_type. Keep this view correct while the API rolls out,
    // then use the server total once every returned task matches the requested group.
    const containsOtherStateTypes = tasks.length !== rawTasks.length;
    return {
      ...group,
      tasks,
      total: containsOtherStateTypes ? tasks.length : (query.data?.pages[0]?.total ?? 0),
      isLoading: query.isLoading,
      isFetchingNextPage: query.isFetchingNextPage,
      hasNextPage: !containsOtherStateTypes && !!query.hasNextPage,
      fetchNextPage: () => {
        void query.fetchNextPage();
      },
    };
  });
  const tableTasks = queryTasks(tableQuery);
  const primaryWorkflow = workflows[0];
  const groupOptions = getVisibleTaskListGroupOptions({
    task_type: true,
    priority: true,
    severity: true,
    epic: true,
    sprint: true,
  });
  const queryError = viewMode === 'list' ? tableQuery.error : boardQueries.find((query) => query.error)?.error;

  const refreshWorkspace = async () => {
    await queryClient.invalidateQueries({
      queryKey: ['pm', workspaceId, 'tasks', 'crm-workspace'],
    });
  };

  return (
    <TaskFilterProvider
      workspaceId={workspaceId}
      assignableMembers={assignableMembers}
      activeTeamId={teamId || null}
      userMemberships={userMemberships}
      labels={labels}
      epics={epics}
      sprints={sprints}
      onChange={setUserFilters}
      externalFilters={userFilters}
      excludedKeys={[objectType === 'company' ? 'company_id' : 'contact_id']}
    >
      <div
        className={cn(
          'flex min-h-0 flex-col overflow-hidden',
          fullHeight ? 'flex-1 bg-transparent' : 'max-h-[470px] bg-background',
        )}
      >
        <div className="flex min-h-11 flex-wrap items-center gap-2 border-b border-border/60 px-3 py-2">
          <div className="relative min-w-[150px] flex-1 sm:max-w-[220px]">
            <Search01Icon className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="Search customer tasks" className="h-7 pl-8 pr-7 text-xs" />
            {searchInput && (
              <button
                type="button"
                aria-label="Clear task search"
                className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded p-0.5 text-muted-foreground hover:bg-accent hover:text-foreground"
                onClick={() => setSearchInput('')}
              >
                <Cancel01Icon className="h-3.5 w-3.5" />
              </button>
            )}
          </div>

          <TaskFilterTrigger />
          <TaskOwnerAvatarFilterRow />

          {teams.length > 0 && (
            <Select value={teamId || '__all__'} onValueChange={(value) => setTeamId(value === '__all__' ? '' : value)}>
              <SelectTrigger className="h-7 w-auto min-w-[112px] gap-1.5 px-2.5 text-xs">
                <span className="text-muted-foreground">Team:</span>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">All teams</SelectItem>
                {teams.map((team) => (
                  <SelectItem key={team.id} value={team.id}>
                    {team.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}

          <div className="ml-auto flex items-center gap-1">
            {viewMode === 'list' && (
              <Select value={listGroupBy} onValueChange={(value) => setListGroupBy(value as TaskListGroupByOption)}>
                <SelectTrigger className="h-7 w-auto min-w-[132px] border-0 bg-transparent px-1.5 text-xs shadow-none">
                  <span className="text-muted-foreground">Group by:</span>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {groupOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}

            <span className="inline-flex h-7 items-center gap-0.5 rounded-md border border-border/70 bg-muted/30 p-0.5">
              <QuickTooltip label="Table view">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label="Table view"
                  aria-pressed={viewMode === 'list'}
                  className={cn('h-6 w-6 rounded-sm', viewMode === 'list' && 'bg-background text-foreground shadow-sm hover:bg-background')}
                  onClick={() => setViewMode('list')}
                >
                  <LayoutTable01Icon className="h-3.5 w-3.5" />
                </Button>
              </QuickTooltip>
              <QuickTooltip label="Board view">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label="Board view"
                  aria-pressed={viewMode === 'board'}
                  className={cn('h-6 w-6 rounded-sm', viewMode === 'board' && 'bg-background text-foreground shadow-sm hover:bg-background')}
                  onClick={() => setViewMode('board')}
                >
                  <LayoutTwoColumnIcon className="h-3.5 w-3.5" />
                </Button>
              </QuickTooltip>
            </span>

            {viewMode === 'list' ? <ListDisplayMenu disabledKeys={scopedListDisabledKeys} /> : <BoardDisplayMenu />}
          </div>
        </div>

        <TaskFilterBar />

        {queryError ? (
          <div className="m-3 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
            {queryError instanceof Error ? queryError.message : 'Unable to load customer tasks.'}
          </div>
        ) : !primaryWorkflow ? (
          <div className="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">No task workflow is available.</div>
        ) : viewMode === 'list' ? (
          tableQuery.isLoading ? (
            <div className="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
              <Loading01Icon className="mr-2 h-4 w-4 animate-spin" />
              Loading tasks…
            </div>
          ) : (
            <TaskListView
              workspaceId={workspaceId}
              workflow={primaryWorkflow}
              workflows={workflows}
              teams={teams}
              assignableMembers={assignableMembers}
              epics={epics}
              sprints={sprints}
              filters={{
                ...userFilters,
                ...(objectType === 'company' ? { company_id: objectId } : { contact_id: objectId }),
              }}
              externalTasks={tableTasks}
              listenForCreatedTasks={false}
              fitContent={!fullHeight}
              onOpenTask={onOpenTask}
              groupBy={listGroupBy}
              onGroupByChange={setListGroupBy}
              showToolbar={false}
              onBulkOperationComplete={refreshWorkspace}
              footer={
                tableQuery.hasNextPage ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="text-xs text-muted-foreground"
                    disabled={tableQuery.isFetchingNextPage}
                    onClick={() => {
                      void tableQuery.fetchNextPage();
                    }}
                  >
                    {tableQuery.isFetchingNextPage && <Loading01Icon className="h-3.5 w-3.5 animate-spin" />}
                    Load more tasks
                  </Button>
                ) : undefined
              }
            />
          )
        ) : (
          <CompanyTaskBoard
            columns={columns}
            workflows={workflows}
            workspaceId={workspaceId}
            teams={teams}
            assignableMembers={assignableMembers}
            onOpenTask={onOpenTask}
            onMoveComplete={() => {
              onTaskActivityChange?.();
            }}
            fullHeight={fullHeight}
          />
        )}
      </div>
    </TaskFilterProvider>
  );
}
