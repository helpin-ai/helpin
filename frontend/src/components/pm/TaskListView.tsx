import { memo, startTransition, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { RefObject } from 'react';
import {
  useReactTable,
  getCoreRowModel,
  getGroupedRowModel,
  getExpandedRowModel,
  getSortedRowModel,
  flexRender,
  createColumnHelper,
  type GroupingState,
  type ExpandedState,
  type Row,
  type SortingState,
  type ColumnSizingState,
  type RowSelectionState,
} from '@tanstack/react-table';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useQueryClient } from '@tanstack/react-query';
import { Copy01Icon, Loading01Icon } from '@/lib/icons';
import { AgentAvatar, resolveAgentPersonaKey } from '@/components/agents/AgentAvatar';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Calendar } from '@/components/ui/calendar';
import { Checkbox } from '@/components/ui/checkbox';
import { format, parseISO } from 'date-fns';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { cn } from '@/lib/utils';
import {
  PRIORITY_CONFIG,
  PriorityIcon as TaskListPriorityIcon,
  SEVERITY_CONFIG,
  SeverityIcon as TaskListSeverityIcon,
  SprintIcon as TaskListRecurringIcon,
  StateTypeIcon as TaskListStateTypeIcon,
  TASK_TYPE_CONFIG,
  TaskTypeIcon as TaskListTaskTypeIcon,
} from '@/lib/pmConstants';
import {
  ArchiveIcon as TaskListArchiveIcon,
  ArrowUpDownIcon as TaskListArrowUpDownIcon,
  Calendar03Icon as TaskListCalendarIcon,
  ChartColumnIcon as TaskListChartIcon,
  CheckmarkCircle02Icon as TaskListDoneCircleIcon,
  ChevronDownIcon as TaskListChevronDownIcon,
  ChevronRightIcon as TaskListChevronRightIcon,
  ChevronUpIcon as TaskListChevronUpIcon,
  Link01Icon as TaskListLinkIcon,
  LinkSquare01Icon as TaskListOpenTaskIcon,
  MoreVerticalIcon as TaskListMoreVerticalIcon,
  StickyNote01Icon as TaskListNoteIcon,
  Tick01Icon as TaskListCheckIcon,
  UserAdd01Icon as TaskListUserAddIcon,
} from '@/lib/pmIcons';
import type {
  Agent,
  AssociationObjectSummary,
  Label,
  Priority,
  Severity,
  StateType,
  Task,
  WorkflowWithStates,
  EpicWithStats,
  SprintWithStats,
} from '@/lib/pmTypes';
import type { AssignableMember, TeamEstimateSettings, WorkspaceTeam } from '@/lib/types';
import { EstimatePicker } from '@/components/pm/EstimatePicker';
import { LabelPicker } from '@/components/pm/LabelPicker';
import { TaskBulkActionsBar } from '@/components/pm/TaskBulkActionsBar';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { showTaskDuplicatedToast } from '@/components/pm/TaskDuplicatedToast';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { buildTaskCopyUrl } from '@/lib/pmTaskLinks';
import { useAgents, useAutomationRulesByWorkflow, useTeamEstimateSettings, useTeamFieldVisibilityForTeam } from '@/hooks/queries';
import { useBoardDisplayStore, type DisplayPropertyKey } from '@/stores/boardDisplayStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { ListDisplayMenu } from '@/components/pm/ListDisplayMenu';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { MultiMemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { OwnerAvatarStack } from '@/components/pm/OwnerAvatarStack';
import {
  TABLE_CONTAINER,
  TABLE_RESIZE_HANDLE,
  TABLE_PINNED_LEFT,
  TABLE_PINNED_RIGHT,
  TABLE_PINNED_HEADER_LEFT,
  TABLE_PINNED_HEADER_RIGHT,
  ROW_HEIGHT,
  GROUP_ROW_HEIGHT,
  dynamicCellStyle,
  pinnedStyle,
  resolveColumnRuntimeSize,
  virtualRowStyle,
  TABLE_HEADER_CELL_ACTIONS,
  ACTIONS_COL_SIZE,
} from '@/lib/tableStyles';
import type { BoardFilters } from '@/stores/pmBoardStore';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { getTaskListPinnedOffsets, type TaskListPinnedOffsets } from '@/components/pm/task-detail/taskListPinnedOffsets';
import {
  getVisibleTaskListGroupOptions,
  type TaskListGroupByOption,
} from '@/components/pm/task-detail/taskListGrouping';
import { getVisibleSprintsForTaskScope } from '@/components/pm/task-detail/taskPlanningScope';
import { ACTIVE_RUN_STATUSES } from '@/components/pm/agentRunConstants';
import { toast } from 'sonner';
import { queryKeys } from '@/lib/queryKeys';
import {
  getAgentAutoRunStateChangeMessage,
  getAgentAutoRunStateChangeToastId,
  shouldNotifyAgentAutoRunStateChange,
} from '@/components/pm/agentAutoRunNotification';

const ALL_PRIORITIES: Priority[] = ['urgent', 'high', 'medium', 'low', 'none'];
const ALL_SEVERITIES: Severity[] = ['critical', 'major', 'minor', 'none'];
const LIST_AGENT_OCTAGON_POINTS = '30,2 70,2 98,30 98,70 70,98 30,98 2,70 2,30';

function TaskListLatestRunAgentBadge({
  agent,
  latestRunStatus,
}: {
  agent: Agent | null;
  latestRunStatus?: string | null;
}) {
  const isWorking = !!latestRunStatus && ACTIVE_RUN_STATUSES.has(latestRunStatus);
  const isGenericAgent = resolveAgentPersonaKey({ agent }) === 'generic';
  const statusDotClassName = latestRunStatus === 'completed'
    ? 'bg-emerald-500 dark:bg-emerald-400'
    : latestRunStatus === 'failed'
      ? 'bg-red-500 dark:bg-red-400'
      : null;

  return (
    <span className="relative block h-5 w-5 shrink-0">
      {isWorking ? (
        <>
          <svg
            viewBox="0 0 100 100"
            aria-hidden="true"
            className="absolute inset-0 h-full w-full overflow-visible motion-safe:animate-spin motion-safe:[animation-duration:2.4s]"
          >
            <polygon
              points={LIST_AGENT_OCTAGON_POINTS}
              fill="none"
              className="stroke-foreground/80"
              strokeWidth={4}
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          <span
            className="absolute inset-[2px] overflow-hidden bg-background/95"
            style={{ clipPath: 'polygon(31% 4%, 69% 4%, 96% 31%, 96% 69%, 69% 96%, 31% 96%, 4% 69%, 4% 31%)' }}
          >
            <AgentAvatar
              agent={agent}
              className="h-full w-full rounded-none border-0 bg-transparent shadow-none"
              genericBare={isGenericAgent}
            />
          </span>
        </>
      ) : (
        <AgentAvatar
          agent={agent}
          className="h-5 w-5 rounded-none border-0 bg-transparent shadow-none"
          genericBare={isGenericAgent}
        />
      )}
      {statusDotClassName ? (
        <span
          aria-hidden="true"
          className={cn(
            'absolute bottom-0 right-0 h-2 w-2 rounded-full ring-1 ring-background',
            statusDotClassName,
          )}
        />
      ) : null}
    </span>
  );
}

const TASK_LIST_HEADER = 'sticky top-0 z-10 bg-card';
const TASK_LIST_HEADER_CELL =
  'relative shrink-0 border-r border-b border-border/60 bg-card px-2.5 py-1.5 text-left text-[11px] font-medium text-muted-foreground last:border-r-0';
const TASK_LIST_HEADER_CELL_SORTABLE = 'cursor-pointer select-none hover:bg-muted';
const TASK_LIST_ROW =
  'group/row flex h-9 cursor-pointer items-center border-b border-border/60 bg-card hover:bg-muted';
const TASK_LIST_CELL =
  'flex shrink-0 items-center self-stretch border-r border-border/60 bg-inherit px-2.5 last:border-r-0';
const TASK_LIST_GROUP_ROW =
  'flex h-9 cursor-pointer items-center border-b border-border/60 bg-muted/20 text-sm font-semibold hover:bg-muted';
const GROUP_HEADER_REPEAT_HEIGHT = 30;

interface TaskListViewProps {
  workspaceId: string;
  workflow: WorkflowWithStates;
  workflows?: WorkflowWithStates[];
  teams: WorkspaceTeam[];
  assignableMembers?: AssignableMember[];
  epics: EpicWithStats[];
  sprints: SprintWithStats[];
  filters?: BoardFilters;
  teamId?: string | null;
  /** When the list is scoped to a single epic, hide the redundant Epic column. */
  epicId?: string | null;
  /** When the list is scoped to a single sprint, hide the redundant Sprint column. */
  sprintId?: string | null;
  /** When provided, use these tasks instead of fetching internally. */
  externalTasks?: Task[];
  onOpenTask: (task: Task) => void;
  groupBy?: TaskListGroupByOption;
  onGroupByChange?: (groupBy: TaskListGroupByOption) => void;
  showToolbar?: boolean;
  footer?: React.ReactNode;
  onBulkOperationComplete?: () => void | Promise<void>;
  /** When provided, the bulk-edit trigger button portals into this element instead of rendering in the toolbar. */
  bulkTriggerContainer?: HTMLElement | null;
}

// Column accessor ID used for each group-by option
const GROUP_COLUMN_MAP: Record<TaskListGroupByOption, string | null> = {
  none: null,
  workflow_state: 'stateName',
  task_type: 'typeName',
  priority: 'priorityName',
  severity: 'severityName',
  epic: 'epicName',
  sprint: 'sprintName',
  owner: 'ownerName',
};

const HIDDEN_GROUP_COLUMNS = ['typeName', 'priorityName', 'severityName'];
const LIST_PAGE_SIZE = 50;
const CHECKBOX_COL_SIZE = 42;
const GROUP_LOAD_SENTINEL_HEIGHT = 28;
const GROUPED_OVERSCAN = 4;
const FLAT_OVERSCAN = 6;

const columnHelper = createColumnHelper<Task>();

function getSelectableTaskRows(rows: Row<Task>[]): Row<Task>[] {
  const selectable: Row<Task>[] = [];
  for (const row of rows) {
    if (row.getIsGrouped()) {
      if (row.getIsExpanded()) {
        selectable.push(...getSelectableTaskRows(row.subRows as Row<Task>[]));
      }
    } else {
      selectable.push(row);
    }
  }
  return selectable;
}

