import { DealsFilterBar, DealsActiveFilterBar } from '@/components/crm/DealsFilterBar';
import { buildCRMDealQueryFields } from '@/lib/crmDealQueryBuilder';
import { parseQueryFilterGroup, serializeQueryFilterGroup, type QueryFilterGroup } from '@/lib/queryBuilder';
import { BoardListViewToggle } from '@/components/design-system/board-list-view-toggle';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate, useSearch } from '@tanstack/react-router';
import {
  Activity01Icon,
  ChartIncreaseIcon,
  DollarCircleIcon,
  PlusSignIcon,
  Search01Icon,
  Settings02Icon,
  SparklesIcon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { QuietPageHeader, QuietPrimaryAction, QuietSearchInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useDealDisplayStore } from '@/stores/dealDisplayStore';
import { useDeals, usePipelines } from '@/hooks/queries';
import { useCRMOwnerMembers } from '@/hooks/useCRMOwnerMembers';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { DealsTable } from '@/components/crm/DealsTable';
import { DealBoard } from '@/components/crm/DealBoard';
import { DealDisplayMenu } from '@/components/crm/DealDisplayMenu';
import { CreateDealDialog } from '@/components/crm/CreateDealDialog';
import { useTitle } from '@/hooks/useTitle';
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation';

function DealsEmptyState({
  hasPipeline,
  hasFilters,
  search,
  onCreateClick,
  onClearSearch,
  onImportClick,
  onPipelineSettingsClick,
}: {
  hasPipeline: boolean;
  hasFilters?: boolean;
  search: string;
  onCreateClick: () => void;
  onClearSearch: () => void;
  onImportClick: () => void;
  onPipelineSettingsClick: () => void;
}) {
  if (search.trim() || hasFilters) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <div className="max-w-md text-center">
          <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/40">
            <Search01Icon className="h-5 w-5 text-muted-foreground" />
          </div>
          <h2 className="mt-4 text-base font-semibold">No matching deals</h2>
          <p className="mt-2 text-sm leading-6 text-muted-foreground">
            {hasFilters ? 'No deals match your search and filters.' : `No deals match “${search}”.`}
          </p>
          <Button size="sm" variant="outline" className="mt-4" onClick={onClearSearch}>
            {hasFilters ? 'Clear filters' : 'Clear search'}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-4 p-4 md:p-6">
      <div className="rounded-lg border bg-card p-5">
        <div className="max-w-3xl">
          <div className="flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/40">
            <DollarCircleIcon className="h-5 w-5 text-muted-foreground" />
          </div>
          <h2 className="mt-4 text-lg font-semibold">{hasPipeline ? 'No deals in this pipeline yet' : 'Set up your sales pipeline'}</h2>
          <p className="mt-2 max-w-xl text-sm leading-6 text-muted-foreground">
            Deals give sales work a home: stage, owner, amount, close date, linked contacts, CRM signals, and AI review suggestions.
          </p>
          <div className="mt-5 flex flex-wrap gap-2">
            {hasPipeline ? (
              <Button size="sm" onClick={onCreateClick}>
                <PlusSignIcon className="h-4 w-4" />
                Create deal
              </Button>
            ) : null}
            <Button size="sm" variant={hasPipeline ? 'outline' : 'default'} onClick={onPipelineSettingsClick}>
              <Settings02Icon className="h-4 w-4" />
              {hasPipeline ? 'Configure pipeline' : 'Configure pipeline'}
            </Button>
            <Button size="sm" variant="outline" onClick={onImportClick}>
              <ChartIncreaseIcon className="h-4 w-4" />
              Import deals
            </Button>
          </div>
        </div>
      </div>

      <div className="grid gap-3 md:grid-cols-3">
        <div className="rounded-lg border bg-card p-4">
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-md border bg-muted/40">
              <Activity01Icon className="h-4 w-4 text-muted-foreground" />
            </div>
            <p className="text-sm font-medium">Track movement</p>
          </div>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">Drag deals through stages or edit stage, owner, probability, amount, and close date from the list.</p>
        </div>
        <div className="rounded-lg border bg-card p-4">
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-md border bg-muted/40">
              <SparklesIcon className="h-4 w-4 text-muted-foreground" />
            </div>
            <p className="text-sm font-medium">Attach intelligence</p>
          </div>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">CRM signals and health scores become more useful when they can attach to real deals.</p>
        </div>
        <div className="rounded-lg border bg-card p-4">
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-md border bg-muted/40">
              <ChartIncreaseIcon className="h-4 w-4 text-muted-foreground" />
            </div>
            <p className="text-sm font-medium">Review actions</p>
          </div>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">High-confidence signals can create or advance deals through the CRM review queue.</p>
        </div>
      </div>
    </div>
  );
}

