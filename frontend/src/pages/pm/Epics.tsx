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
import { format, parseISO } from 'date-fns';
import { useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import {
  Tick01Icon,
  ArrowDown02Icon,
  Calendar03Icon,
  ArrowDown01Icon,
  ArrowRight01Icon,
  FilterHorizontalIcon,
  PlusSignIcon,
  Search01Icon,
  Target01Icon,
  ChartIncreaseIcon,
  ArrowUp02Icon,
  ArrowUpDownIcon,
  UserIcon,
  UserAdd01Icon,
  Cancel01Icon,
  Layers01Icon,
  MinusSignIcon,
  Sun01Icon,
} from '@/lib/icons';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { DisplayPropertiesPopover } from '@/components/pm/DisplayPropertiesPopover';
import { LabelPicker } from '@/components/pm/LabelPicker';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { ObjectivePicker, type ObjectivePickerSelection } from '@/components/pm/ObjectivePicker';
import { Calendar } from '@/components/ui/calendar';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useEpicStates, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import type { EpicWithStats, EpicHealth, EpicWorkflowState, Label, Objective, StateType, UpdateEpicRequest } from '@/lib/pmTypes';
import { buildAssignableMemberNameMap, findAssignableMember } from '@/lib/assignableMembers';
import { STATE_TYPE_ICON_CONFIG } from '@/lib/pmConstants';
import type { AssignableMember, WorkspaceTeam } from '@/lib/types';
import {
  GROUP_ROW_HEIGHT,
  ROW_HEIGHT,
  TABLE_CELL,
  TABLE_CONTAINER,
  TABLE_GROUP_ROW,
  TABLE_GROUP_ROW_INNER,
  TABLE_HEADER,
  TABLE_HEADER_CELL,
  TABLE_HEADER_CELL_SORTABLE,
  TABLE_RESIZE_HANDLE,
  TABLE_ROW,
  dynamicCellStyle,
  resolveColumnRuntimeSize,
  virtualRowStyle,
} from '@/lib/tableStyles';

const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
  no_health: { label: 'No health', color: 'text-muted-foreground' },
  on_track: { label: 'On track', color: 'text-green-600' },
  at_risk: { label: 'At risk', color: 'text-yellow-600' },
  off_track: { label: 'Off track', color: 'text-red-600' },
};

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
  'updated',
];

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
const FILTER_POPOVER_WIDTH = 'w-[220px]';

type EpicGroupBy = (typeof EPIC_GROUP_BY_OPTIONS)[number]['value'];
type EpicFilterKey =
  | 'state'
  | 'health'
  | 'team'
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

function buildSelectedObjectives(
  allObjectives: Objective[],
  linkedObjectives: EpicWithStats['objectives'],
): ObjectivePickerSelection[] {
  return (linkedObjectives ?? []).map((objective) => ({
    id: objective.id,
    name: objective.name,
    archived: !allObjectives.some((candidate) => candidate.id === objective.id),
  }));
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
  if (!matchesSelectedValue(entry.epic.team_id, filters.team)) return false;
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
  teamMap: Map<string, string>,
) {
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
      return entry.epic.owner_member_id ? ownerNameMap.get(entry.epic.owner_member_id) ?? '' : '';
    case 'objective':
      return (entry.objectives ?? []).map((objective) => objective.name).join(', ');
    case 'target_date':
      return entry.epic.deadline ?? '';
    case 'team':
      return entry.epic.team_id ? teamMap.get(entry.epic.team_id) ?? '' : '';
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
  return String(a).localeCompare(String(b), undefined, { sensitivity: 'base', numeric: true });
}

