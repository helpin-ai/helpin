import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { supportReportsService } from '@/lib/services/supportReportsService'
import type { SupportReportParams } from '@/lib/supportReportTypes'

export function useSupportReportOverview(workspaceId: string, params: SupportReportParams) {
  return useQuery({
    queryKey: queryKeys.support.reportOverview(workspaceId, params),
    queryFn: () => supportReportsService.overview(workspaceId, params).then(unwrap),
    enabled: Boolean(workspaceId),
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  })
}
