import { useEffect, useMemo, useState } from 'react';
import { format, parseISO } from 'date-fns';
import { useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import {
  CalendarDays,
  ChevronDown,
  ChevronRight,
  Layers,
  Minus,
  Plus,
  Search,
  Sun,
  Target,
  TrendingUp,
  User,
} from 'lucide-react';
import { Input } from '@/components/ui/input';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { DisplayPropertiesPopover } from '@/components/pm/DisplayPropertiesPopover';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { Button } from '@/components/ui/button';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useEpicStates } from '@/hooks/queries';
import type { EpicWithStats, EpicHealth, EpicWorkflowState, StateType } from '@/lib/pmTypes';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { STATE_TYPE_ICON_CONFIG } from '@/lib/pmConstants';

const healthConfig: Record<EpicHealth, { label: string; color: string }> = {
  no_health: { label: 'No health', color: 'text-muted-foreground' },
  on_track: { label: 'On track', color: 'text-green-600' },
  at_risk: { label: 'At risk', color: 'text-yellow-600' },
  off_track: { label: 'Off track', color: 'text-red-600' },
};

const ALL_PROPERTIES = [
  { key: 'health', label: 'Health' },
  { key: 'progress', label: 'Progress' },
  { key: 'stories', label: 'Stories' },
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
  'health',
  'progress',
  'stories',
  'points',
  'owner',
  'objective',
  'target_date',
];

interface EpicColumn {
  key: string;
  header: string;
  size: number;
  render: (epic: EpicWithStats) => React.ReactNode;
}

interface EpicStateGroup {
  state: EpicWorkflowState;
  epics: EpicWithStats[];
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
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');

  const workspaceId = workspace?.id;
  const slug = workspace?.slug;
  const { teams, findTeamName } = useWorkspaceTeams(workspaceId);
  const isSingleTeam = teams.length <= 1;

  const [visibleColumns, setVisibleColumns] = useState<string[]>(DEFAULT_VISIBLE);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const { data: epicStates = [] } = useEpicStates(workspaceId ?? '');
  const ownerNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const completionPct = (entry: EpicWithStats) => {
    if (entry.stats.story_count === 0) return 0;
    return Math.round((entry.stats.done_story_count / entry.stats.story_count) * 100);
  };

  const allColumns = useMemo<EpicColumn[]>(
    () => [
      {
        key: 'health', header: 'Health', size: 120,
        render: (entry) => (
          <span className={`text-xs ${healthConfig[entry.epic.health]?.color ?? 'text-muted-foreground'}`}>
            {healthConfig[entry.epic.health]?.label ?? 'No updates'}
          </span>
        ),
      },
      {
        key: 'progress', header: 'Progress', size: 90,
        render: (entry) => (
          <div className="flex items-center gap-1.5 text-xs">
            <Sun className="h-3.5 w-3.5 text-amber-500" />
            <span>{completionPct(entry)}%</span>
          </div>
        ),
      },
      {
        key: 'stories', header: 'Stories', size: 90,
        render: (entry) => (
          <span className="text-xs text-muted-foreground">
            {entry.stats.done_story_count}/{entry.stats.story_count}
          </span>
        ),
      },
      {
        key: 'points', header: 'Points', size: 80,
        render: (entry) => (
          <span className="text-xs text-muted-foreground">
            {entry.stats.done_points}/{entry.stats.total_points}
          </span>
        ),
      },
      {
        key: 'owner', header: 'Owner', size: 180,
        render: (entry) => {
          const ownerName = entry.epic.owner_member_id ? ownerNameMap.get(entry.epic.owner_member_id) : null;
          return ownerName ? (
            <div className="flex items-center gap-2 min-w-0">
              <UserAvatar name={ownerName} />
              <span className="truncate text-xs text-muted-foreground">{ownerName}</span>
            </div>
          ) : (
            <User className="h-4 w-4 text-muted-foreground/50" />
          );
        },
      },
      {
        key: 'objective', header: 'Objective', size: 160,
        render: (entry) => {
          const objectives = entry.objectives ?? [];
          return objectives.length > 0 ? (
            <span className="truncate text-xs text-muted-foreground block max-w-[150px]">
              {objectives.map((o) => o.name).join(', ')}
            </span>
          ) : (
            <Minus className="h-3.5 w-3.5 text-muted-foreground" />
          );
        },
      },
      {
        key: 'target_date', header: 'Target date', size: 110,
        render: (entry) => (
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
        ),
      },
      {
        key: 'team', header: 'Team', size: 110,
        render: (entry) => (
          <span className="truncate text-xs text-muted-foreground">
            {findTeamName(entry.epic.team_id) || <Minus className="h-3.5 w-3.5" />}
          </span>
        ),
      },
      {
        key: 'start_date', header: 'Start date', size: 110,
        render: (entry) => (
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
        ),
      },
      {
        key: 'labels', header: 'Labels', size: 120,
        render: (entry) => (
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
        ),
      },
      {
        key: 'created', header: 'Created', size: 100,
        render: (entry) => (
          <span className="text-xs text-muted-foreground">
            {format(parseISO(entry.epic.created_at), 'MMM d')}
          </span>
        ),
      },
      {
        key: 'updated', header: 'Updated', size: 100,
        render: (entry) => (
          <span className="text-xs text-muted-foreground">
            {format(parseISO(entry.epic.updated_at), 'MMM d')}
          </span>
        ),
      },
    ],
    [findTeamName, ownerNameMap],
  );

