import { useSupportTeammatePresence, useUpdateMySupportTeammatePresence } from '@helpin-ai/support-core'
import { toast } from 'sonner'
import { SegmentedControl } from '@mobile/ui/segmented-control'
import { Spinner } from '@mobile/ui/spinner'
import { teammatePresenceDotClass, teammatePresenceLabel } from './teammate-presence'

export function SupportAvailabilityControl({ workspaceId, userId, enabled }: {
  workspaceId: string
  userId?: string
  enabled: boolean
}) {
  const presenceQuery = useSupportTeammatePresence(workspaceId, enabled)
  const updatePresence = useUpdateMySupportTeammatePresence(workspaceId)
  const myPresence = presenceQuery.data?.find((status) => status.user_id === userId)
  const presenceMode = myPresence?.manual_status ?? 'auto'

  if (!enabled) return null
  return (
    <section className="border-b border-border/70 py-3">
      <div className="mb-2 flex items-center justify-between gap-3">
        <span className="text-footnote text-muted-foreground">Support availability</span>
        {presenceQuery.isPending ? (
          <Spinner size={14} />
        ) : (
          <span className="flex items-center gap-1.5 text-footnote text-muted-foreground">
            <span aria-hidden className={`h-2 w-2 rounded-full ${teammatePresenceDotClass(myPresence?.status)}`} />
            {teammatePresenceLabel(myPresence?.status)}
          </span>
        )}
      </div>
      <SegmentedControl
        segments={[
          { value: 'auto', label: 'Auto' },
          { value: 'online', label: 'Online' },
          { value: 'away', label: 'Away' },
          { value: 'offline', label: 'Offline' },
        ]}
        value={presenceMode}
        onChange={(mode) => {
          if (updatePresence.isPending) return
          updatePresence.mutate(mode === 'auto' ? null : mode, {
            onSuccess: () => toast.success('Support status updated'),
            onError: () => toast.error('Could not update support status'),
          })
        }}
        className={updatePresence.isPending ? 'pointer-events-none opacity-60' : undefined}
        size="sm"
      />
      <p className="mt-2 text-caption text-muted-foreground">Automatic follows your live activity. A manual status stays active until you switch back.</p>
    </section>
  )
}