export function TaskListView({
  workspaceId,
  workflow,
  workflows,
  teams,
  assignableMembers = [],
  epics,
  sprints,
  filters,
  teamId,
  epicId,
  sprintId,
  externalTasks,
  onOpenTask,
  groupBy: controlledGroupBy,
  onGroupByChange,
  showToolbar = true,
  footer,
  onBulkOperationComplete,
  bulkTriggerContainer,
}: TaskListViewProps) {
  const queryClient = useQueryClient();
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug ?? null);
  const currentWorkspaceId = useWorkspaceStore((state) => state.currentWorkspace?.id ?? '');
  const fieldVis = useTeamFieldVisibilityForTeam(workspaceId, teamId);
  const { data: automationRules } = useAutomationRulesByWorkflow(workspaceId, workflow.workflow.id);
  const automatedStateIds = useMemo(() => {
    const ids = new Set<string>();
    if (!automationRules) return ids;
    for (const rule of automationRules) {
      if (
        rule.enabled &&
        rule.trigger_type === 'task.state_entered' &&
        rule.action_type === 'start_agent_run'
      ) {
        const stateId = rule.trigger_config?.state_id;
        if (stateId) ids.add(stateId);
      }
    }
    return ids;
  }, [automationRules]);
  const teamEstimateSettings = useTeamEstimateSettings(currentWorkspaceId);
  const displayInit = useBoardDisplayStore((s) => s.init);
  const displayProps = useBoardDisplayStore((s) => s.properties);
  const includeAssociationData = useMemo(
    () => ({
      include_contacts: !!displayProps.contacts,
      include_companies: !!displayProps.companies,
      include_deals: !!displayProps.deals,
      include_support: !!displayProps.support,
    }),
    [displayProps.companies, displayProps.contacts, displayProps.deals, displayProps.support],
  );

  useEffect(() => { displayInit(workspaceId); }, [workspaceId, displayInit]);

  const isExternal = externalTasks !== undefined;
  const [tasks, setTasks] = useState<Task[]>(externalTasks ?? []);
  const [loading, setLoading] = useState(!isExternal);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const [uncontrolledGroupBy, setUncontrolledGroupBy] = useState<TaskListGroupByOption>('workflow_state');
  const groupBy = controlledGroupBy ?? uncontrolledGroupBy;
  const setGroupBy = onGroupByChange ?? setUncontrolledGroupBy;
  const [expanded, setExpanded] = useState<ExpandedState>(true);
  const parentRef = useRef<HTMLDivElement>(null);
  const headerRef = useRef<HTMLDivElement>(null);
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnSizing, setColumnSizing] = useState<ColumnSizingState>({});
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const columnSizingVersion = useMemo(() => JSON.stringify(columnSizing), [columnSizing]);
  const [allLabels, setAllLabels] = useState<Label[]>([]);
  const { data: agents = [] } = useAgents(workspaceId);

  // Per-group pagination state (for workflow_state grouping)
  const [groupHasMore, setGroupHasMore] = useState<Map<string, { hasMore: boolean; total: number; loaded: number }>>(new Map());
  const [groupLoadingId, setGroupLoadingId] = useState<string | null>(null);
  const groupLoadingRef = useRef(false);
  const isPerGroupMode = groupBy === 'workflow_state' && !isExternal;
  const onOpenTaskRef = useRef(onOpenTask);
  const lastClickedTaskIdRef = useRef<string | null>(null);
  const checkboxShiftKeyRef = useRef(false);

  useEffect(() => {
    onOpenTaskRef.current = onOpenTask;
  }, [onOpenTask]);

  const handleOpenTask = useCallback((task: Task) => {
    onOpenTaskRef.current(task);
  }, []);

  useEffect(() => {
    pmLabelService.list(workspaceId).then((r) => { if (r.data) setAllLabels(r.data); });
  }, [workspaceId]);

  useEffect(() => {
    const handleTaskCreated = (event: Event) => {
      const created = (event as CustomEvent<{ task?: Task }>).detail?.task;
      if (!created) return;
      if (created.workspace_id !== workspaceId) return;
      if (created.workflow_id !== workflow.workflow.id) return;
      if (teamId && created.team_id !== teamId) return;
      setTasks((current) => (
        current.some((candidate) => candidate.id === created.id)
          ? current
          : [created, ...current]
      ));
    };
    window.addEventListener('task-created', handleTaskCreated);
    return () => window.removeEventListener('task-created', handleTaskCreated);
  }, [workspaceId, workflow.workflow.id, teamId]);

  // Build lookup maps
  const availableWorkflows = useMemo(
    () => (workflows && workflows.length > 0 ? workflows : [workflow]),
    [workflow, workflows],
  );
  const statesByWorkflowId = useMemo(() => {
    const map = new Map<string, WorkflowWithStates['states']>();
    for (const wf of availableWorkflows) {
      map.set(wf.workflow.id, wf.states);
    }
    return map;
  }, [availableWorkflows]);
  const stateMap = useMemo(() => {
    const stateNameCounts = new Map<string, number>();
    for (const wf of availableWorkflows) {
      for (const s of wf.states) {
        stateNameCounts.set(s.name, (stateNameCounts.get(s.name) ?? 0) + 1);
      }
    }

    const map = new Map<string, { name: string; stateType: string; groupLabel: string; position: number }>();
    for (const wf of availableWorkflows) {
      for (const s of wf.states) {
        const duplicateName = (stateNameCounts.get(s.name) ?? 0) > 1;
        map.set(s.id, {
          name: s.name,
          stateType: s.state_type,
          groupLabel: duplicateName ? `${s.name} · ${wf.workflow.name}` : s.name,
          position: s.position,
        });
      }
    }
    return map;
  }, [availableWorkflows]);

  const assignableMemberMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const ownerNameMap = assignableMemberMap;
  const agentById = useMemo(
    () => new Map(agents.map((agent) => [agent.id, agent])),
    [agents],
  );

  const teamMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const t of teams) {
      map.set(t.id, t.name);
    }
    return map;
  }, [teams]);

  const epicMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const e of epics) {
      map.set(e.epic.id, e.epic.name);
    }
    return map;
  }, [epics]);

  const sprintMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const i of sprints) {
      map.set(i.sprint.id, i.sprint.name);
    }
    return map;
  }, [sprints]);

  const estimateSettingsByTeamId = useMemo(() => {
    const map = new Map<string, TeamEstimateSettings>();
    for (const settings of teamEstimateSettings) {
      map.set(settings.team_id, settings);
    }
    return map;
  }, [teamEstimateSettings]);

  // ── Flat pagination (non-state grouping) ──
  const fetchTasksFlat = useCallback(async (page = 1, append = false) => {
    if (isExternal) return;
    if (page === 1) setLoading(true);
    else setLoadingMore(true);
    const apiFilters: Record<string, string | number | boolean | undefined> = {
      per_page: LIST_PAGE_SIZE,
      page,
      workflow_id: workflow.workflow.id,
      archived: false,
      ...filters,
    };
    if (teamId) apiFilters.team_id = teamId;
    Object.assign(apiFilters, includeAssociationData);

    const res = await pmTaskService.list(workspaceId, apiFilters as Record<string, string>);
    if (res.data) {
      const incoming = res.data.data;
      setTasks((prev) => append ? [...prev, ...incoming] : incoming);
      setHasMore(page < res.data.total_pages);
      setCurrentPage(page);
    }
    setLoading(false);
    setLoadingMore(false);
  }, [workspaceId, workflow.workflow.id, filters, teamId, isExternal, includeAssociationData]);

  // ── Per-state pagination (workflow_state grouping) ──
  const fetchTasksByState = useCallback(async () => {
    if (isExternal) return;
    setLoading(true);
    const boardFilters: Record<string, string | undefined> = {};
    if (filters) {
      for (const [k, v] of Object.entries(filters)) {
        if (v !== undefined && v !== null && v !== '') boardFilters[k] = String(v);
      }
    }
    if (teamId) boardFilters.team_id = teamId;

    const res = await pmTaskService.listBoard(workspaceId, workflow.workflow.id, boardFilters, LIST_PAGE_SIZE, includeAssociationData);
    if (res.data) {
      const allTasks: Task[] = [];
      const perGroup = new Map<string, { hasMore: boolean; total: number; loaded: number }>();
      for (const col of res.data) {
        allTasks.push(...col.tasks);
        perGroup.set(col.state.id, {
          hasMore: col.has_more,
          total: col.task_count,
          loaded: col.tasks.length,
        });
      }
      setTasks(allTasks);
      setGroupHasMore(perGroup);
      setHasMore(false); // disable global load more
    }
    setLoading(false);
  }, [workspaceId, workflow.workflow.id, filters, teamId, isExternal, includeAssociationData]);

  // Load more tasks for a specific state group
  const loadMoreForGroup = useCallback(async (stateId: string) => {
    const info = groupHasMore.get(stateId);
    if (!info?.hasMore || groupLoadingRef.current) return;
    groupLoadingRef.current = true;
    setGroupLoadingId(stateId);

    const boardFilters: Record<string, string | undefined> = {};
    if (filters) {
      for (const [k, v] of Object.entries(filters)) {
        if (v !== undefined && v !== null && v !== '') boardFilters[k] = String(v);
      }
    }
    if (teamId) boardFilters.team_id = teamId;

    const res = await pmTaskService.listBoardColumn(workspaceId, stateId, info.loaded, LIST_PAGE_SIZE, boardFilters, includeAssociationData);
    if (res.data) {
      const newTasks = res.data.tasks;
      const newTotal = res.data.total;
      const newLoaded = info.loaded + newTasks.length;

      // Insert new tasks after the last existing task of this state
      setTasks((prev) => {
        let lastStateIdx = -1;
        for (let i = 0; i < prev.length; i++) {
          if (prev[i].workflow_state_id === stateId) lastStateIdx = i;
        }
        const result = [...prev];
        if (lastStateIdx >= 0) {
          result.splice(lastStateIdx + 1, 0, ...newTasks);
        } else {
          result.push(...newTasks);
        }
        return result;
      });

      setGroupHasMore((prev) => {
        const next = new Map(prev);
        next.set(stateId, {
          hasMore: newLoaded < newTotal,
          total: newTotal,
          loaded: newLoaded,
        });
        return next;
      });
    }
    setGroupLoadingId(null);
    groupLoadingRef.current = false;
  }, [workspaceId, filters, teamId, groupHasMore, includeAssociationData]);

  const loadMore = useCallback(() => {
    if (!loadingMore && hasMore) {
      fetchTasksFlat(currentPage + 1, true);
    }
  }, [fetchTasksFlat, currentPage, loadingMore, hasMore]);

  // Fetch on mount and when dependencies change
  useEffect(() => {
    if (isExternal) return;
    if (isPerGroupMode) {
      fetchTasksByState();
    } else {
      fetchTasksFlat(1, false);
    }
  }, [isExternal, isPerGroupMode, fetchTasksByState, fetchTasksFlat]);

  // Sync external tasks when they change
  useEffect(() => {
    if (isExternal && externalTasks) setTasks(externalTasks);
  }, [isExternal, externalTasks]);

  // Optimistic inline update with rollback on failure
  const updateTaskField = useCallback(
    async (taskId: string, patch: Partial<Task>) => {
      let snapshot: Task[] = [];
      setTasks((current) => {
        snapshot = current;
        return current.map((s) => (s.id === taskId ? { ...s, ...patch } : s));
      });
      const apiPatch = { ...patch };
      delete apiPatch.epic_name;
      delete apiPatch.labels;
      const { error } = await pmTaskService.update(workspaceId, taskId, apiPatch);
      if (error) setTasks(snapshot);
      if (!error && patch.workflow_state_id !== undefined) {
        const previousStateId = snapshot.find((task) => task.id === taskId)?.workflow_state_id;
        if (shouldNotifyAgentAutoRunStateChange({
          fromStateId: previousStateId,
          toStateId: patch.workflow_state_id,
          automatedStateIds,
        })) {
          const stateName = workflow.states.find((state) => state.id === patch.workflow_state_id)?.name ?? 'this state';
          toast.info(getAgentAutoRunStateChangeMessage(stateName), { id: getAgentAutoRunStateChangeToastId(patch.workflow_state_id) });
        }
      }
    },
    [workspaceId, ownerNameMap, automatedStateIds, workflow.states],
  );

  const handleRowCheckboxToggle = useCallback((taskId: string, checked: boolean, shiftKey: boolean) => {
    setRowSelection((current) => {
      const next = { ...current };
      const currentIndex = tasks.findIndex((task) => task.id === taskId);
      const previousIndex = lastClickedTaskIdRef.current
        ? tasks.findIndex((task) => task.id === lastClickedTaskIdRef.current)
        : -1;

      if (shiftKey && previousIndex >= 0 && currentIndex >= 0) {
        const start = Math.min(previousIndex, currentIndex);
        const end = Math.max(previousIndex, currentIndex);
        for (const task of tasks.slice(start, end + 1)) {
          if (checked) next[task.id] = true;
          else delete next[task.id];
        }
      } else if (checked) {
        next[taskId] = true;
      } else {
        delete next[taskId];
      }

      return next;
    });
    lastClickedTaskIdRef.current = taskId;
  }, [tasks]);

  const clearSelection = useCallback(() => {
    setRowSelection({});
    lastClickedTaskIdRef.current = null;
  }, []);

  useEffect(() => {
    const taskIds = new Set(tasks.map((task) => task.id));
    setRowSelection((current) => {
      const next = Object.fromEntries(
        Object.entries(current).filter(([id, selected]) => selected && taskIds.has(id)),
      );
      return Object.keys(next).length === Object.keys(current).length ? current : next;
    });
  }, [tasks]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') clearSelection();
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [clearSelection]);

  // Listen for task events (only for self-fetching mode)
  useEffect(() => {
    if (isExternal) return;
    const handleDeleted = (event: Event) => {
      const detail = (event as CustomEvent)?.detail;
      const taskId = detail?.entity_id as string | undefined;
      if (!taskId) return;
      setTasks((current) => current.filter((s) => s.id !== taskId));
    };

    const handleUpdated = async (event: Event) => {
      const detail = (event as CustomEvent)?.detail;
      const taskId = detail?.entity_id as string | undefined;
      if (!taskId) return;

      let shouldPatch = false;
      setTasks((current) => {
        shouldPatch = current.some((s) => s.id === taskId);
        return current;
      });
      if (!shouldPatch) return;

      const res = await pmTaskService.get(workspaceId, taskId);
      if (!res.data?.task) return;
      const taskDetail = res.data!;
      const merged: Task = {
        ...taskDetail.task,
        labels: taskDetail.labels,
        epic_name: taskDetail.epic_name ?? taskDetail.task.epic_name,
        sprint_name: taskDetail.sprint_name ?? taskDetail.task.sprint_name,
      };
      setTasks((current) =>
        current.map((s) => (
          s.id === taskId
            ? {
                ...merged,
                contacts: s.contacts,
                companies: s.companies,
                deals: s.deals,
                support_conversations: s.support_conversations,
              }
            : s
        )),
      );
    };

    window.addEventListener('task-deleted', handleDeleted);
    window.addEventListener('task-updated', handleUpdated);

    return () => {
      window.removeEventListener('task-deleted', handleDeleted);
      window.removeEventListener('task-updated', handleUpdated);
    };
  }, [isExternal, workspaceId]);

  // Table columns
  const tableColumns = useMemo(
    () => [
      columnHelper.display({
        id: 'select',
        size: CHECKBOX_COL_SIZE,
        enableGrouping: false,
        enableSorting: false,
        enableResizing: false,
        header: ({ table }) => {
          const selectableRows = getSelectableTaskRows(table.getRowModel().rows as Row<Task>[]);
          const selectedCount = selectableRows.filter((row) => row.getIsSelected()).length;
          const checked = selectedCount === 0
            ? false
            : selectedCount === selectableRows.length ? true : 'indeterminate';

          return (
            <Checkbox
              checked={checked}
              onCheckedChange={(value) => {
                const shouldSelect = value === true;
                setRowSelection((current) => {
                  const next = { ...current };
                  for (const row of selectableRows) {
                    if (shouldSelect) next[row.id] = true;
                    else delete next[row.id];
                  }
                  return next;
                });
              }}
              aria-label="Select all tasks"
              onClick={(event) => event.stopPropagation()}
            />
          );
        },
        cell: ({ row }) => (
          <Checkbox
            checked={row.getIsSelected()}
            onMouseDown={(event) => {
              checkboxShiftKeyRef.current = event.shiftKey;
            }}
            onCheckedChange={(value) => {
              handleRowCheckboxToggle(row.original.id, value === true, checkboxShiftKeyRef.current);
              checkboxShiftKeyRef.current = false;
            }}
            onClick={(event) => event.stopPropagation()}
            aria-label={`Select ${row.original.task_key}`}
          />
        ),
      }),
      columnHelper.accessor('task_key', {
        id: 'displayId',
        header: 'ID',
        size: 90,
        cell: (info) => (
          <span className="font-mono text-xs text-muted-foreground">{info.getValue()}</span>
        ),
      }),
      columnHelper.accessor('name', {
        id: 'name',
        header: 'Name',
        size: 280,
        enableGrouping: false,
        cell: (info) => (
          <button
            className="flex max-w-full cursor-pointer items-center gap-1.5 text-left text-sm hover:text-primary"
            onClick={(e) => {
              e.stopPropagation();
              handleOpenTask(info.row.original);
            }}
          >
            {fieldVis.task_type && displayProps.task_type ? (
              <TaskListTaskTypeIcon taskType={info.row.original.task_type} className="h-[18px] w-[18px] shrink-0" />
            ) : null}
            {info.row.original.recurring_template_id ? (
              <span className="inline-flex items-center gap-1 rounded-md border border-sky-200 bg-sky-50 px-1.5 py-0 text-[10px] text-sky-700 dark:border-sky-900/60 dark:bg-sky-950/30 dark:text-sky-200">
                <TaskListRecurringIcon className="h-2.5 w-2.5 shrink-0" />
                <span className="truncate">
                  {info.row.original.recurring_occurrence_number ? `Recurring #${info.row.original.recurring_occurrence_number}` : 'Recurring'}
                </span>
              </span>
            ) : null}
            <span className="min-w-0 truncate">{info.getValue()}</span>
            {info.row.original.latest_run_agent_id && (
              <TaskListLatestRunAgentBadge
                agent={agentById.get(info.row.original.latest_run_agent_id) ?? null}
                latestRunStatus={info.row.original.latest_run_status}
              />
            )}
          </button>
        ),
      }),
      // Group by state identity, not state name, because different workflows can
      // have separate states with the same visible label and independent order.
      columnHelper.accessor(
        (row) => row.workflow_state_id,
        {
          id: 'stateName',
          header: 'State',
          size: 190,
          sortingFn: (a, b) => {
            const aState = stateMap.get(a.original.workflow_state_id);
            const bState = stateMap.get(b.original.workflow_state_id);
            const labelCompare = (aState?.groupLabel ?? '').localeCompare(bState?.groupLabel ?? '');
            if (labelCompare !== 0) return labelCompare;
            return (aState?.position ?? 0) - (bState?.position ?? 0);
          },
          cell: (info) => (
            <InlineStateCell
              task={info.row.original}
              states={statesByWorkflowId.get(info.row.original.workflow_id) ?? workflow.states}
              stateMap={stateMap}
              onUpdate={updateTaskField}
            />
          ),
        }
      ),
      columnHelper.accessor('priority', {
        id: 'priorityIcon',
        header: 'Priority',
        size: 130,
        enableGrouping: false,
        cell: (info) => (
          <InlinePriorityCell
            task={info.row.original}
            onUpdate={updateTaskField}
          />
        ),
      }),
      columnHelper.accessor('severity', {
        id: 'severityIcon',
        header: 'Severity',
        size: 130,
        enableGrouping: false,
        cell: (info) => (
          <InlineSeverityCell
            task={info.row.original}
            onUpdate={updateTaskField}
          />
        ),
      }),
      columnHelper.accessor('estimate', {
        id: 'estimate',
        header: 'Estimate',
        size: 100,
        enableGrouping: false,
        cell: (info) => (
          <InlineEstimateCell
            task={info.row.original}
            estimateSettings={info.row.original.team_id ? estimateSettingsByTeamId.get(info.row.original.team_id) ?? null : null}
            onUpdate={updateTaskField}
          />
        ),
      }),
      columnHelper.accessor(
        (row) => {
          const ownerKeys = row.owner_member_ids ?? [];
          return ownerKeys.length > 0
            ? ownerKeys.map((ownerKey) => ownerNameMap.get(ownerKey) ?? 'Unknown').join(', ')
            : 'Unassigned';
        },
        {
          id: 'ownerName',
          header: 'Owner',
          size: 200,
          cell: (info) => (
            <InlineOwnerCell
              task={info.row.original}
              assignableMembers={assignableMembers}
              ownerNameMap={ownerNameMap}
              onUpdate={updateTaskField}
            />
          ),
        }
      ),
      columnHelper.accessor(
        (row) => (row.team_id ? teamMap.get(row.team_id) ?? 'Unknown' : 'No Team'),
        {
          id: 'teamName',
          header: 'Team',
          size: 180,
          cell: (info) => (
            <InlineTeamCell
              task={info.row.original}
              teams={teams}
              teamMap={teamMap}
              onUpdate={updateTaskField}
            />
          ),
        }
      ),
      columnHelper.accessor(
        (row) => (row.epic_id ? epicMap.get(row.epic_id) ?? 'Unknown' : 'No Epic'),
        {
          id: 'epicName',
          header: 'Epic',
          size: 200,
          cell: (info) => (
            <InlineEpicCell
              task={info.row.original}
              epics={epics}
              epicMap={epicMap}
              onUpdate={updateTaskField}
            />
          ),
        }
      ),
      columnHelper.accessor(
        (row) => (row.sprint_id ? sprintMap.get(row.sprint_id) ?? 'Unknown' : 'No Sprint'),
        {
          id: 'sprintName',
          header: 'Sprint',
          size: 190,
          cell: (info) => (
            <InlineSprintCell
              task={info.row.original}
              taskTeamId={info.row.original.team_id}
              listTeamId={teamId}
              sprints={sprints}
              sprintMap={sprintMap}
              onUpdate={updateTaskField}
            />
          ),
        }
      ),
      columnHelper.display({
        id: 'contacts',
        header: 'Contacts',
        size: 220,
        enableGrouping: false,
        enableSorting: false,
        cell: (info) => <InlineAssociationListCell items={info.row.original.contacts} emptyLabel="No contacts" />,
      }),
      columnHelper.display({
        id: 'companies',
        header: 'Companies',
        size: 220,
        enableGrouping: false,
        enableSorting: false,
        cell: (info) => <InlineAssociationListCell items={info.row.original.companies} emptyLabel="No companies" />,
      }),
      columnHelper.display({
        id: 'deals',
        header: 'Deals',
        size: 220,
        enableGrouping: false,
        enableSorting: false,
        cell: (info) => <InlineAssociationListCell items={info.row.original.deals} emptyLabel="No deals" />,
      }),
      columnHelper.display({
        id: 'support',
        header: 'Support',
        size: 220,
        enableGrouping: false,
        enableSorting: false,
        cell: (info) => <InlineAssociationListCell items={info.row.original.support_conversations} emptyLabel="No tickets" />,
      }),
      columnHelper.accessor(
        (row) => (row.task_type ? TASK_TYPE_CONFIG[row.task_type].label : 'Unknown'),
        {
          id: 'typeName',
          header: 'Type',
          size: 0,
          enableHiding: true,
          cell: () => null,
        }
      ),
      columnHelper.accessor(
        (row) => PRIORITY_CONFIG[row.priority].label,
        {
          id: 'priorityName',
          header: 'Priority Group',
          size: 0,
          enableHiding: true,
          cell: () => null,
        }
      ),
      columnHelper.accessor(
        (row) => SEVERITY_CONFIG[row.severity].label,
        {
          id: 'severityName',
          header: 'Severity Group',
          size: 0,
          enableHiding: true,
          cell: () => null,
        }
      ),
      columnHelper.accessor('deadline', {
        id: 'deadline',
        header: 'Deadline',
        size: 150,
        enableGrouping: false,
        cell: (info) => (
          <InlineDeadlineCell
            task={info.row.original}
            onUpdate={updateTaskField}
          />
        ),
      }),
      columnHelper.display({
        id: 'labels',
        header: 'Labels',
        size: 260,
        enableGrouping: false,
        enableSorting: false,
        cell: (info) => (
          <InlineLabelsCell
            task={info.row.original}
            workspaceId={workspaceId}
            allLabels={allLabels}
            onLabelsChange={setAllLabels}
            setTasks={setTasks}
          />
        ),
      }),
      columnHelper.accessor('updated_at', {
        id: 'updatedAt',
        header: 'Last Updated',
        size: 120,
        enableGrouping: false,
        cell: (info) => {
          const val = info.getValue();
          if (!val) return null;
          return (
            <span className="text-xs text-muted-foreground whitespace-nowrap">
              {format(parseISO(val), 'MMM d')}
            </span>
          );
        },
      }),
      columnHelper.display({
        id: 'actions',
        header: '',
        size: ACTIONS_COL_SIZE,
        enableGrouping: false,
        enableSorting: false,
        enableResizing: false,
        cell: (info) => (
            <InlineActionsCell
              task={info.row.original}
              workspaceId={workspaceId}
              workspaceSlug={workspaceSlug}
              automatedStateIds={automatedStateIds}
              onOpenTask={handleOpenTask}
              setTasks={setTasks}
            />
          ),
        }),
    ],
    [stateMap, statesByWorkflowId, ownerNameMap, teamMap, epicMap, sprintMap, estimateSettingsByTeamId, handleOpenTask, workflow.states, assignableMembers, teams, epics, sprints, updateTaskField, allLabels, workspaceId, workspaceSlug, fieldVis.task_type, displayProps.task_type, agentById, handleRowCheckboxToggle]
  );

  // Team-level disabled keys (for hiding toggles in display menu)
  const teamDisabledKeys = useMemo(() => {
    const keys = new Set<DisplayPropertyKey>();
    // Team-level field visibility
    if (!fieldVis.priority) keys.add('priority');
    if (!fieldVis.severity) keys.add('severity');
    if (!fieldVis.task_type) keys.add('task_type');
    if (!fieldVis.estimate) keys.add('estimate');
    if (!fieldVis.epic) keys.add('epic');
    if (!fieldVis.sprint) keys.add('sprint');
    if (!fieldVis.due_date) keys.add('due_date');
    if (!fieldVis.labels) keys.add('labels');
    if (teamId) keys.add('team');
    // Context scoping: hide columns that are redundant in the current view
    if (epicId) keys.add('epic');
    if (sprintId) keys.add('sprint');
    // CRM columns are not useful inside epic/sprint detail views
    if (epicId || sprintId) {
      keys.add('contacts');
      keys.add('companies');
      keys.add('deals');
      keys.add('support');
    }
    return keys;
  }, [fieldVis, teamId, epicId, sprintId]);

  const visibleGroupOptions = useMemo(
    () =>
      getVisibleTaskListGroupOptions(
        {
          task_type: fieldVis.task_type,
          priority: fieldVis.priority,
          severity: fieldVis.severity,
          epic: fieldVis.epic,
          sprint: fieldVis.sprint,
        },
        { epicId: epicId ?? undefined, sprintId: sprintId ?? undefined },
      ),
    [fieldVis.epic, fieldVis.priority, fieldVis.severity, fieldVis.sprint, fieldVis.task_type, epicId, sprintId],
  );

  useEffect(() => {
    if (visibleGroupOptions.some((option) => option.value === groupBy)) {
      return;
    }
    setGroupBy('workflow_state');
  }, [groupBy, setGroupBy, visibleGroupOptions]);

  const columnVisibility = useMemo(() => {
    const vis: Record<string, boolean> = {};
    for (const c of HIDDEN_GROUP_COLUMNS) vis[c] = false;
    // Team-level visibility (overrides everything)
    if (!fieldVis.priority) { vis['priorityIcon'] = false; vis['priorityName'] = false; }
    if (!fieldVis.severity) { vis['severityIcon'] = false; vis['severityName'] = false; }
    if (!fieldVis.task_type) { vis['typeName'] = false; }
    if (!fieldVis.estimate) vis['estimate'] = false;
    if (!fieldVis.epic) vis['epicName'] = false;
    if (!fieldVis.sprint) vis['sprintName'] = false;
    if (!fieldVis.due_date) vis['deadline'] = false;
    if (!fieldVis.labels) vis['labels'] = false;
    if (teamId) vis['teamName'] = false;
    // Context-scoping: when the list lives inside a specific epic/sprint, the
    // corresponding column is the same value for every row and adds only noise.
    if (epicId) vis['epicName'] = false;
    if (sprintId) vis['sprintName'] = false;
    // User-level display preferences (only hides columns the team allows)
    if (!displayProps.state) vis['stateName'] = false;
    if (!displayProps.priority && vis['priorityIcon'] !== false) vis['priorityIcon'] = false;
    if (!displayProps.severity && vis['severityIcon'] !== false) vis['severityIcon'] = false;
    if (!displayProps.estimate && vis['estimate'] !== false) vis['estimate'] = false;
    if (!displayProps.assignee) vis['ownerName'] = false;
    if (!displayProps.team && vis['teamName'] !== false) vis['teamName'] = false;
    if (!displayProps.epic && vis['epicName'] !== false) vis['epicName'] = false;
    if (!displayProps.sprint && vis['sprintName'] !== false) vis['sprintName'] = false;
    if (!displayProps.contacts) vis['contacts'] = false;
    if (!displayProps.companies) vis['companies'] = false;
    if (!displayProps.deals) vis['deals'] = false;
    if (!displayProps.support) vis['support'] = false;
    if (!displayProps.due_date && vis['deadline'] !== false) vis['deadline'] = false;
    if (!displayProps.labels && vis['labels'] !== false) vis['labels'] = false;
    if (!displayProps.updated_at) vis['updatedAt'] = false;
    return vis;
  }, [fieldVis, teamId, epicId, sprintId, displayProps]);

  const grouping: GroupingState = useMemo(() => {
    const colId = GROUP_COLUMN_MAP[groupBy];
    return colId ? [colId] : [];
  }, [groupBy]);
  const hasGroupedRows = grouping.length > 0;

  const table = useReactTable({
    data: tasks,
    columns: tableColumns,
    state: {
      grouping,
      expanded,
      rowSelection,
      columnVisibility,
      sorting,
      columnSizing,
    },
    onExpandedChange: setExpanded,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    onColumnSizingChange: setColumnSizing,
    enableRowSelection: true,
    enableColumnResizing: true,
    columnResizeMode: 'onChange',
    autoResetExpanded: false,
    getRowId: (row) => row.id,
    getExpandedRowModel: getExpandedRowModel(),
    getGroupedRowModel: getGroupedRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getCoreRowModel: getCoreRowModel(),
  });

  const { rows } = table.getRowModel();
  const selectWidth = table.getColumn('select')?.getSize() ?? CHECKBOX_COL_SIZE;
  const displayIdWidth = table.getColumn('displayId')?.getSize() ?? 90;
  const typeIconColumn = table.getColumn('typeIcon');
  const typeIconWidth = typeIconColumn?.getSize() ?? 40;
  const showTypeIcon = typeIconColumn?.getIsVisible() ?? false;
  const pinnedOffsets = useMemo(() => {
    const offsets = getTaskListPinnedOffsets({
        displayIdWidth,
        typeIconWidth,
        showTypeIcon,
      });
    return {
      displayId: offsets.displayId + selectWidth,
      typeIcon: offsets.typeIcon + selectWidth,
      name: offsets.name + selectWidth,
    };
  }, [displayIdWidth, selectWidth, typeIconWidth, showTypeIcon]);

  const getTrailingGroupStateId = useCallback(
    (index: number): string | null => {
      if (!isPerGroupMode) return null;

      const row = rows[index];
      if (!row || row.getIsGrouped()) return null;

      const nextRow = rows[index + 1];
      const isLastInGroup = !nextRow || nextRow.getIsGrouped();
      if (!isLastInGroup) return null;

      return row.original.workflow_state_id;
    },
    [isPerGroupMode, rows],
  );

  const estimateSize = useCallback(
    (index: number) => {
      const row = rows[index];
      if (!row) return ROW_HEIGHT;
      if (row.getIsGrouped()) {
        return GROUP_ROW_HEIGHT + (row.getIsExpanded() ? GROUP_HEADER_REPEAT_HEIGHT : 0);
      }

      const trailingGroupStateId = getTrailingGroupStateId(index);
      const hasLoadingSentinel = trailingGroupStateId !== null && groupLoadingId === trailingGroupStateId;

      return ROW_HEIGHT + (hasLoadingSentinel ? GROUP_LOAD_SENTINEL_HEIGHT : 0);
    },
    [getTrailingGroupStateId, groupLoadingId, rows],
  );

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize,
    overscan: hasGroupedRows ? GROUPED_OVERSCAN : FLAT_OVERSCAN,
  });

  const groupSummaries = useMemo(() => {
    const summaries = new Map<string, PMGroupSummary>();

    for (const row of rows) {
      if (!row.getIsGrouped()) continue;

      const subRows = row.subRows;
      let totalPoints = 0;
      let completedPoints = 0;

      for (const subRow of subRows) {
        const estimate = subRow.original.estimate ?? 0;
        totalPoints += estimate;
        const stateInfo = stateMap.get(subRow.original.workflow_state_id);
        if (stateInfo?.stateType === 'done') completedPoints += estimate;
      }

      const storyCount = isPerGroupMode && subRows[0]
        ? (groupHasMore.get(subRows[0].original.workflow_state_id)?.total ?? subRows.length)
        : subRows.length;

      const stateType =
        groupBy === 'workflow_state' && subRows[0]
          ? (stateMap.get(subRows[0].original.workflow_state_id)?.stateType as StateType | undefined)
          : undefined;
      const groupLabel =
        groupBy === 'workflow_state' && subRows[0]
          ? (stateMap.get(subRows[0].original.workflow_state_id)?.groupLabel ?? String(row.groupingValue))
          : String(row.groupingValue);

      summaries.set(row.id, {
        groupLabel,
        storyCount,
        totalPoints,
        completedPoints,
        stateType,
      });
    }

    return summaries;
  }, [groupBy, groupHasMore, isPerGroupMode, rows, stateMap]);

  useEffect(() => {
    const headerEl = headerRef.current;
    const scrollEl = parentRef.current;
    if (!headerEl || !scrollEl) return;

    const updateHeight = () => {
      scrollEl.style.setProperty('--task-list-header-height', `${headerEl.offsetHeight}px`);
    };

    updateHeight();

    if (typeof ResizeObserver === 'undefined') return;

    const observer = new ResizeObserver(() => {
      updateHeight();
    });
    observer.observe(headerEl);

    return () => observer.disconnect();
  }, []);

  // Compute total task count (including unloaded) for per-group mode
  const displayTaskCount = isPerGroupMode && groupHasMore.size > 0
    ? Array.from(groupHasMore.values()).reduce((sum, info) => sum + info.total, 0)
    : tasks.length;

  const selectedTasks = useMemo(
    () => tasks.filter((task) => rowSelection[task.id]),
    [rowSelection, tasks],
  );

  const handleBulkOperationComplete = useCallback(async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.tasks(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: ['pm', workspaceId, 'board'] }),
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.sprintPlanning(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.sprintPreviewTasksRoot(workspaceId) }),
      epicId ? queryClient.invalidateQueries({ queryKey: queryKeys.pm.epicTasks(workspaceId, epicId) }) : Promise.resolve(),
      sprintId ? queryClient.invalidateQueries({ queryKey: queryKeys.pm.sprintTasks(workspaceId, sprintId) }) : Promise.resolve(),
    ]);

    if (onBulkOperationComplete) {
      await onBulkOperationComplete();
      return;
    }

    if (isExternal) return;
    if (isPerGroupMode) {
      await fetchTasksByState();
    } else {
      await fetchTasksFlat(1, false);
    }
  }, [
    epicId,
    fetchTasksByState,
    fetchTasksFlat,
    isExternal,
    isPerGroupMode,
    onBulkOperationComplete,
    queryClient,
    sprintId,
    workspaceId,
  ]);

  const toggleGroupSelection = useCallback(
    (groupRow: Row<Task>, checked: boolean) => {
      const leafIds = (groupRow.getLeafRows() as Row<Task>[])
        .filter((leaf) => !leaf.getIsGrouped())
        .map((leaf) => leaf.original.id);
      if (leafIds.length === 0) return;
      setRowSelection((current) => {
        const next = { ...current };
        for (const id of leafIds) {
          if (checked) next[id] = true;
          else delete next[id];
        }
        return next;
      });
    },
    [],
  );

  const getGroupSelectionState = useCallback(
    (groupRow: Row<Task>): boolean | 'indeterminate' => {
      const leaves = (groupRow.getLeafRows() as Row<Task>[])
        .filter((leaf) => !leaf.getIsGrouped());
      if (leaves.length === 0) return false;
      let selected = 0;
      for (const leaf of leaves) {
        if (rowSelection[leaf.original.id]) selected += 1;
      }
      if (selected === 0) return false;
      if (selected === leaves.length) return true;
      return 'indeterminate';
    },
    [rowSelection],
  );

  const renderColumnHeaderRow = (opts?: {
    groupRow?: Row<Task>;
    groupSelectionState?: boolean | 'indeterminate';
  }) => {
    const groupRow = opts?.groupRow;
    const isGroupRepeat = !!groupRow;
    return table.getHeaderGroups().map((headerGroup) => (
      <div key={`${headerGroup.id}-${groupRow ? groupRow.id : 'main'}`} className="flex items-center">
        {headerGroup.headers.map((header) => {
          if (header.column.getIsGrouped()) return null;
          const defSize = header.column.columnDef.size ?? 150;
          const runtimeSize = header.getSize();
          if (defSize === 0 && runtimeSize === 0) return null;
          const isResized = !!columnSizing[header.column.id];
          const canSort = header.column.getCanSort();
          const sorted = header.column.getIsSorted();
          const colId = header.column.id;
          const pinnedClass = colId === 'select' || colId === 'displayId' || colId === 'typeIcon' || colId === 'name'
            ? TABLE_PINNED_HEADER_LEFT
            : colId === 'actions' ? TABLE_PINNED_HEADER_RIGHT : '';
          const pinnedSt = colId === 'select' ? pinnedStyle('left', 0)
            : colId === 'displayId' ? pinnedStyle('left', pinnedOffsets.displayId)
            : colId === 'typeIcon' ? pinnedStyle('left', pinnedOffsets.typeIcon)
            : colId === 'name' ? pinnedStyle('left', pinnedOffsets.name)
            : colId === 'actions' ? pinnedStyle('right', 0) : {};
          const isGroupSelectCell = isGroupRepeat && colId === 'select';
          if (colId === 'actions') {
            return (
              <div
                key={header.id}
                className={TABLE_HEADER_CELL_ACTIONS}
                style={{ ...dynamicCellStyle(defSize, runtimeSize, isResized, 400), ...pinnedSt }}
                aria-hidden="true"
              />
            );
          }
          return (
            <div
              key={header.id}
              className={`${TASK_LIST_HEADER_CELL} ${!isGroupSelectCell && canSort ? TASK_LIST_HEADER_CELL_SORTABLE : ''} ${pinnedClass}`}
              style={{ ...dynamicCellStyle(defSize, runtimeSize, isResized, 400), ...pinnedSt }}
              onClick={!isGroupSelectCell && canSort ? header.column.getToggleSortingHandler() : undefined}
            >
              {isGroupSelectCell ? (
                <div
                  className="flex items-center"
                  onClick={(event) => event.stopPropagation()}
                >
                  <Checkbox
                    checked={opts?.groupSelectionState ?? false}
                    onCheckedChange={(value) => {
                      if (groupRow) toggleGroupSelection(groupRow, value === true);
                    }}
                    aria-label="Select all in group"
                  />
                </div>
              ) : (
                <div className="flex items-center gap-1 overflow-hidden whitespace-nowrap">
                  {header.isPlaceholder
                    ? null
                    : flexRender(header.column.columnDef.header, header.getContext())}
                  {canSort && (
                    <span className="ml-auto shrink-0">
                      {sorted === 'asc' ? (
                        <TaskListChevronUpIcon className="h-3 w-3 text-foreground/80" />
                      ) : sorted === 'desc' ? (
                        <TaskListChevronDownIcon className="h-3 w-3 text-foreground/80" />
                      ) : (
                        <TaskListArrowUpDownIcon className="h-3 w-3 text-muted-foreground" />
                      )}
                    </span>
                  )}
                </div>
              )}
              {header.column.getCanResize() && (
                <div
                  onMouseDown={header.getResizeHandler()}
                  onTouchStart={header.getResizeHandler()}
                  onClick={(e) => e.stopPropagation()}
                  className={`${TABLE_RESIZE_HANDLE} ${header.column.getIsResizing() ? 'bg-primary/50' : ''}`}
                />
              )}
            </div>
          );
        })}
      </div>
    ));
  };

  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
        <Loading01Icon className="mr-2 h-4 w-4 animate-spin" />
        Loading tasks...
      </div>
    );
  }

  const bulkActionsBar = (
    <TaskBulkActionsBar
      selectedTasks={selectedTasks}
      workspaceId={workspaceId}
      teamId={teamId}
      workflow={workflow}
      assignableMembers={assignableMembers}
      epics={epics}
      sprints={sprints}
      labels={allLabels}
      onComplete={handleBulkOperationComplete}
      onClearSelection={clearSelection}
    />
  );

  const portalContainer = bulkTriggerContainer ?? null;
  const usePortal = !!portalContainer;

  return (
    <div className="relative flex min-h-0 flex-1 flex-col gap-2">
      {usePortal && selectedTasks.length > 0 ? createPortal(bulkActionsBar, portalContainer) : null}
      {showToolbar ? (
        <div className="flex items-center gap-2 px-3 pt-2">
          <span className="text-xs text-muted-foreground">Group by:</span>
          <Select value={groupBy} onValueChange={(v) => setGroupBy(v as TaskListGroupByOption)}>
            <SelectTrigger className="h-7 w-[160px] text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {visibleGroupOptions.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span className="text-xs text-muted-foreground">
            {displayTaskCount} {displayTaskCount === 1 ? 'task' : 'tasks'}{!isPerGroupMode && hasMore ? '+' : ''}
          </span>
          <div className="ml-auto flex items-center gap-2">
            {!usePortal ? bulkActionsBar : null}
            <ListDisplayMenu disabledKeys={teamDisabledKeys} />
          </div>
        </div>
      ) : !usePortal && selectedTasks.length > 0 ? (
        <div className="flex items-center justify-end gap-2 px-3 pt-2">
          {bulkActionsBar}
        </div>
      ) : null}

      {/* Table */}
      <div
        ref={parentRef}
        className={TABLE_CONTAINER}
        onScroll={(e) => {
          const el = e.currentTarget;
          const scrollTop = el.scrollTop;

          // Infinite loading (only for flat/global pagination, not per-group mode)
          if (!isExternal && !isPerGroupMode && hasMore && !loadingMore) {
            if (scrollTop + el.clientHeight >= el.scrollHeight - 200) {
              loadMore();
            }
          }
        }}
      >
        <div className="min-w-fit">
        {/* Header */}
        <div ref={headerRef} className={TASK_LIST_HEADER}>
          {hasGroupedRows ? null : renderColumnHeaderRow()}
        </div>

        {/* Virtualized body */}
          <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative', width: '100%' }}>
            {hasGroupedRows ? (
              <StickyPinnedGroupOverlay
                parentRef={parentRef}
                rows={rows}
                virtualizer={virtualizer}
                groupSummaries={groupSummaries}
              />
            ) : null}
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const row = rows[virtualRow.index] as Row<Task>;
              const isGrouped = row.getIsGrouped();

              // Determine if this is the last data row before the next group (for per-group "Load more")
              let showGroupLoadMore = false;
              let groupStateId = '';
              if (isPerGroupMode && !isGrouped) {
                const nextRow = rows[virtualRow.index + 1] as Row<Task> | undefined;
                const isLastInGroup = !nextRow || nextRow.getIsGrouped();
                if (isLastInGroup) {
                  groupStateId = row.original.workflow_state_id;
                  const info = groupHasMore.get(groupStateId);
                  if (info?.hasMore) {
                    showGroupLoadMore = true;
                  }
                }
              }

              return (
                <div
                  key={row.id}
                  data-index={virtualRow.index}
                  style={virtualRowStyle(virtualRow.start)}
                >
                  {isGrouped ? (
                    <>
                      <MemoGroupHeaderRow
                        row={row}
                        summary={groupSummaries.get(row.id)}
                      />
                      {row.getIsExpanded() ? (
                        <div className="border-b border-border/60 bg-card">
                          {renderColumnHeaderRow({
                            groupRow: row,
                            groupSelectionState: getGroupSelectionState(row),
                          })}
                        </div>
                      ) : null}
                    </>
                  ) : (
                    <>
                      <MemoDataRow
                        row={row}
                        isSelected={!!rowSelection[row.original.id]}
                        onOpenTask={handleOpenTask}
                        onToggleSelection={handleRowCheckboxToggle}
                        columnSizing={columnSizing}
                        columnSizingVersion={columnSizingVersion}
                        pinnedOffsets={pinnedOffsets}
                      />
                      {showGroupLoadMore && (
                        <GroupLoadSentinel
                          stateId={groupStateId}
                          isLoading={groupLoadingId === groupStateId}
                          onLoadMore={loadMoreForGroup}
                        />
                      )}
                    </>
                  )}
                </div>
              );
            })}
          </div>
          {loadingMore && !isPerGroupMode && (
            <div className="flex items-center justify-center py-3 text-sm text-muted-foreground">
              <Loading01Icon className="mr-2 h-4 w-4 animate-spin" />
              Loading more tasks...
            </div>
          )}
        </div>
      </div>
      {footer ? (
        <div className="border-t border-border/60 bg-card">
          {footer}
        </div>
      ) : null}
    </div>
  );
}

