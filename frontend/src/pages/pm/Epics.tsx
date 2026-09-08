import { memo, useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type ColumnSizingState,
  type OnChangeFn,
  type Row,
  type SortingState,
  type VisibilityState,
} from '@tanstack/react-table';
import { useVirtualizer } from '@tanstack/react-virtual';
import { useQueryClient } from '@tanstack/react-query';
import { StickyPinnedGroupOverlay } from '@/components/pm/StickyPinnedGroupOverlay';
import { EpicFilterBar } from '@/pages/pm/EpicFilterBar';
import { OwnerAvatarFilterRow } from '@/components/pm/OwnerAvatarFilterRow';
import { EpicColorControl } from '@/components/pm/EpicColorControl';
import { format, parseISO } from 'date-fns';
import { useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import {
  Tick01Icon,
  ArrowDown02Icon,
  Calendar03Icon,
  ArrowDown01Icon,
  ArrowRight01Icon,
  PlusSignIcon,
  Target01Icon,
  ChartIncreaseIcon,
  ArrowUp02Icon,
  ArrowUpDownIcon,
  UserIcon,
  UserAdd01Icon,
  Layers01Icon,
  MinusSignIcon,
  Sun01Icon,
  Tag01Icon,
  WorkflowSquare01Icon,
  Activity01Icon,
} from '@/lib/icons';
import { StateTypeIcon } from '@/lib/pmIcons';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { LabelPicker } from '@/components/pm/LabelPicker';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { InlineEpicDateControl, InlineEpicObjectivesControl } from '@/components/pm/InlineEpicPlanningFields';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { QuietPageHeader, QuietPrimaryAction } from '@/components/design-system/quiet';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useEpicStates, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useEpics } from '@/hooks/queries/useEpics';
import { useLabels } from '@/hooks/queries/useLabels';
import { useObjectives } from '@/hooks/queries/useObjectives';
import { queryKeys } from '@/lib/queryKeys';
import type { EpicWithStats, EpicHealth, EpicWorkflowState, Label, Objective, StateType, UpdateEpicRequest } from '@/lib/pmTypes';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { STATE_TYPE_ICON_CONFIG } from '@/lib/pmConstants';
import type { AssignableMember, WorkspaceTeam } from '@/lib/types';
import {
  GROUP_ROW_HEIGHT,
  ROW_HEIGHT,
  TABLE_CELL,
  TABLE_NAME_TEXT,
  TABLE_CONTAINER,
  TABLE_GROUP_ROW,
  TABLE_GROUP_ROW_INNER,
  TABLE_HEADER,
  TABLE_HEADER_CELL,
  TABLE_HEADER_CELL_SORTABLE,
  TABLE_RESIZE_HANDLE,
  TABLE_ROW,
  TABLE_PINNED_LEFT_NAME,
  TABLE_PINNED_HEADER_LEFT_NAME,
  dynamicCellStyle,
  pinnedStyle,
  resolveColumnRuntimeSize,
  virtualRowStyle } from '@/lib/tableStyles';

const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
  no_health: { label: 'No health', color: 'text-muted-foreground' },
  on_track: { label: 'On track', color: 'text-green-600' },
  at_risk: { label: 'At risk', color: 'text-yellow-600' },
  off_track: { label: 'Off track', color: 'text-red-600' },
};

const ARCHIVED_STATE_VALUE = '__archived__';
const GROUP_HEADER_REPEAT_HEIGHT = 30;
const EMPTY_EPICS: EpicWithStats[] = [];
const EMPTY_LABELS: Label[] = [];

const FILTER_CATEGORY_ICONS: Partial<Record<EpicFilterKey, React.ComponentType<{ className?: string }>>> = {
  state: WorkflowSquare01Icon,
  health: Activity01Icon,
  owner: UserIcon,
  label: Tag01Icon,
  objective: Target01Icon,
};

interface FilterOptionVisualContext {
  epicStateMap: Map<string, EpicWorkflowState>;
  assignableMembers: AssignableMember[];
  allLabels: Label[];
}

interface FilterOptionVisual {
  leading?: React.ReactNode;
  labelClassName?: string;
}

function getFilterOptionVisual(
  key: EpicFilterKey,
  value: string,
  ctx: FilterOptionVisualContext): FilterOptionVisual {
  if (key === 'state') {
    const state = ctx.epicStateMap.get(value);
    if (!state) return {};
    return {
      leading: <StateTypeIcon stateType={state.state_type} className="h-3.5 w-3.5" />,
    };
  }
  if (key === 'health') {
    const config = healthConfig[value as EpicHealth];
    if (!config) return {};
    return { labelClassName: config.color };
  }
  if (key === 'owner') {
    const member = ctx.assignableMembers.find((candidate) => candidate.id === value);
    if (!member) return {};
    return {
      leading: <UserAvatar
          name={member.display_name || member.email}
          avatarUrl={member.avatar_url}
          avatarStyle={member.avatar_style}
          avatarSeed={member.avatar_seed}
          avatarBackgroundMode={member.avatar_background_mode}
          avatarBackgroundColor={member.avatar_background_color}
          className="h-4 w-4"
          fallbackClassName="text-[8px]"
        />,
    };
  }
  if (key === 'label') {
    const label = ctx.allLabels.find((candidate) => candidate.id === value);
    const color = label?.color ?? '#94a3b8';
    return {
      leading: <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: color }} />,
    };
  }
  if (key === 'objective') {
    return { leading: <Target01Icon className="h-3.5 w-3.5 text-muted-foreground" />,
    };
  }
  return {};
}

const ALL_PROPERTIES = [
  { key: 'state', label: 'State' },
  { key: 'health', label: 'Health' },
  { key: 'progress', label: 'Progress' },
  { key: 'tasks', label: 'Tasks' },
  { key: 'points', label: 'Points' },
  { key: 'owner', label: 'Owner' },
  { key: 'objective', label: 'Objective' },
  { key: 'target_date', label: 'Target date' },
  { key: 'team', label: 'Team' },
  { key: 'start_date', label: 'Start date' },
  { key: 'labels', label: 'Labels' },
  { key: 'created', label: 'Created' },
  { key: 'updated', label: 'Updated' },
];

const DEFAULT_VISIBLE = [
  'state',
  'health',
  'progress',
  'tasks',
  'points',
  'owner',
  'target_date',
  'updated'];

const columnHelper = createColumnHelper<EpicWithStats>();
const ALL_HEALTH_OPTIONS: EpicHealth[] = ['no_health', 'on_track', 'at_risk', 'off_track'];
const EPIC_GROUP_BY_OPTIONS = [
  { value: 'state', label: 'State' },
  { value: 'health', label: 'Health' },
  { value: 'team', label: 'Team' },
  { value: 'owner', label: 'Owner' },
  { value: 'objective', label: 'Objective' },
  { value: 'target_date', label: 'Target date' },
  { value: 'start_date', label: 'Start date' },
  { value: 'none', label: 'None' },
] as const;

type EpicGroupBy = (typeof EPIC_GROUP_BY_OPTIONS)[number]['value'];
type EpicFilterKey = 'state'
  | 'health'
  | 'owner'
  | 'label'
  | 'objective'
  | 'has_target_date'
  | 'has_start_date';
type EpicFilterState = Partial<Record<EpicFilterKey, string[]>>;

interface EpicFilterOption {
  value: string;
  label: string;
}

interface EpicFilterDefinition {
  key: EpicFilterKey;
  label: string;
  options: EpicFilterOption[];
}

interface EpicGroup {
  key: string;
  label: string;
  entries: EpicWithStats[];
}

function epicTaskCount(entry: EpicWithStats) {
  return entry.stats.task_count ?? entry.stats.task_count ?? 0;
}

function epicDoneTaskCount(entry: EpicWithStats) {
  return entry.stats.done_task_count ?? entry.stats.done_task_count ?? 0;
}

