import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { crmContactService } from '@/lib/services/crmService';
import { unwrap } from '@/lib/queryUtils';

/**
 * Loads the custom property keys in use on contacts. The key sits under the
 * contacts prefix, so contact mutations refresh it.
 */
export function useContactFilterOptions(wsId: string) {
  return useQuery({
    queryKey: [...queryKeys.crm.contacts(wsId), 'filter-options'],
    queryFn: async () => unwrap(await crmContactService.filterOptions(wsId)),
    enabled: !!wsId,
    staleTime: 60_000,
  });
}
