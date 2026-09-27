import { ObjectiveQuarterBoard } from './ObjectiveQuarterBoard';
import { QuietEmptyState, QuietFilterDropdown, QuietPageHeader, QuietPrimaryAction, QuietSearchInput, QuietStatusText, QuietTextAction } from '@/components/design-system/quiet';
import { PMFilterBar, type PMFilterDefinition, type PMFilterOption } from '@/components/pm/PMFilterControls';
import { OwnerAvatarFilterRow } from '@/components/pm/OwnerAvatarFilterRow';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { EpicColorSwatch } from '@/components/pm/EpicColorSwatch';
import { memo, useCallback, useDeferredValue, useMemo, useState } from 'react';
import { Link, useLocation } from '@tanstack/react-router';
import { format, parseISO } from 'date-fns';
import { useTitle } from '@/hooks/useTitle';
import {
  Activity01Icon,
  WorkflowSquare01Icon,
  ArchiveIcon,
  Calendar03Icon,
  Loading01Icon,
  MoreHorizontalIcon,
  PlusSignIcon,
  Target01Icon,
  Target02Icon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useObjectives, useDeleteObjective } from '@/hooks/queries/useObjectives';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import type { ObjectiveState, ObjectiveWithDetails } from '@/lib/pmTypes';
import { getEpicDoneTaskCount, getEpicTaskCount } from '@/lib/pmTypes';
import { StateTypeIcon } from '@/lib/pmIcons';
import { OBJECTIVE_STATE_CONFIG } from '@/lib/pmConstants';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

const healthConfig: Record<string, { label: string; className: string }> = {
  on_track: { label: 'On Track', className: 'text-quiet-positive' },
  at_risk: { label: 'At Risk', className: 'text-quiet-accent' },
  off_track: { label: 'Off Track', className: 'text-destructive' },
};

const stateFilterOptions: PMFilterOption[] = [
  { value: 'not_started', label: 'Not Started', icon: <StateTypeIcon stateType="unstarted" className="h-3.5 w-3.5 text-zinc-400" /> },
  { value: 'active', label: 'In Progress', icon: <StateTypeIcon stateType="started" className="h-3.5 w-3.5 text-amber-500" /> },
  { value: 'closed', label: 'Done', icon: <StateTypeIcon stateType="done" className="h-3.5 w-3.5 text-green-500" /> },
];

const typeFilterOptions: { value: string; label: string }[] = [
  { value: 'strategic', label: 'Strategic' },
  { value: 'tactical', label: 'Tactical' },
];

const healthFilterOptions: PMFilterOption[] = [
  { value: 'on_track', label: 'On Track', labelClassName: 'text-green-600' },
  { value: 'at_risk', label: 'At Risk', labelClassName: 'text-yellow-600' },
  { value: 'off_track', label: 'Off Track', labelClassName: 'text-red-600' },
];

const OBJECTIVE_CREATE_TOOLTIP = 'Only team managers can create objectives. Ask your team manager for access.';

function CreateObjectiveButton({
  className,
  disabled,
  onClick,
}: {
  className?: string;
  disabled: boolean;
  onClick: () => void;
}) {
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="inline-flex">
            <QuietPrimaryAction className={`gap-2 ${className ?? ''}`} onClick={onClick} disabled={disabled}>
              <PlusSignIcon className="h-4 w-4" />
              Create Objective
            </QuietPrimaryAction>
          </span>
        </TooltipTrigger>
        {disabled && (
          <TooltipContent side="top" className="max-w-[260px] text-xs">
            {OBJECTIVE_CREATE_TOOLTIP}
          </TooltipContent>
        )}
      </Tooltip>
    </TooltipProvider>
  );
}

type ObjectiveFilterKey = 'owner' | 'state' | 'health' | 'team' | 'type';
type ObjectiveFilters = Partial<Record<ObjectiveFilterKey, string[]>>;

