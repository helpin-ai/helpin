import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { supportWebhooksService } from '@/lib/services/supportWebhooksService'
import type { WebhookEndpointInput } from '@/lib/pmTypes'

export function useSupportWebhooks(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.webhooks(workspaceId),
    queryFn: () => supportWebhooksService.list(workspaceId).then(unwrap),
    enabled: Boolean(workspaceId) && enabled,
    // Deliveries and retries update endpoint health in the background.
    refetchInterval: 15_000,
  })
}

/** Recent deliveries of one webhook; refreshes while some are pending. */
export function useWebhookDeliveries(workspaceId: string, webhookId: string | null) {
  return useQuery({
    queryKey: queryKeys.support.webhookDeliveries(workspaceId, webhookId ?? ''),
    queryFn: () => supportWebhooksService.deliveries(workspaceId, webhookId!).then(unwrap),
    enabled: Boolean(workspaceId && webhookId),
    refetchInterval: (query) => (query.state.data?.some((delivery) => delivery.status === 'pending') ? 5_000 : false),
  })
}

function useWebhookMutation<TVariables, TResult>(workspaceId: string, mutationFn: (variables: TVariables) => Promise<TResult>, errorMessage: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn,
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.webhooks(workspaceId) })
    },
    onError: (error: Error) => {
      toast.error(errorMessage, { description: error.message })
    },
  })
}

export function useCreateWebhook(workspaceId: string) {
  return useWebhookMutation(workspaceId, (input: WebhookEndpointInput) =>
    supportWebhooksService.create(workspaceId, input).then(unwrap), 'Could not add the webhook')
}

export function useUpdateWebhook(workspaceId: string) {
  return useWebhookMutation(workspaceId, ({ id, input }: { id: string; input: WebhookEndpointInput }) =>
    supportWebhooksService.update(workspaceId, id, input).then(unwrap), 'Could not save the webhook')
}

export function useDeleteWebhook(workspaceId: string) {
  return useWebhookMutation(workspaceId, (id: string) =>
    supportWebhooksService.remove(workspaceId, id).then((response) => {
      if (response.error) throw new Error(response.error)
    }), 'Could not delete the webhook')
}

export function useRotateWebhookSecret(workspaceId: string) {
  return useWebhookMutation(workspaceId, (id: string) =>
    supportWebhooksService.rotateSecret(workspaceId, id).then(unwrap), 'Could not create a new secret')
}

export function useSendWebhookTest(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => supportWebhooksService.sendTest(workspaceId, id).then(unwrap),
    onSettled: (_data, _error, id) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.webhooks(workspaceId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.webhookDeliveries(workspaceId, id) })
    },
    onError: (error: Error) => {
      toast.error('Could not send the test event', { description: error.message })
    },
  })
}

export function useRedeliverWebhook(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ webhookId, deliveryId }: { webhookId: string; deliveryId: string }) =>
      supportWebhooksService.redeliver(workspaceId, webhookId, deliveryId).then(unwrap),
    onSuccess: (_data, { webhookId }) => {
      toast.success('Event queued to send again')
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.webhookDeliveries(workspaceId, webhookId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.support.webhooks(workspaceId) })
    },
    onError: (error: Error) => {
      toast.error('Could not resend the event', { description: error.message })
    },
  })
}
