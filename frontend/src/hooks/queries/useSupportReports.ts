import { keepPreviousData, useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { supportReportsService } from '@/lib/services/supportReportsService'
import type { SupportReportParams, SupportReportSLAMetric, SupportReportSLAOutcome } from '@/lib/supportReportTypes'

export function useSupportReportOverview(workspaceId: string, params: SupportReportParams) {
  return useQuery({
    queryKey: queryKeys.support.reportOverview(workspaceId, params),
    queryFn: () => supportReportsService.overview(workspaceId, params).then(unwrap),
    enabled: Boolean(workspaceId),
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  })
}

/** Pages through the conversations behind one SLA report count. */
export function useSupportReportSLAConversations(
  workspaceId: string,
  params: SupportReportParams,
  target: { metric: SupportReportSLAMetric; outcome: SupportReportSLAOutcome } | null,
) {
  return useInfiniteQuery({
    queryKey: queryKeys.support.reportSLAConversations(workspaceId, params, target?.metric ?? '', target?.outcome ?? ''),
    queryFn: ({ pageParam }) =>
      supportReportsService.slaConversations(workspaceId, params, target!.metric, target!.outcome, pageParam).then(unwrap),
    initialPageParam: 1,
    getNextPageParam: (last) => (last.page * last.per_page < last.total ? last.page + 1 : undefined),
    enabled: Boolean(workspaceId && target),
    staleTime: 60_000,
  })
}
