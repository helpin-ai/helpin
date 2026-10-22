import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { crmCompanyService } from '@/lib/services/crmService';
import { unwrap } from '@/lib/queryUtils';

/**
 * Loads the industries and custom property keys in use on companies. The key
 * sits under the companies prefix, so company mutations refresh it.
 */
export function useCompanyFilterOptions(wsId: string) {
  return useQuery({
    queryKey: [...queryKeys.crm.companies(wsId), 'filter-options'],
    queryFn: async () => unwrap(await crmCompanyService.filterOptions(wsId)),
    enabled: !!wsId,
    staleTime: 60_000,
  });
}
