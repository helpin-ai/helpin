import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { supportChannelsService } from '@/lib/services/supportChannelsService'

export function useSupportChannels(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.channels(workspaceId),
    queryFn: () => supportChannelsService.list(workspaceId).then(unwrap),
    enabled: Boolean(workspaceId),
    // Delivery health and the last event change in the background.
    refetchInterval: 30_000,
  })
}

function useChannelMutation<TVariables, TResult>(workspaceId: string, mutationFn: (variables: TVariables) => Promise<TResult>, errorMessage: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.channels(workspaceId) })
    },
    onError: (error: Error) => {
      toast.error(errorMessage, { description: error.message })
    },
  })
}

export function useConnectChannel(workspaceId: string) {
  return useChannelMutation(
    workspaceId,
    (input: { token: string; mailboxId: string | null }) =>
      supportChannelsService.connect(workspaceId, { provider: 'telegram', token: input.token, mailbox_id: input.mailboxId }).then(unwrap),
    'Could not connect the bot',
  )
}

export function useUpdateChannel(workspaceId: string) {
  return useChannelMutation(
    workspaceId,
    ({ id, input }: { id: string; input: { mailbox_id?: string; ai_replies_disabled?: boolean } }) =>
      supportChannelsService.update(workspaceId, id, input).then(unwrap),
    'Could not update the bot',
  )
}

export function useReconnectChannel(workspaceId: string) {
  return useChannelMutation(
    workspaceId,
    ({ id, token }: { id: string; token?: string }) => supportChannelsService.reconnect(workspaceId, id, token).then(unwrap),
    'Could not reconnect the bot',
  )
}

export function useDisconnectChannel(workspaceId: string) {
  return useChannelMutation(workspaceId, (id: string) => supportChannelsService.disconnect(workspaceId, id).then(unwrap), 'Could not disconnect the bot')
}
