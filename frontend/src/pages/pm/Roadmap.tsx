import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { isValid, parseISO } from 'date-fns';
import { toast } from 'sonner';

import {
  ArrowDown01Icon,
  Calendar03Icon,
  Layers01Icon,
  PlusSignIcon,
  Target01Icon,
} from '@/lib/icons';
import { useTitle } from '@/hooks/useTitle';
import {
  QuietEmptyState,
  QuietPageHeader,
  QuietPageViewport,
  QuietPrimaryAction,
  QuietSearchInput,
  QuietSectionHeader,
  QuietTextAction,
} from '@/components/design-system/quiet';

import { Skeleton } from '@/components/ui/skeleton';
import { InlineEpicDateControl, InlineEpicObjectivesControl } from '@/components/pm/InlineEpicPlanningFields';
import {
  PMFilterBar,
  PMFilterTrigger,
  type PMFilterDefinition,
  type PMFilterValues,
} from '@/components/pm/PMFilterControls';
import { EpicColorSwatch } from '@/components/pm/EpicColorSwatch';
import { RoadmapTimeline } from '@/components/pm/RoadmapTimeline';
import { getRoadmapEpicRange, roadmapEpicMatchesSearch } from '@/components/pm/roadmapUtils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import { pmRoadmapService } from '@/lib/services/pmRoadmapService';
import type { EpicWithStats, Objective, RoadmapData, RoadmapEpic, UpdateEpicRequest } from '@/lib/pmTypes';
import { getEpicDoneTaskCount, getEpicTaskCount } from '@/lib/pmTypes';

type GroupBy = 'objective' | 'team' | 'epic';
type Zoom = 'month' | 'quarter';
type RoadmapFilterKey = 'objective' | 'team' | 'health' | 'completed';

const PLANNING_PREVIEW_COUNT = 6;
const EMPTY_EPICS: RoadmapEpic[] = [];
const EMPTY_OBJECTIVES: Objective[] = [];

const HEALTH_CONFIG: Record<string, { label: string; marker: string; text: string }> = {
  no_health: { label: 'No health', marker: 'bg-quiet-empty', text: 'text-quiet-text-tertiary' },
  on_track: { label: 'On track', marker: 'bg-quiet-positive', text: 'text-quiet-positive' },
  at_risk: { label: 'At risk', marker: 'bg-quiet-accent', text: 'text-quiet-accent' },
  off_track: { label: 'Off track', marker: 'bg-quiet-accent', text: 'text-quiet-accent' },
};

function parseValidDate(value?: string) {
  if (!value) return undefined;
  const parsed = parseISO(value);
  return isValid(parsed) ? parsed : undefined;
}

function applyRoadmapEpicPatch(entry: RoadmapEpic, patch: UpdateEpicRequest): RoadmapEpic {
  return {
    ...entry,
    epic: {
      ...entry.epic,
      ...(Object.prototype.hasOwnProperty.call(patch, 'planned_start_date')
        ? { planned_start_date: patch.planned_start_date }
        : {}),
      ...(Object.prototype.hasOwnProperty.call(patch, 'deadline')
        ? { deadline: patch.deadline }
        : {}),
    },
  };
}

function buildObjectiveRefs(
  objectives: Objective[],
  current: EpicWithStats['objectives'],
  objectiveIds: string[],
): EpicWithStats['objectives'] {
  const names = new Map<string, string>();
  objectives.forEach((objective) => names.set(objective.id, objective.name));
  current.forEach((objective) => names.set(objective.id, objective.name));
  return objectiveIds.map((id) => ({ id, name: names.get(id) ?? 'Unknown objective' }));
}

