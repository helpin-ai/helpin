import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
} from '@tanstack/react-table';
import { useVirtualizer } from '@tanstack/react-virtual';
import { Archive, ArrowDown, ArrowUp, ArrowUpDown, BarChart3, CalendarDays, Check, ChevronDown, ChevronRight, CircleCheck, EllipsisVertical, ExternalLink, Link2, Loader2, StickyNote, UserPlus } from 'lucide-react';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Calendar } from '@/components/ui/calendar';
import { format, parseISO } from 'date-fns';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import {
  PriorityIcon,
  SeverityIcon,
  TaskTypeIcon,
  StateTypeIcon,
  PRIORITY_CONFIG,
  SEVERITY_CONFIG,
  STORY_TYPE_CONFIG,
} from '@/lib/pmConstants';
import { UserAvatar } from './UserAvatar';
import type {
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
import type { AssignableMember, WorkspaceTeam } from '@/lib/types';
import { EstimatePicker } from '@/components/pm/EstimatePicker';
import { LabelPicker } from '@/components/pm/LabelPicker';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { buildTaskCopyUrl } from '@/lib/pmTaskLinks';
import { useAgents, useTeamFieldVisibilityForTeam } from '@/hooks/queries';
import { useBoardDisplayStore, type DisplayPropertyKey } from '@/stores/boardDisplayStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { ListDisplayMenu } from '@/components/pm/ListDisplayMenu';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { RecurringTemplateBadge } from '@/components/pm/RecurringTemplateBadge';
import {
  TABLE_CONTAINER,
  TABLE_HEADER,
  TABLE_HEADER_CELL,
  TABLE_HEADER_CELL_SORTABLE,
  TABLE_RESIZE_HANDLE,
  TABLE_ROW,
  TABLE_CELL,
  TABLE_GROUP_ROW,
  TABLE_PINNED_LEFT,
  TABLE_PINNED_RIGHT,
  TABLE_PINNED_HEADER_LEFT,
  TABLE_PINNED_HEADER_RIGHT,
  ROW_HEIGHT,
  GROUP_ROW_HEIGHT,
  dynamicCellStyle,
  pinnedStyle,
} from '@/lib/tableStyles';
import type { BoardFilters } from '@/stores/pmBoardStore';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { getTaskListPinnedOffsets, type TaskListPinnedOffsets } from '@/components/pm/task-detail/taskListPinnedOffsets';
import {
  getVisibleTaskListGroupOptions,
  type TaskListGroupByOption,
} from '@/components/pm/task-detail/taskListGrouping';

const ALL_PRIORITIES: Priority[] = ['urgent', 'high', 'medium', 'low', 'none'];
const ALL_SEVERITIES: Severity[] = ['critical', 'major', 'minor', 'none'];

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
  /** When provided, use these stories instead of fetching internally. */
  externalStories?: Task[];
  onOpenTask: (story: Task) => void;
  groupBy?: TaskListGroupByOption;
  onGroupByChange?: (groupBy: TaskListGroupByOption) => void;
  showToolbar?: boolean;
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

const columnHelper = createColumnHelper<Task>();

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
  externalStories,
  onOpenTask,
  groupBy: controlledGroupBy,
  onGroupByChange,
  showToolbar = true,
}: TaskListViewProps) {
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug ?? null);
  const fieldVis = useTeamFieldVisibilityForTeam(workspaceId, teamId);
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

  const isExternal = externalStories !== undefined;
  const [stories, setStories] = useState<Task[]>(externalStories ?? []);
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
  const pinnedGroupRef = useRef<number | null>(null);
  const [pinnedGroupIdx, setPinnedGroupIdx] = useState<number | null>(null);
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnSizing, setColumnSizing] = useState<ColumnSizingState>({});
  const columnSizingVersion = useMemo(() => JSON.stringify(columnSizing), [columnSizing]);
  const [allLabels, setAllLabels] = useState<Label[]>([]);
  const { data: agents = [] } = useAgents(workspaceId);

  // Per-group pagination state (for workflow_state grouping)
  const [groupHasMore, setGroupHasMore] = useState<Map<string, { hasMore: boolean; total: number; loaded: number }>>(new Map());
  const [groupLoadingId, setGroupLoadingId] = useState<string | null>(null);
  const groupLoadingRef = useRef(false);
  const isPerGroupMode = groupBy === 'workflow_state' && !isExternal;

  useEffect(() => {
    pmLabelService.list(workspaceId).then((r) => { if (r.data) setAllLabels(r.data); });
  }, [workspaceId]);

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
    const map = new Map<string, { name: string; stateType: string }>();
    for (const wf of availableWorkflows) {
      for (const s of wf.states) {
        map.set(s.id, { name: s.name, stateType: s.state_type });
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

  // ── Flat pagination (non-state grouping) ──
  const fetchStoriesFlat = useCallback(async (page = 1, append = false) => {
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
      setStories((prev) => append ? [...prev, ...incoming] : incoming);
      setHasMore(page < res.data.total_pages);
      setCurrentPage(page);
    }
    setLoading(false);
    setLoadingMore(false);
  }, [workspaceId, workflow.workflow.id, filters, teamId, isExternal, includeAssociationData]);

  // ── Per-state pagination (workflow_state grouping) ──
  const fetchStoriesByState = useCallback(async () => {
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
      const allStories: Task[] = [];
      const perGroup = new Map<string, { hasMore: boolean; total: number; loaded: number }>();
      for (const col of res.data) {
        allStories.push(...col.stories);
        perGroup.set(col.state.id, {
          hasMore: col.has_more,
          total: col.task_count,
          loaded: col.stories.length,
        });
      }
      setStories(allStories);
      setGroupHasMore(perGroup);
      setHasMore(false); // disable global load more
    }
    setLoading(false);
  }, [workspaceId, workflow.workflow.id, filters, teamId, isExternal, includeAssociationData]);

  // Load more stories for a specific state group
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
      const newStories = res.data.stories;
      const newTotal = res.data.total;
      const newLoaded = info.loaded + newStories.length;

      // Insert new stories after the last existing story of this state
      setStories((prev) => {
        let lastStateIdx = -1;
        for (let i = 0; i < prev.length; i++) {
          if (prev[i].workflow_state_id === stateId) lastStateIdx = i;
        }
        const result = [...prev];
        if (lastStateIdx >= 0) {
          result.splice(lastStateIdx + 1, 0, ...newStories);
        } else {
          result.push(...newStories);
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
      fetchStoriesFlat(currentPage + 1, true);
    }
  }, [fetchStoriesFlat, currentPage, loadingMore, hasMore]);

  // Fetch on mount and when dependencies change
  useEffect(() => {
    if (isExternal) return;
    if (isPerGroupMode) {
      fetchStoriesByState();
    } else {
      fetchStoriesFlat(1, false);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isExternal, isPerGroupMode, fetchStoriesByState, fetchStoriesFlat]);

  // Sync external stories when they change
  useEffect(() => {
    if (isExternal && externalStories) setStories(externalStories);
  }, [isExternal, externalStories]);

  // Optimistic inline update with rollback on failure
  const updateTaskField = useCallback(
    async (storyId: string, patch: Partial<Task>) => {
      const optimisticPatch: Partial<Task> = { ...patch };
      if (Object.prototype.hasOwnProperty.call(patch, 'owner_member_id')) {
        const ownerMemberId = patch.owner_member_id;
        optimisticPatch.owner_name = ownerMemberId ? ownerNameMap.get(ownerMemberId) : undefined;
      }

      let snapshot: Task[] = [];
      setStories((current) => {
        snapshot = current;
        return current.map((s) => (s.id === storyId ? { ...s, ...optimisticPatch } : s));
      });
      const {
        owner_name: _ownerName,
        epic_name: _epicName,
        labels: _labels,
        ...apiPatch
      } = patch;
      const { error } = await pmTaskService.update(workspaceId, storyId, apiPatch);
      if (error) setStories(snapshot);
    },
    [workspaceId, ownerNameMap],
  );

  // Listen for story events (only for self-fetching mode)
  useEffect(() => {
    if (isExternal) return;
    const handleDeleted = (event: Event) => {
      const detail = (event as CustomEvent)?.detail;
      const storyId = detail?.entity_id as string | undefined;
      if (!storyId) return;
      setStories((current) => current.filter((story) => story.id !== storyId));
    };

    const handleUpdated = async (event: Event) => {
      const detail = (event as CustomEvent)?.detail;
      const storyId = detail?.entity_id as string | undefined;
      if (!storyId) return;

      let shouldPatch = false;
      setStories((current) => {
        shouldPatch = current.some((story) => story.id === storyId);
        return current;
      });
      if (!shouldPatch) return;

      const res = await pmTaskService.get(workspaceId, storyId);
      if (!res.data?.task) return;
      const storyDetail = res.data!;
      const merged: Task = {
        ...storyDetail.task,
        labels: storyDetail.labels,
        epic_name: storyDetail.epic_name ?? storyDetail.task.epic_name,
        sprint_name: storyDetail.sprint_name ?? storyDetail.task.sprint_name,
        owner_name: storyDetail.owner_member
          ? (storyDetail.owner_member.display_name ?? storyDetail.owner_member.email)
          : storyDetail.task.owner_name,
      };
      setStories((current) =>
        current.map((story) => (
          story.id === storyId
            ? {
                ...merged,
                contacts: story.contacts,
                companies: story.companies,
                deals: story.deals,
                support_conversations: story.support_conversations,
              }
            : story
        )),
      );
    };

    window.addEventListener('story-deleted', handleDeleted);
    window.addEventListener('task-updated', handleUpdated);

    return () => {
      window.removeEventListener('story-deleted', handleDeleted);
      window.removeEventListener('task-updated', handleUpdated);
    };
  }, [isExternal, workspaceId]);

  // Table columns
  const tableColumns = useMemo(
    () => [
      columnHelper.accessor('display_id', {
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
            className="flex max-w-full cursor-pointer items-center gap-1.5 text-left text-[13px] hover:text-primary"
            onClick={(e) => {
              e.stopPropagation();
              onOpenTask(info.row.original);
            }}
          >
            {fieldVis.task_type && displayProps.task_type ? (
              <TaskTypeIcon taskType={info.row.original.task_type} className="h-4 w-4 shrink-0" />
            ) : null}
            {info.row.original.recurring_template_id ? (
              <RecurringTemplateBadge compact occurrenceNumber={info.row.original.recurring_occurrence_number} />
            ) : null}
            <span className="min-w-0 truncate">{info.getValue()}</span>
            {info.row.original.assigned_agent_id && (
              <AgentAvatar
                agent={agentById.get(info.row.original.assigned_agent_id) ?? null}
                className="h-4 w-4 border-violet-200/80 dark:border-violet-800"
              />
            )}
          </button>
        ),
      }),
      // Hidden grouping columns (values shown in group headers, not as table columns)
      columnHelper.accessor(
        (row) => stateMap.get(row.workflow_state_id)?.name ?? 'Unknown',
        {
          id: 'stateName',
          header: 'State',
          size: 190,
          cell: (info) => (
            <InlineStateCell
              story={info.row.original}
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
            story={info.row.original}
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
            story={info.row.original}
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
            story={info.row.original}
            onUpdate={updateTaskField}
          />
        ),
      }),
      columnHelper.accessor(
        (row) => {
          const ownerKey = row.owner_member_id;
          return ownerKey ? ownerNameMap.get(ownerKey) ?? 'Unknown' : 'Unassigned';
        },
        {
          id: 'ownerName',
          header: 'Owner',
          size: 200,
          cell: (info) => (
            <InlineOwnerCell
              story={info.row.original}
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
              story={info.row.original}
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
              story={info.row.original}
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
              story={info.row.original}
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
        (row) => (row.task_type ? STORY_TYPE_CONFIG[row.task_type].label : 'Unknown'),
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
            story={info.row.original}
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
            story={info.row.original}
            workspaceId={workspaceId}
            allLabels={allLabels}
            onLabelsChange={setAllLabels}
            setStories={setStories}
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
        size: 44,
        enableGrouping: false,
        enableSorting: false,
        enableResizing: false,
        cell: (info) => (
          <InlineActionsCell
            story={info.row.original}
            workspaceId={workspaceId}
            workspaceSlug={workspaceSlug}
            onOpenTask={onOpenTask}
            setStories={setStories}
          />
        ),
      }),
    ],
    [stateMap, statesByWorkflowId, ownerNameMap, teamMap, epicMap, sprintMap, onOpenTask, workflow.states, assignableMembers, teams, epics, sprints, updateTaskField, allLabels, workspaceId, workspaceSlug, fieldVis.task_type, displayProps.task_type]
  );

  // Team-level disabled keys (for hiding toggles in display menu)
  const teamDisabledKeys = useMemo(() => {
    const keys = new Set<DisplayPropertyKey>();
    if (!fieldVis.priority) keys.add('priority');
    if (!fieldVis.severity) keys.add('severity');
    if (!fieldVis.task_type) keys.add('task_type');
    if (!fieldVis.estimate) keys.add('estimate');
    if (!fieldVis.epic) keys.add('epic');
    if (!fieldVis.sprint) keys.add('sprint');
    if (!fieldVis.due_date) keys.add('due_date');
    if (!fieldVis.labels) keys.add('labels');
    if (teamId) keys.add('team');
    return keys;
  }, [fieldVis, teamId]);

  const visibleGroupOptions = useMemo(
    () =>
      getVisibleTaskListGroupOptions({
        task_type: fieldVis.task_type,
        priority: fieldVis.priority,
        severity: fieldVis.severity,
        epic: fieldVis.epic,
        sprint: fieldVis.sprint,
      }),
    [fieldVis.epic, fieldVis.priority, fieldVis.severity, fieldVis.sprint, fieldVis.task_type],
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
  }, [fieldVis, teamId, displayProps]);

  const grouping: GroupingState = useMemo(() => {
    const colId = GROUP_COLUMN_MAP[groupBy];
    return colId ? [colId] : [];
  }, [groupBy]);

  const table = useReactTable({
    data: stories,
    columns: tableColumns,
    state: {
      grouping,
      expanded,
      columnVisibility,
      sorting,
      columnSizing,
    },
    onExpandedChange: setExpanded,
    onSortingChange: setSorting,
    onColumnSizingChange: setColumnSizing,
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
  const typeIconColumn = table.getAllLeafColumns().find((column) => column.id === 'typeIcon');
  const pinnedOffsets = useMemo(
    () =>
      getTaskListPinnedOffsets({
        displayIdWidth: table.getColumn('displayId')?.getSize() ?? 90,
        typeIconWidth: typeIconColumn?.getSize() ?? 40,
        showTypeIcon: typeIconColumn?.getIsVisible() ?? false,
      }),
    [columnSizing, columnVisibility, table, typeIconColumn],
  );
  const pinnedGroupRow = pinnedGroupIdx !== null ? (rows[pinnedGroupIdx] as Row<Task> | undefined) : undefined;

  const estimateSize = useCallback(
    (index: number) => rows[index]?.getIsGrouped() ? GROUP_ROW_HEIGHT : ROW_HEIGHT,
    [rows],
  );

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize,
    overscan: 20,
  });

  // Compute total story count (including unloaded) for per-group mode
  const displayTaskCount = isPerGroupMode && groupHasMore.size > 0
    ? Array.from(groupHasMore.values()).reduce((sum, info) => sum + info.total, 0)
    : stories.length;

  const getGroupTotalCount = (groupRow: Row<Task>) => {
    if (!isPerGroupMode || !groupRow.subRows[0]) return undefined;
    return groupHasMore.get(groupRow.subRows[0].original.workflow_state_id)?.total;
  };

  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        Loading stories...
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2">
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
          <div className="ml-auto">
            <ListDisplayMenu disabledKeys={teamDisabledKeys} />
          </div>
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

          // Track pinned group header
          const vItems = virtualizer.getVirtualItems();
          let newPinnedIdx: number | null = null;
          if (scrollTop > 10) {
            for (const vItem of vItems) {
              if (vItem.start > scrollTop) break;
              if (rows[vItem.index]?.getIsGrouped()) newPinnedIdx = vItem.index;
            }
            if (newPinnedIdx === null && vItems.length > 0) {
              for (let i = vItems[0].index - 1; i >= 0; i--) {
                if (rows[i]?.getIsGrouped()) { newPinnedIdx = i; break; }
              }
            }
          }
          if (newPinnedIdx !== pinnedGroupRef.current) {
            pinnedGroupRef.current = newPinnedIdx;
            setPinnedGroupIdx(newPinnedIdx);
          }
        }}
      >
        <div className="min-w-fit">
        {/* Header */}
        <div ref={headerRef} className={TABLE_HEADER}>
          {table.getHeaderGroups().map((headerGroup) => (
            <div key={headerGroup.id} className="flex items-center">
              {headerGroup.headers.map((header) => {
                if (header.column.getIsGrouped()) return null;
                const defSize = header.column.columnDef.size ?? 150;
                const runtimeSize = header.getSize();
                if (defSize === 0 && runtimeSize === 0) return null;
                const isResized = !!columnSizing[header.column.id];
                const canSort = header.column.getCanSort();
                const sorted = header.column.getIsSorted();
                const colId = header.column.id;
                const pinnedClass = colId === 'displayId' || colId === 'typeIcon' || colId === 'name'
                  ? TABLE_PINNED_HEADER_LEFT
                  : colId === 'actions' ? TABLE_PINNED_HEADER_RIGHT : '';
                const pinnedSt = colId === 'displayId' ? pinnedStyle('left', pinnedOffsets.displayId)
                  : colId === 'typeIcon' ? pinnedStyle('left', pinnedOffsets.typeIcon)
                  : colId === 'name' ? pinnedStyle('left', pinnedOffsets.name)
                  : colId === 'actions' ? pinnedStyle('right', 0) : {};
                return (
                  <div
                    key={header.id}
                    className={`${TABLE_HEADER_CELL} ${canSort ? TABLE_HEADER_CELL_SORTABLE : ''} ${pinnedClass}`}
                    style={{ ...dynamicCellStyle(defSize, runtimeSize, isResized, 400), ...pinnedSt }}
                    onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                  >
                    <div className="flex items-center gap-1 overflow-hidden whitespace-nowrap">
                      {header.isPlaceholder
                        ? null
                        : flexRender(header.column.columnDef.header, header.getContext())}
                      {canSort && (
                        <span className="ml-auto shrink-0">
                          {sorted === 'asc' ? (
                            <ArrowUp className="h-3 w-3 text-foreground/80 stroke-[2.5]" />
                          ) : sorted === 'desc' ? (
                            <ArrowDown className="h-3 w-3 text-foreground/80 stroke-[2.5]" />
                          ) : (
                            <ArrowUpDown className="h-3 w-3 text-muted-foreground stroke-[2]" />
                          )}
                        </span>
                      )}
                    </div>
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
          ))}
        </div>

        {/* Virtualized body */}
          <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative', width: '100%' }}>
            {/* Pinned sticky group header — offset below the table header */}
            {pinnedGroupRow && (
              <div className="sticky z-[5]" style={{ top: headerRef.current?.offsetHeight ?? 0, height: 0, overflow: 'visible' }}>
                <div className="bg-background border-b border-border/50">
                  <MemoGroupHeaderRow row={pinnedGroupRow} groupBy={groupBy} stateMap={stateMap} totalTaskCount={getGroupTotalCount(pinnedGroupRow)} />
                </div>
              </div>
            )}
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
                  ref={virtualizer.measureElement}
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  {isGrouped ? (
                    <MemoGroupHeaderRow row={row} groupBy={groupBy} stateMap={stateMap} totalTaskCount={getGroupTotalCount(row)} />
                  ) : (
                    <>
                      <MemoDataRow
                        row={row}
                        onOpenTask={onOpenTask}
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
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Loading more stories...
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

const MemoGroupHeaderRow = memo(function GroupHeaderRow({
  row,
  groupBy,
  stateMap,
  totalTaskCount,
}: {
  row: Row<Task>;
  groupBy: TaskListGroupByOption;
  stateMap: Map<string, { name: string; stateType: string }>;
  totalTaskCount?: number;
}) {
  const subRows = row.subRows;
  const storyCount = totalTaskCount ?? subRows.length;
  const totalPoints = subRows.reduce((sum, r) => sum + (r.original.estimate ?? 0), 0);
  const completedPoints = subRows.reduce((sum, r) => {
    const stateInfo = stateMap.get(r.original.workflow_state_id);
    if (stateInfo?.stateType === 'done') return sum + (r.original.estimate ?? 0);
    return sum;
  }, 0);

  const stateType =
    groupBy === 'workflow_state' && subRows[0]
      ? (stateMap.get(subRows[0].original.workflow_state_id)?.stateType as StateType | undefined)
      : undefined;

  return (
    <button
      className={`${TABLE_GROUP_ROW} w-full text-left text-xs`}
      onClick={row.getToggleExpandedHandler()}
    >
      {row.getIsExpanded() ? (
        <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
      ) : (
        <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
      )}
      {stateType && <StateTypeIcon stateType={stateType} className="h-4 w-4" />}
      <span>{String(row.groupingValue)}</span>
      <span className="flex items-center gap-3 ml-1 font-normal text-muted-foreground">
        <span className="flex items-center gap-1" title="Stories">
          <StickyNote className="h-3 w-3" /> {storyCount}
        </span>
        <span className="flex items-center gap-1" title="Total Points">
          <BarChart3 className="h-3 w-3" /> {totalPoints}
        </span>
        <span className="flex items-center gap-1" title="Completed Points">
          <CircleCheck className="h-3 w-3" /> {completedPoints}
        </span>
      </span>
    </button>
  );
});

const MemoDataRow = memo(function DataRow({
  row,
  onOpenTask,
  columnSizingVersion,
  pinnedOffsets,
}: {
  row: Row<Task>;
  onOpenTask: (story: Task) => void;
  columnSizingVersion: string;
  pinnedOffsets: TaskListPinnedOffsets;
}) {
  return (
    <div
      data-column-sizing={columnSizingVersion}
      className={`group/row ${TABLE_ROW} cursor-pointer`}
      onClick={() => onOpenTask(row.original)}
    >
      {row.getVisibleCells().map((cell) => {
        // Skip the grouped column entirely — header does the same, keeping alignment
        if (cell.column.getIsGrouped()) return null;
        const defSize = cell.column.columnDef.size ?? 150;
        const runtimeSize = cell.column.getSize();
        const isResized = runtimeSize !== defSize;
        if (defSize === 0 && runtimeSize === 0) return null;
        const colId = cell.column.id;
        const pinnedClass = colId === 'displayId' || colId === 'typeIcon' || colId === 'name'
          ? TABLE_PINNED_LEFT
          : colId === 'actions' ? TABLE_PINNED_RIGHT : '';
        const pinnedSt = colId === 'displayId' ? pinnedStyle('left', pinnedOffsets.displayId)
          : colId === 'typeIcon' ? pinnedStyle('left', pinnedOffsets.typeIcon)
          : colId === 'name' ? pinnedStyle('left', pinnedOffsets.name)
          : colId === 'actions' ? pinnedStyle('right', 0) : {};
        return (
          <div
            key={cell.id}
            className={`${TABLE_CELL} overflow-hidden ${pinnedClass}`}
            style={{ ...dynamicCellStyle(defSize, runtimeSize, isResized, 400), ...pinnedSt }}
          >
            {flexRender(cell.column.columnDef.cell, cell.getContext())}
          </div>
        );
      })}
    </div>
  );
});

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
      <Loader2 className="h-3 w-3 animate-spin" />
      Loading...
    </div>
  ) : null;
}

// ── Inline editable cells ──────────────────────────────────────────

function InlinePriorityCell({
  story,
  onUpdate,
}: {
  story: Task;
  onUpdate: (storyId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const p = story.priority;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          <PriorityIcon priority={p} className="h-3.5 w-3.5" />
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
                        if (pri !== p) onUpdate(story.id, { priority: pri });
                        setOpen(false);
                      }}
                      className="flex items-center gap-2 text-xs"
                    >
                      <PriorityIcon priority={pri} className="h-3.5 w-3.5" />
                      <span>{cfg.label}</span>
                      {p === pri && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
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
  story,
  states,
  stateMap,
  onUpdate,
}: {
  story: Task;
  states: WorkflowWithStates['states'];
  stateMap: Map<string, { name: string; stateType: string }>;
  onUpdate: (storyId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const current = stateMap.get(story.workflow_state_id);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          {current && (
            <StateTypeIcon stateType={current.stateType as 'backlog' | 'unstarted' | 'started' | 'done'} className="h-3.5 w-3.5" />
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
                      if (s.id !== story.workflow_state_id)
                        onUpdate(story.id, { workflow_state_id: s.id });
                      setOpen(false);
                    }}
                    className="flex items-center gap-2 text-xs"
                  >
                    <StateTypeIcon stateType={s.state_type} className="h-3.5 w-3.5" />
                    <span>{s.name}</span>
                    {story.workflow_state_id === s.id && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
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
  story,
  assignableMembers,
  ownerNameMap,
  onUpdate,
}: {
  story: Task;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  onUpdate: (storyId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const ownerKey = story.owner_member_id;
  const ownerName = ownerKey ? ownerNameMap.get(ownerKey) ?? 'Unknown' : null;

  return (
    <MemberPickerPopover
      value={story.owner_member_id || '__none__'}
      members={assignableMembers}
      noneLabel="Unassigned"
      onChange={(value) => {
        void onUpdate(story.id, { owner_member_id: value === '__none__' ? '' : value });
      }}
      renderTrigger={() => {
        const selectedMember = findAssignableMember(assignableMembers, story.owner_member_id);
        return selectedMember ? (
          <>
            <UserAvatar
              name={selectedMember.display_name || selectedMember.email}
              avatarUrl={selectedMember.avatar_url}
              className="h-4 w-4"
              fallbackClassName="text-[7px]"
            />
            <span className="truncate">{ownerName}</span>
          </>
        ) : (
          <>
            <UserPlus className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="text-muted-foreground">Assign</span>
          </>
        );
      }}
    />
  );
}

function InlineSeverityCell({
  story,
  onUpdate,
}: {
  story: Task;
  onUpdate: (storyId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const s = story.severity;

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
              <SeverityIcon severity={s} className="h-3.5 w-3.5" />
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
                        if (sev !== s) onUpdate(story.id, { severity: sev });
                        setOpen(false);
                      }}
                      className="flex items-center gap-2 text-xs"
                    >
                      <SeverityIcon severity={sev} className="h-3.5 w-3.5" />
                      <span>{cfg.label}</span>
                      {s === sev && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
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
  story,
  onUpdate,
}: {
  story: Task;
  onUpdate: (storyId: string, patch: Partial<Task>) => Promise<void>;
}) {
  return (
    <div onClick={(e) => e.stopPropagation()}>
      <EstimatePicker
        value={story.estimate?.toString() ?? ''}
        teamId={story.team_id}
        onChange={(_displayValue, apiValue) => {
          const next = apiValue ?? null;
          if (next !== story.estimate) {
            onUpdate(story.id, { estimate: next as number });
          }
        }}
      />
    </div>
  );
}

function InlineTeamCell({
  story,
  teams,
  teamMap,
  onUpdate,
}: {
  story: Task;
  teams: WorkspaceTeam[];
  teamMap: Map<string, string>;
  onUpdate: (storyId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const teamName = story.team_id ? teamMap.get(story.team_id) ?? 'Unknown' : null;

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
                      const newTeamId = story.team_id === t.id ? undefined : t.id;
                      onUpdate(story.id, { team_id: newTeamId });
                      setOpen(false);
                    }}
                    className="flex items-center gap-2 text-xs"
                  >
                    <span className="truncate">{t.name}</span>
                    {story.team_id === t.id && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
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
  story,
  epics,
  epicMap,
  onUpdate,
}: {
  story: Task;
  epics: EpicWithStats[];
  epicMap: Map<string, string>;
  onUpdate: (storyId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const epicName = story.epic_id ? epicMap.get(story.epic_id) ?? 'Unknown' : null;

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
                      const newEpicId = story.epic_id === e.epic.id ? undefined : e.epic.id;
                      onUpdate(story.id, { epic_id: newEpicId });
                      setOpen(false);
                    }}
                    className="flex items-center gap-2 text-xs"
                  >
                    <span className="truncate">{e.epic.name}</span>
                    {story.epic_id === e.epic.id && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
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
  story,
  sprints,
  sprintMap,
  onUpdate,
}: {
  story: Task;
  sprints: SprintWithStats[];
  sprintMap: Map<string, string>;
  onUpdate: (storyId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const sprintName = story.sprint_id ? sprintMap.get(story.sprint_id) ?? 'Unknown' : null;

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
                {sprints.map((sp) => (
                  <CommandItem
                    key={sp.sprint.id}
                    value={sp.sprint.name}
                    onSelect={() => {
                      const newSprintId = story.sprint_id === sp.sprint.id ? undefined : sp.sprint.id;
                      onUpdate(story.id, { sprint_id: newSprintId });
                      setOpen(false);
                    }}
                    className="flex items-center gap-2 text-xs"
                  >
                    <span className="truncate">{sp.sprint.name}</span>
                    {story.sprint_id === sp.sprint.id && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
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
  story,
  onUpdate,
}: {
  story: Task;
  onUpdate: (storyId: string, patch: Partial<Task>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const v = story.deadline;
  const selected = v ? parseISO(v) : undefined;
  const isOverdue = selected ? selected < new Date() : false;

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
              <CalendarDays className="h-3.5 w-3.5 text-muted-foreground" />
              <span className={isOverdue ? 'text-red-500' : 'text-muted-foreground'}>
                {format(selected, 'MMM d, yyyy')}
              </span>
            </>
          ) : (
            <>
              <CalendarDays className="h-3.5 w-3.5 text-muted-foreground" />
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
              onUpdate(story.id, { deadline: date ? format(date, 'yyyy-MM-dd') : undefined });
              setOpen(false);
            }}
          />
        </PopoverContent>
      )}
    </Popover>
  );
}

function InlineLabelsCell({
  story,
  workspaceId,
  allLabels,
  onLabelsChange,
  setStories,
}: {
  story: Task;
  workspaceId: string;
  allLabels: Label[];
  onLabelsChange: (labels: Label[]) => void;
  setStories: React.Dispatch<React.SetStateAction<Task[]>>;
}) {
  const storyLabels = story.labels ?? [];
  return (
    <div onClick={(e) => e.stopPropagation()}>
      <LabelPicker
        workspaceId={workspaceId}
        teamId={story.team_id || undefined}
        labels={allLabels}
        selectedLabelIds={storyLabels.map((l) => l.id)}
        onLabelsChange={onLabelsChange}
        onChange={async (labelIds) => {
          const currentIds = storyLabels.map((l) => l.id);
          setStories((current) =>
            current.map((s) =>
              s.id === story.id
                ? { ...s, labels: allLabels.filter((l) => labelIds.includes(l.id)) }
                : s
            )
          );
          await pmTaskService.syncLabels(workspaceId, story.id, currentIds, labelIds);
        }}
      />
    </div>
  );
}

function InlineActionsCell({
  story,
  workspaceId,
  workspaceSlug,
  onOpenTask,
  setStories,
}: {
  story: Task;
  workspaceId: string;
  workspaceSlug: string | null;
  onOpenTask: (story: Task) => void;
  setStories: React.Dispatch<React.SetStateAction<Task[]>>;
}) {
  const [archiveOpen, setArchiveOpen] = useState(false);
  const { copy } = useCopyToClipboard();

  const copyLink = (e: React.MouseEvent) => {
    e.stopPropagation();
    const url = buildTaskCopyUrl({
      currentHref: window.location.href,
      displayId: story.display_id,
      origin: window.location.origin,
      slug: workspaceSlug,
      taskId: story.id,
    });
    copy(url);
  };

  const archiveTask = async () => {
    const { error } = await pmTaskService.remove(workspaceId, story.id);
    if (!error) {
      setStories((current) => current.filter((s) => s.id !== story.id));
    }
  };

  return (
    <div onClick={(e) => e.stopPropagation()}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            className="rounded-md p-1 text-muted-foreground opacity-0 transition-opacity hover:bg-accent hover:text-foreground group-hover/row:opacity-100 data-[state=open]:opacity-100 cursor-pointer"
          >
            <EllipsisVertical className="h-4 w-4" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={() => onOpenTask(story)}>
            <ExternalLink className="mr-2 h-3.5 w-3.5" />
            Open Task
          </DropdownMenuItem>
          <DropdownMenuItem onClick={copyLink}>
            <Link2 className="mr-2 h-3.5 w-3.5" />
            Copy Link
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => setArchiveOpen(true)}
            className="text-destructive focus:text-destructive"
          >
            <Archive className="mr-2 h-3.5 w-3.5" />
            Archive Task
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <ConfirmDialog
        open={archiveOpen}
        onOpenChange={setArchiveOpen}
        title="Archive Task"
        description="This task will be hidden from the board and lists. You can restore it later from archived items."
        confirmLabel="Archive"
        onConfirm={archiveTask}
      />
    </div>
  );
}