interface PMGroupRowProps {
  row: Row<Task>;
  summary?: PMGroupSummary;
}

function arePMGroupRowPropsEqual(prev: PMGroupRowProps, next: PMGroupRowProps): boolean {
  return (
    prev.row.id === next.row.id &&
    prev.row.getIsExpanded() === next.row.getIsExpanded() &&
    prev.row.subRows.length === next.row.subRows.length &&
    prev.summary?.groupLabel === next.summary?.groupLabel &&
    prev.summary?.storyCount === next.summary?.storyCount &&
    prev.summary?.totalPoints === next.summary?.totalPoints &&
    prev.summary?.completedPoints === next.summary?.completedPoints &&
    prev.summary?.stateType === next.summary?.stateType
  );
}

const MemoGroupHeaderRow = memo(function GroupHeaderRow({
  row,
  summary,
}: PMGroupRowProps) {
  const storyCount = summary?.storyCount ?? row.subRows.length;
  const totalPoints = summary?.totalPoints ?? 0;
  const completedPoints = summary?.completedPoints ?? 0;
  const stateType = summary?.stateType;
  const groupLabel = summary?.groupLabel ?? String(row.groupingValue);

  return (
    <button
      className={`${TASK_LIST_GROUP_ROW} w-full text-left text-xs`}
      onClick={row.getToggleExpandedHandler()}
    >
      <span className="sticky left-0 z-[1] flex items-center gap-2 px-3">
        {row.getIsExpanded() ? (
          <TaskListChevronDownIcon className="h-3.5 w-3.5 text-muted-foreground" />
        ) : (
          <TaskListChevronRightIcon className="h-3.5 w-3.5 text-muted-foreground" />
        )}
        {stateType && <TaskListStateTypeIcon stateType={stateType} className="h-4 w-4" />}
        <span>{groupLabel}</span>
        <span className="flex items-center gap-3 ml-1 font-normal text-muted-foreground">
          <QuickTooltip label={`${storyCount} ${storyCount === 1 ? 'task' : 'tasks'}`}>
            <span className="flex items-center gap-1">
              <TaskListNoteIcon className="h-3 w-3" /> {storyCount}
            </span>
          </QuickTooltip>
          <QuickTooltip label={`${totalPoints} total ${totalPoints === 1 ? 'point' : 'points'}`}>
            <span className="flex items-center gap-1">
              <TaskListChartIcon className="h-3 w-3" /> {totalPoints}
            </span>
          </QuickTooltip>
          <QuickTooltip label={`${completedPoints} completed ${completedPoints === 1 ? 'point' : 'points'}`}>
            <span className="flex items-center gap-1">
              <TaskListDoneCircleIcon className="h-3 w-3" /> {completedPoints}
            </span>
          </QuickTooltip>
        </span>
      </span>
    </button>
  );
}, arePMGroupRowPropsEqual);