function applyEpicPatch(entry: EpicWithStats, patch: UpdateEpicRequest, allLabels: Label[]): EpicWithStats {
  const nextEpic = { ...entry.epic };

  if (Object.prototype.hasOwnProperty.call(patch, 'epic_state_id')) {
    nextEpic.epic_state_id = patch.epic_state_id;
  }
  if (Object.prototype.hasOwnProperty.call(patch, 'owner_member_id')) {
    nextEpic.owner_member_id = patch.owner_member_id || undefined;
  }
  if (Object.prototype.hasOwnProperty.call(patch, 'team_id')) {
    nextEpic.team_id = patch.team_id;
  }
  if (Object.prototype.hasOwnProperty.call(patch, 'planned_start_date')) {
    nextEpic.planned_start_date = patch.planned_start_date;
  }
  if (Object.prototype.hasOwnProperty.call(patch, 'deadline')) {
    nextEpic.deadline = patch.deadline;
  }
  if (Object.prototype.hasOwnProperty.call(patch, 'health')) {
    nextEpic.health = patch.health ?? entry.epic.health;
  }
  if (patch.color !== undefined) {
    nextEpic.color = patch.color;
  }

  const nextEntry: EpicWithStats = {
    ...entry,
    epic: nextEpic,
  };

  if (Object.prototype.hasOwnProperty.call(patch, 'label_ids')) {
    nextEntry.labels = allLabels.filter((label) => patch.label_ids?.includes(label.id));
  }

  return nextEntry;
}

function buildObjectiveRefs(objectives: Objective[], objectiveIds: string[]) {
  const selected = new Set(objectiveIds);
  return objectives
    .filter((objective) => selected.has(objective.id))
    .map((objective) => ({ id: objective.id, name: objective.name }));
}

function matchesSelectedValue(actual: string | undefined, selected: string[] | undefined) {
  if (!selected || selected.length === 0) return true;
  return selected.includes(actual ?? '__none__');
}

function matchesPresence(hasValue: boolean, selected: string[] | undefined) {
  if (!selected || selected.length === 0 || selected.length === 2) return true;
  const expected = hasValue ? 'yes' : 'no';
  return selected.includes(expected);
}

function epicMatchesFilters(entry: EpicWithStats, filters: EpicFilterState) {
  const labelIds = new Set((entry.labels ?? []).map((label) => label.id));
  const objectiveIds = new Set((entry.objectives ?? []).map((objective) => objective.id));

  // Skip state matching when viewing archived (already filtered server-side)
  const stateFilter = filters.state?.filter((v) => v !== '__archived__');
  if (stateFilter && stateFilter.length > 0 && !matchesSelectedValue(entry.epic.epic_state_id, stateFilter)) return false;
  if (!matchesSelectedValue(entry.epic.health, filters.health)) return false;
  if (!matchesSelectedValue(entry.epic.owner_member_id, filters.owner)) return false;
  if (filters.label?.length && !filters.label.some((id) => labelIds.has(id))) return false;
  if (filters.objective?.length && !filters.objective.some((id) => objectiveIds.has(id))) return false;
  if (!matchesPresence(!!entry.epic.deadline, filters.has_target_date)) return false;
  if (!matchesPresence(!!entry.epic.planned_start_date, filters.has_start_date)) return false;

  return true;
}

function sortEntries(entries: EpicWithStats[]) {
  return [...entries].sort((a, b) => {
    const updatedCmp = b.epic.updated_at.localeCompare(a.epic.updated_at);
    if (updatedCmp !== 0) return updatedCmp;
    return a.epic.name.localeCompare(b.epic.name);
  });
}

function epicCompletionPct(entry: EpicWithStats) {
  const totalTasks = epicTaskCount(entry);
  if (totalTasks === 0) return 0;
  return Math.round((epicDoneTaskCount(entry) / totalTasks) * 100);
}

function getEpicSortValue(
  entry: EpicWithStats,
  columnId: string,
  epicStates: EpicWorkflowState[],
  ownerNameMap: Map<string, string>,
  teamMap: Map<string, string>) {
  switch (columnId) {
    case 'name':
      return entry.epic.name;
    case 'state': {
      const state = epicStates.find((candidate) => candidate.id === entry.epic.epic_state_id);
      return state?.position ?? Number.MAX_SAFE_INTEGER;
    }
    case 'health':
      return ALL_HEALTH_OPTIONS.indexOf(entry.epic.health);
    case 'progress':
      return epicCompletionPct(entry);
    case 'tasks':
      return epicTaskCount(entry);
    case 'points':
      return entry.stats.total_points;
    case 'owner':
      return entry.epic.owner_member_id ? (ownerNameMap.get(entry.epic.owner_member_id) ?? '') : '';
    case 'objective':
      return (entry.objectives ?? []).map((objective) => objective.name).join(', ');
    case 'target_date':
      return entry.epic.deadline ?? '';
    case 'team':
      return entry.epic.team_id ? (teamMap.get(entry.epic.team_id) ?? '') : '';
    case 'start_date':
      return entry.epic.planned_start_date ?? '';
    case 'labels':
      return (entry.labels ?? []).map((label) => label.name).join(', ');
    case 'created':
      return entry.epic.created_at;
    case 'updated':
      return entry.epic.updated_at;
    default:
      return '';
  }
}

function compareEpicSortValues(a: string | number, b: string | number) {
  if (typeof a === 'number' && typeof b === 'number') {
    return a - b;
  }
  return String(a).localeCompare(String(b), undefined, { sensitivity: 'base', numeric: true,
  });
}

function sortEntriesForTable(
  entries: EpicWithStats[],
  sorting: SortingState,
  epicStates: EpicWorkflowState[],
  ownerNameMap: Map<string, string>,
  teamMap: Map<string, string>) {
  if (sorting.length === 0) return sortEntries(entries);

  return [...entries].sort((a, b) => {
    for (const sort of sorting) {
      const aValue = getEpicSortValue(a, sort.id, epicStates, ownerNameMap, teamMap);
      const bValue = getEpicSortValue(b, sort.id, epicStates, ownerNameMap, teamMap);
      const result = compareEpicSortValues(aValue, bValue);
      if (result !== 0) return sort.desc ? -result : result;
    }
    return a.epic.name.localeCompare(b.epic.name);
  });
}

function getDateGroupLabel(value: string | undefined) {
  if (!value) return 'No date';
  return format(parseISO(value), 'MMM d, yyyy');
}

function buildEpicGroups(
  entries: EpicWithStats[],
  groupBy: EpicGroupBy,
  filters: EpicFilterState,
  epicStates: EpicWorkflowState[],
  ownerNameMap: Map<string, string>,
  teamMap: Map<string, string>,
  objectiveNameMap: Map<string, string>,
  preserveEntryOrder = false): EpicGroup[] {
  if (groupBy === 'none') {
    return [{ key: 'all', label: 'All epics', entries }];
  }

  const grouped = new Map<string, EpicGroup>();
  const addToGroup = (key: string, label: string, entry: EpicWithStats) => {
    const existing = grouped.get(key);
    if (existing) {
      existing.entries.push(entry);
      return;
    }
    grouped.set(key, { key, label, entries: [entry] });
  };

  for (const entry of entries) {
    switch (groupBy) {
      case 'state': {
        const state = epicStates.find((candidate) => candidate.id === entry.epic.epic_state_id);
        addToGroup(entry.epic.epic_state_id ?? '__none__', state?.name ?? 'No state', entry);
        break;
      }
      case 'health':
        addToGroup(entry.epic.health, healthConfig[entry.epic.health]?.label ?? 'No health', entry);
        break;
      case 'team':
        addToGroup(entry.epic.team_id ?? '__none__', entry.epic.team_id ? (teamMap.get(entry.epic.team_id) ?? 'Unknown team') : 'No team', entry);
        break;
      case 'owner':
        addToGroup(entry.epic.owner_member_id ?? '__none__', entry.epic.owner_member_id ? (ownerNameMap.get(entry.epic.owner_member_id) ?? 'Unknown owner') : 'Unassigned', entry);
        break;
      case 'target_date':
        addToGroup(entry.epic.deadline ?? '__none__', getDateGroupLabel(entry.epic.deadline), entry);
        break;
      case 'start_date':
        addToGroup(entry.epic.planned_start_date ?? '__none__', getDateGroupLabel(entry.epic.planned_start_date), entry);
        break;
      case 'objective': {
        const scopedObjectiveIds = filters.objective?.length
          ? (entry.objectives ?? []).filter((objective) => filters.objective?.includes(objective.id)).map((objective) => objective.id)
          : (entry.objectives ?? []).map((objective) => objective.id);
        if (scopedObjectiveIds.length === 0) {
          addToGroup('__none__', 'No objective', entry);
          break;
        }
        for (const objectiveId of scopedObjectiveIds) {
          addToGroup(objectiveId, objectiveNameMap.get(objectiveId) ?? 'Unknown objective', entry);
        }
        break;
      }
    }
  }

  const groups = Array.from(grouped.values()).map((group) => ({
    ...group,
    entries: preserveEntryOrder ? group.entries : sortEntries(group.entries),
  }));

  return groups.sort((a, b) => {
    if (groupBy === 'state') {
      const aPos = a.key === '__none__' ? Number.MAX_SAFE_INTEGER : (epicStates.find((state) => state.id === a.key)?.position ?? Number.MAX_SAFE_INTEGER);
      const bPos = b.key === '__none__' ? Number.MAX_SAFE_INTEGER : (epicStates.find((state) => state.id === b.key)?.position ?? Number.MAX_SAFE_INTEGER);
      if (aPos !== bPos) return aPos - bPos;
    }
    if (groupBy === 'health') {
      const aIdx = ALL_HEALTH_OPTIONS.indexOf(a.key as EpicHealth);
      const bIdx = ALL_HEALTH_OPTIONS.indexOf(b.key as EpicHealth);
      if (aIdx !== bIdx) return aIdx - bIdx;
    }
    if (a.key === '__none__') return 1;
    if (b.key === '__none__') return -1;
    if ((groupBy === 'target_date' || groupBy === 'start_date') && a.key !== '__none__' && b.key !== '__none__') {
      return a.key.localeCompare(b.key);
    }
    return a.label.localeCompare(b.label);
  });
}

