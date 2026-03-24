import { useQuery } from '@tanstack/react-query'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { webhookService } from '@/lib/services/webhookService'

export function useWebhookEvents(params: {
  page: number
  per_page?: number
  event_type?: string
  provider?: string
}) {
  return useQuery({
    queryKey: queryKeys.webhookEvents.list({
      page: params.page,
      event_type: params.event_type,
      provider: params.provider,
    }),
    queryFn: async () => unwrap(await webhookService.list(params)),
    staleTime: 10_000,
  })
}
