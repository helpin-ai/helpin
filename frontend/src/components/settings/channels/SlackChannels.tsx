import { useState } from 'react'
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { useConfirm } from '@/components/ui/confirm-dialog'
import { QuietFilterDropdown, QuietSelect } from '@/components/design-system/quiet-select'
import {
  QuietPropertyRow,
  QuietStatusText,
  QuietTextAction,
} from '@/components/design-system/quiet'
import { CRMRecordPicker } from '@/components/automation/CRMRecordPicker'
import { SlackIcon } from '@/lib/icons'
import { slackChannelLabel } from '@/lib/supportChannels'
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams'
import { useEpics } from '@/hooks/queries/useEpics'
import { useInboxScopes } from '@/hooks/queries/useSupport'
import {
  slackService,
  type SlackPurpose,
  type SlackRoute,
} from '@/lib/services/slackService'

const purposes = {
  support: 'Receive customer requests from Slack',
  mixed: 'Receive customer requests from Slack',
  inbox: 'Handle Helpin inboxes in Slack',
  pm: 'Project work',
}
const purposeTooltips: Record<SlackPurpose, string> = {
  support: 'New Slack threads become Helpin support conversations.',
  mixed: 'Mention @Helpin or choose “Create support request” in Slack.',
  pm: 'Create and follow Helpin tasks in Slack.',
  inbox: 'Share new conversations from selected inboxes with your support team.',
}
const emptyRoute: SlackRoute = {
  channel_id: '',
  channel_name: '',
  purpose: 'support',
  mailbox_id: null,
  team_id: null,
  epic_id: null,
}
const errorText = (error: unknown) =>
  error instanceof Error ? error.message : 'Could not update Slack settings'