interface PMGroupSummary {
  groupLabel: string;
  storyCount: number;
  totalPoints: number;
  completedPoints: number;
  stateType?: StateType;
}

interface TaskListVirtualizerLike {
  getVirtualItems: () => Array<{ index: number; start: number }>;
}

function StickyPinnedGroupOverlay({
  parentRef,
  rows,
  virtualizer,
  groupSummaries,
}: {
  parentRef: RefObject<HTMLDivElement | null>;
  rows: Row<Task>[];
  virtualizer: TaskListVirtualizerLike;
  groupSummaries: Map<string, PMGroupSummary>;
}) {
  const pinnedGroupRef = useRef<number | null>(null);
  const pinnedGroupRafRef = useRef<number | null>(null);
  const [pinnedGroupIdx, setPinnedGroupIdx] = useState<number | null>(null);

  useEffect(() => {
    return () => {
      if (pinnedGroupRafRef.current !== null) {
        cancelAnimationFrame(pinnedGroupRafRef.current);
      }
    };
  }, []);

  useEffect(() => {
    const scrollEl = parentRef.current;
    if (!scrollEl) return;

    const syncPinnedGroup = () => {
      const scrollTop = scrollEl.scrollTop;
      const virtualItems = virtualizer.getVirtualItems();
      let newPinnedIdx: number | null = null;

      if (scrollTop > 10) {
        for (const vItem of virtualItems) {
          if (vItem.start > scrollTop) break;
          if (rows[vItem.index]?.getIsGrouped()) newPinnedIdx = vItem.index;
        }
        if (newPinnedIdx === null && virtualItems.length > 0) {
          for (let index = virtualItems[0].index - 1; index >= 0; index -= 1) {
            if (rows[index]?.getIsGrouped()) {
              newPinnedIdx = index;
              break;
            }
          }
        }
      }

      if (newPinnedIdx !== pinnedGroupRef.current) {
        pinnedGroupRef.current = newPinnedIdx;
        startTransition(() => {
          setPinnedGroupIdx((current) => (current === newPinnedIdx ? current : newPinnedIdx));
        });
      }
    };

    const handleScroll = () => {
      if (pinnedGroupRafRef.current !== null) return;
      pinnedGroupRafRef.current = requestAnimationFrame(() => {
        pinnedGroupRafRef.current = null;
        syncPinnedGroup();
      });
    };

    syncPinnedGroup();
    scrollEl.addEventListener('scroll', handleScroll, { passive: true });

    return () => {
      scrollEl.removeEventListener('scroll', handleScroll);
      if (pinnedGroupRafRef.current !== null) {
        cancelAnimationFrame(pinnedGroupRafRef.current);
        pinnedGroupRafRef.current = null;
      }
    };
  }, [parentRef, rows, virtualizer]);

  const pinnedGroupRow = pinnedGroupIdx !== null ? rows[pinnedGroupIdx] : undefined;

  if (!pinnedGroupRow) return null;

  return (
    <div className="sticky z-[5]" style={{ top: 'var(--task-list-header-height, 0px)', height: 0, overflow: 'visible' }}>
      <div className="border-b border-border/60 bg-background">
        <MemoGroupHeaderRow row={pinnedGroupRow} summary={groupSummaries.get(pinnedGroupRow.id)} />
      </div>
    </div>
  );
}

