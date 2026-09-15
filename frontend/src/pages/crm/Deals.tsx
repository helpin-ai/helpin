import { useCallback, useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import {
  Activity01Icon,
  ChartIncreaseIcon,
  Clock03Icon,
  DollarCircleIcon,
  FavouriteIcon,
  LayoutTwoColumnIcon,
  LayoutTable01Icon,
  PlusSignIcon,
  Search01Icon,
  Settings02Icon,
  SparklesIcon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { QuietPageHeader, QuietPrimaryAction, QuietSearchInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useDealDisplayStore } from '@/stores/dealDisplayStore';
import { useDeals, usePipelines } from '@/hooks/queries';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { DealsTable } from '@/components/crm/DealsTable';
import { DealBoard } from '@/components/crm/DealBoard';
import { DealDisplayMenu } from '@/components/crm/DealDisplayMenu';
import { CreateDealDialog } from '@/components/crm/CreateDealDialog';
import { useTitle } from '@/hooks/useTitle';
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation';

function DealPipelinePreview({ stages }: { stages: Array<{ id: string; name: string }> }) {
  const previewStages = stages.length > 0
    ? stages.slice(0, 4)
    : [
        { id: 'lead', name: 'New' },
        { id: 'qualified', name: 'Qualified' },
        { id: 'proposal', name: 'Proposal' },
        { id: 'won', name: 'Won' },
      ];
  const previewDeals = [
    { title: 'Acme expansion', amount: '$18.4k', stageIndex: 0, icon: DollarCircleIcon },
    { title: 'Northstar pilot', amount: '$7.2k', stageIndex: 1, icon: FavouriteIcon },
    { title: 'Renewal risk', amount: '$24k', stageIndex: 2, icon: Clock03Icon },
  ];

  return (
    <div className="rounded-lg border bg-muted/20 p-4">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div>
          <p className="text-sm font-medium">Pipeline preview</p>
          <p className="text-xs text-muted-foreground">Example layout, not workspace data</p>
        </div>
        <div className="text-xs text-muted-foreground">Board</div>
      </div>
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {previewStages.map((stage, index) => {
          const deal = previewDeals.find((item) => item.stageIndex === index);
          const DealIcon = deal?.icon ?? DollarCircleIcon;
          return (
            <div key={stage.id} className="min-h-40 rounded-lg border bg-background p-3">
              <div className="mb-3 flex items-center justify-between gap-2">
                <p className="truncate text-xs font-semibold">{stage.name}</p>
                <span className="text-[11px] text-muted-foreground">{deal ? '1' : '0'}</span>
              </div>
              {deal ? (
                <div className="rounded-md border bg-card p-3 shadow-sm">
                  <div className="flex items-start gap-2">
                    <div className="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-md border bg-muted/40">
                      <DealIcon className="h-3.5 w-3.5 text-muted-foreground" />
                    </div>
                    <p className="min-w-0 truncate text-xs font-medium">{deal.title}</p>
                  </div>
                  <div className="mt-3 flex items-center justify-between gap-2">
                    <span className="text-xs font-semibold">{deal.amount}</span>
                    <span className="h-2 w-2 rounded-full bg-emerald-500" />
                  </div>
                  <div className="mt-3 h-1.5 rounded-full bg-muted">
                    <div className="h-full w-2/3 rounded-full bg-foreground/30" />
                  </div>
                </div>
              ) : (
                <div className="rounded-md border border-dashed border-border/80 px-2 py-9 text-center text-[11px] text-muted-foreground">
                  Empty stage
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function DealsEmptyState({
  stages,
  hasPipeline,
  search,
  onCreateClick,
  onClearSearch,
  onImportClick,
  onPipelineSettingsClick,
}: {
  stages: Array<{ id: string; name: string }>;
  hasPipeline: boolean;
  search: string;
  onCreateClick: () => void;
  onClearSearch: () => void;
  onImportClick: () => void;
  onPipelineSettingsClick: () => void;
}) {
  if (search.trim()) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <div className="max-w-md text-center">
          <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/40">
            <Search01Icon className="h-5 w-5 text-muted-foreground" />
          </div>
          <h2 className="mt-4 text-base font-semibold">No matching deals</h2>
          <p className="mt-2 text-sm leading-6 text-muted-foreground">
            No deals match “{search}”. Clear the search to return to the full pipeline.
          </p>
          <Button size="sm" variant="outline" className="mt-4" onClick={onClearSearch}>
            Clear search
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
            The preview shows how the board starts to look once opportunities are flowing.
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

      <DealPipelinePreview stages={stages} />

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
  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);

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
  const activePipelineId = selectedPipelineId ?? pipelines?.[0]?.id;
  const activePipeline = pipelines?.find((p) => p.id === activePipelineId);
  const activeStages = useMemo(
    () => [...(activePipeline?.stages ?? [])].sort((a, b) => a.position - b.position).map((stage) => ({ id: stage.id, name: stage.name })),
    [activePipeline?.stages],
  );

  // Deals
  const { data, isLoading, refetch } = useDeals(wsId, {
    search: search || undefined,
    pipeline_id: activePipelineId,
  });

  // Assignable members
  const { members: assignableMembers } = useAssignableWorkspaceMembers(wsId);
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

  const deals = data?.data ?? [];
  const showEmptyState = !isLoading && deals.length === 0;

  return (
    <div className="flex h-full flex-col">
      <QuietPageHeader
        variant="shell"
        title="Deals"
        actions={(
          <QuietPrimaryAction className="gap-1.5" onClick={() => setShowCreate(true)}>
            <PlusSignIcon className="h-4 w-4" />
            Add deal
          </QuietPrimaryAction>
        )}
      />

      {/* View controls */}
      <header className="ui-divider-bottom-fade flex flex-wrap items-center gap-2 px-3 py-2">
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

        <div className="ml-auto flex items-center gap-1">
          <DealDisplayMenu mode={view} />
          <QuickTooltip label="Board view">
            <Button
              variant={view === 'board' ? 'default' : 'ghost'}
              size="icon"
              className="h-7 w-7"
              onClick={() => setView('board')}
            >
              <LayoutTwoColumnIcon className="h-4 w-4" />
            </Button>
          </QuickTooltip>
          <QuickTooltip label="List view">
            <Button
              variant={view === 'list' ? 'default' : 'ghost'}
              size="icon"
              className="h-7 w-7"
              onClick={() => setView('list')}
            >
              <LayoutTable01Icon className="h-4 w-4" />
            </Button>
          </QuickTooltip>
        </div>
      </header>

      {/* Content */}
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        {showEmptyState ? (
          <div className="min-h-0 flex-1 overflow-auto">
            <DealsEmptyState
              stages={activeStages}
              hasPipeline={!!activePipeline && activeStages.length > 0}
              search={search}
              onCreateClick={() => setShowCreate(true)}
              onClearSearch={() => setSearch('')}
              onImportClick={handleImportClick}
              onPipelineSettingsClick={handlePipelineSettingsClick}
            />
          </div>
        ) : view === 'board' ? (
          <DealBoard
            deals={deals}
            pipeline={activePipeline}
            workspaceId={wsId}
            assignableMembers={assignableMembers}
            ownerNameMap={ownerNameMap}
            onDealClick={handleDealClick}
            onCreateClick={() => setShowCreate(true)}
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
            onCreateClick={() => setShowCreate(true)}
            onDealUpdated={() => refetch()}
            onDealDeleted={() => refetch()}
          />
        )}
      </div>

      <CreateDealDialog open={showCreate} onOpenChange={setShowCreate} />
    </div>
  );
}