interface EpicViewState {
  groupBy: EpicGroupBy;
  filters: EpicFilterState;
  visibleColumns?: string[];
}

function loadEpicViewState(storageKey: string | null): EpicViewState {
  if (!storageKey) {
    return { groupBy: 'state', filters: {} };
  }
  try {
    const raw = localStorage.getItem(storageKey);
    if (!raw) return { groupBy: 'state', filters: {} };
    const parsed = JSON.parse(raw) as Partial<EpicViewState>;
    return {
      groupBy: EPIC_GROUP_BY_OPTIONS.some((option) => option.value === parsed.groupBy) ? parsed.groupBy! : 'state',
      // Discard the removed Team filter from previously saved views.
      filters: Object.fromEntries(Object.entries(parsed.filters ?? {}).filter(([key]) => key !== 'team')),
      visibleColumns: Array.isArray(parsed.visibleColumns) ? parsed.visibleColumns : undefined,
    };
  } catch {
    return { groupBy: 'state', filters: {} };
  }
}

function saveEpicViewState(
  storageKey: string | null,
  groupBy: EpicGroupBy,
  filters: EpicFilterState,
  visibleColumns: string[]) {
  if (!storageKey) return;
  try {
    localStorage.setItem(storageKey, JSON.stringify({ groupBy, filters, visibleColumns }));
  } catch {
    // Ignore local storage failures.
  }
}

interface EpicVirtualGroupItem {
  type: 'group';
  key: string;
  label: string;
  entryCount: number;
  totalTasks: number;
  totalPoints: number;
  completedPoints: number;
  collapsed: boolean;
}

interface EpicVirtualRowItem {
  type: 'row';
  key: string;
  row: Row<EpicWithStats>;
}

type EpicVirtualItem = EpicVirtualGroupItem | EpicVirtualRowItem;

interface EpicVirtualTableProps {
  data: EpicWithStats[];
  groups: EpicGroup[];
  groupBy: EpicGroupBy;
  collapsedGroupKeys: Set<string>;
  columns: ColumnDef<EpicWithStats, any>[];
  columnVisibility: VisibilityState;
  onRowClick: (entry: EpicWithStats) => void;
  sorting: SortingState;
  onSortingChange: OnChangeFn<SortingState>;
  columnSizing: ColumnSizingState;
  onColumnSizingChange: OnChangeFn<ColumnSizingState>;
  onToggleGroup: (key: string) => void;
}

