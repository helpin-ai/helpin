import { useInfiniteQuery } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { crmContactService } from '@/lib/services/crmService';
import { unwrap } from '@/lib/queryUtils';

interface InfiniteContactFilters {
  lifecycle_stage?: string;
  lead_status?: string;
  owner_member_id?: string;
  search?: string;
}

const PER_PAGE = 50;

export function useInfiniteContacts(wsId: string, filters?: InfiniteContactFilters) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.crm.contacts(wsId), 'infinite', filters],
    queryFn: async ({ pageParam }) => {
      const page = pageParam as number;
      return unwrap(
        await crmContactService.list(wsId, {
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