function DataRowSelectCheckbox({
  taskId,
  taskKey,
  isSelected,
  onToggleSelection,
}: {
  taskId: string;
  taskKey: string;
  isSelected: boolean;
  onToggleSelection: (taskId: string, checked: boolean, shiftKey: boolean) => void;
}) {
  const shiftRef = useRef(false);
  return (
    <Checkbox
      checked={isSelected}
      onMouseDown={(event) => {
        shiftRef.current = event.shiftKey;
      }}
      onCheckedChange={(value) => {
        onToggleSelection(taskId, value === true, shiftRef.current);
        shiftRef.current = false;
      }}
      onClick={(event) => event.stopPropagation()}
      aria-label={`Select ${taskKey}`}
    />
  );
}

interface PMDataRowProps {
  row: Row<Task>;
  isSelected: boolean;
  onOpenTask: (task: Task) => void;
  onToggleSelection: (taskId: string, checked: boolean, shiftKey: boolean) => void;
  columnSizing: ColumnSizingState;
  columnSizingVersion: string;
  pinnedOffsets: TaskListPinnedOffsets;
}

function arePMDataRowPropsEqual(prev: PMDataRowProps, next: PMDataRowProps): boolean {
  return (
    prev.row.id === next.row.id &&
    prev.row.original === next.row.original &&
    prev.isSelected === next.isSelected &&
    prev.columnSizingVersion === next.columnSizingVersion &&
    prev.pinnedOffsets === next.pinnedOffsets &&
    prev.onOpenTask === next.onOpenTask &&
    prev.onToggleSelection === next.onToggleSelection
  );
}

