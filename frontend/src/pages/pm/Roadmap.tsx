import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  Calendar03Icon,
  ArrowDown01Icon,
  PlusSignIcon,
  Target01Icon,
  ChartGanttIcon,
  Layers01Icon,
} from '@/lib/icons';
import { useTitle } from '@/hooks/useTitle';
import { Button } from '@/components/ui/button';
import { QuietPageHeader, QuietPrimaryAction } from '@/components/design-system/quiet';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuCheckboxItem,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { pmRoadmapService } from '@/lib/services/pmRoadmapService';
import { RoadmapTimeline } from '@/components/pm/RoadmapTimeline';
import type { RoadmapData, RoadmapEpic } from '@/lib/pmTypes';
import { getEpicDoneTaskCount, getEpicTaskCount } from '@/lib/pmTypes';

type GroupBy = 'objective' | 'team' | 'epic';
type Zoom = 'month' | 'quarter';

const HEALTH_CONFIG: Record<string, { label: string; dot: string }> = {
  no_health: { label: 'No health', dot: 'bg-zinc-400' },
  on_track: { label: 'On track', dot: 'bg-green-500' },
  at_risk: { label: 'At risk', dot: 'bg-yellow-500' },
  off_track: { label: 'Off track', dot: 'bg-red-500' },
};

