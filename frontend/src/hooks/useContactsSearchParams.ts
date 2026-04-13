import { useCallback } from 'react';
import { parseQueryFilterGroup, serializeQueryFilterGroup, type QueryFilterGroup } from '@/lib/queryBuilder';
import { Route } from '@/routes/_authenticated/w/$slug/crm/contacts/index';
import type { ContactsSearch } from '@/routes/_authenticated/w/$slug/crm/contacts/index';

export function useContactsSearchParams() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const filterGroup = parseQueryFilterGroup(search.filters);

  const setParam = useCallback(
    <K extends keyof ContactsSearch>(key: K, value: ContactsSearch[K]) => {
      void navigate({
        search: (prev) => ({
          ...prev,
          [key]: value || undefined,
        }),
        replace: true,
      });
    },
    [navigate],
  );

  const setParams = useCallback(
    (updates: Partial<ContactsSearch>) => {
      void navigate({
        search: (prev) => {
          const next: Record<string, unknown> = { ...prev };
          for (const [k, v] of Object.entries(updates)) {
            next[k] = v || undefined;
          }
          return next as ContactsSearch;
        },
        replace: true,
      });
    },
    [navigate],
  );

  const clearFilters = useCallback(() => {
    void navigate({
      search: (prev) => ({
        ...prev,
        search: undefined,
        filters: undefined,
        stage: undefined,
        status: undefined,
        owner: undefined,
      }),
      replace: true,
    });
  }, [navigate]);

  const setFilterGroup = useCallback((group?: QueryFilterGroup) => {
    void navigate({
      search: (prev) => ({
        ...prev,
        filters: serializeQueryFilterGroup(group),
      }),
      replace: true,
    });
  }, [navigate]);

  const hasActiveFilters = !!(
    search.search ||
    filterGroup?.rules.length ||
    search.stage ||
    search.status ||
    search.owner
  );

  return { search, filterGroup, setParam, setParams, setFilterGroup, clearFilters, hasActiveFilters };
}