const MemoDataRow = memo(function DataRow({
  row,
  isSelected,
  onOpenTask,
  onToggleSelection,
  columnSizing,
  columnSizingVersion,
  pinnedOffsets,
}: PMDataRowProps) {
  void columnSizingVersion; // used by arePMDataRowPropsEqual for memo comparison
  return (
    <div
      className={TASK_LIST_ROW}
      onClick={() => onOpenTask(row.original)}
    >
      {row.getVisibleCells().map((cell) => {
        // Skip the grouped column entirely — header does the same, keeping alignment
        if (cell.column.getIsGrouped()) return null;
        const { defSize, runtimeSize, isResized } = resolveColumnRuntimeSize(cell.column, columnSizing);
        if (defSize === 0 && runtimeSize === 0) return null;
        const colId = cell.column.id;
        const pinnedClass = colId === 'select' || colId === 'displayId' || colId === 'typeIcon' || colId === 'name'
          ? TABLE_PINNED_LEFT
          : colId === 'actions' ? TABLE_PINNED_RIGHT : '';
        const pinnedSt = colId === 'select' ? pinnedStyle('left', 0)
          : colId === 'displayId' ? pinnedStyle('left', pinnedOffsets.displayId)
          : colId === 'typeIcon' ? pinnedStyle('left', pinnedOffsets.typeIcon)
          : colId === 'name' ? pinnedStyle('left', pinnedOffsets.name)
          : colId === 'actions' ? pinnedStyle('right', 0) : {};
        return (
          <div
            key={cell.id}
            className={`${TASK_LIST_CELL} overflow-hidden ${pinnedClass}`}
            style={{ ...dynamicCellStyle(defSize, runtimeSize, isResized, 400), ...pinnedSt }}
          >
            {colId === 'select' ? (
              <DataRowSelectCheckbox
                taskId={row.original.id}
                taskKey={row.original.task_key}
                isSelected={isSelected}
                onToggleSelection={onToggleSelection}
              />
            ) : (
              flexRender(cell.column.columnDef.cell, cell.getContext())
            )}
          </div>
        );
      })}
    </div>
  );
}, arePMDataRowPropsEqual);

