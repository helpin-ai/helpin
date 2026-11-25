import { useQuery } from '@tanstack/react-query'
import { QuietPropertyRow } from '@/components/design-system/quiet'
import { useWorkspaceAccess } from '@/hooks/queries/useSession'
import { slackService } from '@/lib/services/slackService'
import { slackChannelLabel } from '@/lib/supportChannels'
import { useWorkspaceStore } from '@/stores/workspaceStore'

export function useInboxSlackChannel(workspaceId: string, mailboxId: string) {
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
  return {
    href: `/w/${encodeURIComponent(slug)}/settings/external-tools?tab=apps&slack_inbox=${encodeURIComponent(mailboxId)}`,
    channelName: channel ? slackChannelLabel(channel.channel_name) : null,
  }
}

export function InboxSlackChannelRow({ workspaceId, mailboxId }: { workspaceId: string; mailboxId: string }) {
  const slack = useInboxSlackChannel(workspaceId, mailboxId)
  if (!slack) return null
  return <QuietPropertyRow label="Slack channel" value={
    <a className="text-xs text-muted-foreground hover:text-foreground hover:underline"
      href={slack.href}>
      {slack.channelName ? `${slack.channelName} · Manage` : 'Connect channel'}
    </a>
  } />
}
