import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Plus, Search, LayoutList, SquareKanban } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useDeals, usePipelines } from '@/hooks/queries';
import { DealsTable } from '@/components/crm/DealsTable';
import { DealBoard } from '@/components/crm/DealBoard';
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
  const [view, setView] = useState<'list' | 'board'>('board');

  const { data: pipelines } = usePipelines(wsId);
  const activePipelineId = pipelines?.[0]?.id;
  const { data, isLoading } = useDeals(wsId, {
    search: search || undefined,
    pipeline_id: activePipelineId,
  });

  return (
    <div className="flex h-full flex-col px-4 md:px-6">
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-medium">Deals</h1>
        <div className="flex items-center gap-2">
          <div className="flex rounded-md border">
            <Button
              variant={view === 'board' ? 'secondary' : 'ghost'}
              size="sm"
              className="rounded-r-none"
              onClick={() => setView('board')}
            >
              <SquareKanban className="h-4 w-4" />
            </Button>
            <Button
              variant={view === 'list' ? 'secondary' : 'ghost'}
              size="sm"
              className="rounded-l-none"
              onClick={() => setView('list')}
            >
              <LayoutList className="h-4 w-4" />
            </Button>
          </div>
          <Button size="sm" onClick={() => setShowCreate(true)}>
            <Plus className="mr-1 h-4 w-4" />
            Deal
          </Button>
        </div>
      </div>

      <div className="mb-4 max-w-sm">
        <div className="relative">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search deals..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8"
          />
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        {view === 'list' ? (
          <DealsTable
            deals={data?.data ?? []}
            total={data?.total ?? 0}
            isLoading={isLoading}
            onRowClick={(id) => navigate({ to: '/w/$slug/crm/deals/$dealId', params: { slug: wsSlug, dealId: id } })}
            onCreateClick={() => setShowCreate(true)}
          />
        ) : (
          <DealBoard
            deals={data?.data ?? []}
            pipeline={pipelines?.find((p) => p.id === activePipelineId)}
            onDealClick={(id) => navigate({ to: '/w/$slug/crm/deals/$dealId', params: { slug: wsSlug, dealId: id } })}
            onCreateClick={() => setShowCreate(true)}
          />
        )}
      </div>

      <CreateDealDialog open={showCreate} onOpenChange={setShowCreate} />
    </div>
  );
}
