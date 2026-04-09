import { useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { PlusSignIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useContactsSearchParams } from '@/hooks/useContactsSearchParams';
import { useInfiniteContacts } from '@/hooks/useInfiniteContacts';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { ContactsTable } from '@/components/crm/ContactsTable';
import { ContactsFilterBar } from '@/components/crm/ContactsFilterBar';
import { CreateContactDialog } from '@/components/crm/CreateContactDialog';
import { useTitle } from '@/hooks/useTitle';

export function ContactsPage() {
  useTitle('Contacts');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const [showCreate, setShowCreate] = useState(false);

  const { search: searchParams, hasActiveFilters, clearFilters } = useContactsSearchParams();

  const {
    data,
    isLoading,
    refetch,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteContacts(wsId, {
    search: searchParams.search || undefined,
    lifecycle_stage: searchParams.stage || undefined,
    lead_status: searchParams.status || undefined,
    owner_member_id: searchParams.owner || undefined,
  });

  const contacts = useMemo(
    () => data?.pages.flatMap((p) => p.data) ?? [],
    [data],
  );
  const totalCount = data?.pages[0]?.total ?? 0;

  const { members: assignableMembers } = useAssignableWorkspaceMembers(wsId);
  const ownerNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );

  return (
    <div className="flex h-full flex-col">
      {/* Header bar */}
      <header className="ui-divider-bottom-fade flex flex-wrap items-center gap-2 px-3 py-2">
        <ContactsFilterBar assignableMembers={assignableMembers} />
        <div className="ml-auto flex items-center gap-1">
          <Button size="sm" className="h-7 text-xs" onClick={() => setShowCreate(true)}>
            <PlusSignIcon className="mr-1 h-3.5 w-3.5" />
            Contact
          </Button>
        </div>
      </header>

      {/* Content */}
      <div className="min-h-0 flex-1 overflow-auto p-3">
        <ContactsTable
          contacts={contacts}
          totalCount={totalCount}
          workspaceId={wsId}
          assignableMembers={assignableMembers}
          ownerNameMap={ownerNameMap}
          isLoading={isLoading}
          hasActiveFilters={hasActiveFilters}
          hasNextPage={!!hasNextPage}
          isFetchingNextPage={isFetchingNextPage}
          onFetchNextPage={fetchNextPage}
          onRowClick={(id) => navigate({ to: '/w/$slug/crm/contacts/$contactId', params: { slug: wsSlug, contactId: id } })}
          onCreateClick={() => setShowCreate(true)}
          onClearFilters={clearFilters}
          onContactUpdated={() => refetch()}
          onContactDeleted={() => refetch()}
        />
      </div>

      <CreateContactDialog open={showCreate} onOpenChange={setShowCreate} />
    </div>
  );
}