function sortEntriesForTable(
  entries: EpicWithStats[],
  sorting: SortingState,
  epicStates: EpicWorkflowState[],
  ownerNameMap: Map<string, string>,
  teamMap: Map<string, string>,
) {
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
  preserveEntryOrder = false,
): EpicGroup[] {
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

function loadEpicViewState(storageKey: string | null): { groupBy: EpicGroupBy; filters: EpicFilterState } {
  if (!storageKey) {
    return { groupBy: 'none', filters: {} };
  }
  try {
    const raw = localStorage.getItem(storageKey);
    if (!raw) return { groupBy: 'none', filters: {} };
    const parsed = JSON.parse(raw) as { groupBy?: EpicGroupBy; filters?: EpicFilterState };
    return {
      groupBy: EPIC_GROUP_BY_OPTIONS.some((option) => option.value === parsed.groupBy) ? parsed.groupBy! : 'none',
      filters: parsed.filters ?? {},
    };
  } catch {
    return { groupBy: 'none', filters: {} };
  }
}

function saveEpicViewState(storageKey: string | null, groupBy: EpicGroupBy, filters: EpicFilterState) {
  if (!storageKey) return;
  try {
    localStorage.setItem(storageKey, JSON.stringify({ groupBy, filters }));
  } catch {
    // Ignore local storage failures.
  }
}

function FilterValueSelect({
  definition,
  selected,
  onToggle,
}: {
  definition: EpicFilterDefinition;
  selected: string[];
  onToggle: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const selectedLabels = selected.map((value) => definition.options.find((option) => option.value === value)?.label ?? value);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1 rounded border border-border bg-background px-1.5 py-0.5 text-xs transition-colors hover:bg-accent"
        >
          {selectedLabels.length === 1 ? selectedLabels[0] : `${selectedLabels.length} selected`}
        </button>
      </PopoverTrigger>
      <PopoverContent className={FILTER_POPOVER_WIDTH} align="start">
        <Command>
          <CommandInput placeholder={`Search ${definition.label.toLowerCase()}...`} className="h-8 text-xs" />
          <CommandList>
            <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No results</CommandEmpty>
            <CommandGroup>
              {definition.options.map((option) => {
                const isSelected = selected.includes(option.value);
                return (
                  <CommandItem
                    key={option.value}
                    value={option.label}
                    className="flex items-center gap-2 text-xs"
                    onSelect={() => onToggle(option.value)}
                  >
                    <div className={`flex h-4 w-4 items-center justify-center rounded-sm border ${isSelected ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40'}`}>
                      {isSelected ? <Tick01Icon className="h-3 w-3" /> : null}
                    </div>
                    <span className="truncate">{option.label}</span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

function EpicFilterPill({
  definition,
  selected,
  onToggle,
  onRemove,
}: {
  definition: EpicFilterDefinition;
  selected: string[];
  onToggle: (value: string) => void;
  onRemove: () => void;
}) {
  return (
    <div className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs">
      <span className="font-medium text-muted-foreground">{definition.label}</span>
      <span className="text-muted-foreground/60">is</span>
      <FilterValueSelect definition={definition} selected={selected} onToggle={onToggle} />
      <button
        type="button"
        onClick={onRemove}
        className="rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-accent hover:text-foreground"
      >
        <Cancel01Icon className="h-3 w-3" />
      </button>
    </div>
  );
}

function EpicFilterTrigger({
  definitions,
  visibleKeys,
  onAdd,
  activeCount,
}: {
  definitions: EpicFilterDefinition[];
  visibleKeys: Set<EpicFilterKey>;
  onAdd: (key: EpicFilterKey) => void;
  activeCount: number;
}) {
  const [open, setOpen] = useState(false);
  const availableDefinitions = definitions.filter((definition) => !visibleKeys.has(definition.key));
  const canChooseFilter = availableDefinitions.length > 0;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 min-w-[88px] justify-between gap-2 px-2 text-xs text-muted-foreground"
          disabled={!canChooseFilter}
        >
          <span className="inline-flex items-center gap-1">
            <FilterHorizontalIcon className="h-3.5 w-3.5" />
            Filters
          </span>
          <Badge
            variant="secondary"
            className={`rounded-full px-1.5 py-0 text-[10px] transition-opacity ${activeCount > 0 ? 'opacity-100' : 'opacity-0'}`}
          >
            {activeCount || 0}
          </Badge>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-48 p-0" align="start">
        <Command>
          <CommandInput placeholder="Filter by..." />
          <CommandList>
            <CommandEmpty>No filters.</CommandEmpty>
            <CommandGroup>
              {availableDefinitions.map((definition) => (
                <CommandItem
                  key={definition.key}
                  value={definition.label}
                  onSelect={() => {
                    onAdd(definition.key);
                    setOpen(false);
                  }}
                >
                  {definition.label}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
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
  onToggleGroup,
}: EpicVirtualTableProps) {
  const parentRef = useRef<HTMLDivElement>(null);
  const columnSizingVersion = useMemo(() => JSON.stringify(columnSizing), [columnSizing]);
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
    columnResizeMode: 'onEnd',
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
    (index: number) => (items[index]?.type === 'group' ? GROUP_ROW_HEIGHT : ROW_HEIGHT),
    [items],
  );

  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => parentRef.current,
    estimateSize,
    overscan: groupBy === 'none' ? 8 : 5,
  });

  return (
    <div className="-mt-2 min-h-0 flex-1 rounded-lg border border-border">
      <div
        ref={parentRef}
        className={TABLE_CONTAINER}
        style={{ maxHeight: 'calc(100vh - 220px)' }}
      >
        <div className="min-w-fit">
          <div className={TABLE_HEADER}>
            {table.getHeaderGroups().map((headerGroup) => (
              <div key={headerGroup.id} className="flex items-center">
                {headerGroup.headers.map((header) => {
                  const defSize = header.column.columnDef.size ?? 150;
                  const runtimeSize = header.getSize();
                  const isResized = !!columnSizing[header.column.id];
                  const canSort = header.column.getCanSort();
                  const sorted = header.column.getIsSorted();
                  return (
                    <div
                      key={header.id}
                      className={`${TABLE_HEADER_CELL} ${canSort ? TABLE_HEADER_CELL_SORTABLE : ''}`}
                      style={dynamicCellStyle(defSize, runtimeSize, isResized, 200)}
                      onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                    >
                      <div className="flex items-center gap-1 overflow-hidden whitespace-nowrap">
                        {header.isPlaceholder
                          ? null
                          : flexRender(header.column.columnDef.header, header.getContext())}
                        {canSort ? (
                          <span className="ml-auto shrink-0">
                            {sorted === 'asc' ? (
                              <ArrowUp02Icon className="h-3 w-3 text-foreground/80 stroke-[2.5]" />
                            ) : sorted === 'desc' ? (
                              <ArrowDown02Icon className="h-3 w-3 text-foreground/80 stroke-[2.5]" />
                            ) : (
                              <ArrowUpDownIcon className="h-3 w-3 text-muted-foreground stroke-[2]" />
                            )}
                          </span>
                        ) : null}
                      </div>
                      {header.column.getCanResize() ? (
                        <div
                          onMouseDown={header.getResizeHandler()}
                          onTouchStart={header.getResizeHandler()}
                          onClick={(event) => event.stopPropagation()}
                          className={`${TABLE_RESIZE_HANDLE} ${header.column.getIsResizing() ? 'bg-primary/50' : ''}`}
                        />
                      ) : null}
                    </div>
                  );
                })}
              </div>
            ))}
          </div>

          <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative', width: '100%' }}>
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const item = items[virtualRow.index];
              if (!item) return null;
              return (
                <div
                  key={`${item.type}-${item.key}`}
                  data-index={virtualRow.index}
                  style={virtualRowStyle(virtualRow.start)}
                >
                  {item.type === 'group' ? (
                    <MemoEpicGroupRow item={item} onToggle={onToggleGroup} />
                  ) : (
                    <MemoEpicDataRow
                      row={item.row}
                      onRowClick={onRowClick}
                      columnSizing={columnSizing}
                      columnSizingVersion={columnSizingVersion}
                    />
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
    <button
      type="button"
      className={`${TABLE_GROUP_ROW} w-full text-left text-xs`}
      onClick={() => onToggle(item.key)}
    >
      <span className={TABLE_GROUP_ROW_INNER}>
        {item.collapsed ? (
          <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground" />
        ) : (
          <ArrowDown01Icon className="h-3.5 w-3.5 text-muted-foreground" />
        )}
        <span className="min-w-0 truncate font-medium">{item.label}</span>
        <span className="ml-1 flex items-center gap-3 font-normal text-muted-foreground">
          <span>{item.entryCount} {item.entryCount === 1 ? 'epic' : 'epics'}</span>
          <span>{item.totalTasks} tasks</span>
          <span>{item.completedPoints}/{item.totalPoints} points</span>
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
}

function areEpicDataRowPropsEqual(prev: EpicDataRowProps, next: EpicDataRowProps) {
  return (
    prev.row.id === next.row.id &&
    prev.row.original === next.row.original &&
    prev.columnSizingVersion === next.columnSizingVersion &&
    prev.onRowClick === next.onRowClick
  );
}

const MemoEpicDataRow = memo(function EpicDataRow({
  row,
  onRowClick,
  columnSizing,
  columnSizingVersion,
}: EpicDataRowProps) {
  void columnSizingVersion;
  return (
    <div
      className={`${TABLE_ROW} cursor-pointer`}
      onClick={() => onRowClick(row.original)}
    >
      {row.getVisibleCells().map((cell) => {
        const { defSize, runtimeSize, isResized } = resolveColumnRuntimeSize(cell.column, columnSizing);
        return (
          <div
            key={cell.id}
            className={`${TABLE_CELL} overflow-hidden`}
            style={dynamicCellStyle(defSize, runtimeSize, isResized, 200)}
          >
            {flexRender(cell.column.columnDef.cell, cell.getContext())}
          </div>
        );
      })}
    </div>
  );
}, areEpicDataRowPropsEqual);

function InlineEpicStateCell({
  entry,
  epicStateMap,
  epicStates,
  onUpdate,
}: {
  entry: EpicWithStats;
  epicStateMap: Map<string, EpicWorkflowState>;
  epicStates: EpicWorkflowState[];
  onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const state = entry.epic.epic_state_id ? epicStateMap.get(entry.epic.epic_state_id) : null;
  const stateCfg = state ? STATE_TYPE_ICON_CONFIG[state.state_type as StateType] : null;
  const StateIcon = stateCfg?.icon ?? MinusSignIcon;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
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
        <PopoverContent
          className="w-[220px] p-0"
          align="start"
          side="bottom"
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <Command>
            <CommandInput placeholder="Search states..." className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No states found</CommandEmpty>
              <CommandGroup>
                {epicStates.map((epicState) => {
                  const optionCfg = STATE_TYPE_ICON_CONFIG[epicState.state_type as StateType];
                  const OptionIcon = optionCfg?.icon ?? MinusSignIcon;
                  return (
                    <CommandItem
                      key={epicState.id}
                      value={epicState.name}
                      className="flex items-center gap-2 text-xs"
                      onSelect={() => {
                        if (epicState.id !== entry.epic.epic_state_id) {
                          void onUpdate(entry.epic.id, { epic_state_id: epicState.id });
                        }
                        setOpen(false);
                      }}
                    >
                      <OptionIcon className={`h-3.5 w-3.5 shrink-0 ${optionCfg?.color ?? 'text-muted-foreground'}`} />
                      <span className="truncate">{epicState.name}</span>
                      {entry.epic.epic_state_id === epicState.id ? (
                        <Tick01Icon className="ml-auto h-3.5 w-3.5 text-primary" />
                      ) : null}
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

function InlineEpicHealthCell({
  entry,
  onUpdate,
}: {
  entry: EpicWithStats;
  onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const health = entry.epic.health;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(event) => {
            event.stopPropagation();
            setOpen(true);
          }}
        >
          <span className={healthConfig[health]?.color ?? 'text-muted-foreground'}>
            {healthConfig[health]?.label ?? 'No health'}
          </span>
        </button>
      </PopoverTrigger>
      {open ? (
        <PopoverContent
          className="w-[180px] p-0"
          align="start"
          side="bottom"
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <Command>
            <CommandInput placeholder="Search health..." className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No match</CommandEmpty>
              <CommandGroup>
                {ALL_HEALTH_OPTIONS.map((option) => (
                  <CommandItem
                    key={option}
                    value={healthConfig[option].label}
                    className="flex items-center gap-2 text-xs"
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

function InlineEpicOwnerCell({
  entry,
  assignableMembers,
  ownerNameMap,
  onUpdate,
}: {
  entry: EpicWithStats;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void>;
}) {
  const ownerKey = entry.epic.owner_member_id;
  const ownerName = ownerKey ? ownerNameMap.get(ownerKey) ?? 'Unknown' : null;

  return (
    <MemberPickerPopover
      value={entry.epic.owner_member_id || '__none__'}
      members={assignableMembers}
      noneLabel="Unassigned"
      onChange={(value) => {
        void onUpdate(entry.epic.id, { owner_member_id: value === '__none__' ? '' : value });
      }}
      renderTrigger={() => {
        const selectedMember = findAssignableMember(assignableMembers, entry.epic.owner_member_id);
        return selectedMember ? (
          <>
            <UserAvatar
              name={selectedMember.display_name || selectedMember.email}
              avatarUrl={selectedMember.avatar_url}
              avatarStyle={selectedMember.avatar_style}
              avatarSeed={selectedMember.avatar_seed}
              avatarBackgroundMode={selectedMember.avatar_background_mode}
              avatarBackgroundColor={selectedMember.avatar_background_color}
              className="h-4 w-4"
              fallbackClassName="text-[7px]"
            />
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

function InlineEpicTeamCell({
  entry,
  teams,
  teamMap,
  onUpdate,
}: {
  entry: EpicWithStats;
  teams: WorkspaceTeam[];
  teamMap: Map<string, string>;
  onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const teamName = entry.epic.team_id ? teamMap.get(entry.epic.team_id) ?? 'Unknown' : null;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(event) => {
            event.stopPropagation();
            setOpen(true);
          }}
        >
          {teamName ? (
            <span className="truncate">{teamName}</span>
          ) : (
            <span className="text-muted-foreground">No team</span>
          )}
        </button>
      </PopoverTrigger>
      {open ? (
        <PopoverContent
          className="w-[220px] p-0"
          align="start"
          side="bottom"
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <Command>
            <CommandInput placeholder="Search teams..." className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No teams found</CommandEmpty>
              <CommandGroup>
                {teams.map((team) => (
                  <CommandItem
                    key={team.id}
                    value={team.name}
                    className="flex items-center gap-2 text-xs"
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

function InlineEpicDateCell({
  epicId,
  value,
  emptyLabel,
  onUpdate,
  patchKey,
}: {
  epicId: string;
  value?: string;
  emptyLabel: string;
  onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void>;
  patchKey: 'deadline' | 'planned_start_date';
}) {
  const [open, setOpen] = useState(false);
  const selected = value ? parseISO(value) : undefined;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(event) => {
            event.stopPropagation();
            setOpen(true);
          }}
        >
          <Calendar03Icon className="h-3.5 w-3.5 text-muted-foreground" />
          <span className="text-muted-foreground">
            {selected ? format(selected, 'MMM d') : emptyLabel}
          </span>
        </button>
      </PopoverTrigger>
      {open ? (
        <PopoverContent
          className="w-auto p-0"
          align="start"
          side="bottom"
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <Calendar
            mode="single"
            selected={selected}
            defaultMonth={selected}
            onSelect={(date) => {
              void onUpdate(epicId, { [patchKey]: date ? format(date, 'yyyy-MM-dd') : undefined });
              setOpen(false);
            }}
          />
        </PopoverContent>
      ) : null}
    </Popover>
  );
}

function InlineEpicLabelsCell({
  entry,
  workspaceId,
  allLabels,
  onLabelsChange,
  onUpdate,
}: {
  entry: EpicWithStats;
  workspaceId: string;
  allLabels: Label[];
  onLabelsChange: (labels: Label[]) => void;
  onUpdate: (epicId: string, patch: UpdateEpicRequest) => Promise<void>;
}) {
  const labels = entry.labels ?? [];
  const selectedLabelIds = labels.map((label) => label.id);

  return (
    <div onClick={(event) => event.stopPropagation()} className="group/lbl flex min-w-0 items-center gap-1">
      {labels.length > 0 ? (
        <div className="flex min-w-0 items-center gap-1.5" title={labels.map((l) => l.name).join(', ')}>
          <span
            className="h-2 w-2 shrink-0 rounded-full"
            style={{ backgroundColor: labels[0].color ? (labels[0].color.startsWith('#') ? labels[0].color : `#${labels[0].color}`) : 'var(--muted-foreground)' }}
          />
          <span className="truncate text-xs text-muted-foreground">
            {labels[0].name}{labels.length > 1 ? ` +${labels.length - 1} more` : ''}
          </span>
        </div>
      ) : null}
      <div className={labels.length > 0 ? 'opacity-0 group-hover/lbl:opacity-100 transition-opacity shrink-0' : 'shrink-0'}>
        <LabelPicker
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

function InlineEpicObjectivesCell({
  entry,
  allObjectives,
  selectedObjectives,
  onChange,
}: {
  entry: EpicWithStats;
  allObjectives: Objective[];
  selectedObjectives: ObjectivePickerSelection[];
  onChange: (epicId: string, objectiveIds: string[]) => Promise<void>;
}) {
  const objectives = entry.objectives ?? [];
  return (
    <div onClick={(event) => event.stopPropagation()} className="group/obj flex min-w-0 items-center gap-1">
      {objectives.length > 0 ? (
        <span className="truncate text-xs text-muted-foreground" title={objectives.map((o) => o.name).join(', ')}>
          {objectives[0].name}{objectives.length > 1 ? ` +${objectives.length - 1} more` : ''}
        </span>
      ) : null}
      <div className={objectives.length > 0 ? 'opacity-0 group-hover/obj:opacity-100 transition-opacity shrink-0' : 'shrink-0'}>
        <ObjectivePicker
          objectives={allObjectives}
          selectedObjectiveIds={objectives.map((objective) => objective.id)}
          selectedObjectives={selectedObjectives}
          onChange={(objectiveIds) => onChange(entry.epic.id, objectiveIds)}
          addLabel={objectives.length === 0 ? 'Add objective' : 'Edit'}
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

  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [allLabels, setAllLabels] = useState<Label[]>([]);
  const [allObjectives, setAllObjectives] = useState<Objective[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [groupBy, setGroupBy] = useState<EpicGroupBy>('none');
  const [filters, setFilters] = useState<EpicFilterState>({});
  const [tableSorting, setTableSorting] = useState<SortingState>([]);
  const [tableColumnSizing, setTableColumnSizing] = useState<ColumnSizingState>({});
  const [collapsedGroupKeys, setCollapsedGroupKeys] = useState<Set<string>>(new Set());
  const deferredSearch = useDeferredValue(search);

  const workspaceId = workspace?.id;
  const slug = workspace?.slug;
  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canEdit } = usePermissions(access);
  const { teams, findTeamName } = useAccessibleTeams(workspaceId ?? '');
  const isSingleTeam = teams.length <= 1;
  const storageKey = useMemo(
    () => (workspaceId ? `pm_epics_view_${workspaceId}_${teamId ?? 'all'}` : null),
    [workspaceId, teamId],
  );

  const [visibleColumns, setVisibleColumns] = useState<string[]>(DEFAULT_VISIBLE);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const { data: epicStates = [] } = useEpicStates(workspaceId ?? '');
  const ownerNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
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

  const filterDefinitions = useMemo<EpicFilterDefinition[]>(() => [
    {
      key: 'state',
      label: 'State',
      options: [
        ...epicStates.map((state) => ({ value: state.id, label: state.name })),
        { value: '__archived__', label: 'Archived' },
      ],
    },
    {
      key: 'health',
      label: 'Health',
      options: ALL_HEALTH_OPTIONS.map((health) => ({ value: health, label: healthConfig[health].label })),
    },
    {
      key: 'team',
      label: 'Team',
      options: teams.map((team) => ({ value: team.id, label: team.name })),
    },
    {
      key: 'owner',
      label: 'Owner',
      options: assignableMembers.map((member) => ({
        value: member.id,
        label: member.display_name || member.email,
      })),
    },
    {
      key: 'label',
      label: 'Label',
      options: allLabels.map((label) => ({ value: label.id, label: label.name })),
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
  ], [allLabels, assignableMembers, epicStates, objectiveNameMap, teams]);

  const activeFilterKeys = useMemo(
    () => new Set(Object.entries(filters).filter(([, value]) => value && value.length > 0).map(([key]) => key as EpicFilterKey)),
    [filters],
  );
  const visibleFilterKeys = useMemo(
    () => new Set(Object.keys(filters).map((key) => key as EpicFilterKey)),
    [filters],
  );

  const updateEpicField = useCallback(
    async (epicId: string, patch: UpdateEpicRequest) => {
      setError(null);

      let snapshot: EpicWithStats[] = [];
      setEpics((current) => {
        snapshot = current;
        return current.map((entry) => (
          entry.epic.id === epicId
            ? applyEpicPatch(entry, patch, allLabels)
            : entry
        ));
      });

      const { data, error: updateError } = await pmEpicService.update(workspaceId!, epicId, patch);
      if (updateError || !data) {
        setEpics(snapshot);
        setError(updateError ?? 'Failed to update epic');
        return;
      }

      setEpics((current) => current.map((entry) => (entry.epic.id === epicId ? data : entry)));
    },
    [allLabels, workspaceId],
  );

  const updateEpicObjectives = useCallback(
    async (epicId: string, nextObjectiveIds: string[]) => {
      const currentEntry = epics.find((entry) => entry.epic.id === epicId);
      if (!currentEntry || !workspaceId) return;

      setError(null);
      const snapshot = epics;
      const currentObjectiveIds = (currentEntry.objectives ?? []).map((objective) => objective.id);
      const currentSet = new Set(currentObjectiveIds);
      const nextSet = new Set(nextObjectiveIds);
      const toAdd = nextObjectiveIds.filter((id) => !currentSet.has(id));
      const toRemove = currentObjectiveIds.filter((id) => !nextSet.has(id));

      setEpics((current) => current.map((entry) => (
        entry.epic.id === epicId
          ? { ...entry, objectives: buildObjectiveRefs(allObjectives, nextObjectiveIds) }
          : entry
      )));

      const results = await Promise.all([
        ...toAdd.map((objectiveId) => pmObjectiveService.addEpic(workspaceId, objectiveId, epicId)),
        ...toRemove.map((objectiveId) => pmObjectiveService.removeEpic(workspaceId, objectiveId, epicId)),
      ]);

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
    [allObjectives, epics, workspaceId],
  );

  const columns = useMemo(
    () => [
      columnHelper.accessor((row) => row.epic.name, {
        id: 'name',
        header: 'Name',
        size: 280,
        cell: (info) => (
          <div className="flex max-w-full items-center gap-2.5 text-sm">
            <Layers01Icon className="h-4 w-4 shrink-0 text-violet-500" />
            <span className="min-w-0 truncate font-normal">{info.getValue()}</span>
          </div>
        ),
      }),
      columnHelper.accessor(
        (row) => epicStateMap.get(row.epic.epic_state_id ?? '')?.position ?? Number.MAX_SAFE_INTEGER,
        {
          id: 'state',
        header: 'State',
        size: 140,
        cell: (info) => {
          if (canEdit) {
            return (
              <InlineEpicStateCell
                entry={info.row.original}
                epicStateMap={epicStateMap}
                epicStates={epicStates}
                onUpdate={updateEpicField}
              />
            );
          }

          const state = info.row.original.epic.epic_state_id
            ? epicStateMap.get(info.row.original.epic.epic_state_id)
            : null;
          const stateCfg = state
            ? STATE_TYPE_ICON_CONFIG[state.state_type as StateType]
            : null;
          const StateIcon = stateCfg?.icon ?? MinusSignIcon;
          return (
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <StateIcon className={`h-3.5 w-3.5 shrink-0 ${stateCfg?.color ?? 'text-muted-foreground'}`} />
              <span className="truncate">{state?.name ?? 'No state'}</span>
            </div>
          );
        },
        },
      ),
      columnHelper.accessor((row) => ALL_HEALTH_OPTIONS.indexOf(row.epic.health), {
        id: 'health',
        header: 'Health',
        size: 96,
        cell: (info) => (
          canEdit ? (
            <InlineEpicHealthCell entry={info.row.original} onUpdate={updateEpicField} />
          ) : (
            <span className={`text-xs ${healthConfig[info.row.original.epic.health]?.color ?? 'text-muted-foreground'}`}>
              {healthConfig[info.row.original.epic.health]?.label ?? 'No updates'}
            </span>
          )
        ),
      }),
      columnHelper.accessor((row) => epicCompletionPct(row), {
        id: 'progress',
        header: 'Progress',
        size: 90,
        cell: (info) => (
          <div className="flex items-center gap-1.5 text-xs">
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
          <span className="text-xs text-muted-foreground">
            {epicDoneTaskCount(info.row.original)}/{epicTaskCount(info.row.original)}
          </span>
        ),
      }),
      columnHelper.accessor((row) => row.stats.total_points, {
        id: 'points',
        header: 'Points',
        size: 80,
        cell: (info) => (
          <span className="text-xs text-muted-foreground">
            {info.row.original.stats.done_points}/{info.row.original.stats.total_points}
          </span>
        ),
      }),
      columnHelper.accessor((row) => (row.epic.owner_member_id ? ownerNameMap.get(row.epic.owner_member_id) ?? '' : ''), {
        id: 'owner',
        header: 'Owner',
        size: 140,
        cell: (info) => {
          if (canEdit) {
            return (
              <InlineEpicOwnerCell
                entry={info.row.original}
                assignableMembers={assignableMembers}
                ownerNameMap={ownerNameMap}
                onUpdate={updateEpicField}
              />
            );
          }

          const entry = info.row.original;
          const ownerName = entry.epic.owner_member_id ? ownerNameMap.get(entry.epic.owner_member_id) : null;
          return ownerName ? (
            <div className="flex items-center gap-2 min-w-0">
              <UserAvatar name={ownerName} />
              <span className="truncate text-xs text-muted-foreground">{ownerName}</span>
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
            return (
              <InlineEpicObjectivesCell
                entry={entry}
                allObjectives={allObjectives}
                selectedObjectives={buildSelectedObjectives(allObjectives, entry.objectives)}
                onChange={updateEpicObjectives}
              />
            );
          }
          return objectives.length > 0 ? (
            <span className="truncate text-xs text-muted-foreground block" title={objectives.map((o) => o.name).join(', ')}>
              {objectives[0].name}{objectives.length > 1 ? ` +${objectives.length - 1} more` : ''}
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
        cell: (info) => (
          canEdit ? (
            <InlineEpicDateCell
              epicId={info.row.original.epic.id}
              value={info.row.original.epic.deadline}
              emptyLabel="No date"
              patchKey="deadline"
              onUpdate={updateEpicField}
            />
          ) : (
            <div className="flex items-center gap-1 text-xs text-muted-foreground">
              {info.row.original.epic.deadline ? (
                <>
                  <Calendar03Icon className="h-3.5 w-3.5" />
                  <span>{format(parseISO(info.row.original.epic.deadline), 'MMM d')}</span>
                </>
              ) : (
                <MinusSignIcon className="h-3.5 w-3.5" />
              )}
            </div>
          )
        ),
      }),
      columnHelper.accessor((row) => (row.epic.team_id ? teamMap.get(row.epic.team_id) ?? '' : ''), {
        id: 'team',
        header: 'Team',
        size: 110,
        cell: (info) => (
          canEdit ? (
            <InlineEpicTeamCell
              entry={info.row.original}
              teams={teams}
              teamMap={teamMap}
              onUpdate={updateEpicField}
            />
          ) : (
            <span className="truncate text-xs text-muted-foreground">
              {findTeamName(info.row.original.epic.team_id) || <MinusSignIcon className="h-3.5 w-3.5" />}
            </span>
          )
        ),
      }),
      columnHelper.accessor((row) => row.epic.planned_start_date ?? '', {
        id: 'start_date',
        header: 'Start date',
        size: 110,
        cell: (info) => (
          canEdit ? (
            <InlineEpicDateCell
              epicId={info.row.original.epic.id}
              value={info.row.original.epic.planned_start_date}
              emptyLabel="No date"
              patchKey="planned_start_date"
              onUpdate={updateEpicField}
            />
          ) : (
            <div className="flex items-center gap-1 text-xs text-muted-foreground">
              {info.row.original.epic.planned_start_date ? (
                <>
                  <Calendar03Icon className="h-3.5 w-3.5" />
                  <span>{format(parseISO(info.row.original.epic.planned_start_date), 'MMM d')}</span>
                </>
              ) : (
                <MinusSignIcon className="h-3.5 w-3.5" />
              )}
            </div>
          )
        ),
      }),
      columnHelper.accessor((row) => (row.labels ?? []).map((label) => label.name).join(', '), {
        id: 'labels',
        header: 'Labels',
        size: 220,
        cell: (info) => (
          canEdit ? (
            <InlineEpicLabelsCell
              entry={info.row.original}
              workspaceId={workspaceId!}
              allLabels={allLabels}
              onLabelsChange={setAllLabels}
              onUpdate={updateEpicField}
            />
          ) : (
            (() => {
              const lbls = info.row.original.labels ?? [];
              return lbls.length > 0 ? (
                <div className="flex min-w-0 items-center gap-1.5" title={lbls.map((l) => l.name).join(', ')}>
                  <span
                    className="h-2 w-2 shrink-0 rounded-full"
                    style={{ backgroundColor: lbls[0].color ? (lbls[0].color.startsWith('#') ? lbls[0].color : `#${lbls[0].color}`) : 'var(--muted-foreground)' }}
                  />
                  <span className="truncate text-xs text-muted-foreground">
                    {lbls[0].name}{lbls.length > 1 ? ` +${lbls.length - 1} more` : ''}
                  </span>
                </div>
              ) : (
                <MinusSignIcon className="h-3.5 w-3.5 text-muted-foreground" />
              );
            })()
          )
        ),
      }),
      columnHelper.accessor((row) => row.epic.created_at, {
        id: 'created',
        header: 'Created',
        size: 100,
        cell: (info) => (
          <span className="text-xs text-muted-foreground">
            {format(parseISO(info.row.original.epic.created_at), 'MMM d')}
          </span>
        ),
      }),
      columnHelper.accessor((row) => row.epic.updated_at, {
        id: 'updated',
        header: 'Updated',
        size: 100,
        cell: (info) => (
          <span className="text-xs text-muted-foreground">
            {format(parseISO(info.row.original.epic.updated_at), 'MMM d')}
          </span>
        ),
      }),
    ],
    [
      allLabels,
      allObjectives,
      assignableMembers,
      canEdit,
      columnHelper,
      epicStateMap,
      epicStates,
      findTeamName,
      ownerNameMap,
      teamMap,
      teams,
      updateEpicField,
      updateEpicObjectives,
      workspaceId,
    ],
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

  const showArchived = filters.state?.includes('__archived__') ?? false;

  const loadData = useCallback(async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const [epicsRes, labelsRes, objectivesRes] = await Promise.all([
      pmEpicService.list(workspaceId, { archived: showArchived, team_id: teamId }),
      pmLabelService.list(workspaceId),
      pmObjectiveService.list(workspaceId, { archived: false }),
    ]);
    if (epicsRes.error || !epicsRes.data) {
      setError(epicsRes.error ?? 'Failed to load epics');
      setLoading(false);
      return;
    }
    setEpics(epicsRes.data);
    setAllLabels(labelsRes.data ?? []);
    setAllObjectives((objectivesRes.data ?? []).map((entry) => entry.objective));
    setLoading(false);
  }, [teamId, workspaceId, showArchived]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  useEffect(() => {
    const next = loadEpicViewState(storageKey);
    setGroupBy(next.groupBy);
    setFilters(next.filters);
  }, [storageKey]);

  useEffect(() => {
    saveEpicViewState(storageKey, groupBy, filters);
  }, [filters, groupBy, storageKey]);

  // Refresh when epic is created via global modal
  useEffect(() => {
    const handler = () => { void loadData(); };
    window.addEventListener('epic-created', handler);
    return () => window.removeEventListener('epic-created', handler);
  }, [loadData]);

  const openEpic = useCallback((entry: EpicWithStats) => {
    if (!slug) return;
    navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug, epicId: entry.epic.id } });
  }, [navigate, slug]);

  const handleAddFilter = useCallback((key: EpicFilterKey) => {
    const definition = filterDefinitions.find((item) => item.key === key);
    if (!definition || definition.options.length === 0) return;
    setFilters((current) => ({ ...current, [key]: current[key] ?? [] }));
  }, [filterDefinitions]);

  const handleToggleFilterValue = useCallback((key: EpicFilterKey, value: string) => {
    setFilters((current) => {
      const nextValues = current[key]?.includes(value)
        ? current[key]!.filter((item) => item !== value)
        : [...(current[key] ?? []), value];
      return { ...current, [key]: nextValues };
    });
  }, []);

  const handleRemoveFilter = useCallback((key: EpicFilterKey) => {
    setFilters((current) => {
      const { [key]: _removed, ...rest } = current;
      return rest;
    });
  }, []);

  const handleClearFilters = useCallback(() => {
    setFilters({});
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

  const sortedEpics = useMemo(
    () => sortEntriesForTable(filteredEpics, tableSorting, epicStates, ownerNameMap, teamMap),
    [epicStates, filteredEpics, ownerNameMap, tableSorting, teamMap],
  );

  const groupedEpics = useMemo(
    () => buildEpicGroups(
      sortedEpics,
      groupBy,
      filters,
      epicStates,
      ownerNameMap,
      teamMap,
      objectiveNameMap,
      tableSorting.length > 0,
    ),
    [epicStates, filters, groupBy, objectiveNameMap, ownerNameMap, sortedEpics, tableSorting.length, teamMap],
  );
  useEffect(() => {
    setCollapsedGroupKeys((current) => {
      const validKeys = new Set(groupedEpics.map((group) => group.key));
      return new Set([...current].filter((key) => validKeys.has(key)));
    });
  }, [groupedEpics]);

  const teamLabel = teamId ? findTeamName(teamId) : null;
  const showHeaderIntro = epics.length > 0;
  const showHeaderActions = epics.length > 0;

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        {showHeaderIntro ? (
          <div>
            <h2 className="text-xl font-semibold">Epics{teamLabel && <span className="text-muted-foreground font-normal"> ({teamLabel})</span>}</h2>
            <p className="text-sm text-muted-foreground">Track long-running initiatives and their task progress.</p>
          </div>
        ) : <div />}
        {showHeaderActions && canEdit ? (
          <Button size="sm" className="gap-2" onClick={() => openCreate('epic', { teamId })}>
            <PlusSignIcon className="h-4 w-4" />
            Create Epic
          </Button>
        ) : null}
      </header>

      {error ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {showHeaderActions ? (
        <div className="flex flex-wrap items-center gap-2 px-3 py-2">
          <div className="relative min-w-[160px] flex-1 sm:max-w-[220px]">
            <Search01Icon className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="Search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="h-8 pl-8 pr-8 text-sm"
            />
            {search ? (
              <button
                type="button"
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-0.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                onClick={() => setSearch('')}
                aria-label="Clear search"
              >
                <Cancel01Icon className="h-3.5 w-3.5" />
              </button>
            ) : null}
          </div>
          <EpicFilterTrigger
            definitions={filterDefinitions}
            visibleKeys={visibleFilterKeys}
            activeCount={activeFilterKeys.size}
            onAdd={handleAddFilter}
          />
          <span className="text-xs text-muted-foreground">
            {sortedEpics.length} {sortedEpics.length === 1 ? 'epic' : 'epics'}
          </span>
          <div className="ml-auto flex items-center gap-1.5">
            <Select value={groupBy} onValueChange={(value) => setGroupBy(value as EpicGroupBy)}>
              <SelectTrigger className="h-7 w-auto min-w-[138px] max-w-[190px] gap-1 text-xs">
                <span className="shrink-0 text-muted-foreground">Group by</span>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {EPIC_GROUP_BY_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <DisplayPropertiesPopover
              allProperties={ALL_PROPERTIES}
              visible={visibleColumns}
              onChange={setVisibleColumns}
              iconOnly
            />
          </div>
          {visibleFilterKeys.size > 0 ? (
            <div className="flex basis-full flex-wrap items-center gap-1.5 pt-0.5">
              {filterDefinitions
                .filter((definition) => visibleFilterKeys.has(definition.key))
                .map((definition) => (
                  <EpicFilterPill
                    key={definition.key}
                    definition={definition}
                    selected={filters[definition.key] ?? []}
                    onToggle={(value) => handleToggleFilterValue(definition.key, value)}
                    onRemove={() => handleRemoveFilter(definition.key)}
                  />
                ))}
              <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px] text-muted-foreground" onClick={handleClearFilters}>
                Clear all
              </Button>
            </div>
          ) : null}
        </div>
      ) : null}

      {loading ? null : epics.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 px-4">
          <div className="flex h-14 w-14 items-center justify-center rounded-full bg-violet-500/10 mb-5">
            <Layers01Icon className="h-7 w-7 text-violet-500" />
          </div>
          <h3 className="text-lg font-semibold mb-1.5">Create your first epic</h3>
          <p className="text-sm text-muted-foreground text-center max-w-md mb-6">
            Epics group related tasks into long-running initiatives, giving you a high-level view of progress across your team's work.
          </p>
          <Button
            className="gap-2 mb-8"
            onClick={() => openCreate('epic', { teamId })}
          >
            <PlusSignIcon className="h-4 w-4" />
            Create Epic
          </Button>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 w-full max-w-4xl">
            {[
              { icon: Layers01Icon, title: 'Group tasks', desc: 'Organize related work items under a single initiative' },
              { icon: ChartIncreaseIcon, title: 'Track health', desc: 'Monitor on-track, at-risk, and off-track status at a glance' },
              { icon: Target01Icon, title: 'Hit deadlines', desc: 'Set target dates and watch completion progress in real time' },
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
        <EpicVirtualTable
          data={sortedEpics}
          groups={groupedEpics}
          groupBy={groupBy}
          collapsedGroupKeys={collapsedGroupKeys}
          columns={columns}
          columnVisibility={columnVisibility}
          onRowClick={openEpic}
          sorting={tableSorting}
          onSortingChange={setTableSorting}
          columnSizing={tableColumnSizing}
          onColumnSizingChange={setTableColumnSizing}
          onToggleGroup={handleToggleGroup}
        />
      )}
    </div>
  );
}
