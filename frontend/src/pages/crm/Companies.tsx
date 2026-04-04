import { useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { PlusSignIcon, Search01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCompanies } from '@/hooks/queries';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
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

  const { data, isLoading, refetch } = useCompanies(wsId, { search: search || undefined });

  const { members: assignableMembers } = useAssignableWorkspaceMembers(wsId);
  const ownerNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );

  return (
    <div className="flex h-full flex-col">
      {/* Header bar */}
      <header className="ui-divider-bottom-fade flex flex-wrap items-center gap-2 px-3 py-2">
        <div className="relative">
          <Search01Icon className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search companies..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-7 w-48 pl-7 text-xs"
          />
        </div>

        <div className="ml-auto flex items-center gap-1">
          <Button size="sm" className="h-7 text-xs" onClick={() => setShowCreate(true)}>
            <PlusSignIcon className="mr-1 h-3.5 w-3.5" />
            Company
          </Button>
        </div>
      </header>

      {/* Content */}
      <div className="min-h-0 flex-1 overflow-auto p-3">
        <CompaniesTable
          companies={data?.data ?? []}
          workspaceId={wsId}
          assignableMembers={assignableMembers}
          ownerNameMap={ownerNameMap}
          isLoading={isLoading}
          onRowClick={(id) => navigate({ to: '/w/$slug/crm/companies/$companyId', params: { slug: wsSlug, companyId: id } })}
          onCreateClick={() => setShowCreate(true)}
          onCompanyUpdated={() => refetch()}
          onCompanyDeleted={() => refetch()}
        />
      </div>

      <CreateCompanyDialog open={showCreate} onOpenChange={setShowCreate} />
    </div>
  );
}