export function DealsPage() {
  useTitle('Deals');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const location = useLocation();
  const routeSearch = useSearch({strict:false}) as {filters?:string};
  const filterGroup = parseQueryFilterGroup(routeSearch.filters);
  const setFilterGroup = (group?:QueryFilterGroup) => void navigate({search:(previous:Record<string,unknown>)=>({...previous,filters:serializeQueryFilterGroup(group)}),replace:true} as never);

  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [createStageId, setCreateStageId] = useState<string | undefined>();
  const openCreate = (stageId?: string) => { setCreateStageId(stageId); setShowCreate(true); };

  // Display store init
  const initDisplay = useDealDisplayStore((s) => s.init);
  const showEmptyStages = useDealDisplayStore((s) => s.showEmptyStages);
  useEffect(() => { initDisplay(wsId); }, [wsId, initDisplay]);

  // View mode persistence
  const VIEW_MODE_KEY = `crm_deal_view_mode_${wsId}`;
  const [view, setViewState] = useState<'board' | 'list'>(() => {
    try {
      const saved = localStorage.getItem(VIEW_MODE_KEY);
      return saved === 'list' ? 'list' : 'board';
    } catch { return 'board'; }
  });
  const setView = useCallback((mode: 'board' | 'list') => {
    setViewState(mode);
    try { localStorage.setItem(VIEW_MODE_KEY, mode); } catch { /* Preserve the in-memory preference when storage is unavailable. */ }
  }, [VIEW_MODE_KEY]);

  // Pipeline selector
  const { data: pipelines } = usePipelines(wsId);
  const [selectedPipelineId, setSelectedPipelineId] = useState<string | undefined>();
  const activePipelineId = selectedPipelineId ?? pipelines?.find(p => p.is_default)?.id ?? pipelines?.[0]?.id;
  const activePipeline = pipelines?.find((p) => p.id === activePipelineId);

  // Deals
  const { data, isLoading, refetch } = useDeals(wsId, {
    search: search || undefined,
    filters: routeSearch.filters || undefined,
    pipeline_id: activePipelineId,
  });

  // Unfiltered existence check keeps controls available for empty search/pipeline results.
  const { data: workspaceDeals } = useDeals(wsId, { per_page: 1 });
  const hasWorkspaceDeals = (workspaceDeals?.total ?? workspaceDeals?.data.length ?? 0) > 0;

  // Assignable members
  const { members: assignableMembers } = useCRMOwnerMembers(wsId);
  const ownerNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );

  const handleDealClick = useCallback(
    (id: string) => openDealRoute(navigate as never, location, wsSlug, id),
    [location, navigate, wsSlug],
  );
  const handleImportClick = useCallback(() => {
    if (!wsSlug) return;
    void navigate({ to: '/w/$slug/settings/import', params: { slug: wsSlug } });
  }, [navigate, wsSlug]);
  const handlePipelineSettingsClick = useCallback(() => {
    if (!wsSlug) return;
    void navigate({ to: '/w/$slug/settings/crm-pipelines', params: { slug: wsSlug } });
  }, [navigate, wsSlug]);

  const filterFields = useMemo(() => buildCRMDealQueryFields(assignableMembers, activePipeline?.stages ?? []), [assignableMembers,activePipeline]);
  const deals = data?.data ?? [];
  const showEmptyState = !isLoading && deals.length === 0;

  return (
    <div className="flex h-full flex-col">
      <QuietPageHeader
        variant="shell"
        title="Deals"
        actions={(
          <QuietPrimaryAction className="gap-1.5" onClick={() => openCreate()}>
            <PlusSignIcon className="h-4 w-4" />
            Add deal
          </QuietPrimaryAction>
        )}
      />

      {/* View controls */}
      {hasWorkspaceDeals && <header aria-label="Deal view controls" className="ui-divider-bottom-fade flex flex-wrap items-center gap-2 px-3 py-2">
        {/* Pipeline selector */}
        {pipelines && pipelines.length > 1 && (
          <Select
            value={activePipelineId}
            onValueChange={setSelectedPipelineId}
          >
            <SelectTrigger className="h-7 w-auto gap-1.5 text-xs px-2.5">
              <span className="text-muted-foreground">Pipeline:</span>
              <SelectValue placeholder="Select pipeline" />
            </SelectTrigger>
            <SelectContent>
              {pipelines.map((p) => (
                <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}

        {/* Search */}
        <QuietSearchInput
          containerClassName="w-full sm:w-64"
          placeholder="Search deals..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />

        <DealsFilterBar fields={filterFields} value={filterGroup} onChange={setFilterGroup} />
        <div className="ml-auto flex items-center gap-1">
          <BoardListViewToggle value={view} onChange={setView} />
          <DealDisplayMenu mode={view} />
        </div>
      </header>}

      <DealsActiveFilterBar fields={filterFields} value={filterGroup} onChange={setFilterGroup} />

      {/* Content */}
      <div className="min-h-0 flex-1 overflow-auto">
        {showEmptyState ? (
          <DealsEmptyState
            hasPipeline={!!activePipeline?.stages?.length}
            hasFilters={!!filterGroup?.rules.length}
            search={search}
            onCreateClick={openCreate}
            onClearSearch={() => { setSearch(''); setFilterGroup(undefined); }}
            onImportClick={handleImportClick}
            onPipelineSettingsClick={handlePipelineSettingsClick}
          />
        ) : view === 'board' ? (
          <DealBoard
            deals={deals}
            pipeline={activePipeline}
            workspaceId={wsId}
            assignableMembers={assignableMembers}
            ownerNameMap={ownerNameMap}
            onDealClick={handleDealClick}
            onCreateClick={openCreate}
            onDealUpdated={() => refetch()}
            showEmptyStages={showEmptyStages}
          />
        ) : (
          <DealsTable
            deals={deals}
            pipeline={activePipeline}
            workspaceId={wsId}
            assignableMembers={assignableMembers}
            ownerNameMap={ownerNameMap}
            isLoading={isLoading}
            onDealClick={handleDealClick}
            onCreateClick={openCreate}
            onDealUpdated={() => refetch()}
            onDealDeleted={() => refetch()}
          />
        )}
      </div>

      <CreateDealDialog open={showCreate} onOpenChange={setShowCreate} initialPipelineId={activePipelineId} initialStageId={createStageId} />
    </div>
  );
}