export function SlackChannels({
  workspaceId,
  slug,
  hasSupport,
  canManageFeed,
  initialInbox,
  hasCRM,
  hasPM,
}: {
  workspaceId: string
  slug: string
  hasSupport: boolean
  canManageFeed: boolean
  initialInbox?: string
  hasCRM?: boolean
  hasPM: boolean
}) {
  const key = ['slack', workspaceId]
  const client = useQueryClient()
  const settings = useQuery({
    queryKey: key,
    queryFn: () => slackService.settings(workspaceId),
  })
  const [draft, setDraft] = useState<SlackRoute | null>(null)
  const confirm = useConfirm()
  const refresh = () => client.invalidateQueries({ queryKey: key })
  const install = useMutation({
    mutationFn: () => slackService.start(workspaceId),
    onSuccess: ({ url }) => {
      sessionStorage.setItem(
        'slack-install',
        JSON.stringify({ workspaceId, slug, initialInbox }),
      )
      window.location.assign(url)
    },
    onError: (e) => toast.error(errorText(e)),
  })
  const disconnect = useMutation({
    mutationFn: () => slackService.disconnect(workspaceId),
    onSuccess: refresh,
    onError: (e) => toast.error(errorText(e)),
  })
  const ai = useMutation({
    mutationFn: (enabled: boolean) => slackService.setAI(workspaceId, enabled),
    onSuccess: refresh,
    onError: (e) => toast.error(errorText(e)),
  })
  const data = settings.data
  const scopes = useInboxScopes(workspaceId, hasSupport)
  const inboxNames = (route: SlackRoute) => (route.mailbox_ids ?? []).map(id => id === 'shared' ? 'Main inbox' : scopes.data?.mailboxes.find(box => box.id === id)?.name ?? 'Team inbox').join(', ')
  const [dismissedInbox, setDismissedInbox] = useState<string>()
  const requestedRule = canManageFeed && initialInbox && initialInbox !== dismissedInbox && data?.connected
    ? data.routes?.find(rule => rule.purpose === 'inbox' && rule.mailbox_ids?.includes(initialInbox))
      ?? { ...emptyRoute, purpose: 'inbox' as const, mailbox_ids: [initialInbox] }
    : null
  const editor = draft ?? requestedRule
  const closeEditor = () => { setDraft(null); setDismissedInbox(initialInbox) }
  return (
    <Card className="gap-0 rounded-lg py-0">
      <CardContent className="px-4 py-3">
        <header className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-x-3 gap-y-2 sm:grid-cols-[auto_minmax(0,1fr)_auto]">
          <SlackIcon className="size-5 shrink-0" />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <h2 className="text-sm font-semibold">Slack</h2>
              {data?.connected && <QuietStatusText tone="positive">Connected</QuietStatusText>}
            </div>
            <p className="text-xs text-muted-foreground">
              {settings.isLoading
                ? 'Loading connection…'
                : settings.isError
                  ? 'Could not load connection'
                  : data?.connected
                    ? data.workspace_name
                    : data?.configured
                      ? 'Connect your workspace to use @Helpin in Slack.'
                      : 'Configure the Slack app on your Helpin instance to connect.'}
            </p>
          </div>
          {data?.connected ? (
            <div className="col-start-2 flex items-center gap-1 sm:col-start-3 sm:row-start-1">
              <Button
                size="sm"
                variant="ghost"
                disabled={
                  !data.configured || install.isPending || disconnect.isPending
                }
                onClick={() => install.mutate()}
              >
                Reauthorize
              </Button>
              <Button
                size="sm"
                variant="ghost"
                disabled={disconnect.isPending}
                onClick={async () => {
                  if (
                    await confirm({
                      title: 'Disconnect Slack?',
                      description:
                        'Message delivery will stop for all connected Slack channels.',
                      confirmText: 'Disconnect',
                      variant: 'destructive',
                    })
                  )
                    disconnect.mutate()
                }}
              >
                Disconnect
              </Button>
            </div>
          ) : (
            <Button
              size="sm"
              variant="outline"
              className="col-start-2 justify-self-start sm:col-start-3 sm:row-start-1"
              disabled={!data?.configured || install.isPending}
              onClick={() => install.mutate()}
            >
              Connect Slack
            </Button>
          )}
        </header>
        {data?.connected && (
          <div className="mt-3 border-t">
            <label className="flex items-center justify-between gap-3 border-b py-3 text-sm">
              AI questions, drafts, and summaries
              <Switch
                checked={!!data.ai_enabled}
                disabled={ai.isPending}
                onCheckedChange={(enabled) => ai.mutate(enabled)}
              />
            </label>
            <div className="flex items-center justify-between gap-3 py-3">
              <div className="min-w-0 space-y-1">
                <h3 className="text-sm font-medium">Channel rules</h3>
              </div>
              <Button
                size="sm"
                variant="outline"
                onClick={() =>
                  setDraft({
                    ...emptyRoute,
                    purpose: hasSupport ? 'support' : 'pm',
                  })
                }
              >
                Add rule
              </Button>
            </div>
            {data.routes?.map((route) => (
              <Button
                key={route.channel_id}
                variant="ghost"
                onClick={() => setDraft(route)}
                className="flex h-auto w-full items-center justify-between gap-3 rounded-none border-b py-3 text-left last:border-b-0"
              >
                <span className="truncate font-medium">
                  {slackChannelLabel(route.channel_name)}
                </span>
                <span className="shrink-0 text-xs font-normal text-muted-foreground">
                  {route.purpose === 'inbox' ? inboxNames(route) + ' → Slack' : route.purpose === 'mixed' ? 'Customer requests on demand' : purposes[route.purpose]}
                </span>
              </Button>
            ))}
          </div>
        )}
        {editor && (
          <RouteDialog
            initial={editor}
            canManageFeed={canManageFeed}
            rules={data?.routes ?? []}
            hasSupport={hasSupport}
            hasCRM={hasCRM}
            hasPM={hasPM}
            workspaceId={workspaceId}
            onClose={closeEditor}
            onSaved={() => {
              closeEditor()
              void refresh()
            }}
          />
        )}
      </CardContent>
    </Card>
  )
}

