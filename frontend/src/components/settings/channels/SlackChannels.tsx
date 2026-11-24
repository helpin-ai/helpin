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
import { QuietSelect } from '@/components/design-system/quiet-select'
import {
  QuietPropertyRow,
  QuietStatusText,
  QuietTextAction,
} from '@/components/design-system/quiet'
import { CRMRecordPicker } from '@/components/automation/CRMRecordPicker'
import { SlackIcon } from '@/lib/icons'
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams'
import { useEpics } from '@/hooks/queries/useEpics'
import { useInboxScopes } from '@/hooks/queries/useSupport'
import {
  slackService,
  type SlackPurpose,
  type SlackRoute,
} from '@/lib/services/slackService'

const purposes = {
  support: 'Customer support',
  mixed: 'Support on request',
  pm: 'Project work',
}
const purposeTooltips: Record<SlackPurpose, string> = {
  support: 'Turn every new customer thread into a support request.',
  mixed: 'Open a support request only when someone asks.',
  pm: 'Create and follow tasks in this channel.',
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
  hasCRM,
  hasPM,
}: {
  workspaceId: string
  slug: string
  hasSupport: boolean
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
        JSON.stringify({ workspaceId, slug }),
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
  return (
    <Card className="gap-0 rounded-lg py-0">
      <CardContent className="px-4 py-3">
        <div className="flex items-center gap-3">
          <SlackIcon className="size-5 shrink-0" />
          <div className="min-w-0 flex-1">
            <h2 className="text-sm font-semibold">Slack</h2>
            <p className="text-xs text-muted-foreground">
              {settings.isLoading
                ? 'Loading connection…'
                : settings.isError
                  ? 'Could not load connection'
                  : data?.connected
                    ? data.workspace_name
                    : data?.configured
                      ? 'Connect a workspace to route Slack threads.'
                      : 'Configure the Slack app on your Helpin instance to connect.'}
            </p>
          </div>
          {data?.connected ? (
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
              Add channel
            </Button>
          ) : (
            <Button
              size="sm"
              variant="outline"
              disabled={!data?.configured || install.isPending}
              onClick={() => install.mutate()}
            >
              Connect Slack
            </Button>
          )}
        </div>
        {data?.connected && (
          <div className="mt-3 border-t">
            {data.routes?.map((route) => (
              <Button
                key={route.channel_id}
                variant="ghost"
                onClick={() => setDraft(route)}
                className="flex h-auto w-full items-center justify-between gap-3 rounded-none border-b py-3 text-left"
              >
                <span className="truncate font-medium">
                  #{route.channel_name}
                </span>
                <span className="shrink-0 text-xs font-normal text-muted-foreground">
                  {purposes[route.purpose]}
                </span>
              </Button>
            ))}
            <label className="flex items-center justify-between border-b py-3 text-sm">
              AI questions, drafts, and summaries
              <Switch
                checked={!!data.ai_enabled}
                disabled={ai.isPending}
                onCheckedChange={(enabled) => ai.mutate(enabled)}
              />
            </label>
            <div className="flex items-center justify-between pt-2">
              <QuietStatusText tone="positive">Connected</QuietStatusText>
              <div className="flex items-center gap-1">
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
            </div>
          </div>
        )}
        {draft && (
          <RouteDialog
            initial={draft}
            hasSupport={hasSupport}
            hasCRM={hasCRM}
            hasPM={hasPM}
            workspaceId={workspaceId}
            onClose={() => setDraft(null)}
            onSaved={() => {
              setDraft(null)
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
  hasSupport,
  hasCRM,
  hasPM,
  workspaceId,
  onClose,
  onSaved,
}: {
  initial: SlackRoute
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
  const selected = channels.data?.find((c) => c.id === route.channel_id)
  const pending = save.isPending || remove.isPending
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending) onClose()
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {initial.channel_id ? 'Edit channel' : 'Add Slack channel'}
          </DialogTitle>
        </DialogHeader>
        <QuietPropertyRow
          label="Slack channel"
          value={
            <QuietSelect
              label="Slack channel"
              placeholder="Choose channel"
              value={route.channel_id}
              disabled={!!initial.channel_id}
              options={options(
                channels.data
                  ?.filter((c) => hasSupport || !c.is_ext_shared)
                  .map((c) => ({ id: c.id, name: '#' + c.name })),
              )}
              onChange={(id) =>
                setRoute({
                  ...route,
                  channel_id: id,
                  purpose: hasSupport ? 'support' : 'pm',
                })
              }
            />
          }
        />
        {!channels.isLoading && !channels.data?.length && (
          <p className="text-sm text-muted-foreground">
            {channels.error
              ? errorText(channels.error)
              : 'Invite @Helpin to a Slack channel to make it available here.'}
          </p>
        )}
        <QuietPropertyRow
          label="Use for"
          value={
            <QuietSelect
              label="Channel purpose"
              value={route.purpose}
              options={Object.entries(purposes)
                .filter(([key]) =>
                  key === 'pm'
                    ? hasPM &&
                      !selected?.is_ext_shared &&
                      !selected?.is_pending_ext_shared
                    : hasSupport,
                )
                .map(([value, label]) => ({ value, label, tooltip: purposeTooltips[value as SlackPurpose] }))}
              onChange={(value) =>
                setRoute({ ...route, purpose: value as SlackPurpose })
              }
            />
          }
        />
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
        ) : (
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
              disabled={pending}
              onClick={async () => {
                if (
                  await confirm({
                    title: 'Remove this channel?',
                    description: 'Helpin will stop syncing this channel.',
                    confirmText: 'Remove',
                    variant: 'destructive',
                  })
                )
                  remove.mutate()
              }}
            >
              Remove channel
            </Button>
          )}
          <Button variant="ghost" onClick={onClose} disabled={pending}>
            Cancel
          </Button>
          <Button
            disabled={!route.channel_id || pending}
            onClick={() => save.mutate()}
          >
            Save channel
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
