import { useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { Loading01Icon, Search01Icon, UserGroupIcon, Cancel01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { useContactsSearchParams } from '@/hooks/useContactsSearchParams';
import { useInfiniteContacts } from '@/hooks/useInfiniteContacts';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import { BILLING_CHOOSE_PLAN_SEARCH } from '@/lib/billingNavigation';
import { queryKeys } from '@/lib/queryKeys';
import { crmContactService } from '@/lib/services/crmService';
import { ContactsTable } from '@/components/crm/ContactsTable';
import { ContactsFilterBar } from '@/components/crm/ContactsFilterBar';
import { CRMDataEmptyState } from '@/components/crm/CRMDataEmptyState';
import { useTitle } from '@/hooks/useTitle';
import { toast } from 'sonner';

const isContactLimitError = (error: unknown) =>
  error instanceof Error && error.message.includes('5,000 contacts');

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
  const [showSearch, setShowSearch] = useState(false);
  const searchInputRef = useRef<HTMLInputElement>(null);

  const { search: searchParams, setParam, hasActiveFilters, clearFilters } = useContactsSearchParams();
  const showSeedButton = import.meta.env.DEV;

  useEffect(() => {
    if (searchParams.search) {
      setShowSearch(true);
    }
  }, [searchParams.search]);

  useEffect(() => {
    if (showSearch) {
      searchInputRef.current?.focus();
    }
  }, [showSearch]);

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
  const handleUpgradeClick = () => {
    if (!wsSlug) return;
    void navigate({
      to: '/w/$slug/settings/billing',
      params: { slug: wsSlug },
      search: BILLING_CHOOSE_PLAN_SEARCH,
    });
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
      {/* Header bar */}
      <header className="ui-divider-bottom-fade flex flex-wrap items-center gap-2 px-3 py-2">
        <ContactsFilterBar assignableMembers={assignableMembers} />
        <div className="ml-auto flex items-center gap-1">
          {showSearch ? (
            <div className="relative">
              <Search01Icon className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                ref={searchInputRef}
                placeholder="Search contacts..."
                value={searchParams.search ?? ''}
                onChange={(e) => setParam('search', e.target.value)}
                className="h-7 w-52 pl-7 pr-7 text-xs"
              />
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="absolute right-0 top-0 h-7 w-7 text-muted-foreground"
                onClick={() => {
                  setParam('search', undefined);
                  setShowSearch(false);
                }}
              >
                <Cancel01Icon className="h-3.5 w-3.5" />
              </Button>
            </div>
          ) : (
            <Button
              type="button"
              size="icon"
              variant="outline"
              className="h-7 w-7"
              onClick={() => setShowSearch(true)}
            >
              <Search01Icon className="h-3.5 w-3.5" />
            </Button>
          )}
          {showSeedButton && (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs"
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
        </div>
      </header>

      {/* Content */}
      <div className="min-h-0 flex-1 overflow-hidden p-3">
        {isContactLimitError(error) ? (
          <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
            <div>
              <h2 className="text-sm font-medium">Upgrade to view CRM contacts</h2>
              <p className="mt-1 max-w-sm text-sm text-muted-foreground">
                The Starter plan includes up to 5,000 contacts. Support can keep capturing new contacts, but CRM contact viewing requires the Growth plan once you exceed that limit.
              </p>
            </div>
            <Button size="sm" onClick={handleUpgradeClick}>Upgrade</Button>
          </div>
        ) : showContactsEmptyState ? (
          <CRMDataEmptyState
            kind="contacts"
            onCreateClick={() => openGlobalCreate('crm_contact')}
            onImportClick={handleImportClick}
          />
        ) : (
          <ContactsTable
            contacts={deferredContacts}
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
