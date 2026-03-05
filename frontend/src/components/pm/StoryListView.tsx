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
import { CalendarDays, Check, ChevronDown, ChevronRight, Loader2, UserPlus } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Calendar } from '@/components/ui/calendar';
import { format, parseISO } from 'date-fns';
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
import { UserAvatar } from './UserAvatar';
import type {
  Priority,
  Severity,
  Story,
  WorkflowWithStates,
  EpicWithStats,
  SprintWithStats,
} from '@/lib/pmTypes';
import type { MemberWithUser, WorkspaceTeam } from '@/lib/types';
import { EstimatePicker } from '@/components/pm/EstimatePicker';
import type { BoardFilters } from '@/stores/pmBoardStore';

const ALL_PRIORITIES: Priority[] = ['urgent', 'high', 'medium', 'low', 'none'];
const ALL_SEVERITIES: Severity[] = ['critical', 'major', 'minor', 'none'];

interface StoryListViewProps {
  workspaceId: string;
  workflow: WorkflowWithStates;
  teams: WorkspaceTeam[];
  members: MemberWithUser[];
  epics: EpicWithStats[];
  sprints: SprintWithStats[];
  filters?: BoardFilters;
  teamId?: string | null;
  /** When provided, use these stories instead of fetching internally. */
  externalStories?: Story[];
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

const HIDDEN_GROUP_COLUMNS = ['typeName', 'priorityName', 'severityName'];

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
  externalStories,
  onOpenStory,
}: StoryListViewProps) {
  const isExternal = externalStories !== undefined;
  const [stories, setStories] = useState<Story[]>(externalStories ?? []);
  const [loading, setLoading] = useState(!isExternal);
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

  // Fetch stories (skipped when externalStories is provided)
  const fetchStories = useCallback(async () => {
    if (isExternal) return;
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
  }, [workspaceId, workflow.workflow.id, filters, teamId, isExternal]);

  useEffect(() => {
    if (!isExternal) fetchStories();
  }, [fetchStories, isExternal]);

  // Sync external stories when they change
  useEffect(() => {
    if (isExternal && externalStories) setStories(externalStories);
  }, [isExternal, externalStories]);

  // Optimistic inline update with rollback on failure
  const updateStoryField = useCallback(
    async (storyId: string, patch: Partial<Story>) => {
      let snapshot: Story[] = [];
      setStories((current) => {
        snapshot = current;
        return current.map((s) => (s.id === storyId ? { ...s, ...patch } : s));
      });
      const { error } = await pmStoryService.update(workspaceId, storyId, patch);
      if (error) setStories(snapshot);
    },
    [workspaceId],
  );

  // Listen for story events (only for self-fetching mode)
  useEffect(() => {
    if (isExternal) return;
    const handler = () => { fetchStories(); };
    window.addEventListener('story-created', handler);
    window.addEventListener('story-updated', handler);
    return () => {
      window.removeEventListener('story-created', handler);
      window.removeEventListener('story-updated', handler);
    };
  }, [fetchStories, isExternal]);

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
          size: 150,
          cell: (info) => (
            <InlineStateCell
              story={info.row.original}
              states={workflow.states}
              stateMap={stateMap}
              onUpdate={updateStoryField}
            />
          ),
        }
      ),
      columnHelper.accessor('priority', {
        id: 'priorityIcon',
        header: 'Priority',
        size: 110,
        enableGrouping: false,
        cell: (info) => (
          <InlinePriorityCell
            story={info.row.original}
            onUpdate={updateStoryField}
          />
        ),
      }),
      columnHelper.accessor('severity', {
        id: 'severityIcon',
        header: 'Severity',
        size: 110,
        enableGrouping: false,
        cell: (info) => (
          <InlineSeverityCell
            story={info.row.original}
            onUpdate={updateStoryField}
          />
        ),
      }),
      columnHelper.accessor('estimate', {
        id: 'estimate',
        header: 'Estimate',
        size: 80,
        enableGrouping: false,
        cell: (info) => (
          <InlineEstimateCell
            story={info.row.original}
            onUpdate={updateStoryField}
          />
        ),
      }),
      columnHelper.accessor(
        (row) => (row.owner_id ? memberMap.get(row.owner_id) ?? 'Unknown' : 'Unassigned'),
        {
          id: 'ownerName',
          header: 'Owner',
          size: 150,
          cell: (info) => (
            <InlineOwnerCell
              story={info.row.original}
              members={members}
              memberMap={memberMap}
              onUpdate={updateStoryField}
            />
          ),
        }
      ),
      columnHelper.accessor(
        (row) => (row.team_id ? teamMap.get(row.team_id) ?? 'Unknown' : 'No Team'),
        {
          id: 'teamName',
          header: 'Team',
          size: 130,
          cell: (info) => (
            <InlineTeamCell
              story={info.row.original}
              teams={teams}
              teamMap={teamMap}
              onUpdate={updateStoryField}
            />
          ),
        }
      ),
      columnHelper.accessor(
        (row) => (row.epic_id ? epicMap.get(row.epic_id) ?? 'Unknown' : 'No Epic'),
        {
          id: 'epicName',
          header: 'Epic',
          size: 150,
          cell: (info) => (
            <InlineEpicCell
              story={info.row.original}
              epics={epics}
              epicMap={epicMap}
              onUpdate={updateStoryField}
            />
          ),
        }
      ),
      columnHelper.accessor(
        (row) => (row.sprint_id ? sprintMap.get(row.sprint_id) ?? 'Unknown' : 'No Sprint'),
        {
          id: 'sprintName',
          header: 'Sprint',
          size: 140,
          cell: (info) => (
            <InlineSprintCell
              story={info.row.original}
              sprints={sprints}
              sprintMap={sprintMap}
              onUpdate={updateStoryField}
            />
          ),
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
        size: 120,
        enableGrouping: false,
        cell: (info) => (
          <InlineDeadlineCell
            story={info.row.original}
            onUpdate={updateStoryField}
          />
        ),
      }),
    ],
    [stateMap, memberMap, teamMap, epicMap, sprintMap, onOpenStory, workflow.states, members, teams, epics, sprints, updateStoryField]
  );

  const columnVisibility = useMemo(
    () => Object.fromEntries(HIDDEN_GROUP_COLUMNS.map((c) => [c, false])),
    [],
  );

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
      <div className="flex items-center gap-2 px-3 pt-2">
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

