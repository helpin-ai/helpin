import { useCallback, useDeferredValue, useMemo, useState } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { PlusSignIcon } from '@/lib/icons';
import { QuietPageHeader, QuietPrimaryAction, QuietSearchInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useInfiniteCompanies } from '@/hooks/useInfiniteCompanies';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { buildAssignableMemberNameMap } from '@/lib/assignableMembers';
import {
  CompaniesTable,
  type CompanyGroupByOption,
} from '@/components/crm/CompaniesTable';
import { CreateCompanyDialog } from '@/components/crm/CreateCompanyDialog';
import { CRMDataEmptyState, CRMNoResultsState } from '@/components/crm/CRMDataEmptyState';
import { CompaniesAppliedFilters, CompaniesFilterTrigger } from '@/components/crm/CompaniesFilterBar';
import { useCompanyFilterOptions } from '@/hooks/useCompanyFilterOptions';
import { useTitle } from '@/hooks/useTitle';
import {
  buildCompanyFilterDefinitions,
  companyFilterGroupToValues,
  companyFilterValuesToGroup,
  removeCompanyFilter,
  toggleCompanyFilterValue,
  type CompanyFilterValues,
} from '@/lib/crmCompanyFilters';
import { parseQueryFilterGroup, serializeQueryFilterGroup } from '@/lib/queryBuilder';

const COMPANY_GROUP_BY_OPTIONS: { value: CompanyGroupByOption; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'industry', label: 'Industry' },
  { value: 'owner', label: 'Owner' },
];

