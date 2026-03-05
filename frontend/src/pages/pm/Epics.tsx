import { useEffect, useMemo, useState } from 'react';
import { format, parseISO } from 'date-fns';
import { useNavigate } from '@tanstack/react-router';
import { createColumnHelper, type VisibilityState } from '@tanstack/react-table';
import { useTitle } from '@/hooks/useTitle';
import {
  CalendarDays,
  Hexagon,
  Minus,
  Sun,
  User,
} from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { PMDataTable } from '@/components/pm/PMDataTable';
import { DisplayPropertiesPopover } from '@/components/pm/DisplayPropertiesPopover';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import type { EpicWithStats, EpicHealth } from '@/lib/pmTypes';

const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
  on_track: { label: 'On track', color: 'text-green-600' },
  at_risk: { label: 'At risk', color: 'text-yellow-600' },
  off_track: { label: 'Off track', color: 'text-red-600' },
};

const ALL_PROPERTIES = [
  { key: 'health', label: 'Health' },
  { key: 'status', label: 'Status' },
  { key: 'owner', label: 'Owner' },
  { key: 'team', label: 'Team' },
  { key: 'target_date', label: 'Target date' },
  { key: 'start_date', label: 'Start date' },
  { key: 'stories', label: 'Stories' },
  { key: 'points', label: 'Points' },
  { key: 'created', label: 'Created' },
  { key: 'updated', label: 'Updated' },
  { key: 'labels', label: 'Labels' },
];

const DEFAULT_VISIBLE = [
  'health',
  'status',
  'owner',
  'team',
  'target_date',
  'stories',
  'points',
];

const columnHelper = createColumnHelper<EpicWithStats>();

// ── Main page ──────────────────────────────────────────────────────

interface EpicsPageProps {
  teamId?: string;
}

export function EpicsPage({ teamId }: EpicsPageProps) {
  useTitle('Epics');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const navigate = useNavigate();

  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [visibleColumns, setVisibleColumns] = useState<string[]>(DEFAULT_VISIBLE);

  const workspaceId = workspace?.id;
  const slug = workspace?.slug;
  const { findTeamName } = useWorkspaceTeams(workspaceId);

  const completionPct = (entry: EpicWithStats) => {
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
          <div className="flex items-center gap-2.5 min-w-0">
            <Hexagon className="h-4 w-4 shrink-0 text-violet-500" />
            <span className="truncate font-medium text-sm">{info.row.original.epic.name}</span>
          </div>
        ),
      }),
      columnHelper.display({
        id: 'health',
        header: 'Health',
        size: 120,
        cell: (info) => {
          const entry = info.row.original;
          return (
            <span className={`text-xs ${healthConfig[entry.epic.health]?.color ?? 'text-muted-foreground'}`}>
              {healthConfig[entry.epic.health]?.label ?? 'No updates'}
            </span>
          );
        },
      }),
      columnHelper.display({
        id: 'status',
        header: 'Status',
        size: 80,
        cell: (info) => (
          <div className="flex items-center gap-1.5 text-xs">
            <Sun className="h-3.5 w-3.5 text-amber-500" />
            <span>{completionPct(info.row.original)}%</span>
          </div>
        ),
      }),
      columnHelper.display({
        id: 'owner',
        header: 'Owner',
        size: 80,
        cell: (info) => {
          const entry = info.row.original;
          return entry.epic.owner_id ? (
            <UserAvatar name={entry.epic.owner_id} />
          ) : (
            <User className="h-4 w-4 text-muted-foreground/50" />
          );
        },
      }),
      columnHelper.display({
        id: 'team',
        header: 'Team',
        size: 110,
        cell: (info) => (
          <span className="truncate text-xs text-muted-foreground">
            {findTeamName(info.row.original.epic.team_id) || <Minus className="h-3.5 w-3.5" />}
          </span>
        ),
      }),
      columnHelper.display({
        id: 'target_date',
        header: 'Target date',
        size: 110,
        cell: (info) => {
          const entry = info.row.original;
          return (
            <div className="flex items-center gap-1 text-xs text-muted-foreground">
              {entry.epic.deadline ? (
                <>
                  <CalendarDays className="h-3.5 w-3.5" />
                  <span>{format(parseISO(entry.epic.deadline), 'MMM d')}</span>
                </>
              ) : (
                <Minus className="h-3.5 w-3.5" />
              )}
            </div>
          );
        },
      }),
      columnHelper.display({
        id: 'start_date',
        header: 'Start date',
        size: 110,
        cell: (info) => {
          const entry = info.row.original;
          return (
            <div className="flex items-center gap-1 text-xs text-muted-foreground">
              {entry.epic.planned_start_date ? (
                <>
                  <CalendarDays className="h-3.5 w-3.5" />
                  <span>{format(parseISO(entry.epic.planned_start_date), 'MMM d')}</span>
                </>
              ) : (
                <Minus className="h-3.5 w-3.5" />
              )}
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
        id: 'created',
        header: 'Created',
        size: 100,
        cell: (info) => (
          <span className="text-xs text-muted-foreground">
            {format(parseISO(info.row.original.epic.created_at), 'MMM d')}
          </span>
        ),
      }),
      columnHelper.display({
        id: 'updated',
        header: 'Updated',
        size: 100,
        cell: (info) => (
          <span className="text-xs text-muted-foreground">
            {format(parseISO(info.row.original.epic.updated_at), 'MMM d')}
          </span>
        ),
      }),
      columnHelper.display({
        id: 'labels',
        header: 'Labels',
        size: 120,
        cell: (info) => {
          const entry = info.row.original;
          return (
            <div className="flex flex-wrap gap-1">
              {(entry.labels ?? []).length > 0 ? (
                entry.labels.map((l) => (
                  <span key={l.id} className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium">
                    {l.name}
                  </span>
                ))
              ) : (
                <Minus className="h-3.5 w-3.5 text-muted-foreground" />
              )}
            </div>
          );
        },
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
    const epicsRes = await pmEpicService.list(workspaceId, { archived: false, team_id: teamId });
    if (epicsRes.error || !epicsRes.data) {
      setError(epicsRes.error ?? 'Failed to load epics');
      setLoading(false);
      return;
    }
    setEpics(epicsRes.data);
    setLoading(false);
  };

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId, teamId]);

  // Refresh when epic is created via global modal
  useEffect(() => {
    const handler = () => { loadData(); };
    window.addEventListener('epic-created', handler);
    return () => window.removeEventListener('epic-created', handler);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId, teamId]);

  const openEpic = (entry: EpicWithStats) => {
    if (!slug) return;
    navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug, epicId: entry.epic.id } });
  };

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">Epics</h2>
          <p className="text-sm text-muted-foreground">Track long-running initiatives and their story progress.</p>
        </div>
        <DisplayPropertiesPopover
          allProperties={ALL_PROPERTIES}
          visible={visibleColumns}
          onChange={setVisibleColumns}
        />
      </header>

      {error ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading epics...</p>
      ) : epics.length === 0 ? (
        <Card>
          <CardContent className="py-8 text-center text-sm text-muted-foreground">
            No epics yet. Use the Create button in the header to add your first epic.
          </CardContent>
        </Card>
      ) : (
        <PMDataTable
          data={epics}
          columns={columns}
          columnVisibility={columnVisibility}
          onRowClick={openEpic}
        />
      )}
    </div>
  );
}