// Auto-loading sentinel: triggers loadMore when the virtualizer renders it (i.e. near viewport)
function GroupLoadSentinel({
  stateId,
  isLoading,
  onLoadMore,
}: {
  stateId: string;
  isLoading: boolean;
  onLoadMore: (id: string) => void;
}) {
  const triggered = useRef(false);

  useEffect(() => {
    triggered.current = false;
  }, [stateId]);

  useEffect(() => {
    if (!isLoading && !triggered.current) {
      triggered.current = true;
      onLoadMore(stateId);
    }
  }, [stateId, isLoading, onLoadMore]);

  return isLoading ? (
    <div className="flex w-full items-center justify-center gap-2 py-1.5 text-xs text-muted-foreground">
      <Loading01Icon className="h-3 w-3 animate-spin" />
      Loading...
    </div>
  ) : null;
}

// ── Inline editable cells ──────────────────────────────────────────

function InlinePriorityCell({
  task,
  onUpdate,
}: {
  task: Task;
  onUpdate: (taskId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const p = task.priority;

  if (!open) {
    return (
      <button
        type="button"
        className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        onClick={(e) => { e.stopPropagation(); setOpen(true); }}
      >
        <TaskListPriorityIcon priority={p} className="h-4 w-4" />
        {PRIORITY_CONFIG[p].label}
      </button>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          <TaskListPriorityIcon priority={p} className="h-4 w-4" />
          {PRIORITY_CONFIG[p].label}
        </button>
      </PopoverTrigger>
      {open && (
        <PopoverContent
          className="w-[180px] p-0"
          align="start"
          side="bottom"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Command>
            <CommandInput placeholder="Search..." className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No match</CommandEmpty>
              <CommandGroup>
                {ALL_PRIORITIES.map((pri) => {
                  const cfg = PRIORITY_CONFIG[pri];
                  return (
                    <CommandItem
                      key={pri}
                      value={cfg.label}
                      onSelect={() => {
                        if (pri !== p) onUpdate(task.id, { priority: pri });
                        setOpen(false);
                      }}
                      className="flex items-center gap-2 text-xs"
                    >
                      <TaskListPriorityIcon priority={pri} className="h-4 w-4" />
                      <span>{cfg.label}</span>
                      {p === pri && <TaskListCheckIcon className="ml-auto h-3.5 w-3.5 text-primary" />}
                    </CommandItem>
                  );
                })}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      )}
    </Popover>
  );
}

function InlineStateCell({
  task,
  states,
  stateMap,
  onUpdate,
}: {
  task: Task;
  states: WorkflowWithStates['states'];
  stateMap: Map<string, { name: string; stateType: string }>;
  onUpdate: (taskId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const current = stateMap.get(task.workflow_state_id);

  if (!open) {
    return (
      <button
        type="button"
        className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        onClick={(e) => { e.stopPropagation(); setOpen(true); }}
      >
        {current && (
          <TaskListStateTypeIcon stateType={current.stateType as 'backlog' | 'unstarted' | 'started' | 'done'} className="h-3.5 w-3.5" />
        )}
        {current?.name ?? 'Unknown'}
      </button>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          {current && (
            <TaskListStateTypeIcon stateType={current.stateType as 'backlog' | 'unstarted' | 'started' | 'done'} className="h-3.5 w-3.5" />
          )}
          {current?.name ?? 'Unknown'}
        </button>
      </PopoverTrigger>
      {open && (
        <PopoverContent
          className="w-[200px] p-0"
          align="start"
          side="bottom"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Command>
            <CommandInput placeholder="Search..." className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No match</CommandEmpty>
              <CommandGroup>
                {states.map((s) => (
                  <CommandItem
                    key={s.id}
                    value={s.name}
                    onSelect={() => {
                      if (s.id !== task.workflow_state_id)
                        onUpdate(task.id, { workflow_state_id: s.id });
                      setOpen(false);
                    }}
                    className="flex items-center gap-2 text-xs"
                  >
                    <TaskListStateTypeIcon stateType={s.state_type} className="h-3.5 w-3.5" />
                    <span>{s.name}</span>
                    {task.workflow_state_id === s.id && <TaskListCheckIcon className="ml-auto h-3.5 w-3.5 text-primary" />}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      )}
    </Popover>
  );
}

function InlineOwnerCell({
  task,
  assignableMembers,
  ownerNameMap,
  onUpdate,
}: {
  task: Task;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  onUpdate: (taskId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const ownerMemberIds = task.owner_member_ids ?? [];

  return (
    <MultiMemberPickerPopover
      values={ownerMemberIds}
      members={assignableMembers}
      lazyMount
      onChange={(nextOwnerIds) => {
        void onUpdate(task.id, { owner_member_ids: nextOwnerIds });
      }}
      renderTrigger={() => {
        return ownerMemberIds.length > 0 ? (
          <OwnerAvatarStack
            memberIds={ownerMemberIds}
            nameMap={ownerNameMap}
            members={assignableMembers}
            size="sm"
            max={3}
          />
        ) : (
          <>
            <TaskListUserAddIcon className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="text-muted-foreground">Assign</span>
          </>
        );
      }}
    />
  );
}

function InlineSeverityCell({
  task,
  onUpdate,
}: {
  task: Task;
  onUpdate: (taskId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const s = task.severity;

  if (!open) {
    return (
      <button
        type="button"
        className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        onClick={(e) => { e.stopPropagation(); setOpen(true); }}
      >
        {s !== 'none' ? (
          <>
            <TaskListSeverityIcon severity={s} className="h-4 w-4" />
            {SEVERITY_CONFIG[s].label}
          </>
        ) : (
          <span className="text-muted-foreground">None</span>
        )}
      </button>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          {s !== 'none' ? (
            <>
              <TaskListSeverityIcon severity={s} className="h-4 w-4" />
              {SEVERITY_CONFIG[s].label}
            </>
          ) : (
            <span className="text-muted-foreground">None</span>
          )}
        </button>
      </PopoverTrigger>
      {open && (
        <PopoverContent
          className="w-[180px] p-0"
          align="start"
          side="bottom"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Command>
            <CommandInput placeholder="Search..." className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No match</CommandEmpty>
              <CommandGroup>
                {ALL_SEVERITIES.map((sev) => {
                  const cfg = SEVERITY_CONFIG[sev];
                  return (
                    <CommandItem
                      key={sev}
                      value={cfg.label}
                      onSelect={() => {
                        if (sev !== s) onUpdate(task.id, { severity: sev });
                        setOpen(false);
                      }}
                      className="flex items-center gap-2 text-xs"
                    >
                      <TaskListSeverityIcon severity={sev} className="h-4 w-4" />
                      <span>{cfg.label}</span>
                      {s === sev && <TaskListCheckIcon className="ml-auto h-3.5 w-3.5 text-primary" />}
                    </CommandItem>
                  );
                })}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      )}
    </Popover>
  );
}

function InlineEstimateCell({
  task,
  estimateSettings,
  onUpdate,
}: {
  task: Task;
  estimateSettings: TeamEstimateSettings | null;
  onUpdate: (taskId: string, patch: Partial<Task>) => Promise<void>;
}) {
  return (
    <div onClick={(e) => e.stopPropagation()}>
      <EstimatePicker
        value={task.estimate?.toString() ?? ''}
        teamId={task.team_id}
        estimateSettings={estimateSettings}
        lazyMount
        onChange={(_displayValue, apiValue) => {
          const next = apiValue ?? null;
          if (next !== task.estimate) {
            onUpdate(task.id, { estimate: next as number });
          }
        }}
      />
    </div>
  );
}

function InlineTeamCell({
  task,
  teams,
  teamMap,
  onUpdate,
}: {
  task: Task;
  teams: WorkspaceTeam[];
  teamMap: Map<string, string>;
  onUpdate: (taskId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const teamName = task.team_id ? teamMap.get(task.team_id) ?? 'Unknown' : null;

  if (!open) {
    return (
      <button
        type="button"
        className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        onClick={(e) => { e.stopPropagation(); setOpen(true); }}
      >
        {teamName ? (
          <span className="truncate">{teamName}</span>
        ) : (
          <span className="text-muted-foreground">No Team</span>
        )}
      </button>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          {teamName ? (
            <span className="truncate">{teamName}</span>
          ) : (
            <span className="text-muted-foreground">No Team</span>
          )}
        </button>
      </PopoverTrigger>
      {open && (
        <PopoverContent
          className="w-[200px] p-0"
          align="start"
          side="bottom"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Command>
            <CommandInput placeholder="Search teams..." className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No teams found</CommandEmpty>
              <CommandGroup>
                {teams.map((t) => (
                  <CommandItem
                    key={t.id}
                    value={t.name}
                    onSelect={() => {
                      const newTeamId = task.team_id === t.id ? undefined : t.id;
                      onUpdate(task.id, { team_id: newTeamId });
                      setOpen(false);
                    }}
                    className="flex items-center gap-2 text-xs"
                  >
                    <span className="truncate">{t.name}</span>
                    {task.team_id === t.id && <TaskListCheckIcon className="ml-auto h-3.5 w-3.5 text-primary" />}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      )}
    </Popover>
  );
}

function InlineEpicCell({
  task,
  epics,
  epicMap,
  onUpdate,
}: {
  task: Task;
  epics: EpicWithStats[];
  epicMap: Map<string, string>;
  onUpdate: (taskId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const epicName = task.epic_id ? epicMap.get(task.epic_id) ?? 'Unknown' : null;

  if (!open) {
    return (
      <button
        type="button"
        className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        onClick={(e) => { e.stopPropagation(); setOpen(true); }}
      >
        {epicName ? (
          <span className="truncate">{epicName}</span>
        ) : (
          <span className="text-muted-foreground">No Epic</span>
        )}
      </button>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          {epicName ? (
            <span className="truncate">{epicName}</span>
          ) : (
            <span className="text-muted-foreground">No Epic</span>
          )}
        </button>
      </PopoverTrigger>
      {open && (
        <PopoverContent
          className="w-[220px] p-0"
          align="start"
          side="bottom"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Command>
            <CommandInput placeholder="Search epics..." className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No epics found</CommandEmpty>
              <CommandGroup>
                {epics.map((e) => (
                  <CommandItem
                    key={e.epic.id}
                    value={e.epic.name}
                    onSelect={() => {
                      const newEpicId = task.epic_id === e.epic.id ? undefined : e.epic.id;
                      onUpdate(task.id, { epic_id: newEpicId });
                      setOpen(false);
                    }}
                    className="flex items-center gap-2 text-xs"
                  >
                    <span className="truncate">{e.epic.name}</span>
                    {task.epic_id === e.epic.id && <TaskListCheckIcon className="ml-auto h-3.5 w-3.5 text-primary" />}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      )}
    </Popover>
  );
}

function InlineSprintCell({
  task,
  taskTeamId,
  listTeamId,
  sprints,
  sprintMap,
  onUpdate,
}: {
  task: Task;
  taskTeamId?: string | null;
  listTeamId?: string | null;
  sprints: SprintWithStats[];
  sprintMap: Map<string, string>;
  onUpdate: (taskId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const sprintName = task.sprint_id ? sprintMap.get(task.sprint_id) ?? 'Unknown' : null;
  const visibleSprints = useMemo(
    () => getVisibleSprintsForTaskScope(sprints, { taskTeamId, listTeamId }),
    [listTeamId, sprints, taskTeamId],
  );

  if (!open) {
    return (
      <button
        type="button"
        className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        onClick={(e) => { e.stopPropagation(); setOpen(true); }}
      >
        {sprintName ? (
          <span className="truncate">{sprintName}</span>
        ) : (
          <span className="text-muted-foreground">No Sprint</span>
        )}
      </button>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          {sprintName ? (
            <span className="truncate">{sprintName}</span>
          ) : (
            <span className="text-muted-foreground">No Sprint</span>
          )}
        </button>
      </PopoverTrigger>
      {open && (
        <PopoverContent
          className="w-[220px] p-0"
          align="start"
          side="bottom"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Command>
            <CommandInput placeholder="Search sprints..." className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No sprints found</CommandEmpty>
              <CommandGroup>
                {visibleSprints.map((sp) => (
                  <CommandItem
                    key={sp.sprint.id}
                    value={sp.sprint.name}
                    onSelect={() => {
                      const newSprintId = task.sprint_id === sp.sprint.id ? undefined : sp.sprint.id;
                      onUpdate(task.id, { sprint_id: newSprintId });
                      setOpen(false);
                    }}
                    className="flex items-center gap-2 text-xs"
                  >
                    <span className="truncate">{sp.sprint.name}</span>
                    {task.sprint_id === sp.sprint.id && <TaskListCheckIcon className="ml-auto h-3.5 w-3.5 text-primary" />}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      )}
    </Popover>
  );
}

function InlineAssociationListCell({
  items,
  emptyLabel,
}: {
  items?: AssociationObjectSummary[];
  emptyLabel: string;
}) {
  if (!items || items.length === 0) {
    return <span className="text-xs text-muted-foreground">{emptyLabel}</span>;
  }

  const visible = items.slice(0, 2);
  const remaining = items.length - visible.length;
  const label = visible.map((item) => item.title).join(', ');
  const fullLabel = items.map((item) => item.title).join(', ');

  return (
    <span className="flex min-w-0 items-center gap-1 text-xs" title={fullLabel}>
      <span className="truncate">{label}</span>
      {remaining > 0 ? (
        <span className="shrink-0 text-muted-foreground">+{remaining}</span>
      ) : null}
    </span>
  );
}

function InlineDeadlineCell({
  task,
  onUpdate,
}: {
  task: Task;
  onUpdate: (taskId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const v = task.deadline;
  const selected = v ? parseISO(v) : undefined;
  const isOverdue = selected ? selected < new Date() : false;

  if (!open) {
    return (
      <button
        type="button"
        className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        onClick={(e) => { e.stopPropagation(); setOpen(true); }}
      >
        {selected ? (
          <>
            <TaskListCalendarIcon className="h-3.5 w-3.5 text-muted-foreground" />
            <span className={isOverdue ? 'text-red-500' : 'text-muted-foreground'}>
              {format(selected, 'MMM d, yyyy')}
            </span>
          </>
        ) : (
          <>
            <TaskListCalendarIcon className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="text-muted-foreground">No date</span>
          </>
        )}
      </button>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          {selected ? (
            <>
              <TaskListCalendarIcon className="h-3.5 w-3.5 text-muted-foreground" />
              <span className={isOverdue ? 'text-red-500' : 'text-muted-foreground'}>
                {format(selected, 'MMM d, yyyy')}
              </span>
            </>
          ) : (
            <>
              <TaskListCalendarIcon className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="text-muted-foreground">No date</span>
            </>
          )}
        </button>
      </PopoverTrigger>
      {open && (
        <PopoverContent
          className="w-auto p-0"
          align="start"
          side="bottom"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Calendar
            mode="single"
            selected={selected}
            defaultMonth={selected}
            onSelect={(date) => {
              onUpdate(task.id, { deadline: date ? format(date, 'yyyy-MM-dd') : undefined });
              setOpen(false);
            }}
          />
        </PopoverContent>
      )}
    </Popover>
  );
}

function InlineLabelsCell({
  task,
  workspaceId,
  allLabels,
  onLabelsChange,
  setTasks,
}: {
  task: Task;
  workspaceId: string;
  allLabels: Label[];
  onLabelsChange: (labels: Label[]) => void;
  setTasks: React.Dispatch<React.SetStateAction<Task[]>>;
}) {
  const storyLabels = task.labels ?? [];
  return (
    <div onClick={(e) => e.stopPropagation()}>
      <LabelPicker
        workspaceId={workspaceId}
        teamId={task.team_id || undefined}
        labels={allLabels}
        selectedLabelIds={storyLabels.map((l) => l.id)}
        onLabelsChange={onLabelsChange}
        onChange={async (labelIds) => {
          const currentIds = storyLabels.map((l) => l.id);
          setTasks((current) =>
            current.map((s) =>
              s.id === task.id
                ? { ...s, labels: allLabels.filter((l) => labelIds.includes(l.id)) }
                : s
            )
          );
          await pmTaskService.syncLabels(workspaceId, task.id, currentIds, labelIds);
        }}
      />
    </div>
  );
}

function InlineActionsCell({
  task,
  workspaceId,
  workspaceSlug,
  automatedStateIds,
  onOpenTask,
  setTasks,
}: {
  task: Task;
  workspaceId: string;
  workspaceSlug: string | null;
  automatedStateIds: Set<string>;
  onOpenTask: (task: Task) => void;
  setTasks: React.Dispatch<React.SetStateAction<Task[]>>;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [archiveOpen, setArchiveOpen] = useState(false);
  const [duplicateConfirmOpen, setDuplicateConfirmOpen] = useState(false);
  const [duplicating, setDuplicating] = useState(false);
  const { copy } = useCopyToClipboard();

  const copyLink = (e: React.MouseEvent) => {
    e.stopPropagation();
    const url = buildTaskCopyUrl({
      currentHref: window.location.href,
      displayId: task.display_id,
      taskKey: task.task_key,
      origin: window.location.origin,
      slug: workspaceSlug,
      taskId: task.id,
    });
    copy(url);
  };

  const archiveTask = async () => {
    const { error } = await pmTaskService.remove(workspaceId, task.id);
    if (!error) {
      setTasks((current) => current.filter((s) => s.id !== task.id));
    }
  };

  const duplicateTask = async () => {
    if (duplicating) return;
    setDuplicateConfirmOpen(false);
    setDuplicating(true);
    try {
      const { data, error } = await pmTaskService.duplicate(workspaceId, task.id);
      if (error || !data) {
        toast.error(error ?? 'Failed to duplicate task');
        return;
      }
      setTasks((current) => [data.task, ...current]);
      showTaskDuplicatedToast({
        taskName: data.task.name,
        taskKey: data.task.task_key,
        taskType: data.task.task_type,
        onOpen: () => onOpenTask(data.task),
      });
    } finally {
      setDuplicating(false);
    }
  };

  const requestDuplicateTask = () => {
    if (automatedStateIds.has(task.workflow_state_id)) {
      setDuplicateConfirmOpen(true);
      return;
    }
    void duplicateTask();
  };

  const trigger = (
    <button
      type="button"
      className="rounded-md p-1 text-foreground transition-colors hover:bg-accent cursor-pointer"
      onClick={(event) => {
        event.stopPropagation();
        if (!menuOpen) {
          setMenuOpen(true);
        }
      }}
    >
      <TaskListMoreVerticalIcon className="h-4 w-4" strokeWidth={2.5} />
    </button>
  );

  if (!menuOpen && !archiveOpen && !duplicateConfirmOpen) {
    return <div onClick={(e) => e.stopPropagation()}>{trigger}</div>;
  }

  return (
    <div onClick={(e) => e.stopPropagation()}>
      <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen}>
        <DropdownMenuTrigger asChild>{trigger}</DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={requestDuplicateTask} disabled={duplicating}>
            <Copy01Icon className="mr-2 h-3.5 w-3.5" />
            Duplicate Task
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => onOpenTask(task)}>
            <TaskListOpenTaskIcon className="mr-2 h-3.5 w-3.5" />
            Open Task
          </DropdownMenuItem>
          <DropdownMenuItem onClick={copyLink}>
            <TaskListLinkIcon className="mr-2 h-3.5 w-3.5" />
            Copy Link
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => setArchiveOpen(true)}
            className="text-destructive focus:text-destructive"
          >
            <TaskListArchiveIcon className="mr-2 h-3.5 w-3.5" />
            Archive Task
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      {archiveOpen ? (
        <ConfirmDialog
          open={archiveOpen}
          onOpenChange={setArchiveOpen}
          title="Archive Task"
          description="This task will be hidden from the board and lists. You can restore it later from archived items."
          confirmLabel="Archive"
          onConfirm={archiveTask}
        />
      ) : null}
      {duplicateConfirmOpen ? (
        <ConfirmDialog
          open={duplicateConfirmOpen}
          onOpenChange={setDuplicateConfirmOpen}
          title="Duplicate task and start agent?"
          description="This task is in an auto-run state. Duplicating it will create a copy in the same state and start the assigned agent automatically."
          confirmLabel="Duplicate and start agent"
          onConfirm={duplicateTask}
        />
      ) : null}
    </div>
  );
}
