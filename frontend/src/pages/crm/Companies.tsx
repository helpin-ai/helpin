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
  EMPTY_COMPANY_FILTERS,
  companyFilterGroupToState,
  companyFiltersToGroup,
  removeCompanyFilter,
  toggleCompanyFilterValue,
  type CompanyFilterConditions,
  type CompanyFilterModes,
  type CompanyFilterState,
} from '@/lib/crmCompanyFilters';
import {
  isPMFilterConditionComplete,
  optionModeUsesValues,
  type PMFilterCondition,
  type PMFilterOptionMode,
} from '@/lib/pmFilterConditions';
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
  const { data: filterOptions } = useCompanyFilterOptions(wsId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(wsId);
  const filterDefinitions = useMemo(
    () => buildCompanyFilterDefinitions(filterOptions, assignableMembers),
    [filterOptions, assignableMembers],
  );
  const numberFilterKeys = useMemo(
    () => new Set(filterDefinitions.filter((definition) => definition.kind === 'number').map((definition) => definition.key)),
    [filterDefinitions],
  );
  const appliedFilters = useMemo(
    () => companyFilterGroupToState(parseQueryFilterGroup(routeSearch.filters), numberFilterKeys),
    [routeSearch.filters, numberFilterKeys],
  );
  const appliedFilterGroup = companyFiltersToGroup(appliedFilters);
  const serializedFilters = serializeQueryFilterGroup(appliedFilterGroup);
  const appliedFilterCount = appliedFilterGroup?.rules.length ?? 0;
  // Fields added from the trigger but still awaiting a value keep their pill
  // visible; incomplete number conditions stay here until they can be applied.
  const [pendingFilterKeys, setPendingFilterKeys] = useState<Set<string>>(() => new Set());
  const [draftConditions, setDraftConditions] = useState<CompanyFilterConditions>({});
  // Operators chosen for option filters that do not have values yet.
  const [draftModes, setDraftModes] = useState<CompanyFilterModes>({});
  const filterValues = appliedFilters.values;
  const filterConditions = useMemo(
    () => ({ ...appliedFilters.conditions, ...draftConditions }),
    [appliedFilters.conditions, draftConditions],
  );
  const filterModes = useMemo(
    () => ({ ...appliedFilters.modes, ...draftModes }),
    [appliedFilters.modes, draftModes],
  );
  const setAppliedFilters = useCallback((next: CompanyFilterState) => {
    void navigate({
      search: (previous: Record<string, unknown>) => ({
        ...previous,
        filters: serializeQueryFilterGroup(companyFiltersToGroup(next)),
      }),
      replace: true,
    } as never);
  }, [navigate]);
  const visibleFilterKeys = useMemo(() => {
    const keys = new Set([...pendingFilterKeys, ...Object.keys(filterConditions), ...Object.keys(appliedFilters.modes)]);
    for (const [key, selected] of Object.entries(filterValues)) {
      if (selected.length > 0) keys.add(key);
    }
    return keys;
  }, [filterValues, filterConditions, appliedFilters.modes, pendingFilterKeys]);
  const hasActiveFilters = Boolean(serializedFilters);

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

  const ownerNameMap = useMemo(
    () => buildAssignableMemberNameMap(assignableMembers),
    [assignableMembers],
  );

  const withoutKey = <T,>(record: Record<string, T>, key: string) => {
    const next = { ...record };
    delete next[key];
    return next;
  };
  const addFilter = (key: string) => {
    setPendingFilterKeys((current) => new Set(current).add(key));
    if (filterDefinitions.find((definition) => definition.key === key)?.kind === 'number') {
      setDraftConditions((current) => ({ ...current, [key]: { operator: 'gt' } }));
    }
  };
  const toggleFilter = (key: string, value: string) => {
    const mode = filterModes[key];
    setDraftModes((current) => withoutKey(current, key));
    setAppliedFilters({
      ...appliedFilters,
      values: toggleCompanyFilterValue(filterValues, key, value),
      modes: mode ? { ...appliedFilters.modes, [key]: mode } : appliedFilters.modes,
    });
  };
  const changeOptionMode = (key: string, mode: PMFilterOptionMode) => {
    const hasValues = (filterValues[key] ?? []).length > 0;
    if (!optionModeUsesValues(mode) || hasValues) {
      setDraftModes((current) => withoutKey(current, key));
      setAppliedFilters({ ...appliedFilters, modes: { ...appliedFilters.modes, [key]: mode } });
      return;
    }
    // Waiting for values: keep the operator locally and stop applying the old one.
    setDraftModes((current) => ({ ...current, [key]: mode }));
    if (appliedFilters.modes[key]) {
      setAppliedFilters({ ...appliedFilters, modes: withoutKey(appliedFilters.modes, key) });
    }
  };
  const changeCondition = (key: string, condition: PMFilterCondition) => {
    if (isPMFilterConditionComplete(condition)) {
      setDraftConditions((current) => withoutKey(current, key));
      setAppliedFilters({ ...appliedFilters, conditions: { ...appliedFilters.conditions, [key]: condition } });
      return;
    }
    // Keep the pill while its number is missing, and stop applying the old condition.
    setDraftConditions((current) => ({ ...current, [key]: condition }));
    if (appliedFilters.conditions[key]) {
      setAppliedFilters({ ...appliedFilters, conditions: withoutKey(appliedFilters.conditions, key) });
    }
  };
  const removeFilter = (key: string) => {
    setPendingFilterKeys((current) => {
      const next = new Set(current);
      next.delete(key);
      return next;
    });
    setDraftConditions((current) => withoutKey(current, key));
    setDraftModes((current) => withoutKey(current, key));
    setAppliedFilters(removeCompanyFilter(appliedFilters, key));
  };
  const clearFilters = () => {
    setPendingFilterKeys(new Set());
    setDraftConditions({});
    setDraftModes({});
    setAppliedFilters(EMPTY_COMPANY_FILTERS);
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
          activeCount={appliedFilterCount}
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
        conditions={filterConditions}
        onConditionChange={changeCondition}
        optionModes={filterModes}
        onOptionModeChange={changeOptionMode}
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
