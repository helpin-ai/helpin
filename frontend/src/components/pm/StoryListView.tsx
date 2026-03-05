import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  useReactTable,
  getCoreRowModel,
  getGroupedRowModel,
  getExpandedRowModel,
  flexRender,
  createColumnHelper,
  type GroupingState,
  type ExpandedState,
  type Row,
} from '@tanstack/react-table';
import { useVirtualizer } from '@tanstack/react-virtual';
import { ChevronDown, ChevronRight, Loader2 } from 'lucide-react';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { pmStoryService } from '@/lib/services/pmStoryService';
import {
  PriorityIcon,
  SeverityIcon,
  StoryTypeIcon,
  StateTypeIcon,
  PRIORITY_CONFIG,
  SEVERITY_CONFIG,
  STORY_TYPE_CONFIG,
} from '@/lib/pmConstants';
import type {
  Story,
  WorkflowWithStates,
  EpicWithStats,
  SprintWithStats,
} from '@/lib/pmTypes';
import type { MemberWithUser, WorkspaceTeam } from '@/lib/types';
import type { BoardFilters } from '@/stores/pmBoardStore';

interface StoryListViewProps {
  workspaceId: string;
  workflow: WorkflowWithStates;
  teams: WorkspaceTeam[];
  members: MemberWithUser[];
  epics: EpicWithStats[];
  sprints: SprintWithStats[];
  filters: BoardFilters;
  teamId: string | null;
  onOpenStory: (story: Story) => void;
}

type GroupByOption =
  | 'none'
  | 'workflow_state'
  | 'story_type'
  | 'priority'
  | 'severity'
  | 'epic'
  | 'sprint'
  | 'owner'
  | 'team';

const GROUP_BY_OPTIONS: { value: GroupByOption; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'workflow_state', label: 'Workflow State' },
  { value: 'story_type', label: 'Story Type' },
  { value: 'priority', label: 'Priority' },
  { value: 'severity', label: 'Severity' },
  { value: 'epic', label: 'Epic' },
  { value: 'sprint', label: 'Sprint' },
  { value: 'owner', label: 'Owner' },
  { value: 'team', label: 'Team' },
];

// Column accessor ID used for each group-by option
const GROUP_COLUMN_MAP: Record<GroupByOption, string | null> = {
  none: null,
  workflow_state: 'stateName',
  story_type: 'typeName',
  priority: 'priorityName',
  severity: 'severityName',
  epic: 'epicName',
  sprint: 'sprintName',
  owner: 'ownerName',
  team: 'teamName',
};

const columnHelper = createColumnHelper<Story>();