export function RoadmapPage() {
  useTitle('Roadmap');
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const slug = workspace?.slug ?? '';
  const navigate = useNavigate();
  const openCreate = useGlobalCreateStore((s) => s.openCreate);

  const { data: access } = useWorkspaceAccess(workspaceId);
  const { canEdit } = usePermissions(access);

  const { teams } = useAccessibleTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const memberNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers]);

  const teamNameMap = useMemo(() => {
    const m = new Map<string, string>();
    for (const t of teams) {
      m.set(t.id, t.name);
    }
    return m;
  }, [teams]);

  // View state
  const [groupBy, setGroupBy] = useState<GroupBy>('objective');
  const [zoom, setZoom] = useState<Zoom>('quarter');
  const [showCompleted, setShowCompleted] = useState(false);
  const [filterTeamId, setFilterTeamId] = useState<string | undefined>();
  const [filterObjectiveId, setFilterObjectiveId] = useState<string | undefined>();
  const [filterHealth, setFilterHealth] = useState<string | undefined>();

  // Data
  const [data, setData] = useState<RoadmapData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!workspaceId) return;
    setLoading(true);
    pmRoadmapService
      .getData(workspaceId, {
        team_id: filterTeamId,
        objective_id: filterObjectiveId,
        health: filterHealth,
        show_completed: showCompleted,
      })
      .then((res) => {
        if (res.data) setData(res.data);
      })
      .finally(() => setLoading(false));
  }, [workspaceId, filterTeamId, filterObjectiveId, filterHealth, showCompleted]);

  // Listen for epic creation events
  useEffect(() => {
    const refresh = () => {
      if (!workspaceId) return;
      pmRoadmapService
        .getData(workspaceId, {
          team_id: filterTeamId,
          objective_id: filterObjectiveId,
          health: filterHealth,
          show_completed: showCompleted,
        })
        .then((res) => {
          if (res.data) setData(res.data);
        });
    };
    window.addEventListener('epic-created', refresh);
    return () => window.removeEventListener('epic-created', refresh);
  }, [workspaceId, filterTeamId, filterObjectiveId, filterHealth, showCompleted]);

  const epics = data?.epics ?? [];
  const objectives = data?.objectives ?? [];

  const unscheduled = useMemo(
    () => epics.filter((e) => !e.epic.planned_start_date || !e.epic.deadline),
    [epics]);

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  const hasActiveFilters = !!(filterTeamId || filterObjectiveId || filterHealth || showCompleted);
  const pageHeader = (
    <QuietPageHeader
      variant="shell"
      title="Roadmap"
      description="Epics across objectives on a timeline."
      actions={
        canEdit ? (
          <QuietPrimaryAction className="gap-1.5" onClick={() => openCreate('epic')}>
            <PlusSignIcon className="h-4 w-4" />
            Add epic
          </QuietPrimaryAction>
        ) : null
      }
    />
  );

  // ── Filtered empty state ───────────────────────────────────
  // (shown inline within the main view, not as a full-page takeover)

  // ── Onboarding empty state (only when no epics exist at all) ─
  if (!loading && data !== null && epics.length === 0 && !hasActiveFilters) {
    return (
      <div className="flex h-full min-h-0 flex-col">
        {pageHeader}
        <div className="min-h-0 flex-1 overflow-auto">
          <div className="mx-auto max-w-4xl">
            <div className="flex flex-col items-center px-4 py-16">
              <div className="flex h-14 w-14 items-center justify-center rounded-full bg-teal-500/10 mb-5">
                <ChartGanttIcon className="h-7 w-7 text-teal-500" />
              </div>
              <h3 className="text-base font-medium mb-1">Plan your roadmap</h3>
              <p className="text-sm text-muted-foreground text-center max-w-md">
                Link epics to objectives and set start & target dates to visualize
                your initiatives on a timeline.
              </p>
              <div className="flex items-center gap-3 mt-6">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => navigate({ to: '/w/$slug/pm/epics', params: { slug } })}
                >
                  <Layers01Icon className="h-4 w-4 mr-1.5" />
                  Browse Epics
                </Button>
              </div>
              <div className="w-full max-w-4xl mt-10">
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                  {[
                    { icon: Layers01Icon, title: 'Create epics', description: 'Group related tasks into epics — your key initiatives' },
                    { icon: Target01Icon, title: 'Link objectives', description: 'Connect epics to objectives for strategic alignment' },
                    { icon: Calendar03Icon, title: 'Set dates', description: 'Add start and target dates to place epics on the timeline' },
                  ].map(({ icon: Icon, title, description }) => (
                    <div key={title} className="flex flex-col items-center text-center rounded-lg border border-border/50 bg-muted/30 p-6">
                      <Icon className="h-5 w-5 text-muted-foreground mb-3" />
                      <p className="text-sm font-medium mb-1">{title}</p>
                      <p className="text-sm text-muted-foreground leading-relaxed">{description}</p>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (loading) {
    return <div className="flex h-full min-h-0 flex-col">{pageHeader}</div>;
  }

  // ── Main view ───────────────────────────────────────────────
  return (
    <div className="flex h-full min-h-0 flex-col">
      {pageHeader}
      <div className="min-h-0 flex-1 overflow-auto">
        <div className="mx-auto max-w-[1600px] p-4 md:p-6 2xl:max-w-[1920px] min-[2560px]:max-w-[2400px]">
          {/* Controls */}
        <div className="mb-5 flex flex-wrap items-center justify-end gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="sm" className="h-8 text-xs gap-1">
                Group: {groupBy === 'objective' ? 'Objective' : groupBy === 'team' ? 'Team' : 'Epic'}
                <ArrowDown01Icon className="h-3 w-3" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => setGroupBy('epic')}>Epic</DropdownMenuItem>
              <DropdownMenuItem onClick={() => setGroupBy('objective')}>Objective</DropdownMenuItem>
              {teams.length > 1 && <DropdownMenuItem onClick={() => setGroupBy('team')}>Team</DropdownMenuItem>}
            </DropdownMenuContent>
          </DropdownMenu>

          {/* Zoom */}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="sm" className="h-8 text-xs gap-1">
                Zoom: {zoom === 'month' ? 'Month' : 'Quarter'}
                <ArrowDown01Icon className="h-3 w-3" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => setZoom('month')}>Month</DropdownMenuItem>
              <DropdownMenuItem onClick={() => setZoom('quarter')}>Quarter</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          {/* Filters */}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" size="sm" className="h-8 text-xs gap-1">
                Filters
                {(filterTeamId || filterObjectiveId || filterHealth) && <span className="flex h-4 w-4 items-center justify-center rounded-full bg-primary text-primary-foreground text-[10px]">
                    {[filterTeamId, filterObjectiveId, filterHealth].filter(Boolean).length}
                  </span>}
                <ArrowDown01Icon className="h-3 w-3" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-72 max-h-96">
              {/* Objective filter */}
              {objectives.length > 0 && (
                <>
                  <div className="px-2 py-1 text-[11px] font-medium text-muted-foreground uppercase">Objective</div>
                  <DropdownMenuCheckboxItem
                    className="py-1 pl-2 pr-6 text-sm rounded-md"
                    checked={!filterObjectiveId}
                    onCheckedChange={() => setFilterObjectiveId(undefined)}
                  >
                    All objectives
                  </DropdownMenuCheckboxItem>
                  {objectives.map((obj) => (
                    <DropdownMenuCheckboxItem
                      key={obj.id}
                      className="py-1 pl-2 pr-6 text-sm rounded-md"
                      checked={filterObjectiveId === obj.id}
                      onCheckedChange={() =>
                        setFilterObjectiveId(filterObjectiveId === obj.id ? undefined : obj.id)
                      }
                    >
                      <span className="truncate">{obj.name}</span>
                    </DropdownMenuCheckboxItem>
                  ))}
                </>
              )}

              {/* Team filter */}
              {teams.length > 1 && (
                <>
                  <div className="px-2 py-1 text-[11px] font-medium text-muted-foreground uppercase mt-1">Team</div>
                  <DropdownMenuCheckboxItem
                    className="py-1 pl-2 pr-6 text-sm rounded-md"
                    checked={!filterTeamId}
                    onCheckedChange={() => setFilterTeamId(undefined)}
                  >
                    All teams
                  </DropdownMenuCheckboxItem>
                  {teams.map((team) => (
                    <DropdownMenuCheckboxItem
                      key={team.id}
                      className="py-1 pl-2 pr-6 text-sm rounded-md"
                      checked={filterTeamId === team.id}
                      onCheckedChange={() =>
                        setFilterTeamId(filterTeamId === team.id ? undefined : team.id)
                      }
                    >
                      {team.name}
                    </DropdownMenuCheckboxItem>
                  ))}
                </>
              )}

              {/* Health filter */}
              <div className="px-2 py-1 text-[11px] font-medium text-muted-foreground uppercase mt-1">Health</div>
              <DropdownMenuCheckboxItem
                className="py-1 pl-2 pr-6 text-sm rounded-md"
                checked={!filterHealth}
                onCheckedChange={() => setFilterHealth(undefined)}
              >
                All
              </DropdownMenuCheckboxItem>
              {Object.entries(HEALTH_CONFIG).map(([key, { label, dot }]) => (
                <DropdownMenuCheckboxItem
                  key={key}
                  className="py-1 pl-2 pr-6 text-sm rounded-md"
                  checked={filterHealth === key}
                  onCheckedChange={() =>
                    setFilterHealth(filterHealth === key ? undefined : key)
                  }
                >
                  <span className="flex items-center gap-1.5">
                    <span className={`h-2 w-2 rounded-full ${dot}`} />
                      {label}
                    </span>
                  </DropdownMenuCheckboxItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>

            {/* Show completed toggle */}
            <Button variant={showCompleted ? 'secondary' : 'outline'} size="sm" className="h-8 text-xs" onClick={() => setShowCompleted(!showCompleted)}>
              {showCompleted ? 'Showing completed' : 'Show completed'}
            </Button>
          </div>

          {epics.length === 0 && hasActiveFilters ? (
            <div className="flex flex-col items-center justify-center py-20 px-4">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted mb-3">
                <ChartGanttIcon className="h-5 w-5 text-muted-foreground" />
              </div>
              <p className="text-sm font-medium mb-1">No epics match your filters</p>
              <p className="text-sm text-muted-foreground mb-4">Try adjusting or clearing your filters.</p>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setFilterTeamId(undefined);
                  setFilterObjectiveId(undefined);
                  setFilterHealth(undefined);
                  setShowCompleted(false);
                }}
              >
                Clear filters
              </Button>
            </div>
          ) : (
            <RoadmapTimeline epics={epics} objectives={objectives} groupBy={groupBy} zoom={zoom} slug={slug} teamNameMap={teamNameMap} memberNameMap={memberNameMap} />
          )}

          {/* Unscheduled epics */}
          {unscheduled.length > 0 && (
            <div className="mt-6">
              <h3 className="text-xs font-medium uppercase tracking-wide text-muted-foreground mb-2 px-1">Unscheduled epics ({unscheduled.length})</h3>
              <p className="text-[11px] text-muted-foreground/60 mb-3 px-1">Add start and target dates to place these on the timeline.</p>
              <div className="divide-y divide-border/40 rounded-lg border border-border">
                {unscheduled.map((epic) => (
                  <UnscheduledEpicRow key={epic.epic.id} epic={epic} slug={slug} teamName={epic.epic.team_id ? teamNameMap.get(epic.epic.team_id) : undefined} />
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Unscheduled epic row ────────────────────────────────────────

function UnscheduledEpicRow({ epic, slug, teamName }: { epic: RoadmapEpic; slug: string; teamName?: string }) {
  const navigate = useNavigate();
  const e = epic.epic;
  const health = e.health || 'no_health';
  const hc = HEALTH_CONFIG[health];
  const totalTasks = getEpicTaskCount(epic.stats);
  const pct = totalTasks > 0 ? Math.round((getEpicDoneTaskCount(epic.stats) / totalTasks) * 100) : 0;

  return (
    <button
      type="button"
      onClick={() =>
        navigate({
          to: '/w/$slug/pm/epics/$epicId',
          params: { slug, epicId: e.id },
        })
      }
      className="flex items-center gap-3 px-3 py-2.5 w-full text-left hover:bg-muted/30 transition-colors"
    >
      <span className="flex items-center gap-1.5 shrink-0">
        <span className={`h-2 w-2 rounded-full ${hc?.dot}`} />
      </span>
      <span className="text-sm truncate flex-1 min-w-0">{e.name}</span>
      {epic.objectives.length > 0 && <span className="text-[11px] text-muted-foreground truncate max-w-[120px] shrink-0 hidden sm:block">{epic.objectives[0].name}</span>}
      {teamName && <span className="text-[11px] text-muted-foreground shrink-0 hidden md:block">{teamName}</span>}
      <div className="flex items-center gap-1.5 shrink-0">
        <div className="h-1.5 w-12 rounded-full bg-muted overflow-hidden">
          <div className={`h-full rounded-full ${health === 'off_track' ? 'bg-red-500/70' : health === 'at_risk' ? 'bg-yellow-500/70' : health === 'on_track' ? 'bg-green-500/70' : 'bg-primary/60'}`} style={{ width: `${pct}%` }} />
        </div>
        <span className="text-[11px] text-muted-foreground tabular-nums w-7 text-right">{pct}%</span>
      </div>
    </button>
  );
}
