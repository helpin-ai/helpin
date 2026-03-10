import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Columns2, LayoutList, Plus, Search } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
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

export function DealsPage() {
  useTitle('Deals');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
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
    try { localStorage.setItem(VIEW_MODE_KEY, mode); } catch {}
  }, [VIEW_MODE_KEY]);

  // Pipeline selector
  const { data: pipelines } = usePipelines(wsId);
  const [selectedPipelineId, setSelectedPipelineId] = useState<string | undefined>();
  const activePipelineId = selectedPipelineId ?? pipelines?.[0]?.id;
  const activePipeline = pipelines?.find((p) => p.id === activePipelineId);

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
    (id: string) => navigate({ to: '/w/$slug/crm/deals/$dealId', params: { slug: wsSlug, dealId: id } }),
    [navigate, wsSlug],
  );

  return (
    <div className="flex h-full flex-col">
      {/* Header bar */}
      <header className="flex flex-wrap items-center gap-2 border-b border-border/70 px-3 py-2">
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
        <div className="relative">
          <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search deals..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-7 w-48 pl-7 text-xs"
          />
        </div>

        <div className="ml-auto flex items-center gap-1">
          <DealDisplayMenu mode={view} />
          <QuickTooltip label="Board view">
            <Button
              variant={view === 'board' ? 'default' : 'ghost'}
              size="icon"
              className="h-7 w-7"
              onClick={() => setView('board')}
            >
              <Columns2 className="h-4 w-4" />
            </Button>
          </QuickTooltip>
          <QuickTooltip label="List view">
            <Button
              variant={view === 'list' ? 'default' : 'ghost'}
              size="icon"
              className="h-7 w-7"
              onClick={() => setView('list')}
            >
              <LayoutList className="h-4 w-4" />
            </Button>
          </QuickTooltip>
          <Button size="sm" className="ml-2 h-7 text-xs" onClick={() => setShowCreate(true)}>
            <Plus className="mr-1 h-3.5 w-3.5" />
            Deal
          </Button>
        </div>
      </header>

      {/* Content */}
      <div className="min-h-0 flex-1 overflow-auto">
        {view === 'board' ? (
          <DealBoard
            deals={data?.data ?? []}
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
            deals={data?.data ?? []}
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
