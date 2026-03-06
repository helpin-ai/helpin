import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { createColumnHelper, type VisibilityState } from '@tanstack/react-table';
import { format, parseISO } from 'date-fns';
import { CalendarDays, LayoutGrid, LayoutList, Minus, Plus } from 'lucide-react';
import { useTitle } from '@/hooks/useTitle';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { PMDataTable } from '@/components/pm/PMDataTable';
import { DisplayPropertiesPopover } from '@/components/pm/DisplayPropertiesPopover';
import { pmSprintService } from '@/lib/services/pmSprintService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import type { SprintStatus, SprintWithStats } from '@/lib/pmTypes';
import { SPRINT_STATUS_CONFIG } from '@/lib/pmConstants';

const ALL_PROPERTIES = [
  { key: 'status', label: 'Status' },
  { key: 'team', label: 'Team' },
  { key: 'progress', label: 'Progress' },
  { key: 'stories', label: 'Stories' },
  { key: 'points', label: 'Points' },
  { key: 'start_date', label: 'Start date' },
  { key: 'end_date', label: 'End date' },
];

const DEFAULT_VISIBLE = [
  'status',
  'team',
  'progress',
  'stories',
  'points',
  'start_date',
  'end_date',
];

const columnHelper = createColumnHelper<SprintWithStats>();

interface SprintsPageProps {
  teamId?: string;
}

