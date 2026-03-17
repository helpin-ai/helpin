import { useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { format, parseISO } from 'date-fns';
import { useTitle } from '@/hooks/useTitle';
import {
  Archive,
  CalendarDays,
  Crosshair,
  Filter,
  Hexagon,
  ListChecks,
  Loader2,
  MoreHorizontal,
  Plus,
  Target,
  TrendingUp,
  X,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useObjectives, useDeleteObjective } from '@/hooks/queries/useObjectives';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import type { ObjectiveState, ObjectiveWithDetails } from '@/lib/pmTypes';
import { OBJECTIVE_STATE_CONFIG } from '@/lib/pmConstants';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';

const healthConfig: Record<string, { label: string; className: string }> = {
  on_track: { label: 'On Track', className: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' },
  at_risk: { label: 'At Risk', className: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  off_track: { label: 'Off Track', className: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' },
};

const stateFilterOptions: { value: string; label: string }[] = [
  { value: 'not_started', label: 'Not Started' },
  { value: 'active', label: 'In Progress' },
  { value: 'closed', label: 'Done' },
];

const typeFilterOptions: { value: string; label: string }[] = [
  { value: 'strategic', label: 'Strategic' },
  { value: 'tactical', label: 'Tactical' },
];

const healthFilterOptions: { value: string; label: string }[] = [
  { value: 'on_track', label: 'On Track' },
  { value: 'at_risk', label: 'At Risk' },
  { value: 'off_track', label: 'Off Track' },
];

function FilterChip({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: { value: string; label: string }[];
  value: string;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const selected = options.find((o) => o.value === value);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className={`inline-flex items-center gap-1 rounded-md border px-2.5 py-1 text-xs transition-colors cursor-pointer ${
            value
              ? 'border-primary/30 bg-primary/5 text-foreground'
              : 'border-border/60 text-muted-foreground hover:bg-accent'
          }`}
        >
          {selected ? selected.label : label}
          {value && (
            <span
              className="ml-0.5 rounded-full hover:bg-accent p-0.5"
              onClick={(e) => { e.stopPropagation(); onChange(''); }}
            >
              <X className="h-2.5 w-2.5" />
            </span>
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-40 p-0.5" align="start">
        <div className="flex flex-col">
          {options.map((opt) => (
            <button
              key={opt.value}
              type="button"
              className={`flex items-center rounded-sm px-2 py-1.5 text-xs transition-colors cursor-pointer ${
                value === opt.value ? 'bg-accent text-foreground font-medium' : 'text-muted-foreground hover:bg-accent hover:text-foreground'
              }`}
              onClick={() => { onChange(value === opt.value ? '' : opt.value); setOpen(false); }}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}

export function ObjectivesPage() {
  useTitle('Objectives');

  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const navigate = useNavigate();
  const openCreate = useGlobalCreateStore((s) => s.openCreate);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { canEdit, isAdmin } = usePermissions(access);
  const { teams } = useAccessibleTeams(workspaceId || '');

  // Filters
  const [filterState, setFilterState] = useState('');
  const [filterTeam, setFilterTeam] = useState('');
  const [filterType, setFilterType] = useState('');
  const [filterHealth, setFilterHealth] = useState('');

  const { data: objectives = [], isLoading: loading } = useObjectives(workspaceId, {
    archived: false,
    state: filterState || undefined,
    team_id: filterTeam || undefined,
    objective_type: filterType || undefined,
  });

  const deleteObjective = useDeleteObjective(workspaceId);

  // Health is client-side filtered (not in API)
  const filtered = useMemo(() => {
    if (!filterHealth) return objectives;
    return objectives.filter((o) => o.objective.health === filterHealth);
  }, [objectives, filterHealth]);

  const handleArchive = async (id: string) => {
    if (!workspaceId) return;
    deleteObjective.mutate(id);
  };

  const activeFilterCount = [filterState, filterTeam, filterType, filterHealth].filter(Boolean).length;

  const clearAllFilters = () => {
    setFilterState('');
    setFilterTeam('');
    setFilterType('');
    setFilterHealth('');
  };

  const teamOptions = useMemo(
    () => teams.map((t) => ({ value: t.id, label: t.name })),
    [teams],
  );

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (objectives.length === 0 && !activeFilterCount) {
    return (
      <div className="flex flex-col items-center justify-center py-16 px-4">
        <div className="flex h-14 w-14 items-center justify-center rounded-full bg-amber-500/10 mb-5">
          <Target className="h-7 w-7 text-amber-500" />
        </div>
        <h3 className="text-lg font-semibold mb-1.5">Create your first objective</h3>
        <p className="text-sm text-muted-foreground text-center max-w-md mb-6">
          Objectives align your team around measurable goals with key results, keeping everyone focused on outcomes that matter.
        </p>
        {canEdit && (
          <Button className="gap-2 mb-8" onClick={() => openCreate('objective')}>
            <Plus className="h-4 w-4" />
            Create Objective
          </Button>
        )}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 w-full max-w-4xl">
          {[
            { icon: Crosshair, title: 'Set goals', desc: 'Define clear objectives with measurable key results' },
            { icon: TrendingUp, title: 'Measure progress', desc: 'Track completion across key results and linked epics' },
            { icon: ListChecks, title: 'Align teams', desc: 'Connect objectives to team work for shared accountability' },
          ].map((item) => (
            <div key={item.title} className="flex flex-col items-center text-center rounded-lg border border-border/50 bg-muted/30 p-6">
              <item.icon className="h-5 w-5 text-muted-foreground mb-3" />
              <p className="text-sm font-medium mb-1">{item.title}</p>
              <p className="text-[13px] text-muted-foreground leading-relaxed">{item.desc}</p>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto">
      <div className="mb-4 flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Objectives</h1>
          <p className="text-sm text-muted-foreground">Set measurable goals and track key results across your team.</p>
        </div>
        {canEdit && (
          <Button size="sm" onClick={() => openCreate('objective')}>
            Create Objective
          </Button>
        )}
      </div>

      {/* Filters */}
      <div className="mb-4 flex items-center gap-2 flex-wrap">
        <Filter className="h-3.5 w-3.5 text-muted-foreground" />
        <FilterChip label="Status" options={stateFilterOptions} value={filterState} onChange={setFilterState} />
        <FilterChip label="Health" options={healthFilterOptions} value={filterHealth} onChange={setFilterHealth} />
        {teamOptions.length > 0 && (
          <FilterChip label="Team" options={teamOptions} value={filterTeam} onChange={setFilterTeam} />
        )}
        <FilterChip label="Type" options={typeFilterOptions} value={filterType} onChange={setFilterType} />
        {activeFilterCount > 0 && (
          <button
            type="button"
            className="text-xs text-muted-foreground hover:text-foreground cursor-pointer ml-1"
            onClick={clearAllFilters}
          >
            Clear all
          </button>
        )}
      </div>

      {filtered.length > 0 ? (
        <div className="grid gap-4 sm:grid-cols-1 md:grid-cols-2 xl:grid-cols-3 max-w-6xl">
          {filtered.map((obj) => (
            <ObjectiveCard
              key={obj.objective.id}
              data={obj}
              canEdit={canEdit}
              isAdmin={isAdmin}
              onArchive={() => handleArchive(obj.objective.id)}
              onClick={() => navigate({ to: `/w/${workspace!.slug}/pm/objectives/${obj.objective.id}` } as any)}
            />
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <p className="text-sm text-muted-foreground">No objectives match the current filters</p>
          <button
            type="button"
            className="mt-2 text-xs text-primary hover:underline cursor-pointer"
            onClick={clearAllFilters}
          >
            Clear filters
          </button>
        </div>
      )}
    </div>
  );
}

function formatDate(iso: string) {
  try { return format(parseISO(iso.slice(0, 10)), 'MMM d'); } catch { return iso.slice(0, 10); }
}

function ObjectiveCard({
  data,
  canEdit,
  isAdmin,
  onArchive,
  onClick,
}: {
  data: ObjectiveWithDetails;
  canEdit: boolean;
  isAdmin: boolean;
  onArchive: () => void;
  onClick: () => void;
}) {
  const { objective, stats, epics } = data;
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const isStrategic = objective.objective_type === 'strategic';
  const stateCfg = OBJECTIVE_STATE_CONFIG[objective.state as ObjectiveState] ?? OBJECTIVE_STATE_CONFIG.not_started;
  const health = healthConfig[objective.health] ?? healthConfig.on_track;

  const krProgress = Math.round(stats.key_result_avg_pct);
  const epicProgress = Math.round(stats.epic_progress_pct);
  const hasKr = stats.key_result_count > 0;
  const hasEpics = epics.length > 0;

  const dateLabel = useMemo(() => {
    if (objective.planned_start_date && objective.deadline)
      return `${formatDate(objective.planned_start_date)} → ${formatDate(objective.deadline)}`;
    if (objective.deadline) return `Due ${formatDate(objective.deadline)}`;
    if (objective.planned_start_date) return `From ${formatDate(objective.planned_start_date)}`;
    return '';
  }, [objective.planned_start_date, objective.deadline]);

  return (
    <article
      className="group flex flex-col rounded-lg border border-border/60 bg-card transition-all hover:shadow-md hover:border-border cursor-pointer"
      onClick={onClick}
    >
      {/* Header */}
      <div className="p-3.5 pb-0">
        <div className="flex items-start gap-2">
          <span className={`mt-[2px] shrink-0 ${isStrategic ? 'text-violet-500' : 'text-blue-500'}`}>
            {isStrategic ? <Crosshair className="h-3.5 w-3.5" /> : <Target className="h-3.5 w-3.5" />}
          </span>
          <p className="min-w-0 flex-1 text-sm font-semibold text-foreground leading-snug line-clamp-2">{objective.name}</p>
          {canEdit && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6 -mt-0.5 -mr-1 shrink-0 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity"
                  onClick={(e) => e.stopPropagation()}
                >
                  <MoreHorizontal className="h-3.5 w-3.5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
                {isAdmin && (
                  <DropdownMenuItem onClick={() => setArchiveConfirmOpen(true)}>
                    <Archive className="mr-2 h-4 w-4 text-amber-500" />
                    Archive
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>

        {/* Meta row */}
        <div className="mt-2 mb-3 flex items-center justify-between">
          <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium leading-none ${stateCfg.badge}`}>
            {stateCfg.label}
          </span>
          {objective.state !== 'closed' && (
            <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium leading-none ${health.className}`}>
              {health.label}
            </span>
          )}
          {dateLabel && (
            <span className="flex items-center gap-1 text-xs text-muted-foreground">
              <CalendarDays className="h-3 w-3" />
              {dateLabel}
            </span>
          )}
        </div>
      </div>

      {/* Progress bars */}
      {(hasKr || hasEpics) && (
        <div className="mx-3.5 py-2.5 border-t border-border/40 grid grid-cols-[1fr_80px_32px] items-center gap-x-2 gap-y-2">
          {hasKr && (
            <>
              <span className="text-[11px] text-muted-foreground">KR Progress</span>
              <Progress value={krProgress} className="h-1.5 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />
              <span className="text-xs font-medium tabular-nums text-right">{krProgress}%</span>
            </>
          )}
          {hasEpics && (
            <>
              <span className="text-[11px] text-muted-foreground">Epic Progress</span>
              <Progress value={epicProgress} className="h-1.5 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />
              <span className="text-xs font-medium tabular-nums text-right">{epicProgress}%</span>
            </>
          )}
        </div>
      )}

      {/* Linked epics */}
      {hasEpics && (
        <div className="mx-3.5 pb-3 pt-2 border-t border-border/40 space-y-0.5">
          {epics.slice(0, 4).map((e) => {
            const epicPct = e.stats.story_count > 0
              ? Math.round((e.stats.done_story_count / e.stats.story_count) * 100)
              : 0;
            return (
              <div key={e.epic.id} className="flex items-center gap-1.5 py-0.5">
                <Hexagon className="h-3 w-3 shrink-0 text-violet-400" />
                <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">{e.epic.name}</span>
                <div className="w-16 shrink-0">
                  <Progress value={epicPct} className="h-1 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />
                </div>
                <span className="w-7 text-right text-[11px] text-muted-foreground tabular-nums">{epicPct}%</span>
              </div>
            );
          })}
          {epics.length > 4 && (
            <p className="text-[10px] text-muted-foreground pl-4">+{epics.length - 4} more</p>
          )}
        </div>
      )}

      <ConfirmDialog
        open={archiveConfirmOpen}
        onOpenChange={setArchiveConfirmOpen}
        title="Archive objective"
        description="This objective will be hidden from the list. You can restore it later from archived items."
        confirmLabel="Archive"
        variant="default"
        onConfirm={onArchive}
      />
    </article>
  );
}
