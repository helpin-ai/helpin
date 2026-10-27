import { Switch } from '@/components/ui/switch'
import { useChatSettings, useUpdateChatSettings } from '@/hooks/queries/useSupport'

/** Workspace switch for Helpin AI raising conversation priority; on unless turned off. */
export function SupportAIPrioritySetting({ workspaceId, editable }: { workspaceId: string; editable: boolean }) {
  const { data: installation, isLoading } = useChatSettings(workspaceId)
  const update = useUpdateChatSettings(workspaceId)
  const enabled = installation?.settings?.ai_priority_enabled ?? true

  return (
    <div className="flex items-start justify-between gap-4 rounded-lg border border-border px-4 py-3">
      <div className="min-w-0">
        <label htmlFor="ai-priority-enabled" className="text-sm font-medium">
          Let Helpin AI raise priority
        </label>
        <p className="mt-0.5 text-xs text-muted-foreground">
          Raises priority when a customer describes an outage, lost data, a security problem or a broken core feature. It never lowers priority or changes one a teammate set.
        </p>
      </div>
      <Switch
        id="ai-priority-enabled"
        checked={enabled}
        disabled={!editable || isLoading || update.isPending}
        onCheckedChange={(checked) => update.mutate({ ai_priority_enabled: checked })}
      />
    </div>
  )
}