// ── Inline editable cells ──────────────────────────────────────────

function InlinePriorityCell({
  story,
  onUpdate,
}: {
  story: Story;
  onUpdate: (storyId: string, patch: Partial<Story>) => Promise<void>;
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
  story: Story;
  states: WorkflowWithStates['states'];
  stateMap: Map<string, { name: string; stateType: string }>;
  onUpdate: (storyId: string, patch: Partial<Story>) => Promise<void>;
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
  members,
  memberMap,
  onUpdate,
}: {
  story: Story;
  members: MemberWithUser[];
  memberMap: Map<string, string>;
  onUpdate: (storyId: string, patch: Partial<Story>) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const ownerName = story.owner_id ? memberMap.get(story.owner_id) ?? 'Unknown' : null;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          {ownerName ? (
            <>
              <UserAvatar name={ownerName} className="h-4 w-4" />
              <span className="truncate">{ownerName}</span>
            </>
          ) : (
            <>
              <UserPlus className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="text-muted-foreground">Assign</span>
            </>
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
            <CommandInput placeholder="Search members..." className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No members found</CommandEmpty>
              <CommandGroup>
                {members.map((m) => (
                  <CommandItem
                    key={m.user_id}
                    value={m.full_name || m.email}
                    onSelect={() => {
                      const newOwnerId = story.owner_id === m.user_id ? undefined : m.user_id;
                      onUpdate(story.id, { owner_id: newOwnerId });
                      setOpen(false);
                    }}
                    className="flex items-center gap-2 text-xs"
                  >
                    <UserAvatar name={m.full_name || m.email} className="h-5 w-5" />
                    <span className="truncate">{m.full_name || m.email}</span>
                    {story.owner_id === m.user_id && (
                      <Check className="ml-auto h-3.5 w-3.5 text-primary" />
                    )}
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

function InlineSeverityCell({
  story,
  onUpdate,
}: {
  story: Story;
  onUpdate: (storyId: string, patch: Partial<Story>) => Promise<void>;
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
  story: Story;
  onUpdate: (storyId: string, patch: Partial<Story>) => Promise<void>;
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
  story: Story;
  teams: WorkspaceTeam[];
  teamMap: Map<string, string>;
  onUpdate: (storyId: string, patch: Partial<Story>) => Promise<void>;
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
  story: Story;
  epics: EpicWithStats[];
  epicMap: Map<string, string>;
  onUpdate: (storyId: string, patch: Partial<Story>) => Promise<void>;
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
  story: Story;
  sprints: SprintWithStats[];
  sprintMap: Map<string, string>;
  onUpdate: (storyId: string, patch: Partial<Story>) => Promise<void>;
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

function InlineDeadlineCell({
  story,
  onUpdate,
}: {
  story: Story;
  onUpdate: (storyId: string, patch: Partial<Story>) => Promise<void>;
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