export function CompaniesPage() {
  useTitle('Companies');
  const [tableToolbarContainer, setTableToolbarContainer] = useState<HTMLSpanElement | null>(null);
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [groupBy, setGroupBy] = useState<CompanyGroupByOption>('none');
  const [showCreate, setShowCreate] = useState(false);

  // Applied filters live in the URL as a query-builder group, as on Deals and Contacts.
  const routeSearch = useSearch({ strict: false }) as { filters?: string };
  const filterValues = useMemo(
    () => companyFilterGroupToValues(parseQueryFilterGroup(routeSearch.filters)),
    [routeSearch.filters],
  );
  const serializedFilters = serializeQueryFilterGroup(companyFilterValuesToGroup(filterValues));
  // Fields added from the trigger but still awaiting a value keep their pill visible.
  const [pendingFilterKeys, setPendingFilterKeys] = useState<Set<string>>(() => new Set());
  const setFilterValues = useCallback((next: CompanyFilterValues) => {
    void navigate({
      search: (previous: Record<string, unknown>) => ({
        ...previous,
        filters: serializeQueryFilterGroup(companyFilterValuesToGroup(next)),
      }),
      replace: true,
    } as never);
  }, [navigate]);
  const visibleFilterKeys = useMemo(() => {
    const keys = new Set(pendingFilterKeys);
    for (const [key, selected] of Object.entries(filterValues)) {
      if (selected.length > 0) keys.add(key);
    }
    return keys;
  }, [filterValues, pendingFilterKeys]);
  const hasActiveFilters = Boolean(serializedFilters);

  const { data: filterOptions } = useCompanyFilterOptions(wsId);
  const customPropertyKeys = useMemo(
    () => (filterOptions?.custom_properties ?? []).map((summary) => summary.key),
    [filterOptions],
  );

  const {
    data,
    isLoading,
    refetch,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteCompanies(wsId, { search: search || undefined, filters: serializedFilters });
  const companies = useMemo(
    () => data?.pages.flatMap((page) => page.data) ?? [],
    [data],
  );
  const deferredCompanies = useDeferredValue(companies);
  const totalCount = data?.pages[0]?.total ?? 0;
  const normalizedSearch = search.trim();
  const isNarrowed = !!normalizedSearch || hasActiveFilters;
  const showCompaniesEmptyState = !isLoading && deferredCompanies.length === 0 && totalCount === 0 && !isNarrowed;
  const showNoResultsState = !isLoading && deferredCompanies.length === 0 && isNarrowed;

  const { members: assignableMembers } = useAssignableWorkspaceMembers(wsId);
  const ownerNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );
  const filterDefinitions = useMemo(
    () => buildCompanyFilterDefinitions(filterOptions, assignableMembers),
    [filterOptions, assignableMembers],
  );

  const addFilter = (key: string) => setPendingFilterKeys((current) => new Set(current).add(key));
  const toggleFilter = (key: string, value: string) => setFilterValues(toggleCompanyFilterValue(filterValues, key, value));
  const removeFilter = (key: string) => {
    setPendingFilterKeys((current) => {
      const next = new Set(current);
      next.delete(key);
      return next;
    });
    setFilterValues(removeCompanyFilter(filterValues, key));
  };
  const clearFilters = () => {
    setPendingFilterKeys(new Set());
    setFilterValues({});
  };
  const clearSearchAndFilters = () => {
    setSearch('');
    clearFilters();
  };


  return (
    <div className="flex h-full flex-col">
      <QuietPageHeader
        variant="shell"
        title="Companies"
        context={!isLoading ? totalCount : undefined}
        actions={(
          <QuietPrimaryAction className="gap-1.5" onClick={() => setShowCreate(true)}>
            <PlusSignIcon className="h-4 w-4" />
            Add company
          </QuietPrimaryAction>
        )}
      />

      <header className="ui-divider-bottom-fade flex flex-wrap items-center gap-2 px-3 py-2">
        <QuietSearchInput
          containerClassName="w-full sm:w-64"
          placeholder="Search companies..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <CompaniesFilterTrigger
          definitions={filterDefinitions}
          values={filterValues}
          visibleKeys={visibleFilterKeys}
          onAdd={addFilter}
          onToggle={toggleFilter}
        />
        <div className="ml-auto flex items-center">
          <Select value={groupBy} onValueChange={(value) => setGroupBy(value as CompanyGroupByOption)}>
            <SelectTrigger className="h-7 w-auto min-w-[130px] max-w-[160px] gap-1 border-0 bg-transparent px-1.5 text-xs shadow-none hover:bg-accent focus-visible:ring-0 focus-visible:border-transparent">
              <span className="shrink-0 text-muted-foreground">Group by:</span>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {COMPANY_GROUP_BY_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span ref={setTableToolbarContainer} className="contents" />
        </div>
      </header>
      <CompaniesAppliedFilters
        definitions={filterDefinitions}
        values={filterValues}
        visibleKeys={visibleFilterKeys}
        onToggle={toggleFilter}
        onRemove={removeFilter}
        onClearAll={clearFilters}
      />

      {/* Content */}
      <div className="min-h-0 flex-1 overflow-hidden p-3">
        {showCompaniesEmptyState ? (
          <CRMDataEmptyState
            kind="companies"
            onCreateClick={() => setShowCreate(true)}
          />
        ) : showNoResultsState ? (
          <CRMNoResultsState
            kind="companies"
            query={normalizedSearch}
            hasFilters={hasActiveFilters}
            onClear={clearSearchAndFilters}
          />
        ) : (
          <CompaniesTable
            key={wsId}
            companies={deferredCompanies}
            customPropertyKeys={customPropertyKeys}
            workspaceId={wsId}
            assignableMembers={assignableMembers}
            ownerNameMap={ownerNameMap}
            groupBy={groupBy}
            isLoading={isLoading}
            hasNextPage={!!hasNextPage}
            isFetchingNextPage={isFetchingNextPage}
            onFetchNextPage={fetchNextPage}
            onRowClick={(id) => navigate({ to: '/w/$slug/crm/companies/$companyId', params: { slug: wsSlug, companyId: id } })}
            onCreateClick={() => setShowCreate(true)}
            onCompanyUpdated={() => refetch()}
            onCompanyDeleted={() => refetch()}
            toolbarContainer={tableToolbarContainer}
          />
        )}
      </div>

      <CreateCompanyDialog open={showCreate} onOpenChange={setShowCreate} />
    </div>
  );
}
