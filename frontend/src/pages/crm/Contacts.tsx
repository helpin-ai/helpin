import { useDeferredValue, useMemo, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { Loading01Icon, UserGroupIcon, PlusSignIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { QuietPageHeader, QuietPrimaryAction, QuietSearchInput } from '@/components/design-system/quiet';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useContactsSearchParams } from '@/hooks/useContactsSearchParams';
import { useInfiniteContacts } from '@/hooks/useInfiniteContacts';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { ContactLimitNotice, isContactLimitError } from '@edition';
import { queryKeys } from '@/lib/queryKeys';
import { crmContactService } from '@/lib/services/crmService';
import { ContactsTable } from '@/components/crm/ContactsTable';
import { ContactsActiveFilterBar, ContactsFilterBar } from '@/components/crm/ContactsFilterBar';
import { CRMDataEmptyState } from '@/components/crm/CRMDataEmptyState';
import { useTitle } from '@/hooks/useTitle';
import { toast } from 'sonner';


export function ContactsPage() {
  useTitle('Contacts');
  const { currentWorkspace } = useWorkspaceStore();
  const openGlobalCreate = useGlobalCreateStore((s) => s.openCreate);
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [seedConfirmOpen, setSeedConfirmOpen] = useState(false);
  const [isSeeding, setIsSeeding] = useState(false);
  const [tableToolbarContainer, setTableToolbarContainer] = useState<HTMLSpanElement | null>(null);

  const { search: searchParams, setParam, hasActiveFilters, clearFilters } = useContactsSearchParams();
  const showSeedButton = import.meta.env.DEV;

  const {
    data,
    isLoading,
    refetch,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    error,
  } = useInfiniteContacts(wsId, {
    search: searchParams.search || undefined,
    filters: searchParams.filters || undefined,
    lifecycle_stage: searchParams.stage || undefined,
    lead_status: searchParams.status || undefined,
    owner_member_id: searchParams.owner || undefined,
  });

  const contacts = useMemo(
    () => data?.pages.flatMap((p) => p.data) ?? [],
    [data],
  );
  const deferredContacts = useDeferredValue(contacts);
  const totalCount = data?.pages[0]?.total ?? 0;

  const { members: assignableMembers } = useAssignableWorkspaceMembers(wsId);
  const ownerNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const showContactsEmptyState = !isLoading && !hasActiveFilters && deferredContacts.length === 0 && totalCount === 0;

  const handleImportClick = () => {
    if (!wsSlug) return;
    void navigate({ to: '/w/$slug/settings/import', params: { slug: wsSlug } });
  };
  const handleSeedContacts = async () => {
    if (!wsId || isSeeding) return;
    setIsSeeding(true);
    try {
      const { data: result, error } = await crmContactService.seed({
        workspace_id: wsId,
        count: 500,
      });
      if (error) {
        toast.error(error);
        return;
      }
      await queryClient.invalidateQueries({ queryKey: queryKeys.crm.contacts(wsId) });
      await refetch();
      toast.success(`Created ${result?.created ?? 500} test contacts`);
      setSeedConfirmOpen(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to seed contacts');
    } finally {
      setIsSeeding(false);
    }
  };

  return (
    <div className="flex h-full flex-col">
      <QuietPageHeader
        variant="shell"
        title="Contacts"
        context={!isLoading ? totalCount : undefined}
        actions={(
          <QuietPrimaryAction
            className="gap-1.5"
            onClick={() => openGlobalCreate('crm_contact')}
          >
            <PlusSignIcon className="h-4 w-4" />
            Add contact
          </QuietPrimaryAction>
        )}
      />

      <header className="ui-divider-bottom-fade flex flex-wrap items-center gap-2 px-3 py-2">
        <QuietSearchInput
          containerClassName="min-w-[180px] flex-1 sm:max-w-64"
          placeholder="Search contacts..."
          value={searchParams.search ?? ''}
          onChange={(event) => setParam('search', event.target.value)}
        />
        <ContactsFilterBar assignableMembers={assignableMembers} />
        <div className="ml-auto flex max-w-full flex-wrap items-center justify-end gap-1">
          {showSeedButton && (
            <Button
              size="sm"
              variant="outline"
              className="hidden h-7 text-xs sm:inline-flex"
              onClick={() => setSeedConfirmOpen(true)}
              disabled={!wsId || isSeeding}
            >
              {isSeeding ? (
                <Loading01Icon className="mr-1 h-3.5 w-3.5 animate-spin" />
              ) : (
                <UserGroupIcon className="mr-1 h-3.5 w-3.5" />
              )}
              Seed 500
            </Button>
          )}
          <span ref={setTableToolbarContainer} className="contents" />
        </div>
      </header>

      <ContactsActiveFilterBar assignableMembers={assignableMembers} />

      {/* Content */}
      <div className="min-h-0 flex-1 overflow-hidden p-3">
        {isContactLimitError(error) ? (
          <ContactLimitNotice workspaceSlug={wsSlug} />
        ) : showContactsEmptyState ? (
          <CRMDataEmptyState
            kind="contacts"
            onCreateClick={() => openGlobalCreate('crm_contact')}
            onImportClick={handleImportClick}
          />
        ) : (
          <ContactsTable
            contacts={deferredContacts}
            workspaceId={wsId}
            assignableMembers={assignableMembers}
            ownerNameMap={ownerNameMap}
            toolbarContainer={tableToolbarContainer}
            isLoading={isLoading}
            hasActiveFilters={hasActiveFilters}
            hasNextPage={!!hasNextPage}
            isFetchingNextPage={isFetchingNextPage}
            onFetchNextPage={fetchNextPage}
            onRowClick={(id) => navigate({ to: '/w/$slug/crm/contacts/$contactId', params: { slug: wsSlug, contactId: id } })}
            onCreateClick={() => openGlobalCreate('crm_contact')}
            onClearFilters={clearFilters}
            onContactUpdated={() => refetch()}
            onContactDeleted={() => refetch()}
          />
        )}
      </div>

      <ConfirmDialog
        open={seedConfirmOpen}
        onOpenChange={(open) => { if (!isSeeding) setSeedConfirmOpen(open); }}
        title="Seed 500 test contacts"
        description="This will create 500 synthetic contacts in the current workspace so you can test list performance and scrolling."
        confirmLabel={isSeeding ? 'Seeding...' : 'Seed contacts'}
        variant="default"
        onConfirm={handleSeedContacts}
      />
    </div>
  );
}
