import { useCallback } from 'react';
import { Route } from '@/routes/_authenticated/w/$slug/crm/contacts/index';
import type { ContactsSearch } from '@/routes/_authenticated/w/$slug/crm/contacts/index';

export function useContactsSearchParams() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();

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
        stage: undefined,
        status: undefined,
        owner: undefined,
      }),
      replace: true,
    });
  }, [navigate]);

  const hasActiveFilters = !!(search.search || search.stage || search.status || search.owner);

  return { search, setParam, setParams, clearFilters, hasActiveFilters };
}
