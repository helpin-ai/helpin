import { useQuery } from '@tanstack/react-query'
import { QuietPropertyRow } from '@/components/design-system/quiet'
import { useWorkspaceAccess } from '@/hooks/queries/useSession'
import { slackService } from '@/lib/services/slackService'
import { slackChannelLabel } from '@/lib/supportChannels'
import { useWorkspaceStore } from '@/stores/workspaceStore'

export function InboxSlackChannelRow({ workspaceId, mailboxId, compact = false }: {
  workspaceId: string
  mailboxId: string
  compact?: boolean
}) {
  const slug = useWorkspaceStore(state => state.currentWorkspace?.slug)
  const { data: access } = useWorkspaceAccess(workspaceId)
  const allowed = !!access?.permissions?.includes('settings.manage') && !!access.permissions.includes('support.admin')
  const { data } = useQuery({
    queryKey: ['slack', workspaceId],
    queryFn: () => slackService.settings(workspaceId),
    enabled: allowed,
  })
  if (!allowed || !data?.configured || !slug) return null
  const channel = data.routes?.find(rule => rule.purpose === 'inbox' && rule.mailbox_ids?.includes(mailboxId))
  const link = (
    <a className="text-xs text-muted-foreground hover:text-foreground hover:underline"
      href={`/w/${encodeURIComponent(slug)}/settings/external-tools?tab=apps&slack_inbox=${encodeURIComponent(mailboxId)}`}>
      {compact ? (channel ? `Slack · ${slackChannelLabel(channel.channel_name)}` : 'Connect Slack') : (channel ? `${slackChannelLabel(channel.channel_name)} · Manage` : 'Connect channel')}
    </a>
  )
  return compact ? link : <QuietPropertyRow label="Slack channel" value={link} />
}