function RouteDialog({
  initial,
  rules,
  canManageFeed,
  hasSupport,
  hasCRM,
  hasPM,
  workspaceId,
  onClose,
  onSaved,
}: {
  initial: SlackRoute
  rules: SlackRoute[]
  canManageFeed: boolean
  hasSupport: boolean
  hasCRM?: boolean
  hasPM: boolean
  workspaceId: string
  onClose: () => void
  onSaved: () => void
}) {
  const [route, setRoute] = useState(initial)
  const channels = useQuery({
    queryKey: ['slackChannels', workspaceId],
    queryFn: () => slackService.channels(workspaceId),
  })
  const { teams } = useAccessibleTeams(workspaceId)
  const { data: scopes } = useInboxScopes(workspaceId, hasSupport)
  const { data: epics } = useEpics(route.purpose === 'pm' ? workspaceId : '', {
    team_id: route.team_id ?? undefined,
    archived: false,
  })
  const save = useMutation({
    mutationFn: () => slackService.save(workspaceId, route),
    onSuccess: onSaved,
  })
  const remove = useMutation({
    mutationFn: () => slackService.remove(workspaceId, initial.channel_id),
    onSuccess: onSaved,
  })
  const confirm = useConfirm()
  const options = (items: { id: string; name: string }[] = []) =>
    items.map((item) => ({ value: item.id, label: item.name }))
  const selected = channels.data?.find(c => c.id === route.channel_id)
  const isSlackConnectIntake = selected?.is_ext_shared && (route.purpose === 'support' || route.purpose === 'mixed')
  const inboxOptions = [{ value: 'shared', label: 'Main inbox' }, ...options(scopes?.mailboxes)].map(option => {
    const other = rules.find(rule => rule.channel_id !== initial.channel_id && rule.purpose === 'inbox' && rule.mailbox_ids?.includes(option.value))
    return other ? { ...option, disabled: true, label: option.label + ' · ' + slackChannelLabel(other.channel_name) } : option
  })
  const restrictedInPublic = route.purpose === 'inbox' && selected && !selected.is_private && route.mailbox_ids?.some(id => id !== 'shared')
  const readOnly = initial.purpose === 'inbox' && !!initial.channel_id && !canManageFeed
  const pending = save.isPending || remove.isPending
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending) onClose()
      }}
    >
      <DialogContent className={isSlackConnectIntake ? 'grid-cols-1 sm:max-w-2xl' : undefined}>
        <DialogHeader>
          <DialogTitle>
            {initial.channel_id ? 'Edit channel rule' : 'Add channel rule'}
          </DialogTitle>
        </DialogHeader>
        <QuietPropertyRow label="Purpose" value={
          <QuietSelect label="Channel purpose" value={route.purpose === 'mixed' ? 'support' : route.purpose}
            options={(['support', 'inbox', 'pm'] as const).filter(purpose => purpose === 'pm' ? hasPM : hasSupport && (purpose !== 'inbox' || canManageFeed))
              .map(value => ({ value, label: purposes[value], tooltip: purposeTooltips[value] }))}
            onChange={value => setRoute({ ...route, purpose: value as SlackPurpose, channel_id: initial.channel_id || '' })} />
        } />
        {route.purpose === 'inbox' && <QuietPropertyRow label="Inboxes" value={
          <QuietFilterDropdown label="Inboxes" multiple showSelectedNames selected={route.mailbox_ids ?? []}
            options={inboxOptions} emptyLabel="Choose inboxes"
            onChange={mailbox_ids => setRoute({ ...route, mailbox_ids })} />
        } />}
        <QuietPropertyRow label="Slack channel" value={
          <QuietFilterDropdown label="Slack channel" value={route.channel_id} emptyLabel="Choose channel"
            disabled={!!initial.channel_id}
            options={(channels.data ?? []).filter(c => !(c.is_ext_shared || c.is_pending_ext_shared) || route.purpose === 'support' || route.purpose === 'mixed')
              .map(c => ({value:c.id, label:slackChannelLabel(c.name) + (c.is_ext_shared ? ' · Slack Connect' : c.is_private ? ' · Private' : ' · Public')}))}
            onChange={channel_id => setRoute({ ...route, channel_id })} />
        } />
        {!initial.channel_id && <details className="text-xs text-muted-foreground">
          <summary className="w-fit cursor-pointer">Missing a private channel?</summary>
          <p className="mt-1">In Slack, use <code>/invite @Helpin</code>, then <button type="button" className="underline" onClick={() => void channels.refetch()}>refresh channels</button>.</p>
        </details>}
        {channels.isError && <p role="alert" className="text-xs text-destructive">{errorText(channels.error)}</p>}
        {route.purpose === 'inbox' && selected && !selected.is_private &&
          <p className="text-xs text-muted-foreground">{restrictedInPublic
            ? 'Team inboxes require a private Slack channel whose members have access to those inboxes.'
            : `Everyone with access to ${slackChannelLabel(selected.name)} can read shared conversations, files and internal notes.`}</p>}
        {(route.purpose === 'support' || route.purpose === 'mixed') && <QuietPropertyRow label="Create a conversation" value={
          <QuietSelect label="Create a conversation" value={route.purpose}
            options={[{value:'support', label:'Every new customer thread'}, {value:'mixed', label:'Only on request'}]}
            onChange={value => setRoute({ ...route, purpose: value as SlackPurpose })} />
        } />}
        {route.purpose === 'pm' ? (
          <>
            <QuietPropertyRow
              label="Default team"
              value={
                <QuietSelect
                  label="Default team"
                  value={route.team_id ?? 'none'}
                  options={[
                    { value: 'none', label: 'Choose on creation' },
                    ...options(teams),
                  ]}
                  onChange={(id) =>
                    setRoute({
                      ...route,
                      team_id: id === 'none' ? null : id,
                      epic_id: null,
                    })
                  }
                />
              }
            />
            <QuietPropertyRow
              label="Default epic"
              value={
                <QuietSelect
                  label="Default epic"
                  value={route.epic_id ?? 'none'}
                  options={[
                    { value: 'none', label: 'None' },
                    ...options(epics?.map((e) => e.epic)),
                  ]}
                  onChange={(id) =>
                    setRoute({ ...route, epic_id: id === 'none' ? null : id })
                  }
                />
              }
            />
          </>
        ) : route.purpose !== 'inbox' && (
          <QuietPropertyRow
            label="Support inbox"
            value={
              <QuietSelect
                label="Support inbox"
                value={route.mailbox_id ?? 'default'}
                options={[
                  { value: 'default', label: 'Main inbox' },
                  ...options(scopes?.mailboxes),
                ]}
                onChange={(id) =>
                  setRoute({
                    ...route,
                    mailbox_id: id === 'default' ? null : id,
                  })
                }
              />
            }
          />
        )}
        {hasCRM && route.purpose !== 'pm' && selected?.is_ext_shared && (
          <QuietPropertyRow
            label={<span title="Default CRM company for new conversations">CRM company</span>}
            value={
              <div className="flex min-w-0 items-center gap-2 pl-1.5 [&>[role=combobox]]:min-w-0 [&>[role=combobox]]:flex-1">
                <CRMRecordPicker
                  workspaceId={workspaceId}
                  targetType="crm_company"
                  value={route.crm_company_id ?? ''}
                  onChange={(id) => setRoute({ ...route, crm_company_id: id })}
                />
                {route.crm_company_id && (
                  <QuietTextAction onClick={() => setRoute({ ...route, crm_company_id: null })} aria-label="Clear default company">
                    Clear
                  </QuietTextAction>
                )}
              </div>
            }
          />
        )}
        {(save.error || remove.error) && (
          <p role="alert" className="text-sm text-destructive">
            {errorText(save.error || remove.error)}
          </p>
        )}
        <DialogFooter>
          {initial.channel_id && (
            <Button
              variant="ghost"
              disabled={pending || readOnly}
              onClick={async () => {
                if (
                  await confirm({
                    title: 'Remove this rule?',
                    description: initial.purpose === 'inbox' ? 'Helpin will stop sharing conversations and updates in this channel.' : 'New requests will use the workspace defaults.',
                    confirmText: 'Remove',
                    variant: 'destructive',
                  })
                )
                  remove.mutate()
              }}
            >
              Remove rule
            </Button>
          )}
          <Button variant="ghost" onClick={onClose} disabled={pending}>
            Cancel
          </Button>
          <Button
            disabled={!route.channel_id || pending || readOnly || !!restrictedInPublic || (route.purpose === 'inbox' && !route.mailbox_ids?.length)}
            onClick={() => save.mutate()}
          >
            Save rule
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