function EpicVirtualTable({
  data,
  groups,
  groupBy,
  collapsedGroupKeys,
  columns,
  columnVisibility,
  onRowClick,
  sorting,
  onSortingChange,
  columnSizing,
  onColumnSizingChange,
  onToggleGroup }: EpicVirtualTableProps) {
  const parentRef = useRef<HTMLDivElement>(null);
  const columnSizingVersion = useMemo(() => JSON.stringify(columnSizing), [columnSizing]);
  const columnVisibilityVersion = useMemo(() => JSON.stringify(columnVisibility), [columnVisibility]);
  const table = useReactTable({
    data,
    columns,
    state: {
      columnVisibility,
      sorting,
      columnSizing,
    },
    onSortingChange,
    onColumnSizingChange,
    enableColumnResizing: true,
    columnResizeMode: 'onChange',
    manualSorting: true,
    getCoreRowModel: getCoreRowModel(),
  });

  const rowByEpicId = useMemo(() => {
    const map = new Map<string, Row<EpicWithStats>>();
    for (const row of table.getRowModel().rows) {
      map.set(row.original.epic.id, row);
    }
    return map;
  }, [table, data, columnVisibility, columnSizingVersion]);

  const items = useMemo<EpicVirtualItem[]>(() => {
    if (groupBy === 'none') {
      return data.flatMap((entry) => {
        const row = rowByEpicId.get(entry.epic.id);
        return row ? [{ type: 'row' as const, key: entry.epic.id, row }] : [];
      });
    }

    const nextItems: EpicVirtualItem[] = [];
    for (const group of groups) {
      const collapsed = collapsedGroupKeys.has(group.key);
      nextItems.push({
        type: 'group',
        key: group.key,
        label: group.label,
        entryCount: group.entries.length,
        totalTasks: group.entries.reduce((sum, entry) => sum + epicTaskCount(entry), 0),
        totalPoints: group.entries.reduce((sum, entry) => sum + entry.stats.total_points, 0),
        completedPoints: group.entries.reduce((sum, entry) => sum + entry.stats.done_points, 0),
        collapsed,
      });

      if (!collapsed) {
        for (const entry of group.entries) {
          const row = rowByEpicId.get(entry.epic.id);
          if (row) nextItems.push({ type: 'row', key: entry.epic.id, row });
        }
      }
    }
    return nextItems;
  }, [collapsedGroupKeys, data, groupBy, groups, rowByEpicId]);

  const estimateSize = useCallback(
    (index: number) => {
      const item = items[index];
      if (!item) return ROW_HEIGHT;
      if (item.type === 'group') {
        return GROUP_ROW_HEIGHT + (item.collapsed ? 0 : GROUP_HEADER_REPEAT_HEIGHT);
      }
      return ROW_HEIGHT;
    },
    [items],
  );

  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => parentRef.current,
    estimateSize,
    overscan: groupBy === 'none' ? 8 : 5,
  });

  const hasGroups = groupBy !== 'none';

  const renderColumnHeaderRow = () => (
    <>
      {table.getHeaderGroups().map((headerGroup) => (
        <div key={headerGroup.id} className="flex items-center">
          {headerGroup.headers.map((header) => {
            const defSize = header.column.columnDef.size ?? 150;
            const runtimeSize = header.getSize();
            const isResized = !!columnSizing[header.column.id];
            const canSort = header.column.getCanSort();
            const sorted = header.column.getIsSorted();
            const colId = header.column.id;
            const isNamePinned = colId === 'name';
            const pinnedClass = isNamePinned ? TABLE_PINNED_HEADER_LEFT_NAME : '';
            const pinnedSt = isNamePinned ? pinnedStyle('left', 0) : {};
            return (
              <div
                key={header.id}
                className={`${TABLE_HEADER_CELL} ${canSort ? TABLE_HEADER_CELL_SORTABLE : ''} ${pinnedClass}`}
                style={{
                  ...dynamicCellStyle(defSize, runtimeSize, isResized, 200),
                  ...pinnedSt,
                }}
                onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
              >
                <div className="flex items-center gap-1 overflow-hidden whitespace-nowrap">
                  {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                  {canSort ? <span className="ml-auto shrink-0">{sorted === 'asc' ? <ArrowUp02Icon className="h-3 w-3 text-foreground/80 stroke-[2.5]" /> : sorted === 'desc' ? <ArrowDown02Icon className="h-3 w-3 text-foreground/80 stroke-[2.5]" /> : <ArrowUpDownIcon className="h-3 w-3 text-muted-foreground stroke-[2]" />}</span> : null}
                </div>
                {header.column.getCanResize() ? <div onMouseDown={header.getResizeHandler()} onTouchStart={header.getResizeHandler()} onClick={(event) => event.stopPropagation()} className={`${TABLE_RESIZE_HANDLE} ${header.column.getIsResizing() ? 'bg-primary/50' : ''}`} /> : null}
              </div>
            );
          })}
        </div>
      ))}
    </>
  );

  return (
    <div className="flex h-full min-h-0 flex-1 flex-col">
      <div ref={parentRef} className={TABLE_CONTAINER}>
        <div className="min-w-fit">
          {hasGroups ? null : <div className={TABLE_HEADER}>{renderColumnHeaderRow()}</div>}

          <div
            style={{
              height: `${virtualizer.getTotalSize()}px`,
              position: 'relative',
              width: '100%',
            }}
          >
            <StickyPinnedGroupOverlay parentRef={parentRef} items={items} virtualizer={virtualizer} isPinnedItem={(item) => item.type === 'group'} renderHeader={(item) => (item.type === 'group' ? <MemoEpicGroupRow item={item} onToggle={onToggleGroup} /> : null)} />
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const item = items[virtualRow.index];
              if (!item) return null;
              return (
                <div key={`${item.type}-${item.key}`} data-index={virtualRow.index} style={virtualRowStyle(virtualRow.start)}>
                  {item.type === 'group' ? (
                    <>
                      <MemoEpicGroupRow item={item} onToggle={onToggleGroup} />
                      {item.collapsed ? null : <div className="border-b border-border/60 bg-card">{renderColumnHeaderRow()}</div>}
                    </>
                  ) : (
                    <MemoEpicDataRow row={item.row} onRowClick={onRowClick} columnSizing={columnSizing} columnSizingVersion={columnSizingVersion} columnVisibility={columnVisibility} columnVisibilityVersion={columnVisibilityVersion} />
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}

interface EpicGroupRowProps {
  item: EpicVirtualGroupItem;
  onToggle: (key: string) => void;
}

const MemoEpicGroupRow = memo(function EpicGroupRow({ item, onToggle }: EpicGroupRowProps) {
  return (
    <button type="button" className={`${TABLE_GROUP_ROW} w-full text-left text-ui`} onClick={() => onToggle(item.key)}>
      <span className={TABLE_GROUP_ROW_INNER}>
        {item.collapsed ? <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground" /> : <ArrowDown01Icon className="h-3.5 w-3.5 text-muted-foreground" />}
        <span className="min-w-0 truncate font-medium">{item.label}</span>
        <span className="ml-1 flex items-center gap-3 font-normal text-muted-foreground">
          <span>
            {item.entryCount} {item.entryCount === 1 ? 'epic' : 'epics'}
          </span>
          <span>{item.totalTasks} tasks</span>
          <span>
            {item.completedPoints}/{item.totalPoints} points
          </span>
        </span>
      </span>
    </button>
  );
});

interface EpicDataRowProps {
  row: Row<EpicWithStats>;
  onRowClick: (entry: EpicWithStats) => void;
  columnSizing: Record<string, number>;
  columnSizingVersion: string;
  columnVisibility: VisibilityState;
  columnVisibilityVersion: string;
}

function areEpicDataRowPropsEqual(prev: EpicDataRowProps, next: EpicDataRowProps) {
  return prev.row.id === next.row.id && prev.row.original === next.row.original && prev.columnSizingVersion === next.columnSizingVersion && prev.columnVisibilityVersion === next.columnVisibilityVersion && prev.onRowClick === next.onRowClick;
}

const MemoEpicDataRow = memo(function EpicDataRow({ row, onRowClick, columnSizing, columnSizingVersion, columnVisibility, columnVisibilityVersion }: EpicDataRowProps) {
  void columnSizingVersion;
  void columnVisibilityVersion;
  return (
    <div className={`${TABLE_ROW} cursor-pointer`} onClick={() => onRowClick(row.original)}>
      {row
        .getAllCells()
        .filter((cell) => columnVisibility[cell.column.id] !== false)
        .map((cell) => {
          const { defSize, runtimeSize, isResized } = resolveColumnRuntimeSize(cell.column, columnSizing);
          const colId = cell.column.id;
          const isNamePinned = colId === 'name';
          const pinnedClass = isNamePinned ? TABLE_PINNED_LEFT_NAME : '';
          const pinnedSt = isNamePinned ? pinnedStyle('left', 0) : {};
          return (
            <div
              key={cell.id}
              className={`${TABLE_CELL} overflow-hidden ${pinnedClass}`}
              style={{
                ...dynamicCellStyle(defSize, runtimeSize, isResized, 200),
                ...pinnedSt,
              }}
            >
              {flexRender(cell.column.columnDef.cell, cell.getContext())}
            </div>
          );
        })}
    </div>
  );
}, areEpicDataRowPropsEqual);

function InlineEpicStateCell({ entry, epicStateMap, epicStates, onUpdate }: { entry: EpicWithStats; epicStateMap: Map<string, EpicWorkflowState>; epicStates: EpicWorkflowState[]; onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void> }) {
  const [open, setOpen] = useState(false);
  const state = entry.epic.epic_state_id ? epicStateMap.get(entry.epic.epic_state_id) : null;
  const stateCfg = state ? STATE_TYPE_ICON_CONFIG[state.state_type as StateType] : null;
  const StateIcon = stateCfg?.icon ?? MinusSignIcon;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-ui transition-colors hover:bg-accent cursor-pointer"
          onClick={(event) => {
            event.stopPropagation();
            setOpen(true);
          }}
        >
          <StateIcon className={`h-3.5 w-3.5 shrink-0 ${stateCfg?.color ?? 'text-muted-foreground'}`} />
          <span className="truncate">{state?.name ?? 'No state'}</span>
        </button>
      </PopoverTrigger>
      {open ? (
        <PopoverContent className="w-[220px] p-0" align="start" side="bottom" onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>
          <Command>
            <CommandInput placeholder="Search states..." className="h-8 text-ui" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-ui text-muted-foreground">No states found</CommandEmpty>
              <CommandGroup>
                {epicStates.map((epicState) => {
                  const optionCfg = STATE_TYPE_ICON_CONFIG[epicState.state_type as StateType];
                  const OptionIcon = optionCfg?.icon ?? MinusSignIcon;
                  return (
                    <CommandItem
                      key={epicState.id}
                      value={epicState.name}
                      className="flex items-center gap-2 text-ui"
                      onSelect={() => {
                        if (epicState.id !== entry.epic.epic_state_id) {
                          void onUpdate(entry.epic.id, {
                            epic_state_id: epicState.id,
                          });
                        }
                        setOpen(false);
                      }}
                    >
                      <OptionIcon className={`h-3.5 w-3.5 shrink-0 ${optionCfg?.color ?? 'text-muted-foreground'}`} />
                      <span className="truncate">{epicState.name}</span>
                      {entry.epic.epic_state_id === epicState.id ? <Tick01Icon className="ml-auto h-3.5 w-3.5 text-primary" /> : null}
                    </CommandItem>
                  );
                })}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      ) : null}
    </Popover>
  );
}

function InlineEpicHealthCell({ entry, onUpdate }: { entry: EpicWithStats; onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void> }) {
  const [open, setOpen] = useState(false);
  const health = entry.epic.health;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="rounded-md px-1.5 py-0.5 text-ui transition-colors hover:bg-accent cursor-pointer"
          onClick={(event) => {
            event.stopPropagation();
            setOpen(true);
          }}
        >
          <span className={healthConfig[health]?.color ?? 'text-muted-foreground'}>{healthConfig[health]?.label ?? 'No health'}</span>
        </button>
      </PopoverTrigger>
      {open ? (
        <PopoverContent className="w-[180px] p-0" align="start" side="bottom" onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>
          <Command>
            <CommandInput placeholder="Search health..." className="h-8 text-ui" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-ui text-muted-foreground">No match</CommandEmpty>
              <CommandGroup>
                {ALL_HEALTH_OPTIONS.map((option) => (
                  <CommandItem
                    key={option}
                    value={healthConfig[option].label}
                    className="flex items-center gap-2 text-ui"
                    onSelect={() => {
                      if (option !== health) {
                        void onUpdate(entry.epic.id, { health: option });
                      }
                      setOpen(false);
                    }}
                  >
                    <span className={healthConfig[option].color}>{healthConfig[option].label}</span>
                    {health === option ? <Tick01Icon className="ml-auto h-3.5 w-3.5 text-primary" /> : null}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      ) : null}
    </Popover>
  );
}

function InlineEpicOwnerCell({ entry, assignableMembers, ownerNameMap, onUpdate }: { entry: EpicWithStats; assignableMembers: AssignableMember[]; ownerNameMap: Map<string, string>; onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void> }) {
  const ownerKey = entry.epic.owner_member_id;
  const ownerName = ownerKey ? (ownerNameMap.get(ownerKey) ?? 'Unknown') : null;

  return (
    <MemberPickerPopover
      value={entry.epic.owner_member_id || '__none__'}
      members={assignableMembers}
      noneLabel="Unassigned"
      onChange={(value) => {
        void onUpdate(entry.epic.id, {
          owner_member_id: value === '__none__' ? '' : value,
        });
      }}
      renderTrigger={() => {
        const selectedMember = findAssignableMember(assignableMembers, entry.epic.owner_member_id);
        return selectedMember ? (
          <>
            <UserAvatar name={selectedMember.display_name || selectedMember.email} avatarUrl={selectedMember.avatar_url} avatarStyle={selectedMember.avatar_style} avatarSeed={selectedMember.avatar_seed} avatarBackgroundMode={selectedMember.avatar_background_mode} avatarBackgroundColor={selectedMember.avatar_background_color} className="h-4 w-4" fallbackClassName="text-[7px]" />
            <span className="truncate">{ownerName}</span>
          </>
        ) : (
          <>
            <UserAdd01Icon className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="text-muted-foreground">Assign</span>
          </>
        );
      }}
    />
  );
}

function InlineEpicTeamCell({ entry, teams, teamMap, onUpdate }: { entry: EpicWithStats; teams: WorkspaceTeam[]; teamMap: Map<string, string>; onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void> }) {
  const [open, setOpen] = useState(false);
  const teamName = entry.epic.team_id ? (teamMap.get(entry.epic.team_id) ?? 'Unknown') : null;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-ui transition-colors hover:bg-accent cursor-pointer"
          onClick={(event) => {
            event.stopPropagation();
            setOpen(true);
          }}
        >
          {teamName ? <span className="truncate">{teamName}</span> : <span className="text-muted-foreground">No team</span>}
        </button>
      </PopoverTrigger>
      {open ? (
        <PopoverContent className="w-[220px] p-0" align="start" side="bottom" onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>
          <Command>
            <CommandInput placeholder="Search teams..." className="h-8 text-ui" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-ui text-muted-foreground">No teams found</CommandEmpty>
              <CommandGroup>
                {teams.map((team) => (
                  <CommandItem
                    key={team.id}
                    value={team.name}
                    className="flex items-center gap-2 text-ui"
                    onSelect={() => {
                      if (team.id !== entry.epic.team_id) {
                        void onUpdate(entry.epic.id, { team_id: team.id });
                      }
                      setOpen(false);
                    }}
                  >
                    <span className="truncate">{team.name}</span>
                    {entry.epic.team_id === team.id ? <Tick01Icon className="ml-auto h-3.5 w-3.5 text-primary" /> : null}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      ) : null}
    </Popover>
  );
}

function InlineEpicLabelsCell({ entry, workspaceId, allLabels, onLabelsChange, onUpdate }: { entry: EpicWithStats; workspaceId: string; allLabels: Label[]; onLabelsChange: (labels: Label[]) => void; onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void> }) {
  const labels = entry.labels ?? [];
  const selectedLabelIds = labels.map((label) => label.id);

  return (
    <div onClick={(event) => event.stopPropagation()} className="group/lbl flex min-w-0 items-center gap-1">
      {labels.length > 0 ? (
        <div className="flex min-w-0 items-center gap-1.5" title={labels.map((l) => l.name).join(', ')}>
          <span
            className="h-2 w-2 shrink-0 rounded-full"
            style={{
              backgroundColor: labels[0].color ? (labels[0].color.startsWith('#') ? labels[0].color : `#${labels[0].color}`) : 'var(--muted-foreground)',
            }}
          />
          <span className="truncate text-ui text-muted-foreground">
            {labels[0].name}
            {labels.length > 1 ? ` +${labels.length - 1} more` : ''}
          </span>
        </div>
      ) : null}
      <div className={labels.length > 0 ? 'opacity-0 group-hover/lbl:opacity-100 transition-opacity shrink-0' : 'shrink-0'}>
        <LabelPicker
          triggerClassName="text-[length:var(--text-ui)]"
          workspaceId={workspaceId}
          teamId={entry.epic.team_id || undefined}
          labels={allLabels}
          selectedLabelIds={selectedLabelIds}
          onLabelsChange={onLabelsChange}
          onChange={async (labelIds) => {
            await onUpdate(entry.epic.id, { label_ids: labelIds });
          }}
          triggerOnly
        />
      </div>
    </div>
  );
}

// ── Main page ──────────────────────────────────────────────────────

interface EpicsPageProps {
  teamId?: string;
}

export function EpicsPage({ teamId }: EpicsPageProps) {
  useTitle('Epics');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const navigate = useNavigate();
  const openCreate = useGlobalCreateStore((s) => s.openCreate);

  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [groupBy, setGroupBy] = useState<EpicGroupBy>('state');
  const [filters, setFilters] = useState<EpicFilterState>({});
  const [tableSorting, setTableSorting] = useState<SortingState>([]);
  const [tableColumnSizing, setTableColumnSizing] = useState<ColumnSizingState>({});
  const [collapsedGroupKeys, setCollapsedGroupKeys] = useState<Set<string>>(new Set());
  const deferredSearch = useDeferredValue(search);

  const workspaceId = workspace?.id;
  const slug = workspace?.slug;
  const showArchived = filters.state?.includes(ARCHIVED_STATE_VALUE) ?? false;
  const epicFilters = useMemo(() => ({ archived: showArchived, team_id: teamId }), [showArchived, teamId]);
  const objectiveFilters = useMemo(() => ({ archived: false }), []);
  const queryClient = useQueryClient();
  const epicsQuery = useEpics(workspaceId ?? '', epicFilters);
  const labelsQuery = useLabels(workspaceId ?? '');
  const objectivesQuery = useObjectives(workspaceId ?? '', objectiveFilters);
  const epics = epicsQuery.data ?? EMPTY_EPICS;
  const allLabels = labelsQuery.data ?? EMPTY_LABELS;
  const allObjectives = useMemo(
    () => (objectivesQuery.data ?? []).map((entry) => entry.objective),
    [objectivesQuery.data],
  );
  const epicsQueryKey = useMemo(
    () => [...queryKeys.pm.epics(workspaceId ?? ''), epicFilters] as const,
    [epicFilters, workspaceId],
  );
  const labelsQueryKey = useMemo(
    () => [...queryKeys.pm.labels(workspaceId ?? ''), undefined] as const,
    [workspaceId],
  );
  const setEpics = useCallback(
    (next: EpicWithStats[] | ((current: EpicWithStats[]) => EpicWithStats[])) => {
      queryClient.setQueryData<EpicWithStats[]>(epicsQueryKey, (current = []) =>
        typeof next === 'function' ? next(current) : next,
      );
    },
    [epicsQueryKey, queryClient],
  );
  const setAllLabels = useCallback(
    (labels: Label[]) => queryClient.setQueryData<Label[]>(labelsQueryKey, labels),
    [labelsQueryKey, queryClient],
  );
  const loading = epicsQuery.isLoading;
  const loadError = error ?? (epicsQuery.error instanceof Error ? epicsQuery.error.message : null);
  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit } = usePermissions(access);
  const { teams, findTeamName } = useAccessibleTeams(workspaceId ?? '');
  const isSingleTeam = teams.length <= 1;
  const storageKey = useMemo(() => (workspaceId ? `pm_epics_view_${workspaceId}_${teamId ?? 'all'}` : null), [workspaceId, teamId]);

  const [visibleColumns, setVisibleColumns] = useState<string[]>(DEFAULT_VISIBLE);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const { data: epicStates = [] } = useEpicStates(workspaceId ?? '');
  const ownerNameMap = useMemo(() => buildAssignableMemberNameMap(assignableMembers), [assignableMembers]);
  const teamMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const team of teams) {
      map.set(team.id, team.name);
    }
    return map;
  }, [teams]);
  const objectiveNameMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const objective of allObjectives) {
      map.set(objective.id, objective.name);
    }
    for (const epicEntry of epics) {
      for (const objective of epicEntry.objectives ?? []) {
        if (!map.has(objective.id)) {
          map.set(objective.id, `${objective.name} (Archived)`);
        }
      }
    }
    return map;
  }, [allObjectives, epics]);
  const epicStateMap = useMemo(() => {
    const map = new Map<string, EpicWorkflowState>();
    for (const state of epicStates) {
      map.set(state.id, state);
    }
    return map;
  }, [epicStates]);

  const filterDefinitions = useMemo<EpicFilterDefinition[]>(
    () => [
      {
        key: 'owner',
        label: 'Owner',
        options: assignableMembers.map((member) => ({
          value: member.id,
          label: member.display_name || member.email,
        })),
      },
      {
        key: 'state',
        label: 'State',
        options: [
          ...epicStates.map((state) => ({
            value: state.id,
            label: state.name,
          })),
          { value: '__archived__', label: 'Archived' },
        ],
      },
      {
        key: 'health',
        label: 'Health',
        options: ALL_HEALTH_OPTIONS.map((health) => ({
          value: health,
          label: healthConfig[health].label,
        })),
      },
      {
        key: 'label',
        label: 'Label',
        options: allLabels.map((label) => ({
          value: label.id,
          label: label.name,
        })),
      },
      {
        key: 'objective',
        label: 'Objective',
        options: Array.from(objectiveNameMap.entries()).map(([value, label]) => ({ value, label })),
      },
      {
        key: 'has_target_date',
        label: 'Target date',
        options: [
          { value: 'yes', label: 'Has target date' },
          { value: 'no', label: 'No target date' },
        ],
      },
      {
        key: 'has_start_date',
        label: 'Start date',
        options: [
          { value: 'yes', label: 'Has start date' },
          { value: 'no', label: 'No start date' },
        ],
      },
    ],
    [allLabels, assignableMembers, epicStates, objectiveNameMap],
  );

  const updateEpicField = useCallback(
    async (epicId: string, patch: UpdateEpicRequest) => {
      if (!workspaceId) return;
      setError(null);
      await queryClient.cancelQueries({ queryKey: epicsQueryKey, exact: true });

      let snapshot: EpicWithStats[] = [];
      setEpics((current) => {
        snapshot = current;
        return current.map((entry) => (entry.epic.id === epicId ? applyEpicPatch(entry, patch, allLabels) : entry));
      });

      const { data, error: updateError } = await pmEpicService.update(workspaceId, epicId, patch);
      if (updateError || !data) {
        setEpics(snapshot);
        setError(updateError ?? 'Failed to update epic');
        return;
      }

      setEpics((current) => current.map((entry) => (entry.epic.id === epicId ? data : entry)));
      queryClient.setQueryData(queryKeys.pm.epic(workspaceId, epicId), data);
      void queryClient.invalidateQueries({ queryKey: queryKeys.pm.epics(workspaceId) });
    },
    [allLabels, epicsQueryKey, queryClient, setEpics, workspaceId],
  );

  const updateEpicObjectives = useCallback(
    async (epicId: string, nextObjectiveIds: string[]) => {
      const currentEntry = epics.find((entry) => entry.epic.id === epicId);
      if (!currentEntry || !workspaceId) return;

      setError(null);
      await queryClient.cancelQueries({ queryKey: epicsQueryKey, exact: true });
      const snapshot = epics;
      const currentObjectiveIds = (currentEntry.objectives ?? []).map((objective) => objective.id);
      const currentSet = new Set(currentObjectiveIds);
      const nextSet = new Set(nextObjectiveIds);
      const toAdd = nextObjectiveIds.filter((id) => !currentSet.has(id));
      const toRemove = currentObjectiveIds.filter((id) => !nextSet.has(id));

      setEpics((current) =>
        current.map((entry) =>
          entry.epic.id === epicId
            ? {
                ...entry,
                objectives: buildObjectiveRefs(allObjectives, nextObjectiveIds),
              }
            : entry,
        ),
      );

      const results = await Promise.all([...toAdd.map((objectiveId) => pmObjectiveService.addEpic(workspaceId, objectiveId, epicId)), ...toRemove.map((objectiveId) => pmObjectiveService.removeEpic(workspaceId, objectiveId, epicId))]);

      const failed = results.find((result) => result.error);
      const refreshed = await pmEpicService.get(workspaceId, epicId);
      if (!refreshed.data) {
        setEpics(snapshot);
        setError(failed?.error ?? refreshed.error ?? 'Failed to refresh epic');
        return;
      }

      setEpics((current) => current.map((entry) => (entry.epic.id === epicId ? refreshed.data! : entry)));
      if (failed) {
        setError(failed.error ?? 'Failed to update objectives');
      }
    },
    [allObjectives, epics, epicsQueryKey, queryClient, setEpics, workspaceId],
  );

  const columns = useMemo(
    () => [
      columnHelper.accessor((row) => row.epic.name, {
        id: 'name',
        header: 'Name',
        size: 280,
        cell: (info) => (
          <div className={`flex max-w-full items-center gap-1.5 ${TABLE_NAME_TEXT}`}>
            <EpicColorControl
              compact
              value={info.row.original.epic.color}
              onChange={canEdit ? (color) => { void updateEpicField(info.row.original.epic.id, { color }); } : undefined}
            />
            <span className="min-w-0 truncate">{info.getValue()}</span>
          </div>
        ),
      }),
      columnHelper.accessor((row) => epicStateMap.get(row.epic.epic_state_id ?? '')?.position ?? Number.MAX_SAFE_INTEGER, {
        id: 'state',
        header: 'State',
        size: 140,
        cell: (info) => {
          if (canEdit) {
            return <InlineEpicStateCell entry={info.row.original} epicStateMap={epicStateMap} epicStates={epicStates} onUpdate={updateEpicField} />;
          }

          const state = info.row.original.epic.epic_state_id ? epicStateMap.get(info.row.original.epic.epic_state_id) : null;
          const stateCfg = state ? STATE_TYPE_ICON_CONFIG[state.state_type as StateType] : null;
          const StateIcon = stateCfg?.icon ?? MinusSignIcon;
          return (
            <div className="flex items-center gap-1.5 text-ui text-muted-foreground">
              <StateIcon className={`h-3.5 w-3.5 shrink-0 ${stateCfg?.color ?? 'text-muted-foreground'}`} />
              <span className="truncate">{state?.name ?? 'No state'}</span>
            </div>
          );
        },
      }),
      columnHelper.accessor((row) => ALL_HEALTH_OPTIONS.indexOf(row.epic.health), {
        id: 'health',
        header: 'Health',
        size: 96,
        cell: (info) => (canEdit ? <InlineEpicHealthCell entry={info.row.original} onUpdate={updateEpicField} /> : <span className={`text-ui ${healthConfig[info.row.original.epic.health]?.color ?? 'text-muted-foreground'}`}>{healthConfig[info.row.original.epic.health]?.label ?? 'No updates'}</span>),
      }),
      columnHelper.accessor((row) => epicCompletionPct(row), {
        id: 'progress',
        header: 'Progress',
        size: 90,
        cell: (info) => (
          <div className="flex items-center gap-1.5 text-ui">
            <Sun01Icon className="h-3.5 w-3.5 text-amber-500" />
            <span>{epicCompletionPct(info.row.original)}%</span>
          </div>
        ),
      }),
      columnHelper.accessor((row) => epicTaskCount(row), {
        id: 'tasks',
        header: 'Tasks',
        size: 90,
        cell: (info) => (
          <span className="text-ui text-muted-foreground">
            {epicDoneTaskCount(info.row.original)}/{epicTaskCount(info.row.original)}
          </span>
        ),
      }),
      columnHelper.accessor((row) => row.stats.total_points, {
        id: 'points',
        header: 'Points',
        size: 80,
        cell: (info) => (
          <span className="text-ui text-muted-foreground">
            {info.row.original.stats.done_points}/{info.row.original.stats.total_points}
          </span>
        ),
      }),
      columnHelper.accessor((row) => (row.epic.owner_member_id ? (ownerNameMap.get(row.epic.owner_member_id) ?? '') : ''), {
        id: 'owner',
        header: 'Owner',
        size: 140,
        cell: (info) => {
          if (canEdit) {
            return <InlineEpicOwnerCell entry={info.row.original} assignableMembers={assignableMembers} ownerNameMap={ownerNameMap} onUpdate={updateEpicField} />;
          }

          const entry = info.row.original;
          const ownerName = entry.epic.owner_member_id ? ownerNameMap.get(entry.epic.owner_member_id) : null;
          return ownerName ? (
            <div className="flex items-center gap-2 min-w-0">
              <UserAvatar name={ownerName} />
              <span className="truncate text-ui text-muted-foreground">{ownerName}</span>
            </div>
          ) : (
            <UserIcon className="h-4 w-4 text-muted-foreground/50" />
          );
        },
      }),
      columnHelper.accessor((row) => (row.objectives ?? []).map((objective) => objective.name).join(', '), {
        id: 'objective',
        header: 'Objective',
        size: 240,
        cell: (info) => {
          const entry = info.row.original;
          const objectives = entry.objectives ?? [];
          if (canEdit) {
            return <InlineEpicObjectivesControl entry={entry} allObjectives={allObjectives} onChange={updateEpicObjectives} />;
          }
          return objectives.length > 0 ? (
            <span className="truncate text-ui text-muted-foreground block" title={objectives.map((o) => o.name).join(', ')}>
              {objectives[0].name}
              {objectives.length > 1 ? ` +${objectives.length - 1} more` : ''}
            </span>
          ) : (
            <MinusSignIcon className="h-3.5 w-3.5 text-muted-foreground" />
          );
        },
      }),
      columnHelper.accessor((row) => row.epic.deadline ?? '', {
        id: 'target_date',
        header: 'Target date',
        size: 110,
        cell: (info) =>
          canEdit ? (
            <InlineEpicDateControl epicId={info.row.original.epic.id} value={info.row.original.epic.deadline} emptyLabel="No date" patchKey="deadline" onUpdate={updateEpicField} />
          ) : (
            <div className="flex items-center gap-1 text-ui text-muted-foreground">
              {info.row.original.epic.deadline ? (
                <>
                  <Calendar03Icon className="h-3.5 w-3.5" />
                  <span>{format(parseISO(info.row.original.epic.deadline), 'MMM d')}</span>
                </>
              ) : (
                <MinusSignIcon className="h-3.5 w-3.5" />
              )}
            </div>
          ),
      }),
      columnHelper.accessor((row) => (row.epic.team_id ? (teamMap.get(row.epic.team_id) ?? '') : ''), {
        id: 'team',
        header: 'Team',
        size: 110,
        cell: (info) => (canEdit ? <InlineEpicTeamCell entry={info.row.original} teams={teams} teamMap={teamMap} onUpdate={updateEpicField} /> : <span className="truncate text-ui text-muted-foreground">{findTeamName(info.row.original.epic.team_id) || <MinusSignIcon className="h-3.5 w-3.5" />}</span>),
      }),
      columnHelper.accessor((row) => row.epic.planned_start_date ?? '', {
        id: 'start_date',
        header: 'Start date',
        size: 110,
        cell: (info) =>
          canEdit ? (
            <InlineEpicDateControl epicId={info.row.original.epic.id} value={info.row.original.epic.planned_start_date} emptyLabel="No date" patchKey="planned_start_date" onUpdate={updateEpicField} />
          ) : (
            <div className="flex items-center gap-1 text-ui text-muted-foreground">
              {info.row.original.epic.planned_start_date ? (
                <>
                  <Calendar03Icon className="h-3.5 w-3.5" />
                  <span>{format(parseISO(info.row.original.epic.planned_start_date), 'MMM d')}</span>
                </>
              ) : (
                <MinusSignIcon className="h-3.5 w-3.5" />
              )}
            </div>
          ),
      }),
      columnHelper.accessor((row) => (row.labels ?? []).map((label) => label.name).join(', '), {
        id: 'labels',
        header: 'Labels',
        size: 220,
        cell: (info) =>
          canEdit ? (
            <InlineEpicLabelsCell entry={info.row.original} workspaceId={workspaceId!} allLabels={allLabels} onLabelsChange={setAllLabels} onUpdate={updateEpicField} />
          ) : (
            (() => {
              const lbls = info.row.original.labels ?? [];
              return lbls.length > 0 ? (
                <div className="flex min-w-0 items-center gap-1.5" title={lbls.map((l) => l.name).join(', ')}>
                  <span
                    className="h-2 w-2 shrink-0 rounded-full"
                    style={{
                      backgroundColor: lbls[0].color ? (lbls[0].color.startsWith('#') ? lbls[0].color : `#${lbls[0].color}`) : 'var(--muted-foreground)',
                    }}
                  />
                  <span className="truncate text-ui text-muted-foreground">
                    {lbls[0].name}
                    {lbls.length > 1 ? ` +${lbls.length - 1} more` : ''}
                  </span>
                </div>
              ) : (
                <MinusSignIcon className="h-3.5 w-3.5 text-muted-foreground" />
              );
            })()
          ),
      }),
      columnHelper.accessor((row) => row.epic.created_at, {
        id: 'created',
        header: 'Created',
        size: 100,
        cell: (info) => <span className="text-ui text-muted-foreground">{format(parseISO(info.row.original.epic.created_at), 'MMM d')}</span>,
      }),
      columnHelper.accessor((row) => row.epic.updated_at, {
        id: 'updated',
        header: 'Updated',
        size: 100,
        cell: (info) => <span className="text-ui text-muted-foreground">{format(parseISO(info.row.original.epic.updated_at), 'MMM d')}</span>,
      }),
    ],
    [allLabels, allObjectives, assignableMembers, canEdit, columnHelper, epicStateMap, epicStates, findTeamName, ownerNameMap, setAllLabels, teamMap, teams, updateEpicField, updateEpicObjectives, workspaceId],
  );

  const columnVisibility = useMemo<VisibilityState>(() => {
    const visible = new Set(visibleColumns);
    if (isSingleTeam) visible.delete('team');
    return {
      name: true,
      state: visible.has('state'),
      health: visible.has('health'),
      progress: visible.has('progress'),
      tasks: visible.has('tasks'),
      points: visible.has('points'),
      owner: visible.has('owner'),
      objective: visible.has('objective'),
      target_date: visible.has('target_date'),
      team: visible.has('team'),
      start_date: visible.has('start_date'),
      labels: visible.has('labels'),
      created: visible.has('created'),
      updated: visible.has('updated'),
    };
  }, [visibleColumns, isSingleTeam]);

  const hasLoadedViewStateRef = useRef(false);

  useEffect(() => {
    hasLoadedViewStateRef.current = false;
    const next = loadEpicViewState(storageKey);
    setGroupBy(next.groupBy);
    setFilters(next.filters);
    if (next.visibleColumns) setVisibleColumns(next.visibleColumns);
    hasLoadedViewStateRef.current = true;
  }, [storageKey]);

  useEffect(() => {
    if (!hasLoadedViewStateRef.current) return;
    if (!storageKey) return;
    // visibleColumns is persisted synchronously inside its onChange handler so
    // we don't read it here; instead preserve whatever is currently in storage.
    const current = loadEpicViewState(storageKey);
    saveEpicViewState(storageKey, groupBy, filters, current.visibleColumns ?? visibleColumns);
  }, [filters, groupBy, storageKey, visibleColumns]);

  // Refresh when epic is created via global modal
  useEffect(() => {
    const handler = () => {
      if (!workspaceId) return;
      void queryClient.invalidateQueries({ queryKey: queryKeys.pm.epics(workspaceId) });
    };
    window.addEventListener('epic-created', handler);
    return () => window.removeEventListener('epic-created', handler);
  }, [queryClient, workspaceId]);

  const openEpic = useCallback(
    (entry: EpicWithStats) => {
      if (!slug) return;
      navigate({
        to: '/w/$slug/pm/epics/$epicId',
        params: { slug, epicId: entry.epic.id },
      });
    },
    [navigate, slug],
  );

  const handleClearFilters = useCallback(() => {
    setFilters({});
    setSearch('');
  }, []);

  const handleToggleGroup = useCallback((key: string) => {
    setCollapsedGroupKeys((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }, []);

  const filteredEpics = useMemo(() => {
    const q = deferredSearch.trim().toLowerCase();
    return epics.filter((entry) => {
      if (q && !entry.epic.name.toLowerCase().includes(q)) return false;
      return epicMatchesFilters(entry, filters);
    });
  }, [deferredSearch, epics, filters]);

  const sortedEpics = useMemo(() => sortEntriesForTable(filteredEpics, tableSorting, epicStates, ownerNameMap, teamMap), [epicStates, filteredEpics, ownerNameMap, tableSorting, teamMap]);

  const groupedEpics = useMemo(() => buildEpicGroups(sortedEpics, groupBy, filters, epicStates, ownerNameMap, teamMap, objectiveNameMap, tableSorting.length > 0), [epicStates, filters, groupBy, objectiveNameMap, ownerNameMap, sortedEpics, tableSorting.length, teamMap]);
  useEffect(() => {
    setCollapsedGroupKeys((current) => {
      const validKeys = new Set(groupedEpics.map((group) => group.key));
      return new Set([...current].filter((key) => validKeys.has(key)));
    });
  }, [groupedEpics]);

  const teamLabel = teamId ? findTeamName(teamId) : null;
  const showControls = epics.length > 0;

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="flex h-full min-h-0 flex-1 flex-col">
      <QuietPageHeader
        variant="shell"
        title="Epics"
        context={teamLabel}
        description="Track long-running initiatives and their task progress."
        actions={
          canEdit ? (
            <QuietPrimaryAction className="gap-1.5" onClick={() => openCreate('epic', { teamId })}>
              <PlusSignIcon className="h-4 w-4" />
              Add epic
            </QuietPrimaryAction>
          ) : null
        }
      />

      {loadError ? <div className="mx-4 mt-3 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive md:mx-6">{loadError}</div> : null}

      {showControls ? (
        <EpicFilterBar
          search={search}
          onSearchChange={setSearch}
          ownerFilter={assignableMembers.length > 0 && (
            <OwnerAvatarFilterRow
              workspaceId={workspaceId!}
              members={assignableMembers}
              selectedIds={filters.owner ?? []}
              className="ml-0"
              onToggle={(memberId) => {
                setFilters((current) => {
                  const selected = current.owner ?? [];
                  const owner = selected.includes(memberId)
                    ? selected.filter((id) => id !== memberId)
                    : [...selected, memberId];
                  if (owner.length === 0) {
                    const { owner: _omit, ...rest } = current;
                    return rest;
                  }
                  return { ...current, owner };
                });
              }}
            />
          )}
          categories={filterDefinitions
            .filter((definition) => definition.key !== 'has_target_date' && definition.key !== 'has_start_date')
            .map((definition) => {
              const isStateKey = definition.key === 'state';
              const baseOptions = isStateKey ? definition.options.filter((option) => option.value !== ARCHIVED_STATE_VALUE) : definition.options;
              const selected = (filters[definition.key] ?? []).filter((value) => !isStateKey || value !== ARCHIVED_STATE_VALUE);
              const options = baseOptions.map((option) => ({
                ...option,
                ...getFilterOptionVisual(definition.key, option.value, {
                  epicStateMap,
                  assignableMembers,
                  allLabels,
                }),
              }));
              const IconCmp = FILTER_CATEGORY_ICONS[definition.key];
              return {
                key: definition.key,
                label: definition.label,
                options,
                selected,
                icon: IconCmp ? <IconCmp className="h-3.5 w-3.5" /> : undefined,
              };
            })}
          onCategoryChange={(key, next) => {
            const filterKey = key as EpicFilterKey;
            setFilters((current) => {
              const previous = current[filterKey] ?? [];
              const preservedArchived = filterKey === 'state' && previous.includes(ARCHIVED_STATE_VALUE) ? [ARCHIVED_STATE_VALUE] : [];
              const merged = [...next, ...preservedArchived];
              if (merged.length === 0) {
                const { [filterKey]: _omit, ...rest } = current;
                return rest;
              }
              return { ...current, [filterKey]: merged };
            });
          }}
          onClearAll={handleClearFilters}
          showArchived={(filters.state ?? []).includes(ARCHIVED_STATE_VALUE)}
          onToggleShowArchived={(next) => {
            setFilters((current) => {
              const previous = current.state ?? [];
              const without = previous.filter((value) => value !== ARCHIVED_STATE_VALUE);
              const updated = next ? [...without, ARCHIVED_STATE_VALUE] : without;
              if (updated.length === 0) {
                const { state: _omit, ...rest } = current;
                return rest;
              }
              return { ...current, state: updated };
            });
          }}
          groupBy={groupBy}
          groupByOptions={
            EPIC_GROUP_BY_OPTIONS as unknown as {
              value: string;
              label: string;
            }[]
          }
          onGroupByChange={(value) => setGroupBy(value as EpicGroupBy)}
          displayProperties={ALL_PROPERTIES}
          visibleProperties={visibleColumns}
          onVisiblePropertiesChange={(next) => {
            setVisibleColumns(next);
            if (storageKey) {
              const current = loadEpicViewState(storageKey);
              saveEpicViewState(storageKey, current.groupBy ?? groupBy, current.filters ?? filters, next);
            }
          }}
        />
      ) : null}

      {loading ? null : epics.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 px-4">
          <div className="flex h-14 w-14 items-center justify-center rounded-full bg-violet-500/10 mb-5">
            <Layers01Icon className="h-7 w-7 text-violet-500" />
          </div>
          <h3 className="text-lg font-semibold mb-1.5">Create your first epic</h3>
          <p className="text-sm text-muted-foreground text-center max-w-md mb-6">Epics group related tasks into long-running initiatives, giving you a high-level view of progress across your team's work.</p>
          <div className="mt-6 grid w-full max-w-4xl grid-cols-1 gap-4 sm:grid-cols-3">
            {[
              {
                icon: Layers01Icon,
                title: 'Group tasks',
                desc: 'Organize related work items under a single initiative',
              },
              {
                icon: ChartIncreaseIcon,
                title: 'Track health',
                desc: 'Monitor on-track, at-risk, and off-track status at a glance',
              },
              {
                icon: Target01Icon,
                title: 'Hit deadlines',
                desc: 'Set target dates and watch completion progress in real time',
              },
            ].map((item) => (
              <div key={item.title} className="flex flex-col items-center text-center rounded-lg border border-border/50 bg-muted/30 p-6">
                <item.icon className="h-5 w-5 text-muted-foreground mb-3" />
                <p className="text-sm font-medium mb-1">{item.title}</p>
                <p className="text-sm text-muted-foreground leading-relaxed">{item.desc}</p>
              </div>
            ))}
          </div>
        </div>
      ) : sortedEpics.length === 0 ? (
        <div className="rounded-lg border border-border/60 bg-muted/20 px-4 py-10 text-center">
          <p className="text-sm font-medium">No epics match the current search and filters.</p>
          <p className="mt-1 text-sm text-muted-foreground">Adjust the active filters or clear them to see more epics.</p>
        </div>
      ) : (
        <EpicVirtualTable data={sortedEpics} groups={groupedEpics} groupBy={groupBy} collapsedGroupKeys={collapsedGroupKeys} columns={columns} columnVisibility={columnVisibility} onRowClick={openEpic} sorting={tableSorting} onSortingChange={setTableSorting} columnSizing={tableColumnSizing} onColumnSizingChange={setTableColumnSizing} onToggleGroup={handleToggleGroup} />
      )}
    </div>
  );
}
