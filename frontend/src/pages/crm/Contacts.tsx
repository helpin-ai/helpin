import { useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Plus, Search } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useContacts } from '@/hooks/queries';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { ContactsTable } from '@/components/crm/ContactsTable';
import { CreateContactDialog } from '@/components/crm/CreateContactDialog';
import { useTitle } from '@/hooks/useTitle';

export function ContactsPage() {
  useTitle('Contacts');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);

  const { data, isLoading, refetch } = useContacts(wsId, { search: search || undefined });

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
          <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search contacts..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-7 w-48 pl-7 text-xs"
          />
        </div>

        <div className="ml-auto flex items-center gap-1">
          <Button size="sm" className="h-7 text-xs" onClick={() => setShowCreate(true)}>
            <Plus className="mr-1 h-3.5 w-3.5" />
            Contact
          </Button>
        </div>
      </header>

      {/* Content */}
      <div className="min-h-0 flex-1 overflow-auto p-3">
        <ContactsTable
          contacts={data?.data ?? []}
          workspaceId={wsId}
          assignableMembers={assignableMembers}
          ownerNameMap={ownerNameMap}
          isLoading={isLoading}
          onRowClick={(id) => navigate({ to: '/w/$slug/crm/contacts/$contactId', params: { slug: wsSlug, contactId: id } })}
          onCreateClick={() => setShowCreate(true)}
          onContactUpdated={() => refetch()}
          onContactDeleted={() => refetch()}
        />
      </div>

      <CreateContactDialog open={showCreate} onOpenChange={setShowCreate} />
    </div>
  );
}
