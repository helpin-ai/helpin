import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { inboundEmailService } from '@/lib/services/inboundEmailService'
import type { IncomingStatus } from '@/lib/inboundEmailTypes'

export function useInboundEmails(status: IncomingStatus = 'attention', query = '', page = 1) {
  return useQuery({
    queryKey: queryKeys.inboundEmails.list(status, query, page),
    queryFn: async () => unwrap(await inboundEmailService.list(status, query, page)),
    refetchInterval: 10_000,
  })
}

export function useInboundEmail(id: string) {
  return useQuery({
    queryKey: queryKeys.inboundEmails.detail(id),
    queryFn: async () => unwrap(await inboundEmailService.detail(id)),
    refetchInterval: (query) => (query.state.data?.status === 'completed' ? false : 2_000),
  })
}

export function useInboundConversations(id: string, query: string, enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.inboundEmails.conversations(id, query),
    queryFn: async () => unwrap(await inboundEmailService.conversations(id, query)),
    enabled,
  })
}

export function useRecoverInboundEmail(id: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async (conversationID?: string) =>
      unwrap(await inboundEmailService.recover(id, conversationID)),
    onSettled: () => client.invalidateQueries({ queryKey: queryKeys.inboundEmails.all() }),
  })
}
