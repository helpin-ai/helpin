import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { slackService, type SlackNotificationPreferences } from '@/lib/services/slackService'
import { useAuthStore } from '@/stores/authStore'

export function useSlackNotificationSettings(workspaceId: string) {
  const userId = useAuthStore(state => state.user?.id)
  const key = ['slack', workspaceId, 'notifications', userId] as const
  const client = useQueryClient()
  const settings = useQuery({
    queryKey: key,
    queryFn: () => slackService.notificationPreferences(workspaceId),
    enabled: Boolean(workspaceId && userId),
    refetchOnWindowFocus: 'always',
  })
  const save = useMutation({
    mutationFn: ({ row, enabled }: { row: string; enabled: boolean }) =>
      slackService.updateNotificationPreferences(workspaceId, { categories: { [row]: enabled } }),
    onSuccess: (_, { row, enabled }) => {
      client.setQueryData<SlackNotificationPreferences>(key, previous => previous
        ? { ...previous, categories: { ...previous.categories, [row]: enabled } } : previous)
    },
    onError: () => {
      toast.error('Couldn’t save your Slack notification preference.')
      void client.invalidateQueries({ queryKey: key })
    },
  })
  return { settings, save }
}

export type SlackNotificationControls = ReturnType<typeof useSlackNotificationSettings>

export function SlackConnectionNotice({ controls, slug, canManage }: {
  controls: SlackNotificationControls
  slug: string
  canManage: boolean
}) {
  const { settings } = controls
  const pref = settings.data
  if (settings.isError) return <p role="alert" className="text-sm">Couldn’t load Slack settings. <Button variant="ghost" onClick={() => settings.refetch()}>Try again</Button></p>
  if (!pref || pref.linked) return null
  return <p className="px-1 text-sm text-muted-foreground">{!pref.team_id ? (canManage
    ? <Link to="/w/$slug/settings/external-tools" params={{ slug }} search={{ tab: 'apps' }} className="underline">Connect Slack to enable direct messages</Link>
    : 'Ask a workspace admin to connect Slack to enable direct messages.') : <>
    Send <strong>link account</strong> to {pref.bot_user_id ? (
      <a className="underline" href={`slack://user?team=${encodeURIComponent(pref.team_id)}&id=${encodeURIComponent(pref.bot_user_id)}`}>@Helpin in Slack</a>
    ) : '@Helpin in Slack'} to enable direct messages.
  </>}</p>
}
