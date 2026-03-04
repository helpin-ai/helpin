import { useEffect, useMemo, useState } from 'react';
import { format, parseISO } from 'date-fns';
import { useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import {
  CalendarDays,
  Hexagon,
  Minus,
  Settings2,
  Sun,
  User,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import type { EpicWithStats, EpicHealth } from '@/lib/pmTypes';

const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
  on_track: { label: 'On track', color: 'text-green-600' },
  at_risk: { label: 'At risk', color: 'text-yellow-600' },
  off_track: { label: 'Off track', color: 'text-red-600' },
};

// ── Column definitions ─────────────────────────────────────────────

type ColumnKey =
  | 'health'
  | 'status'
  | 'owner'
  | 'team'
  | 'target_date'
  | 'start_date'
  | 'stories'
  | 'points'
  | 'created'
  | 'updated'
  | 'labels';

interface ColumnDef {
  key: ColumnKey;
  label: string;
  width: string;
}

const ALL_COLUMNS: ColumnDef[] = [
  { key: 'health', label: 'Health', width: '120px' },
  { key: 'status', label: 'Status', width: '80px' },
  { key: 'owner', label: 'Owner', width: '80px' },
  { key: 'team', label: 'Team', width: '110px' },
  { key: 'target_date', label: 'Target date', width: '110px' },
  { key: 'start_date', label: 'Start date', width: '110px' },
  { key: 'stories', label: 'Stories', width: '90px' },
  { key: 'points', label: 'Points', width: '80px' },
  { key: 'created', label: 'Created', width: '100px' },
  { key: 'updated', label: 'Updated', width: '100px' },
  { key: 'labels', label: 'Labels', width: '120px' },
];

const DEFAULT_VISIBLE: ColumnKey[] = [
  'health',
  'status',
  'owner',
  'team',
  'target_date',
  'stories',
  'points',
];

// ── Cell renderer ──────────────────────────────────────────────────

function EpicCell({
  columnKey,
  entry,
  completionPct,
  findTeamName,
}: {
  columnKey: ColumnKey;
  entry: EpicWithStats;
  completionPct: number;
  findTeamName: (id: string | undefined) => string | undefined;
}) {
  switch (columnKey) {
    case 'health':
      return (
        <span className={`text-xs ${healthConfig[entry.epic.health]?.color ?? 'text-muted-foreground'}`}>
          {healthConfig[entry.epic.health]?.label ?? 'No updates'}
        </span>
      );
    case 'status':
      return (
        <div className="flex items-center gap-1.5 text-xs">
          <Sun className="h-3.5 w-3.5 text-amber-500" />
          <span>{completionPct}%</span>
        </div>
      );
    case 'owner':
      return entry.epic.owner_id ? (
        <Avatar className="h-6 w-6 border border-border/80">
          <AvatarFallback className="text-[9px] font-semibold bg-muted/60">
            {entry.epic.owner_id.slice(0, 2).toUpperCase()}
          </AvatarFallback>
        </Avatar>
      ) : (
        <User className="h-4 w-4 text-muted-foreground/50" />
      );
    case 'team':
      return (
        <span className="truncate text-xs text-muted-foreground">
          {findTeamName(entry.epic.team_id) || <Minus className="h-3.5 w-3.5" />}
        </span>
      );
    case 'target_date':
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
    case 'start_date':
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
    case 'stories':
      return (
        <span className="text-xs text-muted-foreground">
          {entry.stats.done_story_count}/{entry.stats.story_count}
        </span>
      );
    case 'points':
      return (
        <span className="text-xs text-muted-foreground">
          {entry.stats.done_points}/{entry.stats.total_points}
        </span>
      );
    case 'created':
      return (
        <span className="text-xs text-muted-foreground">
          {format(parseISO(entry.epic.created_at), 'MMM d')}
        </span>
      );
    case 'updated':
      return (
        <span className="text-xs text-muted-foreground">
          {format(parseISO(entry.epic.updated_at), 'MMM d')}
        </span>
      );
    case 'labels':
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
    default:
      return null;
  }
}

// ── Display Properties Toggle ──────────────────────────────────────

function DisplayPropertiesPopover({
  visible,
  onChange,
}: {
  visible: ColumnKey[];
  onChange: (next: ColumnKey[]) => void;
}) {
  const toggle = (key: ColumnKey) => {
    onChange(
      visible.includes(key)
        ? visible.filter((k) => k !== key)
        : [...visible, key]
    );
  };

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="h-8 gap-1.5 text-xs">
          <Settings2 className="h-3.5 w-3.5" />
          Display
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-72 p-3" align="end">
        <p className="mb-2.5 text-xs font-medium text-muted-foreground">Display properties</p>
        <div className="flex flex-wrap gap-1.5">
          {ALL_COLUMNS.map((col) => {
            const active = visible.includes(col.key);
            return (
              <button
                key={col.key}
                type="button"
                onClick={() => toggle(col.key)}
                className={`rounded-full border px-2.5 py-1 text-xs transition-colors cursor-pointer
                  ${active
                    ? 'border-border bg-accent font-medium text-foreground'
                    : 'border-border/60 text-muted-foreground hover:border-border hover:text-foreground'}
                `}
              >
                {col.label}
              </button>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}

// ── Main page ──────────────────────────────────────────────────────

export function EpicsPage() {
  useTitle('Epics');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const navigate = useNavigate();

  const [epics, setEpics] = useState<EpicWithStats[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [visibleColumns, setVisibleColumns] = useState<ColumnKey[]>(DEFAULT_VISIBLE);

  const workspaceId = workspace?.id;
  const slug = workspace?.slug;
  const { findTeamName } = useWorkspaceTeams(workspaceId);

  const activeColumns = useMemo(
    () => ALL_COLUMNS.filter((col) => visibleColumns.includes(col.key)),
    [visibleColumns]
  );

  const gridTemplate = useMemo(
    () => `1fr ${activeColumns.map((c) => c.width).join(' ')}`,
    [activeColumns]
  );

  const loadData = async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const epicsRes = await pmEpicService.list(workspaceId, { archived: false });
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
  }, [workspaceId]);

  // Refresh when epic is created via global modal
  useEffect(() => {
    const handler = () => { loadData(); };
    window.addEventListener('epic-created', handler);
    return () => window.removeEventListener('epic-created', handler);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceId]);

  const completionPct = (entry: EpicWithStats) => {
    if (entry.stats.story_count === 0) return 0;
    return Math.round((entry.stats.done_story_count / entry.stats.story_count) * 100);
  };

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
        <DisplayPropertiesPopover visible={visibleColumns} onChange={setVisibleColumns} />
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
        <div className="rounded-md border border-border/60">
          {/* Table header */}
          <div
            className="grid items-center gap-2 border-b border-border/60 bg-muted/30 px-4 py-2 text-xs font-medium text-muted-foreground"
            style={{ gridTemplateColumns: gridTemplate }}
          >
            <span>Name</span>
            {activeColumns.map((col) => (
              <span key={col.key}>{col.label}</span>
            ))}
          </div>

          {/* Table rows */}
          {epics.map((entry) => (
            <div
              key={entry.epic.id}
              role="button"
              tabIndex={0}
              onClick={() => openEpic(entry)}
              onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openEpic(entry); } }}
              className="grid cursor-pointer items-center gap-2 border-b border-border/40 px-4 py-3 text-sm transition-colors last:border-b-0 hover:bg-accent/40"
              style={{ gridTemplateColumns: gridTemplate }}
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <Hexagon className="h-4 w-4 shrink-0 text-violet-500" />
                <span className="truncate font-medium">{entry.epic.name}</span>
              </div>
              {activeColumns.map((col) => (
                <div key={col.key} className="min-w-0">
                  <EpicCell
                    columnKey={col.key}
                    entry={entry}
                    completionPct={completionPct(entry)}
                    findTeamName={findTeamName}
                  />
                </div>
              ))}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
