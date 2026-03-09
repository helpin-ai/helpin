import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Plus, Search } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCompanies } from '@/hooks/queries';
import { CompaniesTable } from '@/components/crm/CompaniesTable';
import { CreateCompanyDialog } from '@/components/crm/CreateCompanyDialog';
import { useTitle } from '@/hooks/useTitle';

export function CompaniesPage() {
  useTitle('Companies');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);

  const { data, isLoading } = useCompanies(wsId, { search: search || undefined });

  return (
    <div className="flex h-full flex-col px-4 md:px-6">
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Companies</h1>
        <Button size="sm" onClick={() => setShowCreate(true)}>
          <Plus className="mr-1 h-4 w-4" />
          Company
        </Button>
      </div>

      <div className="mb-4 max-w-sm">
        <div className="relative">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search companies..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8"
          />
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        <CompaniesTable
          companies={data?.data ?? []}
          total={data?.total ?? 0}
          isLoading={isLoading}
          onRowClick={(id) => navigate({ to: '/w/$slug/crm/companies/$companyId', params: { slug: wsSlug, companyId: id } })}
        />
      </div>

      <CreateCompanyDialog open={showCreate} onOpenChange={setShowCreate} />
    </div>
  );
}
