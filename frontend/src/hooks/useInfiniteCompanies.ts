import { useInfiniteQuery } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { crmCompanyService } from '@/lib/services/crmService';
import { unwrap } from '@/lib/queryUtils';

interface InfiniteCompanyFilters {
  industry?: string;
  owner_member_id?: string;
  search?: string;
}

const PER_PAGE = 50;

export function useInfiniteCompanies(wsId: string, filters?: InfiniteCompanyFilters) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.crm.companies(wsId), 'infinite', filters],
    queryFn: async ({ pageParam }) => {
      const page = pageParam as number;
      return unwrap(
        await crmCompanyService.list(wsId, {
          ...filters,
          page,
          per_page: PER_PAGE,
        }),
      );
    },
    initialPageParam: 1,
    getNextPageParam: (lastPage) => {
      const loaded = lastPage.page * PER_PAGE;
      return loaded < lastPage.total ? lastPage.page + 1 : undefined;
    },
    enabled: !!wsId,
  });
}