export function RoadmapPage() {
  useTitle('Roadmap');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const slug = workspace?.slug ?? '';
  const navigate = useNavigate();
  const openCreate = useGlobalCreateStore((state) => state.openCreate);

  const { data: access } = useWorkspaceAccess(workspaceId);
  const { canEdit } = usePermissions(access);
  const { teams } = useAccessibleTeams(workspaceId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(workspaceId);
  const memberNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const teamNameMap = useMemo(() => {
    const map = new Map<string, string>();
    teams.forEach((team) => map.set(team.id, team.name));
    return map;
  }, [teams]);

  const [groupBy, setGroupBy] = useState<GroupBy>('objective');
  const [zoom, setZoom] = useState<Zoom>('quarter');
  const [showCompleted, setShowCompleted] = useState(false);
  const [filterTeamId, setFilterTeamId] = useState<string>();
  const [filterObjectiveId, setFilterObjectiveId] = useState<string>();
  const [filterHealth, setFilterHealth] = useState<string>();
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search);
  const [showAllPlanning, setShowAllPlanning] = useState(false);

  const [data, setData] = useState<RoadmapData | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [savingEpicIds, setSavingEpicIds] = useState<Set<string>>(new Set());
  const requestVersion = useRef(0);

  const loadRoadmap = useCallback(async (showLoadingState = true) => {
    if (!workspaceId) return;
    const version = ++requestVersion.current;
    if (showLoadingState) setLoading(true);
    setLoadError(null);
    const result = await pmRoadmapService.getData(workspaceId, {
      team_id: filterTeamId,
      objective_id: filterObjectiveId,
      health: filterHealth,
      show_completed: showCompleted,
    });
    if (version !== requestVersion.current) return;
    if (result.data) setData(result.data);
    else setLoadError(result.error ?? 'Failed to load the roadmap');
    setLoading(false);
  }, [filterHealth, filterObjectiveId, filterTeamId, showCompleted, workspaceId]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Route filters intentionally trigger a server refresh.
    void loadRoadmap();
  }, [loadRoadmap]);

  useEffect(() => {
    const refresh = () => void loadRoadmap(false);
    window.addEventListener('epic-created', refresh);
    window.addEventListener('epic-updated', refresh);
    return () => {
      window.removeEventListener('epic-created', refresh);
      window.removeEventListener('epic-updated', refresh);
    };
  }, [loadRoadmap]);

  const epics = data?.epics ?? EMPTY_EPICS;
  const objectives = data?.objectives ?? EMPTY_OBJECTIVES;
  const visibleEpics = useMemo(
    () => epics.filter((entry) => roadmapEpicMatchesSearch(entry, deferredSearch, teamNameMap)),
    [deferredSearch, epics, teamNameMap],
  );
  const planningEpics = useMemo(
    () => visibleEpics.filter((entry) => !getRoadmapEpicRange(entry)),
    [visibleEpics],
  );
  const markSaving = useCallback((epicId: string, saving: boolean) => {
    setSavingEpicIds((current) => {
      const next = new Set(current);
      if (saving) next.add(epicId);
      else next.delete(epicId);
      return next;
    });
  }, []);

  const replaceEpic = useCallback((epicId: string, replacement: RoadmapEpic | null) => {
    setData((current) => current
      ? {
          ...current,
          epics: replacement
            ? current.epics.map((entry) => (entry.epic.id === epicId ? replacement : entry))
            : current.epics.filter((entry) => entry.epic.id !== epicId),
        }
      : current);
  }, []);

  const updateEpicField = useCallback(async (epicId: string, patch: UpdateEpicRequest) => {
    const previous = data?.epics.find((entry) => entry.epic.id === epicId);
    if (!previous || !workspaceId) return;

    markSaving(epicId, true);
    replaceEpic(epicId, applyRoadmapEpicPatch(previous, patch));
    try {
      const result = await pmEpicService.update(workspaceId, epicId, patch);
      if (!result.data || result.error) throw new Error(result.error ?? 'Failed to update epic dates');
      replaceEpic(epicId, result.data);
    } catch (error) {
      replaceEpic(epicId, previous);
      toast.error(error instanceof Error ? error.message : 'Failed to update epic dates');
    } finally {
      markSaving(epicId, false);
    }
  }, [data?.epics, markSaving, replaceEpic, workspaceId]);

  const updateEpicObjectives = useCallback(async (epicId: string, nextObjectiveIds: string[]) => {
    const previous = data?.epics.find((entry) => entry.epic.id === epicId);
    if (!previous || !workspaceId) return;

    const currentObjectiveIds = previous.objectives.map((objective) => objective.id);
    const currentSet = new Set(currentObjectiveIds);
    const nextSet = new Set(nextObjectiveIds);
    const toAdd = nextObjectiveIds.filter((id) => !currentSet.has(id));
    const toRemove = currentObjectiveIds.filter((id) => !nextSet.has(id));
    const optimistic = {
      ...previous,
      objectives: buildObjectiveRefs(objectives, previous.objectives, nextObjectiveIds),
    };

    markSaving(epicId, true);
    replaceEpic(epicId, optimistic);
    try {
      const results = await Promise.all([
        ...toAdd.map((objectiveId) => pmObjectiveService.addEpic(workspaceId, objectiveId, epicId)),
        ...toRemove.map((objectiveId) => pmObjectiveService.removeEpic(workspaceId, objectiveId, epicId)),
      ]);
      const failed = results.find((result) => result.error);
      if (failed) throw new Error(failed.error ?? 'Failed to update objectives');

      if (filterObjectiveId && !nextSet.has(filterObjectiveId)) {
        replaceEpic(epicId, null);
      } else {
        const refreshed = await pmEpicService.get(workspaceId, epicId);
        if (refreshed.data) replaceEpic(epicId, refreshed.data);
      }
    } catch (error) {
      replaceEpic(epicId, previous);
      toast.error(error instanceof Error ? error.message : 'Failed to update objectives');
      void loadRoadmap(false);
    } finally {
      markSaving(epicId, false);
    }
  }, [data?.epics, filterObjectiveId, loadRoadmap, markSaving, objectives, replaceEpic, workspaceId]);

  const clearFilters = () => {
    setFilterTeamId(undefined);
    setFilterObjectiveId(undefined);
    setFilterHealth(undefined);
    setShowCompleted(false);
    setSearch('');
  };

  const clearFieldFilters = () => {
    setFilterTeamId(undefined);
    setFilterObjectiveId(undefined);
    setFilterHealth(undefined);
    setShowCompleted(false);
  };

  if (!workspace) return <p className="text-sm text-quiet-text-tertiary">Workspace not found.</p>;

  const hasActiveFilters = Boolean(
    filterTeamId || filterObjectiveId || filterHealth || showCompleted || search.trim(),
  );
  const pageHeader = (
    <QuietPageHeader
      variant="shell"
      title="Roadmap"
      description="Plan epic timing and see how work supports each objective."
      actions={canEdit ? (
        <QuietPrimaryAction className="gap-1.5" onClick={() => openCreate('epic')}>
          <PlusSignIcon className="h-4 w-4" />
          Add epic
        </QuietPrimaryAction>
      ) : null}
    />
  );

  if (loading && data === null) {
    return (
      <div className="flex h-full min-h-0 flex-col">
        {pageHeader}
        <QuietPageViewport className="min-h-0 flex-1">
          <RoadmapLoadingState />
        </QuietPageViewport>
      </div>
    );
  }

  if (loadError && data === null) {
    return (
      <div className="flex h-full min-h-0 flex-col">
        {pageHeader}
        <QuietPageViewport className="min-h-0 flex-1">
          <QuietEmptyState
            title="The roadmap could not be loaded"
            description={loadError}
            action={<QuietTextAction onClick={() => void loadRoadmap()}>Try again</QuietTextAction>}
          />
        </QuietPageViewport>
      </div>
    );
  }

  if (epics.length === 0 && !hasActiveFilters) {
    return (
      <div className="flex h-full min-h-0 flex-col">
        {pageHeader}
        <QuietPageViewport className="min-h-0 flex-1">
          <QuietEmptyState
            title="Plan your first initiative"
            description="Create an epic, add its timing, and link it to an objective. Scheduled epics will appear here automatically."
            action={(
              <QuietTextAction onClick={() => navigate({ to: '/w/$slug/pm/epics', params: { slug } })}>
                Browse epics
              </QuietTextAction>
            )}
          >
            <div className="max-w-[620px] border-t border-quiet-divider-strong">
              <RoadmapSetupStep icon={Layers01Icon} title="Create epics" detail="Group related tasks into an initiative." />
              <RoadmapSetupStep icon={Calendar03Icon} title="Set timing" detail="Add both a start and target date." />
              <RoadmapSetupStep icon={Target01Icon} title="Link objectives" detail="Show how the initiative supports company goals." />
            </div>
          </QuietEmptyState>
        </QuietPageViewport>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      {pageHeader}
      <QuietPageViewport className="min-h-0 flex-1 p-0 md:p-0" contentClassName="max-w-none">
        <div aria-busy={loading}>
          <RoadmapToolbar
            search={search}
            onSearchChange={setSearch}
            groupBy={groupBy}
            onGroupByChange={setGroupBy}
            zoom={zoom}
            onZoomChange={setZoom}
            showCompleted={showCompleted}
            onShowCompletedChange={setShowCompleted}
            teams={teams}
            objectives={objectives}
            filterTeamId={filterTeamId}
            onFilterTeamIdChange={setFilterTeamId}
            filterObjectiveId={filterObjectiveId}
            onFilterObjectiveIdChange={setFilterObjectiveId}
            filterHealth={filterHealth}
            onFilterHealthChange={setFilterHealth}
            onClearFilters={clearFieldFilters}
          />

          {loadError ? (
            <p className="border-b border-quiet-divider-strong px-4 py-2 text-[12.5px] text-quiet-accent sm:px-6 lg:px-8">
              {loadError} The last loaded roadmap is still shown.
            </p>
          ) : null}

          {visibleEpics.length === 0 ? (
            <QuietEmptyState
              className="border-t-0"
              title="No epics match this view"
              description="Adjust the search or filters to bring initiatives back into the roadmap."
              action={<QuietTextAction onClick={clearFilters}>Clear filters</QuietTextAction>}
            />
          ) : (
            <>
              <section>
                <RoadmapTimeline
                  epics={visibleEpics}
                  objectives={objectives}
                  groupBy={groupBy}
                  zoom={zoom}
                  slug={slug}
                  teamNameMap={teamNameMap}
                  memberNameMap={memberNameMap}
                />
              </section>

              {planningEpics.length > 0 ? (
                <div className="mx-auto w-full max-w-7xl px-4 pb-8 pt-7 sm:px-6 lg:px-8">
                  <PlanningQueue
                    epics={planningEpics}
                    objectives={objectives}
                    slug={slug}
                    teamNameMap={teamNameMap}
                    canEdit={canEdit}
                    savingEpicIds={savingEpicIds}
                    showAll={showAllPlanning}
                    onShowAllChange={setShowAllPlanning}
                    onUpdateField={updateEpicField}
                    onUpdateObjectives={updateEpicObjectives}
                  />
                </div>
              ) : null}
            </>
          )}
        </div>
      </QuietPageViewport>
    </div>
  );
}

