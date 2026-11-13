import { useMemo, useState } from 'react'
import { formatDistanceToNowStrict } from 'date-fns'
import { QuietDropdown } from '@/components/design-system/quiet-dropdown'
import { QuietEmptyState } from '@/components/design-system/quiet'
import { Button } from '@/components/ui/button'
import { useConfirm } from '@/components/ui/confirm-dialog'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { useInboxScopes } from '@/hooks/queries/useSupport'
import { useConnectChannel, useDisconnectChannel, useReconnectChannel, useSupportChannels, useUpdateChannel } from '@/hooks/queries/useSupportChannels'
import { MoreHorizontalIcon, PlusSignIcon, TelegramIcon } from '@/lib/icons'
import type { SupportChannelAccount } from '@/lib/pmTypes'

// The workspace default inbox; the API stores it as no mailbox.
const DEFAULT_INBOX = ''

type Inbox = { id: string; name: string }

function health(account: SupportChannelAccount): { text: string; tone: 'muted' | 'warning' | 'error' } {
  if (account.status === 'reauthorization_required') return { text: 'Telegram rejected the bot token. Reconnect with a new token.', tone: 'error' }
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
  const { data: accounts, isLoading, error } = useSupportChannels(workspaceId)
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

  if (isLoading) return <Skeleton className="h-40 w-full" />
  if (error) {
    return <p className="text-sm text-destructive">Could not load channels: {error instanceof Error ? error.message : 'unknown error'}</p>
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="max-w-2xl text-sm text-muted-foreground">
          Customers who message your Telegram bot privately appear here as conversations. Replies from Helpin go back to them in Telegram.
        </p>
        {editable ? (
          <Button size="sm" onClick={() => setConnectOpen(true)}>
            <PlusSignIcon className="h-4 w-4" />
            Connect Telegram bot
          </Button>
        ) : null}
      </div>

      {!accounts || accounts.length === 0 ? (
        <QuietEmptyState
          title="No channels connected"
          description="Create a bot with @BotFather in Telegram, then connect it here with its token."
          action={editable ? <Button size="sm" variant="outline" onClick={() => setConnectOpen(true)}>Connect a bot</Button> : undefined}
        />
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border" aria-label="Connected channels">
          {accounts.map((account) => {
            const state = health(account)
            return (
              <li key={account.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
                <TelegramIcon className="h-5 w-5 shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">@{account.handle}</p>
                  <p className={`truncate text-xs ${TONE_CLASS[state.tone]}`}>{account.display_name ? `${account.display_name} · ` : ''}{state.text}</p>
                </div>
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
                {editable ? (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button size="icon" variant="ghost" aria-label={`Actions for @${account.handle}`}>
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
    </div>
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
        <button type="button" aria-label="Inbox" disabled={disabled} className="max-w-48 truncate rounded-md border border-border px-2.5 py-1 text-left text-xs hover:bg-muted disabled:opacity-60">
          {current}
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