export function SprintsPage({ teamId }: SprintsPageProps) {
  useTitle('Sprints');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id;

  const [sprints, setSprints] = useState<SprintWithStats[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [visibleColumns, setVisibleColumns] = useState<string[]>(DEFAULT_VISIBLE);

  const VIEW_MODE_KEY = `pm_view_mode_sprints_${workspaceId}`;
  const [viewMode, setViewModeState] = useState<'cards' | 'table'>(() => {
    try {
      const saved = localStorage.getItem(VIEW_MODE_KEY);
      return saved === 'table' ? 'table' : 'cards';
    } catch { return 'cards'; }
  });
  const setViewMode = useCallback((mode: 'cards' | 'table') => {
    setViewModeState(mode);
    try { localStorage.setItem(VIEW_MODE_KEY, mode); } catch {}
  }, [VIEW_MODE_KEY]);

  const { findTeamName } = useWorkspaceTeams(workspaceId);
  const navigate = useNavigate();
  const openCreate = useGlobalCreateStore((s) => s.openCreate);

  const pct = (entry: SprintWithStats) => {
    if (entry.stats.story_count === 0) return 0;
    return Math.round((entry.stats.done_story_count / entry.stats.story_count) * 100);
  };

  const columns = useMemo(
    () => [
      columnHelper.display({
        id: 'name',
        header: 'Name',
        size: 999,
        cell: (info) => (
          <span className="truncate font-medium text-sm">{info.row.original.sprint.name}</span>
        ),
      }),
      columnHelper.display({
        id: 'status',
        header: 'Status',
        size: 100,
        cell: (info) => {
          const cfg = SPRINT_STATUS_CONFIG[info.row.original.sprint.status as SprintStatus];
          return (
            <span className={`text-xs ${cfg?.color ?? 'text-muted-foreground'}`}>
              {cfg?.label ?? info.row.original.sprint.status}
            </span>
          );
        },
      }),
      columnHelper.display({
        id: 'team',
        header: 'Team',
        size: 120,
        cell: (info) => (
          <span className="truncate text-xs text-muted-foreground">
            {findTeamName(info.row.original.sprint.team_id) || <Minus className="h-3.5 w-3.5" />}
          </span>
        ),
      }),
      columnHelper.display({
        id: 'progress',
        header: 'Progress',
        size: 140,
        cell: (info) => {
          const entry = info.row.original;
          return (
            <div className="flex items-center gap-2">
              <Progress value={pct(entry)} className="h-1.5 w-16" />
              <span className="text-xs text-muted-foreground">{pct(entry)}%</span>
            </div>
          );
        },
      }),
      columnHelper.display({
        id: 'stories',
        header: 'Stories',
        size: 90,
        cell: (info) => {
          const entry = info.row.original;
          return (
            <span className="text-xs text-muted-foreground">
              {entry.stats.done_story_count}/{entry.stats.story_count}
            </span>
          );
        },
      }),
      columnHelper.display({
        id: 'points',
        header: 'Points',
        size: 80,
        cell: (info) => {
          const entry = info.row.original;
          return (
            <span className="text-xs text-muted-foreground">
              {entry.stats.done_points}/{entry.stats.total_points}
            </span>
          );
        },
      }),
      columnHelper.display({
        id: 'start_date',
        header: 'Start date',
        size: 110,
        cell: (info) => (
          <div className="flex items-center gap-1 text-xs text-muted-foreground">
            <CalendarDays className="h-3.5 w-3.5" />
            <span>{format(parseISO(info.row.original.sprint.start_date), 'MMM d')}</span>
          </div>
        ),
      }),
      columnHelper.display({
        id: 'end_date',
        header: 'End date',
        size: 110,
        cell: (info) => (
          <div className="flex items-center gap-1 text-xs text-muted-foreground">
            <CalendarDays className="h-3.5 w-3.5" />
            <span>{format(parseISO(info.row.original.sprint.end_date), 'MMM d')}</span>
          </div>
        ),
      }),
    ],
    [findTeamName]
  );

  const columnVisibility = useMemo<VisibilityState>(() => {
    const vis: VisibilityState = { name: true };
    for (const prop of ALL_PROPERTIES) {
      vis[prop.key] = visibleColumns.includes(prop.key);
    }
    return vis;
  }, [visibleColumns]);

  const loadData = async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const res = await pmSprintService.list(workspaceId, { archived: false, team_id: teamId });
    if (res.error || !res.data) {
      setError(res.error ?? 'Failed to load sprints');
      setLoading(false);
      return;
    }
    setSprints(res.data);
    setLoading(false);
  };

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId, teamId]);

  // Refresh when sprint is created via global modal
  useEffect(() => {
    const handler = () => { loadData(); };
    window.addEventListener('sprint-created', handler);
    return () => window.removeEventListener('sprint-created', handler);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId, teamId]);

  const openSprint = (entry: SprintWithStats) => {
    if (!workspace) return;
    navigate({ to: '/w/$slug/pm/sprints/$sprintId', params: { slug: workspace.slug, sprintId: entry.sprint.id } });
  };

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">Sprints</h2>
          <p className="text-sm text-muted-foreground">Plan cycles and monitor story completion.</p>
        </div>
        <div className="flex items-center gap-2">
          {viewMode === 'table' && (
            <DisplayPropertiesPopover
              allProperties={ALL_PROPERTIES}
              visible={visibleColumns}
              onChange={setVisibleColumns}
            />
          )}
          <div className="flex items-center rounded-md border border-border/60">
            <Button
              variant={viewMode === 'cards' ? 'secondary' : 'ghost'}
              size="sm"
              className="h-8 rounded-r-none px-2.5"
              onClick={() => setViewMode('cards')}
            >
              <LayoutGrid className="h-4 w-4" />
            </Button>
            <Button
              variant={viewMode === 'table' ? 'secondary' : 'ghost'}
              size="sm"
              className="h-8 rounded-l-none px-2.5"
              onClick={() => setViewMode('table')}
            >
              <LayoutList className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </header>

      {error ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading sprints...</p>
      ) : sprints.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-3 py-8 text-center text-sm text-muted-foreground">
            <p>No sprints yet.</p>
            <Button
              size="sm"
              className="gap-1.5"
              onClick={() => openCreate('sprint', { teamId })}
            >
              <Plus className="h-3.5 w-3.5" />
              Create Sprint
            </Button>
          </CardContent>
        </Card>
      ) : viewMode === 'cards' ? (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {sprints.map((entry) => (
            <Card
              key={entry.sprint.id}
              className="cursor-pointer transition hover:shadow-md"
              onClick={() => openSprint(entry)}
            >
              <CardHeader className="pb-2">
                <CardTitle className="text-base">{entry.sprint.name}</CardTitle>
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <span className={SPRINT_STATUS_CONFIG[entry.sprint.status as SprintStatus]?.color}>
                    {SPRINT_STATUS_CONFIG[entry.sprint.status as SprintStatus]?.label ?? entry.sprint.status}
                  </span>
                  {findTeamName(entry.sprint.team_id) && (
                    <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium">
                      {findTeamName(entry.sprint.team_id)}
                    </span>
                  )}
                </div>
              </CardHeader>
              <CardContent className="space-y-2">
                <Progress value={pct(entry)} />
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  <span>{entry.stats.done_story_count}/{entry.stats.story_count} stories</span>
                  <span>{entry.stats.done_points}/{entry.stats.total_points} pts</span>
                </div>
                <p className="text-xs text-muted-foreground">
                  {format(parseISO(entry.sprint.start_date), 'MMM d')} - {format(parseISO(entry.sprint.end_date), 'MMM d, yyyy')}
                </p>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : (
        <PMDataTable
          data={sprints}
          columns={columns}
          columnVisibility={columnVisibility}
          onRowClick={openSprint}
        />
      )}
    </div>
  );
}