function RoadmapToolbar({
  search,
  onSearchChange,
  groupBy,
  onGroupByChange,
  zoom,
  onZoomChange,
  showCompleted,
  onShowCompletedChange,
  teams,
  objectives,
  filterTeamId,
  onFilterTeamIdChange,
  filterObjectiveId,
  onFilterObjectiveIdChange,
  filterHealth,
  onFilterHealthChange,
  onClearFilters,
}: {
  search: string;
  onSearchChange: (value: string) => void;
  groupBy: GroupBy;
  onGroupByChange: (value: GroupBy) => void;
  zoom: Zoom;
  onZoomChange: (value: Zoom) => void;
  showCompleted: boolean;
  onShowCompletedChange: (value: boolean) => void;
  teams: Array<{ id: string; name: string }>;
  objectives: Objective[];
  filterTeamId?: string;
  onFilterTeamIdChange: (value?: string) => void;
  filterObjectiveId?: string;
  onFilterObjectiveIdChange: (value?: string) => void;
  filterHealth?: string;
  onFilterHealthChange: (value?: string) => void;
  onClearFilters: () => void;
}) {
  const [visibleFilterKeys, setVisibleFilterKeys] = useState<Set<RoadmapFilterKey>>(() => {
    const keys = new Set<RoadmapFilterKey>();
    if (filterObjectiveId) keys.add('objective');
    if (filterTeamId) keys.add('team');
    if (filterHealth) keys.add('health');
    if (showCompleted) keys.add('completed');
    return keys;
  });
  const filterDefinitions = useMemo<PMFilterDefinition<RoadmapFilterKey>[]>(() => [
    {
      key: 'objective',
      label: 'Objective',
      options: objectives.map((objective) => ({ value: objective.id, label: objective.name })),
      singleSelect: true,
      searchableValues: true,
    },
    {
      key: 'team',
      label: 'Team',
      options: teams.map((team) => ({ value: team.id, label: team.name })),
      singleSelect: true,
      searchableValues: true,
    },
    {
      key: 'health',
      label: 'Health',
      options: Object.entries(HEALTH_CONFIG).map(([value, config]) => ({
        value,
        label: config.label,
        icon: <span className={`h-[5px] w-[5px] shrink-0 rounded-full ${config.marker}`} />,
      })),
      singleSelect: true,
    },
    {
      key: 'completed',
      label: 'Completed epics',
      options: [{ value: 'true', label: 'Included' }],
      singleSelect: true,
    },
  ], [objectives, teams]);
  const filterValues = useMemo<PMFilterValues<RoadmapFilterKey>>(() => ({
    objective: filterObjectiveId ? [filterObjectiveId] : [],
    team: filterTeamId ? [filterTeamId] : [],
    health: filterHealth ? [filterHealth] : [],
    completed: showCompleted ? ['true'] : [],
  }), [filterHealth, filterObjectiveId, filterTeamId, showCompleted]);
  const activeFilterCount = Object.values(filterValues).filter((values) => values && values.length > 0).length;

  const clearFilterValue = (key: RoadmapFilterKey) => {
    if (key === 'objective') onFilterObjectiveIdChange(undefined);
    if (key === 'team') onFilterTeamIdChange(undefined);
    if (key === 'health') onFilterHealthChange(undefined);
    if (key === 'completed') onShowCompletedChange(false);
  };

  const handleAddFilter = (key: RoadmapFilterKey) => {
    setVisibleFilterKeys((current) => new Set(current).add(key));
  };

  const handleToggleFilter = (key: RoadmapFilterKey, value: string) => {
    const isSelected = filterValues[key]?.includes(value) ?? false;
    if (key === 'objective') onFilterObjectiveIdChange(isSelected ? undefined : value);
    if (key === 'team') onFilterTeamIdChange(isSelected ? undefined : value);
    if (key === 'health') onFilterHealthChange(isSelected ? undefined : value);
    if (key === 'completed') onShowCompletedChange(!isSelected);
  };

  const handleRemoveFilter = (key: RoadmapFilterKey) => {
    clearFilterValue(key);
    setVisibleFilterKeys((current) => {
      const next = new Set(current);
      next.delete(key);
      return next;
    });
  };

  const handleClearFilters = () => {
    setVisibleFilterKeys(new Set());
    onClearFilters();
  };

  return (
    <div>
      <div className="ui-divider-bottom-fade flex min-h-11 flex-wrap items-center gap-2 px-3 py-2">
        <QuietSearchInput
          containerClassName="w-full sm:w-[220px]"
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
          placeholder="Search roadmap…"
        />

        <PMFilterTrigger
          definitions={filterDefinitions}
          values={filterValues}
          visibleKeys={visibleFilterKeys}
          activeCount={activeFilterCount}
          onAdd={handleAddFilter}
          onToggle={handleToggleFilter}
        />

        <div className="ml-auto flex items-center gap-3">
          <QuietDropdown label="Group by" selected={[groupBy]} onSelect={value => onGroupByChange(value as typeof groupBy)}
            options={[{ value: 'epic', label: 'Epic' }, { value: 'objective', label: 'Objective' }, ...(teams.length > 1 ? [{ value: 'team', label: 'Team' }] : [])]}
            contentProps={{ align: 'end' }} trigger={<QuietTextAction className="h-8 gap-1.5 px-1">
              Group by {groupBy}<ArrowDown01Icon className="h-3 w-3 text-quiet-muted" />
            </QuietTextAction>} />
          <QuietDropdown label="Zoom" selected={[zoom]} onSelect={value => onZoomChange(value as typeof zoom)}
            options={[{ value: 'month', label: 'Month' }, { value: 'quarter', label: 'Quarter' }]}
            contentProps={{ align: 'end' }} trigger={<QuietTextAction className="h-8 gap-1.5 px-1">
              {zoom === 'month' ? 'Month' : 'Quarter'}<ArrowDown01Icon className="h-3 w-3 text-quiet-muted" />
            </QuietTextAction>} />
        </div>
      </div>

      <PMFilterBar
        definitions={filterDefinitions}
        values={filterValues}
        visibleKeys={visibleFilterKeys}
        onToggle={handleToggleFilter}
        onRemove={handleRemoveFilter}
        onClearAll={handleClearFilters}
      />
    </div>
  );
}

