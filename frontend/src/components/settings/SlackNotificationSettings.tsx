import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { toast } from 'sonner'
import { SettingsSection } from './SettingsSection'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { slackService, type SlackNotificationPreferences } from '@/lib/services/slackService'
import { useAuthStore } from '@/stores/authStore'

export function SlackNotificationSettings({ workspaceId, slug, canManage }: {
  workspaceId: string
  slug: string
  canManage: boolean
}) {
  const userId = useAuthStore(state => state.user?.id)
  const key = ['slack', workspaceId, 'notifications', userId] as const
  const client = useQueryClient()
  const settings = useQuery({
    queryKey: key,
    queryFn: () => slackService.notificationPreferences(workspaceId),
    refetchOnWindowFocus: 'always',
  })
  const save = useMutation({
    mutationFn: (patch: Partial<Record<'assignments' | 'mentions', boolean>>) =>
      slackService.updateNotificationPreferences(workspaceId, patch),
    onSuccess: (_, patch) => {
      client.setQueryData<SlackNotificationPreferences>(key, previous =>
        previous ? { ...previous, ...patch } : previous)
    },
    onError: () => {
      toast.error('Couldn’t save your Slack notification preferences.')
      void client.invalidateQueries({ queryKey: key })
    },
  })

  const pref = settings.data
  return (
    <SettingsSection title="Slack direct messages">
      {settings.isPending ? <Skeleton className="my-3 h-8" /> : settings.isError || !pref ? (
        <p role="alert" className="py-2 text-sm">Couldn’t load Slack settings. <Button variant="ghost" onClick={() => settings.refetch()}>Try again</Button></p>
      ) : !pref.team_id ? (
        <p className="pt-3 text-sm text-muted-foreground">
          {canManage
            ? <Link to="/w/$slug/settings/external-tools" params={{ slug }} search={{ tab: 'apps' }} className="underline">Connect this workspace to Slack</Link>
            : 'Ask a workspace admin to connect Slack.'}
        </p>
      ) : !pref.linked ? (
        <p className="pt-3 text-sm text-muted-foreground">
          Send <strong>link account</strong> to {pref.bot_user_id ? (
            <a className="underline" href={`slack://user?team=${encodeURIComponent(pref.team_id)}&id=${encodeURIComponent(pref.bot_user_id)}`}>@Helpin in Slack</a>
          ) : '@Helpin in Slack'} to link your account.
        </p>
      ) : (
        <div className="divide-y divide-quiet-divider-strong">
          {(['assignments', 'mentions'] as const).map(kind => (
            <label key={kind} className="flex items-center justify-between gap-4 py-3 text-sm">
              {kind === 'assignments' ? 'When someone assigns me work' : 'When I’m @mentioned'}
              <Switch checked={pref[kind]} disabled={save.isPending}
                onCheckedChange={enabled => save.mutate({ [kind]: enabled })} />
            </label>
          ))}
        </div>
      )}
    </SettingsSection>
  )
}