export function ObjectivesPage() {
  const { pathname } = useLocation();
  const title = pathname.endsWith('/team-goals') ? 'Team Goals' : 'Objectives';
  useTitle(title);
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const openCreate = useGlobalCreateStore((s) => s.openCreate);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { canEdit, isAdmin, isTeamManager, teamMemberships } = usePermissions(access);
  const { teams } = useAccessibleTeams(workspaceId);
  const { members } = useAssignableWorkspaceMembers(workspaceId);
  const [filters, setFilters] = useState<ObjectiveFilters>({});
  const [search, setSearch] = useState('');
  const query = useDeferredValue(search.trim().toLowerCase());
  const filterTeam = filters.team?.[0] ?? '';
  const { data: objectives = [], isLoading, isError, refetch } = useObjectives(workspaceId, {
    archived: false,
    state: filters.state?.[0],
    team_id: filterTeam || undefined,
    objective_type: filters.type?.[0],
  });
  const deleteObjective = useDeleteObjective(workspaceId);
  const definitions: PMFilterDefinition<ObjectiveFilterKey>[] = useMemo(() => [
    { key: 'owner', label: 'Owner', searchableValues: true, options: members.map(member => ({
      value: member.id, label: member.display_name || member.email,
      icon: <UserAvatar name={member.display_name || member.email} avatarUrl={member.avatar_url}
        avatarStyle={member.avatar_style} avatarSeed={member.avatar_seed}
        avatarBackgroundMode={member.avatar_background_mode} avatarBackgroundColor={member.avatar_background_color}
        fallbackColorSeed={member.email} className="h-5 w-5" />,
    })) },
    { key: 'state', label: 'Status', singleSelect: true, options: stateFilterOptions },
    { key: 'health', label: 'Health', singleSelect: true, options: healthFilterOptions },
    { key: 'team', label: 'Team', singleSelect: true, searchableValues: true, options: teams.map(team => ({ value: team.id, label: team.name })) },
    { key: 'type', label: 'Type', singleSelect: true, options: typeFilterOptions },
  ], [members, teams]);
  const toggleFilter = (key: ObjectiveFilterKey, value: string) => setFilters(current => {
    const selected = current[key] ?? [];
    const next = selected.includes(value) ? selected.filter(item => item !== value)
      : key === 'owner' ? [...selected, value] : [value];
    return { ...current, [key]: next };
  });
  const visibleKeys = new Set(definitions.filter(definition => filters[definition.key]?.length).map(definition => definition.key));
  const clearAll = () => { setFilters({}); setSearch(''); };
  const hasFilters = visibleKeys.size > 0 || search.trim().length > 0;
  const filtered = useMemo(() => objectives.filter(({ objective, owner_member_ids, owners }) =>
    (!query || objective.name.toLowerCase().includes(query)) &&
    (!filters.health?.length || filters.health.includes(objective.health)) &&
    (!filters.owner?.length || filters.owner.some(id => (owner_member_ids ?? owners).includes(id)))
  ), [objectives, query, filters.health, filters.owner]);
  const handleArchive = useCallback((id: string) => {
    if (workspaceId) deleteObjective.mutate(id);
  }, [deleteObjective, workspaceId]);
  const canCreateObjective = canEdit && (isAdmin || (filterTeam ? isTeamManager(filterTeam) : teamMemberships.some(tm => tm.role === 'owner')));

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden">
      <QuietPageHeader variant="shell" className="shrink-0" title={title} description="Set measurable goals and track key results across your team."
        actions={canEdit && <CreateObjectiveButton disabled={!canCreateObjective} onClick={() => openCreate('objective')} />} />
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-quiet-divider-strong px-4 py-2 md:px-6">
        <QuietSearchInput aria-label="Search objectives" placeholder="Search objectives…" value={search}
          onChange={event => setSearch(event.target.value)} containerClassName="w-[220px] max-w-full" />
        {definitions.map(definition => definition.key === 'owner' && members.length > 0 ? (
          <div key="owner" className="flex items-center gap-1.5">
            <span className="text-xs text-quiet-text-secondary">Owner</span>
            <OwnerAvatarFilterRow workspaceId={workspaceId} members={members} selectedIds={filters.owner ?? []}
              className="ml-0" onToggle={id => toggleFilter('owner', id)} />
          </div>
        ) : (
          <QuietFilterDropdown key={definition.key} label={definition.label} emptyLabel={definition.label}
            icon={definition.key === 'state' ? <WorkflowSquare01Icon className="h-3.5 w-3.5" /> : definition.key === 'health' ? <Activity01Icon className="h-3.5 w-3.5" /> : undefined}
            value={filters[definition.key]?.[0] ?? ''} options={definition.options.map(option => ({ ...option, leading: option.icon }))}
            onChange={value => toggleFilter(definition.key, value)} />
        ))}
        {search && visibleKeys.size === 0 && <QuietTextAction onClick={clearAll}>Clear all</QuietTextAction>}
      </div>
      <div className="shrink-0">
        <PMFilterBar definitions={definitions} values={filters} visibleKeys={visibleKeys}
          onToggle={toggleFilter} onRemove={key => setFilters(current => ({ ...current, [key]: [] }))} onClearAll={clearAll} />
      </div>
      {isLoading ? <div role="status" className="flex items-center gap-2 p-6 text-sm text-quiet-text-tertiary"><Loading01Icon className="h-4 w-4 animate-spin" />Loading objectives…</div>
        : isError ? <QuietEmptyState title="Couldn’t load objectives" description="Try loading your objectives again."
          action={<QuietTextAction onClick={() => void refetch()}>Retry</QuietTextAction>} />
        : <ObjectiveQuarterBoard key={workspaceId} objectives={filtered} allObjectives={objectives}
          emptyState={hasFilters && filtered.length === 0 ? <QuietEmptyState title="No objectives match these filters"
            description="Adjust your search or clear filters to see more objectives."
            action={<QuietTextAction onClick={clearAll}>Clear filters</QuietTextAction>} /> : undefined}
          canCreate={canCreateObjective} expandClosed={!!query || filters.state?.[0] === 'closed'}
          onCreate={objectiveDates => openCreate('objective', { objectiveDates, teamId: filterTeam || undefined })}
          renderCard={objective => <MemoObjectiveCard data={objective} slug={workspace?.slug ?? ''}
            canEdit={canEdit} isAdmin={isAdmin} onArchive={handleArchive} />} />}

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
  slug,
}: {
  data: ObjectiveWithDetails;
  canEdit: boolean;
  isAdmin: boolean;
  onArchive: (id: string) => void;
  slug: string;
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
      className="group relative flex min-w-0 flex-col rounded-lg border border-quiet-divider-strong bg-quiet-surface transition-colors hover:border-quiet-field hover:bg-quiet-row-hover focus-within:outline-2 focus-within:outline-quiet-text-primary"
      style={{ contentVisibility: 'auto', containIntrinsicSize: '280px' }}
    >
      {/* Header */}
      <div className="p-3.5 pb-0">
        <div className="flex items-start gap-2">
          <span className={`mt-[2px] shrink-0 ${isStrategic ? 'text-quiet-lifecycle' : 'text-quiet-text-tertiary'}`}>
            {isStrategic ? <Target02Icon className="h-3.5 w-3.5" /> : <Target01Icon className="h-3.5 w-3.5" />}
          </span>
          <h2 className="m-0 min-w-0 flex-1 text-[13.5px] font-semibold leading-5 tracking-[-0.008em] text-quiet-text-primary"><Link to="/w/$slug/pm/objectives/$objectiveId" params={{ slug, objectiveId: objective.id }} className="line-clamp-2 after:absolute after:inset-0 after:rounded-lg focus-visible:outline-none">{objective.name}</Link></h2>
          {canEdit && isAdmin && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={`Actions for ${objective.name}`}
                  className="relative z-10 h-6 w-6 -mt-0.5 -mr-1 shrink-0 text-quiet-text-tertiary sm:opacity-0 sm:group-hover:opacity-100 focus-visible:opacity-100 data-[state=open]:opacity-100 transition-opacity"
                  onClick={(e) => e.stopPropagation()}
                >
                  <MoreHorizontalIcon className="h-3.5 w-3.5" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
                {isAdmin && (
                  <DropdownMenuItem onClick={() => setArchiveConfirmOpen(true)}>
                    <ArchiveIcon className="mr-2 h-4 w-4 text-amber-500" />
                    Archive
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>

        {/* Meta row */}
        <div className="mt-2 mb-3 flex flex-wrap items-center gap-x-3 gap-y-1">
          <QuietStatusText className={stateCfg.color}>
            {stateCfg.label}
          </QuietStatusText>
          {objective.state !== 'closed' && (
            <span className={`ml-auto whitespace-nowrap text-xs ${health.className}`}>
              {health.label}
            </span>
          )}
          {dateLabel && (
            <span className="ml-auto flex items-center gap-1 whitespace-nowrap text-xs text-muted-foreground">
              <Calendar03Icon className="h-3 w-3" />
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
              <Progress value={krProgress} className="h-1.5 bg-quiet-divider-light [&>[data-slot=progress-indicator]]:bg-quiet-positive" />
              <span className="text-xs font-medium tabular-nums text-right">{krProgress}%</span>
            </>
          )}
          {hasEpics && (
            <>
              <span className="text-[11px] text-muted-foreground">Epic Progress</span>
              <Progress value={epicProgress} className="h-1.5 bg-quiet-divider-light [&>[data-slot=progress-indicator]]:bg-quiet-positive" />
              <span className="text-xs font-medium tabular-nums text-right">{epicProgress}%</span>
            </>
          )}
        </div>
      )}

      {/* Linked epics */}
      {hasEpics && (
        <div className="mx-3.5 pb-3 pt-2 border-t border-border/40 space-y-0.5">
          {epics.slice(0, 4).map((e) => {
            const totalTasks = getEpicTaskCount(e.stats);
            const epicPct = totalTasks > 0
              ? Math.round((getEpicDoneTaskCount(e.stats) / totalTasks) * 100)
              : 0;
            return (
              <div key={e.epic.id} className="flex items-center gap-1.5 py-0.5">
                <EpicColorSwatch color={e.epic.color} />
                <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">{e.epic.name}</span>
                <div className="w-16 shrink-0">
                  <Progress value={epicPct} className="h-1 bg-quiet-divider-light [&>[data-slot=progress-indicator]]:bg-quiet-positive" />
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
        onConfirm={() => onArchive(objective.id)}
      />
    </article>
  );
}

const MemoObjectiveCard = memo(ObjectiveCard, (prev, next) => (
  prev.data === next.data &&
  prev.canEdit === next.canEdit &&
  prev.isAdmin === next.isAdmin &&
  prev.onArchive === next.onArchive &&
  prev.slug === next.slug
));