export function StoryListView({
  workspaceId,
  workflow,
  teams,
  members,
  epics,
  sprints,
  filters,
  teamId,
  onOpenStory,
}: StoryListViewProps) {
  const [stories, setStories] = useState<Story[]>([]);
  const [loading, setLoading] = useState(true);
  const [groupBy, setGroupBy] = useState<GroupByOption>('workflow_state');
  const [expanded, setExpanded] = useState<ExpandedState>(true);
  const parentRef = useRef<HTMLDivElement>(null);

  // Build lookup maps
  const stateMap = useMemo(() => {
    const map = new Map<string, { name: string; stateType: string }>();
    for (const s of workflow.states) {
      map.set(s.id, { name: s.name, stateType: s.state_type });
    }
    return map;
  }, [workflow.states]);

  const memberMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const m of members) {
      map.set(m.user_id, m.full_name);
    }
    return map;
  }, [members]);

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

  // Fetch stories
  const fetchStories = useCallback(async () => {
    setLoading(true);
    const apiFilters: Record<string, string | number | boolean | undefined> = {
      per_page: 500,
      workflow_id: workflow.workflow.id,
      archived: false,
      ...filters,
    };
    if (teamId) apiFilters.team_id = teamId;

    const res = await pmStoryService.list(workspaceId, apiFilters as Record<string, string>);
    if (res.data) {
      setStories(res.data.data);
    }
    setLoading(false);
  }, [workspaceId, workflow.workflow.id, filters, teamId]);

  useEffect(() => {
    fetchStories();
  }, [fetchStories]);

  // Listen for story events
  useEffect(() => {
    const handler = () => { fetchStories(); };
    window.addEventListener('story-created', handler);
    window.addEventListener('story-updated', handler);
    return () => {
      window.removeEventListener('story-created', handler);
      window.removeEventListener('story-updated', handler);
    };
  }, [fetchStories]);

  // Table columns
  const tableColumns = useMemo(
    () => [
      columnHelper.accessor('display_id', {
        id: 'displayId',
        header: 'ID',
        size: 80,
        cell: (info) => (
          <span className="font-mono text-xs text-muted-foreground">TP-{info.getValue()}</span>
        ),
      }),
      columnHelper.accessor('story_type', {
        id: 'typeIcon',
        header: '',
        size: 36,
        enableGrouping: false,
        cell: (info) => <StoryTypeIcon storyType={info.getValue()} className="h-4 w-4" />,
      }),
      columnHelper.accessor('name', {
        id: 'name',
        header: 'Name',
        size: 999,
        enableGrouping: false,
        cell: (info) => (
          <button
            className="max-w-full truncate text-left text-sm hover:text-primary hover:underline"
            onClick={(e) => {
              e.stopPropagation();
              onOpenStory(info.row.original);
            }}
          >
            {info.getValue()}
          </button>
        ),
      }),
      // Hidden grouping columns (values shown in group headers, not as table columns)
      columnHelper.accessor(
        (row) => stateMap.get(row.workflow_state_id)?.name ?? 'Unknown',
        {
          id: 'stateName',
          header: 'State',
          size: 130,
          cell: (info) => {
            const stateInfo = stateMap.get(info.row.original.workflow_state_id);
            return (
              <span className="flex items-center gap-1.5 text-xs">
                {stateInfo && (
                  <StateTypeIcon stateType={stateInfo.stateType as 'backlog' | 'unstarted' | 'started' | 'done'} className="h-3.5 w-3.5" />
                )}
                {info.getValue()}
              </span>
            );
          },
        }
      ),
      columnHelper.accessor('priority', {
        id: 'priorityIcon',
        header: 'Priority',
        size: 90,
        enableGrouping: false,
        cell: (info) => {
          const p = info.getValue();
          return (
            <span className="flex items-center gap-1.5 text-xs">
              <PriorityIcon priority={p} className="h-3.5 w-3.5" />
              {PRIORITY_CONFIG[p].label}
            </span>
          );
        },
      }),
      columnHelper.accessor('severity', {
        id: 'severityIcon',
        header: 'Severity',
        size: 90,
        enableGrouping: false,
        cell: (info) => {
          const s = info.getValue();
          if (s === 'none') return null;
          return (
            <span className="flex items-center gap-1.5 text-xs">
              <SeverityIcon severity={s} className="h-3.5 w-3.5" />
              {SEVERITY_CONFIG[s].label}
            </span>
          );
        },
      }),
      columnHelper.accessor('estimate', {
        id: 'estimate',
        header: 'Estimate',
        size: 70,
        enableGrouping: false,
        cell: (info) => {
          const v = info.getValue();
          return v != null ? <span className="text-xs text-muted-foreground">{v} pts</span> : null;
        },
      }),
      columnHelper.accessor(
        (row) => (row.owner_id ? memberMap.get(row.owner_id) ?? 'Unknown' : 'Unassigned'),
        {
          id: 'ownerName',
          header: 'Owner',
          size: 130,
          cell: (info) => <span className="truncate text-xs">{info.getValue()}</span>,
        }
      ),
      columnHelper.accessor(
        (row) => (row.team_id ? teamMap.get(row.team_id) ?? 'Unknown' : 'No Team'),
        {
          id: 'teamName',
          header: 'Team',
          size: 120,
          cell: (info) => {
            const v = info.getValue();
            return v !== 'No Team' ? <span className="truncate text-xs">{v}</span> : null;
          },
        }
      ),
      columnHelper.accessor(
        (row) => (row.epic_id ? epicMap.get(row.epic_id) ?? 'Unknown' : 'No Epic'),
        {
          id: 'epicName',
          header: 'Epic',
          size: 140,
          cell: (info) => {
            const v = info.getValue();
            return v !== 'No Epic' ? <span className="truncate text-xs">{v}</span> : null;
          },
        }
      ),
      columnHelper.accessor(
        (row) => (row.sprint_id ? sprintMap.get(row.sprint_id) ?? 'Unknown' : 'No Sprint'),
        {
          id: 'sprintName',
          header: 'Sprint',
          size: 130,
          cell: (info) => {
            const v = info.getValue();
            return v !== 'No Sprint' ? <span className="truncate text-xs">{v}</span> : null;
          },
        }
      ),
      columnHelper.accessor(
        (row) => (row.story_type ? STORY_TYPE_CONFIG[row.story_type].label : 'Unknown'),
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
        size: 100,
        enableGrouping: false,
        cell: (info) => {
          const v = info.getValue();
          if (!v) return null;
          const d = new Date(v);
          const now = new Date();
          const isOverdue = d < now;
          return (
            <span className={`text-xs ${isOverdue ? 'text-red-500' : 'text-muted-foreground'}`}>
              {d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
            </span>
          );
        },
      }),
    ],
    [stateMap, memberMap, teamMap, epicMap, sprintMap, onOpenStory]
  );

  // Hidden columns (for grouping only)
  const hiddenGroupColumns = ['typeName', 'priorityName', 'severityName'];

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
      columnVisibility: Object.fromEntries(hiddenGroupColumns.map((c) => [c, false])),
    },
    onExpandedChange: setExpanded,
    getExpandedRowModel: getExpandedRowModel(),
    getGroupedRowModel: getGroupedRowModel(),
    getCoreRowModel: getCoreRowModel(),
  });

  const { rows } = table.getRowModel();

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: (index) => {
      const row = rows[index];
      return row?.getIsGrouped() ? 40 : 36;
    },
    overscan: 20,
  });

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
      {/* Group By control */}
      <div className="flex items-center gap-2 px-3">
        <span className="text-xs text-muted-foreground">Group by:</span>
        <Select value={groupBy} onValueChange={(v) => setGroupBy(v as GroupByOption)}>
          <SelectTrigger className="h-7 w-[160px] text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {GROUP_BY_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <span className="text-xs text-muted-foreground">
          {stories.length} {stories.length === 1 ? 'story' : 'stories'}
        </span>
      </div>

      {/* Table */}
      <div className="min-h-0 flex-1 overflow-hidden rounded-md border border-border/70">
        {/* Header */}
        <div className="border-b border-border/70 bg-muted/50">
          {table.getHeaderGroups().map((headerGroup) => (
            <div key={headerGroup.id} className="flex items-center">
              {headerGroup.headers.map((header) => {
                if (header.column.getIsGrouped()) return null;
                const size = header.getSize();
                if (size === 0) return null;
                return (
                  <div
                    key={header.id}
                    className="px-2 py-1.5 text-xs font-medium text-muted-foreground"
                    style={{
                      width: size === 999 ? undefined : size,
                      flex: size === 999 ? '1 1 0%' : undefined,
                      minWidth: size === 999 ? 200 : undefined,
                    }}
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </div>
                );
              })}
            </div>
          ))}
        </div>

        {/* Virtualized body */}
        <div ref={parentRef} className="overflow-auto" style={{ height: 'calc(100% - 30px)' }}>
          <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative', width: '100%' }}>
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const row = rows[virtualRow.index] as Row<Story>;
              const isGrouped = row.getIsGrouped();

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
                    <GroupHeaderRow row={row} />
                  ) : (
                    <DataRow row={row} onOpenStory={onOpenStory} />
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

function GroupHeaderRow({ row }: { row: Row<Story> }) {
  return (
    <button
      className="flex w-full items-center gap-2 border-b border-border/50 bg-muted/30 px-3 py-2 text-left text-xs font-semibold hover:bg-muted/50"
      onClick={row.getToggleExpandedHandler()}
    >
      {row.getIsExpanded() ? (
        <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
      ) : (
        <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
      )}
      <span>{String(row.groupingValue)}</span>
      <span className="font-normal text-muted-foreground">({row.subRows.length})</span>
    </button>
  );
}

function DataRow({ row, onOpenStory }: { row: Row<Story>; onOpenStory: (story: Story) => void }) {
  return (
    <div
      className="flex cursor-pointer items-center border-b border-border/30 transition-colors hover:bg-muted/30"
      onClick={() => onOpenStory(row.original)}
    >
      {row.getVisibleCells().map((cell) => {
        // Skip the grouped column entirely — header does the same, keeping alignment
        if (cell.column.getIsGrouped()) return null;
        const size = cell.column.getSize();
        if (size === 0) return null;
        return (
          <div
            key={cell.id}
            className="overflow-hidden px-2 py-1.5"
            style={{
              width: size === 999 ? undefined : size,
              flex: size === 999 ? '1 1 0%' : undefined,
              minWidth: size === 999 ? 200 : undefined,
            }}
          >
            {flexRender(cell.column.columnDef.cell, cell.getContext())}
          </div>
        );
      })}
    </div>
  );
}
