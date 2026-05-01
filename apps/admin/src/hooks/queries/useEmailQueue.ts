import { useQuery } from '@tanstack/react-query'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { emailQueueService } from '@/lib/services/emailQueueService'

export function useEmailQueue() {
  return useQuery({
    queryKey: queryKeys.emailQueue.list(),
    queryFn: async () => unwrap(await emailQueueService.list()),
    refetchInterval: 10_000,
  })
}

export function useEmailDiagnostics() {
  return useQuery({
    queryKey: queryKeys.emailQueue.diagnostics(),
    queryFn: async () => unwrap(await emailQueueService.diagnostics()),
    refetchInterval: 15_000,
  })
}