function PlanningQueue({
  epics,
  objectives,
  slug,
  teamNameMap,
  canEdit,
  savingEpicIds,
  showAll,
  onShowAllChange,
  onUpdateField,
  onUpdateObjectives,
}: {
  epics: RoadmapEpic[];
  objectives: Objective[];
  slug: string;
  teamNameMap: Map<string, string>;
  canEdit: boolean;
  savingEpicIds: Set<string>;
  showAll: boolean;
  onShowAllChange: (value: boolean) => void;
  onUpdateField: (epicId: string, patch: UpdateEpicRequest) => Promise<void>;
  onUpdateObjectives: (epicId: string, objectiveIds: string[]) => Promise<void>;
}) {
  const navigate = useNavigate();
  const visible = showAll ? epics : epics.slice(0, PLANNING_PREVIEW_COUNT);
  const hasOverflow = epics.length > PLANNING_PREVIEW_COUNT;

  return (
    <section>
      <QuietSectionHeader
        title="Needs planning"
        count={epics.length}
        action={hasOverflow ? (
          <QuietTextAction onClick={() => onShowAllChange(!showAll)}>
            {showAll ? 'Show less' : `Show all ${epics.length}`}
          </QuietTextAction>
        ) : null}
        className="mb-0.5"
      />
      <p className="mb-3 max-w-[680px] text-[12.5px] leading-5 text-quiet-text-tertiary">
        Add the missing start and target dates to place these epics on the timeline. Objectives are optional.
      </p>

      <div className="border-y border-quiet-divider-strong">
        <div className="hidden grid-cols-[minmax(220px,1.5fr)_minmax(180px,1fr)_120px_120px_minmax(150px,.8fr)] items-center gap-4 border-b border-quiet-divider-strong px-4 py-2 md:grid">
          {['Epic', 'Objective', 'Start', 'Target', 'Team / progress'].map((label) => (
            <span key={label} className="text-[11.5px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">{label}</span>
          ))}
        </div>
        {visible.map((entry) => {
          const epic = entry.epic;
          const health = HEALTH_CONFIG[epic.health || 'no_health'] ?? HEALTH_CONFIG.no_health;
          const totalTasks = getEpicTaskCount(entry.stats);
          const completedTasks = getEpicDoneTaskCount(entry.stats);
          const progress = totalTasks > 0 ? Math.round((completedTasks / totalTasks) * 100) : 0;
          const start = parseValidDate(epic.planned_start_date);
          const target = parseValidDate(epic.deadline);
          const invalidRange = Boolean(start && target && target < start);
          const saving = savingEpicIds.has(epic.id);
          const teamName = epic.team_id ? teamNameMap.get(epic.team_id) : undefined;

          return (
            <div
              key={epic.id}
              className="relative grid grid-cols-2 gap-x-4 gap-y-3 border-b border-quiet-divider-light px-4 py-3 transition-colors last:border-b-0 hover:bg-quiet-row-hover md:grid-cols-[minmax(220px,1.5fr)_minmax(180px,1fr)_120px_120px_minmax(150px,.8fr)] md:items-center md:gap-y-0"
            >
              <span aria-hidden="true" className={`absolute inset-y-0 left-0 w-[3px] ${health.marker}`} />

              <div className="col-span-2 min-w-0 md:col-span-1">
                <button
                  type="button"
                  className="flex max-w-full min-w-0 items-center gap-2 text-left text-[13.5px] font-semibold tracking-[-0.008em] text-quiet-text-primary hover:text-quiet-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-quiet-text-primary"
                  onClick={() => navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug, epicId: epic.id } })}
                >
                  <EpicColorSwatch color={epic.color} />
                  <span className="truncate">{epic.name}</span>
                </button>
                <p className="mt-0.5 truncate text-[11.5px] text-quiet-muted md:hidden">
                  {teamName ?? 'No team'} · <span className={health.text}>{health.label}</span>
                </p>
              </div>

              <div className="col-span-2 min-w-0 md:col-span-1">
                <span className="mb-1 block text-[11.5px] font-semibold uppercase tracking-[0.06em] text-quiet-muted md:hidden">Objective</span>
                <InlineEpicObjectivesControl
                  entry={entry}
                  allObjectives={objectives}
                  onChange={onUpdateObjectives}
                  disabled={!canEdit || saving}
                />
              </div>

              <div className="min-w-0">
                <span className="mb-1 block text-[11.5px] font-semibold uppercase tracking-[0.06em] text-quiet-muted md:hidden">Start</span>
                <InlineEpicDateControl
                  epicId={epic.id}
                  value={epic.planned_start_date}
                  emptyLabel="Add start"
                  ariaLabel={`Set start date for ${epic.name}`}
                  patchKey="planned_start_date"
                  maxDate={target}
                  onUpdate={onUpdateField}
                  disabled={!canEdit || saving}
                />
              </div>

              <div className="min-w-0">
                <span className="mb-1 block text-[11.5px] font-semibold uppercase tracking-[0.06em] text-quiet-muted md:hidden">Target</span>
                <InlineEpicDateControl
                  epicId={epic.id}
                  value={epic.deadline}
                  emptyLabel="Add target"
                  ariaLabel={`Set target date for ${epic.name}`}
                  patchKey="deadline"
                  minDate={start}
                  onUpdate={onUpdateField}
                  disabled={!canEdit || saving}
                />
              </div>

              <div className="col-span-2 min-w-0 md:col-span-1">
                <div className="flex min-w-0 items-center gap-1 text-[12.5px] text-quiet-text-tertiary">
                  <span className="truncate">{teamName ?? 'No team'}</span>
                  <span aria-hidden="true" className="text-quiet-muted">·</span>
                  <span className={`shrink-0 ${health.text}`}>{health.label}</span>
                </div>
                <div className="mt-1 flex items-center gap-2">
                  <div className="h-0.5 w-16 overflow-hidden bg-quiet-divider-strong">
                    <div className="h-full bg-quiet-text-primary" style={{ width: `${progress}%` }} />
                  </div>
                  <span className="text-[11.5px] tabular-nums text-quiet-muted">
                    {completedTasks}/{totalTasks} · {progress}%
                  </span>
                  {invalidRange ? <span className="text-[11.5px] text-quiet-accent">Check dates</span> : null}
                  {saving ? <span className="text-[11.5px] text-quiet-muted">Saving…</span> : null}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function RoadmapSetupStep({
  icon: Icon,
  title,
  detail,
}: {
  icon: typeof Layers01Icon;
  title: string;
  detail: string;
}) {
  return (
    <div className="flex items-start gap-3 border-b border-quiet-divider-light py-3">
      <Icon className="mt-0.5 h-[15px] w-[15px] shrink-0 text-quiet-muted" />
      <span>
        <span className="block text-sm font-medium text-quiet-text-primary">{title}</span>
        <span className="mt-0.5 block text-[12.5px] text-quiet-text-tertiary">{detail}</span>
      </span>
    </div>
  );
}

function RoadmapLoadingState() {
  return (
    <div aria-label="Loading roadmap" className="space-y-7">
      <div className="flex flex-wrap items-center gap-3 border-b border-quiet-divider-strong pb-3">
        <Skeleton className="h-9 w-[220px]" />
        <Skeleton className="h-7 w-24" />
        <Skeleton className="ml-auto h-7 w-32" />
        <Skeleton className="h-7 w-20" />
      </div>
      <div>
        <Skeleton className="mb-3 h-4 w-28" />
        <div className="border-y border-quiet-divider-strong">
          {[0, 1, 2].map((row) => (
            <div key={row} className="grid grid-cols-[1.5fr_1fr_120px_120px_.8fr] gap-4 border-b border-quiet-divider-light px-4 py-3 last:border-b-0">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-2/3" />
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-4 w-24" />
            </div>
          ))}
        </div>
      </div>
      <div>
        <Skeleton className="mb-3 h-4 w-20" />
        <Skeleton className="h-64 w-full rounded-none" />
      </div>
    </div>
  );
}