  const visibleCols = useMemo(() => {
    const visible = new Set(visibleColumns);
    if (isSingleTeam) visible.delete('team');
    return allColumns.filter((c) => visible.has(c.key));
  }, [allColumns, visibleColumns, isSingleTeam]);

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

  const filteredEpics = useMemo(() => {
    if (!search.trim()) return epics;
    const q = search.toLowerCase();
    return epics.filter((e) => e.epic.name.toLowerCase().includes(q));
  }, [epics, search]);

  // Group epics by state, following epicStates order
  const groups = useMemo<EpicStateGroup[]>(() => {
    const byState = new Map<string, EpicWithStats[]>();
    const noState: EpicWithStats[] = [];
    for (const e of filteredEpics) {
      const sid = e.epic.epic_state_id;
      if (!sid) { noState.push(e); continue; }
      const arr = byState.get(sid) ?? [];
      arr.push(e);
      byState.set(sid, arr);
    }
    const result: EpicStateGroup[] = [];
    for (const state of epicStates) {
      const items = byState.get(state.id);
      if (items && items.length > 0) {
        result.push({ state, epics: items });
      }
    }
    if (noState.length > 0) {
      result.push({
        state: { id: '__none__', workspace_id: '', name: 'No state', state_type: 'unstarted', position: 999, is_default: false, created_at: '', updated_at: '' } as EpicWorkflowState,
        epics: noState,
      });
    }
    return result;
  }, [filteredEpics, epicStates]);

  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set());
  const toggleGroup = (stateId: string) => {
    setCollapsedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(stateId)) next.delete(stateId);
      else next.add(stateId);
      return next;
    });
  };

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
            <h2 className="text-xl font-semibold">Epics</h2>
            <p className="text-sm text-muted-foreground">Track long-running initiatives and their story progress.</p>
          </div>
        ) : <div />}
        {showHeaderActions && (
          <div className="flex items-center gap-2">
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
              <Input
                placeholder="Search epics…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="h-8 w-48 pl-8 text-xs"
              />
            </div>
            <DisplayPropertiesPopover
              allProperties={ALL_PROPERTIES}
              visible={visibleColumns}
              onChange={setVisibleColumns}
            />
          </div>
        )}
      </header>

      {error ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {loading ? null : epics.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 px-4">
          <div className="flex h-14 w-14 items-center justify-center rounded-full bg-violet-500/10 mb-5">
            <Layers className="h-7 w-7 text-violet-500" />
          </div>
          <h3 className="text-lg font-semibold mb-1.5">Create your first epic</h3>
          <p className="text-sm text-muted-foreground text-center max-w-md mb-6">
            Epics group related stories into long-running initiatives, giving you a high-level view of progress across your team's work.
          </p>
          <Button
            className="gap-2 mb-8"
            onClick={() => openCreate('epic', { teamId })}
          >
            <Plus className="h-4 w-4" />
            Create Epic
          </Button>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 w-full max-w-4xl">
            {[
              { icon: Layers, title: 'Group stories', desc: 'Organize related work items under a single initiative' },
              { icon: TrendingUp, title: 'Track health', desc: 'Monitor on-track, at-risk, and off-track status at a glance' },
              { icon: Target, title: 'Hit deadlines', desc: 'Set target dates and watch completion progress in real time' },
            ].map((item) => (
              <div key={item.title} className="flex flex-col items-center text-center rounded-lg border border-border/50 bg-muted/30 p-6">
                <item.icon className="h-5 w-5 text-muted-foreground mb-3" />
                <p className="text-sm font-medium mb-1">{item.title}</p>
                <p className="text-[13px] text-muted-foreground leading-relaxed">{item.desc}</p>
              </div>
            ))}
          </div>
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto rounded-lg border border-border">
          {/* Header */}
          <div className="sticky top-0 z-10 border-b border-border/60 bg-background">
            <div className="flex items-center">
              <div className="relative px-2.5 py-1.5 text-left text-[11px] font-medium text-muted-foreground border-r border-border/60" style={{ flex: '1 1 0%', minWidth: 200 }}>
                Name
              </div>
              {visibleCols.map((col) => (
                <div key={col.key} className="px-2.5 py-1.5 text-left text-[11px] font-medium text-muted-foreground border-r border-border/60 last:border-r-0" style={{ width: col.size }}>
                  {col.header}
                </div>
              ))}
            </div>
          </div>

          {/* Grouped rows */}
          <div className="overflow-auto" style={{ maxHeight: 'calc(100vh - 220px)' }}>
            {groups.map((group) => {
              const isCollapsed = collapsedGroups.has(group.state.id);
              const stateCfg = STATE_TYPE_ICON_CONFIG[group.state.state_type as StateType];
              const StateIcon = stateCfg?.icon ?? Minus;
              return (
                <div key={group.state.id}>
                  {/* Group header */}
                  <button
                    type="button"
                    onClick={() => toggleGroup(group.state.id)}
                    className="flex h-9 w-full cursor-pointer items-center gap-2 border-b border-border/50 bg-muted/20 px-3 text-sm hover:bg-muted/40 transition-colors"
                  >
                    {isCollapsed ? (
                      <ChevronRight className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    ) : (
                      <ChevronDown className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    )}
                    <StateIcon className={`h-3.5 w-3.5 shrink-0 ${stateCfg?.color ?? 'text-muted-foreground'}`} />
                    <span className="font-medium text-[13px]">{group.state.name}</span>
                    <span className="text-[11px] text-muted-foreground/60 tabular-nums">{group.epics.length}</span>
                  </button>

                  {/* Epic rows */}
                  {!isCollapsed && group.epics.map((entry) => (
                    <div
                      key={entry.epic.id}
                      className="group flex h-9 items-center border-b border-border/50 transition-colors hover:bg-muted/50 cursor-pointer"
                      onClick={() => openEpic(entry)}
                    >
                      <div className="flex items-center gap-2.5 px-2.5 self-stretch border-r border-border/60 overflow-hidden" style={{ flex: '1 1 0%', minWidth: 200 }}>
                        <Layers className="h-4 w-4 shrink-0 text-violet-500" />
                        <span className="truncate font-medium text-sm">{entry.epic.name}</span>
                      </div>
                      {visibleCols.map((col) => (
                        <div key={col.key} className="flex items-center px-2.5 self-stretch border-r border-border/60 last:border-r-0 overflow-hidden" style={{ width: col.size }}>
                          {col.render(entry)}
                        </div>
                      ))}
                    </div>
                  ))}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
