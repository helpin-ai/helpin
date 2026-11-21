import { useMemo, useState } from 'react'
import { formatDistanceToNowStrict } from 'date-fns'
import { QuietDropdown } from '@/components/design-system/quiet-dropdown'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { useConfirm } from '@/components/ui/confirm-dialog'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { useInboxScopes } from '@/hooks/queries/useSupport'
import { useConnectChannel, useDisconnectChannel, useReconnectChannel, useSupportChannels, useUpdateChannel } from '@/hooks/queries/useSupportChannels'
import { ArrowDown01Icon, MoreHorizontalIcon, TelegramIcon } from '@/lib/icons'
import type { SupportChannelAccount } from '@/lib/pmTypes'

// The workspace default inbox; the API stores it as no mailbox.
const DEFAULT_INBOX = ''

type Inbox = { id: string; name: string }

function health(account: SupportChannelAccount): { text: string; tone: 'muted' | 'warning' | 'error' } {
  if (account.status === 'reauthorization_required') return { text: 'Telegram rejected this token. Choose “Replace bot token” in the actions menu.', tone: 'error' }
  if (account.failed_deliveries > 0) {
    return { text: `${account.failed_deliveries} ${account.failed_deliveries === 1 ? 'reply was' : 'replies were'} not delivered`, tone: 'warning' }
  }
  if (account.unconfirmed_deliveries > 0) {
    return { text: `${account.unconfirmed_deliveries} ${account.unconfirmed_deliveries === 1 ? 'reply needs' : 'replies need'} checking`, tone: 'warning' }
  }
  if (account.last_event_at) return { text: `Last message ${formatDistanceToNowStrict(new Date(account.last_event_at), { addSuffix: true })}`, tone: 'muted' }
  return { text: 'No messages yet', tone: 'muted' }
}

const TONE_CLASS = { muted: 'text-muted-foreground', warning: 'text-amber-700 dark:text-amber-300', error: 'text-destructive' }

/** Connected Telegram bots: each feeds its chosen inbox. */
export function SupportChannels({ workspaceId, editable }: { workspaceId: string; editable: boolean }) {
  const { data: allAccounts, isLoading, error } = useSupportChannels(workspaceId)
  const accounts = allAccounts?.filter(account => account.provider === 'telegram');
  const { data: scopes } = useInboxScopes(workspaceId)
  const connect = useConnectChannel(workspaceId)
  const update = useUpdateChannel(workspaceId)
  const reconnect = useReconnectChannel(workspaceId)
  const disconnect = useDisconnectChannel(workspaceId)
  const confirm = useConfirm()
  const [connectOpen, setConnectOpen] = useState(false)
  const [replacing, setReplacing] = useState<SupportChannelAccount | null>(null)

  const inboxes = useMemo<Inbox[]>(() => (scopes?.mailboxes ?? []).map((mailbox) => ({ id: mailbox.id, name: mailbox.name })), [scopes])

  const remove = async (account: SupportChannelAccount) => {
    const confirmed = await confirm({
      title: `Disconnect @${account.handle}?`,
      description: 'Helpin stops receiving its messages, and replies not yet delivered are cancelled. Conversations stay in the inbox.',
      confirmText: 'Disconnect',
      variant: 'destructive',
    })
    if (confirmed) disconnect.mutate(account.id)
  }

  return (
    <Card id="telegram" className="gap-0 rounded-lg py-0">
      <CardContent className="px-4 py-3">
        <div className="flex items-center gap-3">
          <TelegramIcon className="size-5 shrink-0" />
          <div className="min-w-0 flex-1">
            <h2 className="text-sm font-semibold">Telegram</h2>
            {error ? (
              <p role="alert" className="text-xs text-destructive">Could not load Telegram bots{error instanceof Error ? `: ${error.message}` : '.'}</p>
            ) : (
              <p className="text-xs text-muted-foreground">
                {isLoading ? 'Loading connection…' : 'Receive and reply to private support messages.'}
              </p>
            )}
          </div>
          {editable && (
            <Button size="sm" variant="outline" disabled={isLoading || !!error} onClick={() => setConnectOpen(true)}>
              Connect bot
            </Button>
          )}
        </div>
        {!!accounts?.length && (
          <ul className="mt-3 divide-y border-t" aria-label="Connected Telegram bots">
            {accounts.map((account) => {
              const state = health(account)
              return (
                <li key={account.id} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-2 py-3 last:pb-0 sm:grid-cols-[minmax(0,1fr)_auto_auto]">
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">@{account.handle}</p>
                    <p className={`break-words text-xs ${TONE_CLASS[state.tone]}`}>{account.display_name ? `${account.display_name} · ` : ''}{state.text}</p>
                  </div>
                  <div className="col-span-2 row-start-2 flex min-w-0 flex-wrap items-center gap-3 sm:col-span-1 sm:col-start-2 sm:row-start-1">
                    <InboxPicker
                      inboxes={inboxes}
                      value={account.mailbox_id ?? DEFAULT_INBOX}
                      disabled={!editable}
                      onChange={(mailboxId) => update.mutate({ id: account.id, input: { mailbox_id: mailboxId } })}
                    />
                    <label className="flex items-center gap-2 text-xs text-muted-foreground">
                      <Switch
                        checked={!account.ai_replies_disabled}
                        disabled={!editable}
                        aria-label={`AI replies for @${account.handle}`}
                        onCheckedChange={(enabled) => update.mutate({ id: account.id, input: { ai_replies_disabled: !enabled } })}
                      />
                      AI replies
                    </label>
                  </div>
                  {editable ? (
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button size="icon" variant="ghost" className="col-start-2 row-start-1 size-8 sm:col-start-3" aria-label={`Actions for @${account.handle}`}>
                          <MoreHorizontalIcon className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onSelect={() => reconnect.mutate({ id: account.id })}>Reconnect</DropdownMenuItem>
                        <DropdownMenuItem onSelect={() => setReplacing(account)}>Replace bot token</DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem className="text-destructive focus:text-destructive" onSelect={() => void remove(account)}>Disconnect</DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  ) : null}
                </li>
              )
            })}
          </ul>
        )}
      </CardContent>

      <TokenDialog
        open={connectOpen}
        onOpenChange={setConnectOpen}
        title="Connect a Telegram bot"
        description="In Telegram, message @BotFather, send /newbot, and paste the token it gives you."
        inboxes={inboxes}
        saving={connect.isPending}
        submitText="Connect bot"
        onSubmit={async (token, mailboxId) => {
          try {
            await connect.mutateAsync({ token, mailboxId })
            return true
          } catch {
            return false
          }
        }}
      />
      <TokenDialog
        open={replacing !== null}
        onOpenChange={(open) => !open && setReplacing(null)}
        title={replacing ? `Replace the token for @${replacing.handle}` : 'Replace token'}
        description="Use this after revoking the token in @BotFather. The new token must belong to the same bot."
        saving={reconnect.isPending}
        submitText="Replace token"
        onSubmit={async (token) => {
          if (!replacing) return false
          try {
            await reconnect.mutateAsync({ id: replacing.id, token })
            return true
          } catch {
            return false
          }
        }}
      />
    </Card>
  )
}

