import { useQuery } from '@tanstack/react-query'
import { useWorkspaceAccess } from '@/hooks/queries/useSession'
import { slackService } from '@/lib/services/slackService'
import { slackChannelLabel } from '@/lib/supportChannels'
import { useWorkspaceStore } from '@/stores/workspaceStore'

export function useInboxSlackChannel(workspaceId: string, mailboxId?: string | null) {
  const slug = useWorkspaceStore(state => state.currentWorkspace?.slug)
  const { data: access } = useWorkspaceAccess(workspaceId)
  const allowed = !!access?.permissions?.includes('settings.manage') && !!access.permissions.includes('support.admin')
  const { data } = useQuery({
    queryKey: ['slack', workspaceId],
    queryFn: () => slackService.settings(workspaceId),
    enabled: allowed && !!mailboxId,
  })
  if (!allowed || !data?.configured || !slug || !mailboxId) return null
  const channel = data.routes?.find(rule => rule.purpose === 'inbox' && rule.mailbox_ids?.includes(mailboxId))
  return {
    href: `/w/${encodeURIComponent(slug)}/settings/external-tools?tab=apps&slack_inbox=${encodeURIComponent(mailboxId)}`,
    channelName: channel ? slackChannelLabel(channel.channel_name) : null,
  }
}
