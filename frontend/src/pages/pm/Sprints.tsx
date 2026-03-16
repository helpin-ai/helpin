import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { createColumnHelper, type VisibilityState } from '@tanstack/react-table';
import { format, parseISO } from 'date-fns';
import { Archive, ArchiveRestore, CalendarDays, LayoutGrid, LayoutList, Minus, MoreHorizontal, Plus, Timer, BarChart3, CheckCircle2 } from 'lucide-react';
import { useTitle } from '@/hooks/useTitle';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { PMDataTable } from '@/components/pm/PMDataTable';
import { DisplayPropertiesPopover } from '@/components/pm/DisplayPropertiesPopover';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
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
  const [showArchived, setShowArchived] = useState(false);
  const [hasArchivedSprints, setHasArchivedSprints] = useState(false);

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
              <Progress value={pct(entry)} className="h-1.5 w-16 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />
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
        cell: (info) => {
          const d = info.row.original.sprint.start_date;
          return d ? (
            <div className="flex items-center gap-1 text-xs text-muted-foreground">
              <CalendarDays className="h-3.5 w-3.5" />
              <span>{format(parseISO(d), 'MMM d')}</span>
            </div>
          ) : <Minus className="h-3.5 w-3.5 text-muted-foreground" />;
        },
      }),
      columnHelper.display({
        id: 'end_date',
        header: 'End date',
        size: 110,
        cell: (info) => {
          const d = info.row.original.sprint.end_date;
          return d ? (
            <div className="flex items-center gap-1 text-xs text-muted-foreground">
              <CalendarDays className="h-3.5 w-3.5" />
              <span>{format(parseISO(d), 'MMM d')}</span>
            </div>
          ) : <Minus className="h-3.5 w-3.5 text-muted-foreground" />;
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
    const [res, archivedRes] = await Promise.all([
      pmSprintService.list(workspaceId, { archived: showArchived, team_id: teamId }),
      pmSprintService.list(workspaceId, { archived: true, team_id: teamId }),
    ]);
    if (res.error || !res.data) {
      setError(res.error ?? 'Failed to load sprints');
      setLoading(false);
      return;
    }
    setHasArchivedSprints(Boolean(archivedRes.data?.length));
    setSprints(res.data);
    setLoading(false);
  };

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId, teamId, showArchived]);

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

  const showHeaderActions = sprints.length > 0 || showArchived;
  const showHeaderIntro = sprints.length > 0;
  const showArchivedToggle = hasArchivedSprints || showArchived;

  const handleArchiveToggle = async (entry: SprintWithStats) => {
    if (!workspaceId) return;
    const next = !entry.sprint.archived;
    await pmSprintService.update(workspaceId, entry.sprint.id, { archived: next });
    loadData();
  };

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="space-y-4 max-w-7xl mx-auto">
      <header className="flex items-center justify-between">
        {showHeaderIntro ? (
          <div>
            <h2 className="text-xl font-semibold">Sprints</h2>
            <p className="text-sm text-muted-foreground">Plan cycles and monitor story completion.</p>
          </div>
        ) : <div />}
        {showHeaderActions && (
          <div className="flex items-center gap-2">
            {showArchivedToggle && (
              <Button
                variant={showArchived ? 'secondary' : 'ghost'}
                size="sm"
                className="h-8 text-xs"
                onClick={() => setShowArchived((v) => !v)}
              >
                {showArchived ? 'Showing archived' : 'Show archived'}
              </Button>
            )}
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
        )}
      </header>

      {error ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {loading ? null : sprints.length === 0 ? (
        showArchived ? (
          <div className="flex flex-col items-center justify-center py-16 px-4">
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-muted/50 mb-5">
              <Archive className="h-7 w-7 text-muted-foreground" />
            </div>
            <h3 className="text-lg font-semibold mb-1.5">No archived sprints</h3>
            <p className="text-sm text-muted-foreground text-center max-w-md mb-6">
              Archived sprints will appear here. You can archive completed sprints to keep your active list clean.
            </p>
            <Button variant="outline" onClick={() => setShowArchived(false)}>
              View active sprints
            </Button>
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center py-16 px-4">
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-emerald-500/10 mb-5">
              <Timer className="h-7 w-7 text-emerald-500" />
            </div>
            <h3 className="text-lg font-semibold mb-1.5">Create your first sprint</h3>
            <p className="text-sm text-muted-foreground text-center max-w-md mb-6">
              Sprints are time-boxed cycles that help your team plan, focus, and deliver work in a predictable rhythm.
            </p>
            <Button
              className="gap-2 mb-8"
              onClick={() => openCreate('sprint', { teamId })}
            >
              <Plus className="h-4 w-4" />
              Create Sprint
            </Button>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 w-full max-w-4xl">
              {[
                { icon: CalendarDays, title: 'Set a cadence', desc: 'Define start and end dates for focused work cycles' },
                { icon: BarChart3, title: 'Track progress', desc: 'Monitor story and point completion in real time' },
                { icon: CheckCircle2, title: 'Ship consistently', desc: 'Build momentum with regular delivery milestones' },
              ].map((item) => (
                <div key={item.title} className="flex flex-col items-center text-center rounded-lg border border-border/50 bg-muted/30 p-6">
                  <item.icon className="h-5 w-5 text-muted-foreground mb-3" />
                  <p className="text-sm font-medium mb-1">{item.title}</p>
                  <p className="text-[13px] text-muted-foreground leading-relaxed">{item.desc}</p>
                </div>
              ))}
            </div>
          </div>
        )
      ) : viewMode === 'cards' ? (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {sprints.map((entry) => (
            <SprintCard
              key={entry.sprint.id}
              entry={entry}
              findTeamName={findTeamName}
              pct={pct}
              onOpen={() => openSprint(entry)}
              onArchiveToggle={() => handleArchiveToggle(entry)}
            />
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

// ── Sprint Card with archive action ──────────────────────────────────

function SprintCard({
  entry,
  findTeamName,
  pct,
  onOpen,
  onArchiveToggle,
}: {
  entry: SprintWithStats;
  findTeamName: (id?: string | null) => string | undefined;
  pct: (entry: SprintWithStats) => number;
  onOpen: () => void;
  onArchiveToggle: () => void;
}) {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const isArchived = entry.sprint.archived;

  return (
    <>
      <Card
        className="group cursor-pointer transition-all border-border/60 hover:shadow-md hover:border-border"
        onClick={onOpen}
      >
        <CardHeader className="pb-2">
          <div className="flex items-start justify-between gap-2">
            <CardTitle className="text-base">{entry.sprint.name}</CardTitle>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6 shrink-0 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity"
                  onClick={(e) => e.stopPropagation()}
                >
                  <MoreHorizontal className="h-3.5 w-3.5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
                {isArchived ? (
                  <DropdownMenuItem onClick={onArchiveToggle}>
                    <ArchiveRestore className="mr-2 h-4 w-4 text-blue-500" />
                    Unarchive
                  </DropdownMenuItem>
                ) : (
                  <DropdownMenuItem onClick={() => setConfirmOpen(true)}>
                    <Archive className="mr-2 h-4 w-4 text-amber-500" />
                    Archive
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
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
          <Progress value={pct(entry)} className="h-1.5 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span>{entry.stats.done_story_count}/{entry.stats.story_count} stories</span>
            <span>{entry.stats.done_points}/{entry.stats.total_points} pts</span>
          </div>
          {entry.sprint.start_date && entry.sprint.end_date ? (
            <p className="text-xs text-muted-foreground">
              {format(parseISO(entry.sprint.start_date), 'MMM d')} - {format(parseISO(entry.sprint.end_date), 'MMM d, yyyy')}
            </p>
          ) : (
            <p className="text-xs text-muted-foreground">No dates set</p>
          )}
        </CardContent>
      </Card>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="Archive sprint"
        description="This sprint will be hidden from the active list. You can view and restore it from the archived sprints view."
        confirmLabel="Archive"
        variant="default"
        onConfirm={onArchiveToggle}
      />
    </>
  );
}