function InboxPicker({ inboxes, value, disabled, onChange }: { inboxes: Inbox[]; value: string; disabled: boolean; onChange: (mailboxId: string) => void }) {
  const options = [{ value: DEFAULT_INBOX, label: 'Main inbox' }, ...inboxes.map((inbox) => ({ value: inbox.id, label: inbox.name }))]
  const current = options.find((option) => option.value === value)?.label ?? 'Archived inbox'
  return (
    <QuietDropdown
      label="Inbox"
      selected={[value]}
      options={options}
      disabled={disabled}
      onSelect={(next) => next !== value && onChange(next)}
      trigger={(
        <button type="button" aria-label={`Inbox: ${current}`} disabled={disabled} className="inline-flex max-w-48 items-center gap-1.5 rounded-md border border-border px-2.5 py-1 text-left text-xs hover:bg-muted disabled:opacity-60">
          <span className="truncate">{current}</span>
          <ArrowDown01Icon className="size-3 shrink-0 text-muted-foreground" />
        </button>
      )}
    />
  )
}

function TokenDialog({
  open,
  onOpenChange,
  title,
  description,
  inboxes,
  saving,
  submitText,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  inboxes?: Inbox[]
  saving: boolean
  submitText: string
  onSubmit: (token: string, mailboxId: string | null) => Promise<boolean>
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {open ? (
          <TokenForm
            inboxes={inboxes}
            saving={saving}
            submitText={submitText}
            onCancel={() => onOpenChange(false)}
            onSubmit={async (token, mailboxId) => {
              if (await onSubmit(token, mailboxId)) onOpenChange(false)
            }}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function TokenForm({
  inboxes,
  saving,
  submitText,
  onCancel,
  onSubmit,
}: {
  inboxes?: Inbox[]
  saving: boolean
  submitText: string
  onCancel: () => void
  onSubmit: (token: string, mailboxId: string | null) => Promise<void>
}) {
  const [token, setToken] = useState('')
  const [mailboxId, setMailboxId] = useState(DEFAULT_INBOX)
  const [error, setError] = useState<string | null>(null)

  return (
    <form
      className="space-y-5"
      onSubmit={(event) => {
        event.preventDefault()
        const trimmed = token.trim()
        // Bot tokens look like 123456789:AA... ; catch pasted usernames or links early.
        if (!/^\d+:[\w-]{20,}$/.test(trimmed)) {
          setError('Paste the full token from @BotFather, for example 123456789:AAH…')
          return
        }
        setError(null)
        void onSubmit(trimmed, mailboxId === DEFAULT_INBOX ? null : mailboxId)
      }}
    >
      <div className="space-y-1.5">
        <Label htmlFor="channel-token">Bot token</Label>
        <Input
          id="channel-token"
          type="password"
          autoComplete="off"
          spellCheck={false}
          value={token}
          placeholder="123456789:AAH…"
          onChange={(event) => setToken(event.target.value)}
          aria-invalid={Boolean(error)}
          aria-describedby={error ? 'channel-token-error' : undefined}
        />
        {error ? <p id="channel-token-error" role="alert" className="text-xs text-destructive">{error}</p> : null}
      </div>
      {inboxes ? (
        <div className="space-y-1.5">
          <Label>New conversations go to</Label>
          <InboxPicker inboxes={inboxes} value={mailboxId} disabled={false} onChange={setMailboxId} />
          <p className="text-xs text-muted-foreground">Routing rules can still move them to another inbox.</p>
        </div>
      ) : null}
      <DialogFooter>
        <Button type="button" variant="ghost" onClick={onCancel}>Cancel</Button>
        <Button type="submit" disabled={saving}>{saving ? 'Connecting…' : submitText}</Button>
      </DialogFooter>
    </form>
  )
}
