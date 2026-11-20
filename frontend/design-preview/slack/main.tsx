import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { TooltipProvider } from '@/components/ui/tooltip'
import { WorkspaceChannelSettings } from '@/pages/settings/SupportChannelsSettingsPage'
import { ConfirmProvider } from '@/components/ui/confirm-dialog'
import { ConversationRow } from '@/components/support/ConversationRow'
import type { SupportConversation } from '@/lib/pmTypes'
import {
  QuietPageHeader,
  QuietPropertyRow,
  QuietSectionHeader,
  QuietStatusText,
} from '@/components/design-system/quiet'
import { QuietConversationComposer } from '@/components/design-system/conversation-composer'
import { MessageDeliveryStatus } from '@/components/support/MessageDeliveryStatus'
import { HelpinLogo } from '@/components/layout/HelpinLogo'
import {
  SlackIcon,
  Mail01Icon,
  Message01Icon,
  Settings02Icon,
  CheckListIcon,
} from '@/lib/icons'
import {
  slackService,
  type SlackSettings,
  type SlackRoute,
} from '@/lib/services/slackService'
import '@/index.css'

const screens = ['Channels', 'Support inbox', 'PM task', 'Inbox list'] as const
const ws = 'northstar'
const routes: SlackRoute[] = [
  {
    channel_id: 'C1',
    channel_name: 'acme-support',
    purpose: 'support',
    mailbox_id: null,
    team_id: null,
    epic_id: null,
  },
  {
    channel_id: 'C2',
    channel_name: 'payments',
    purpose: 'pm',
    mailbox_id: null,
    team_id: 'payments',
    epic_id: null,
  },
]
const client = new QueryClient({
  defaultOptions: { queries: { staleTime: Infinity, retry: false } },
})
let settings: SlackSettings = {
  configured: true,
  connected: true,
  ai_enabled: true,
  workspace_name: 'Northstar',
  routes,
}
const availableChannels = [
  ...routes.map((r) => ({
    id: r.channel_id,
    name: r.channel_name,
    is_ext_shared: r.purpose === 'support',
    is_private: true,
  })),
  { id: 'C3', name: 'product-feedback', is_ext_shared: false },
]
client.setQueryData(['slack', ws], settings)
slackService.settings = async () => settings
slackService.channels = async () => availableChannels
slackService.save = async (_, route) => {
  const channel = availableChannels.find((c) => c.id === route.channel_id)!
  const next = { ...route, channel_name: channel.name }
  settings = {
    ...settings,
    routes: [
      ...settings.routes.filter((r) => r.channel_id !== route.channel_id),
      next,
    ],
  }
  return next
}
slackService.remove = async (_, id) => {
  settings = {
    ...settings,
    routes: settings.routes.filter((r) => r.channel_id !== id),
  }
  return true
}
slackService.setAI = async (_, enabled) => {
  settings = { ...settings, ai_enabled: enabled }
  return true
}
slackService.disconnect = async () => {
  settings = { ...settings, connected: false, routes: [] }
  return true
}
slackService.start = async () => ({ url: location.href })
const team = { id: 'payments', name: 'Payments' }
const mailbox = { id: 'support', name: 'Customer support' }
const epic = { id: 'checkout', name: 'Checkout reliability', team_id: team.id }
routes[0].mailbox_id = mailbox.id
routes[1].epic_id = epic.id
// Keep all settings and inbox queries inside this standalone preview.
window.fetch = async (input) => {
  const path = new URL(
    input instanceof Request ? input.url : String(input),
    location.href,
  ).pathname
  const body = path.endsWith('/settings')
    ? { teams: [team], people: [], memberships: [], user_memberships: [] }
    : path.endsWith('/me')
      ? {
          membership: { role: 'admin' },
          permissions: ['settings.manage', 'support.admin', 'support.edit'],
          team_memberships: [{ team_id: team.id }],
        }
      : path.endsWith('/scopes')
        ? { mailboxes: [mailbox] }
        : path.endsWith('/epics')
          ? [{ epic }]
          : []
  return Response.json(body)
}

function Channels() {
  return (
    <div className="max-w-3xl space-y-4">
      <QuietPageHeader
        title="Channels"
        description="Manage where conversations reach your team."
      />
      <WorkspaceChannelSettings
        workspaceId={ws}
        slug={ws}
        manageSlack
        manageSupport
      />
    </div>
  )
}
function InboxList({ open }: { open: () => void }) {
  const records: SupportConversation[] = [
    {
      id: 'acme',
      customer_name: 'Lena Park',
      subject: 'Checkout returns an error',
      source: 'slack',
      list_last_message_preview:
        'Thanks, Lena. I’ve shared the example with our payments team.',
    },
    {
      id: 'email',
      customer_name: 'Owen Mills',
      subject: 'Invoice copy for October',
      source: 'email',
      list_last_message_preview:
        'Could you send a copy to our billing address?',
    },
    {
      id: 'chat',
      customer_name: 'Mia Chen',
      subject: 'Adding a teammate',
      source: 'widget',
      list_last_message_preview: 'That worked. Thank you!',
    },
  ].map((item, i) => ({
    ...item,
    source: item.source as SupportConversation['source'],
    status: 'open',
    workspace_id: ws,
    display_id: 1042 - i,
    priority: 'medium',
    created_at: new Date(Date.now() - (i + 1) * 60000).toISOString(),
    updated_at: new Date(Date.now() - (i + 1) * 60000).toISOString(),
  }))
  return (
    <div className="max-w-xl">
      <QuietPageHeader title="Support inbox" />
      <div className="mt-5 overflow-hidden rounded-lg border">
        {records.map((c) => (
          <ConversationRow
            key={c.id}
            workspaceId={ws}
            conversation={c}
            moveOptions={[]}
            onSelectConversation={() => {
              if (c.source === 'slack') open()
            }}
          />
        ))}
      </div>
    </div>
  )
}
function Message({
  who,
  time,
  text,
  own = false,
}: {
  who: string
  time: string
  text: string
  own?: boolean
}) {
  return (
    <article className="flex items-start gap-3 py-3">
      <Avatar className="size-8">
        <AvatarFallback
          className={
            own ? 'bg-muted text-xs' : 'bg-amber-100 text-xs text-amber-950'
          }
        >
          {who
            .split(' ')
            .map((n) => n[0])
            .join('')}
        </AvatarFallback>
      </Avatar>
      <div>
        <p className="mb-1 text-sm font-medium">
          {who}{' '}
          <span className="ml-2 text-xs font-normal text-muted-foreground">
            {time}
          </span>
        </p>
        <p className="max-w-md text-sm leading-6">{text}</p>
        {own && (
          <MessageDeliveryStatus
            onClick={() => {}}
            delivery={{
              state: 'sent',
              channels: [
                {
                  channel: 'slack',
                  state: 'sent',
                  description: 'Sent to Slack',
                },
              ],
            }}
          />
        )}
      </div>
    </article>
  )
}
function Inbox({ openTask }: { openTask: () => void }) {
  const [note, setNote] = useState(false)
  return (
    <>
      <QuietPageHeader
        title="Checkout returns an error"
        navigation={
          <span className="text-xs text-muted-foreground">
            Support inbox / Conversation 1042
          </span>
        }
      />
      <div className="mt-6 grid gap-6 md:grid-cols-[minmax(0,1fr)_335px]">
        <section className="border-t">
          <Message
            who="Lena Park"
            time="09:41"
            text="Our checkout returns an error after we apply a discount code. It started this morning."
          />
          <Message
            own
            who="Sam Ellis"
            time="09:46"
            text="Thanks, Lena. I’ve shared the example with our payments team. I’ll update you here."
          />
          <QuietConversationComposer
            className="mt-7 p-3"
            tone={note ? 'note' : 'default'}
          >
            <div className="flex gap-1">
              {['Reply', 'Internal note'].map((l, i) => (
                <Button
                  key={l}
                  variant={note === !!i ? 'secondary' : 'ghost'}
                  size="sm"
                  onClick={() => setNote(!!i)}
                >
                  {l}
                </Button>
              ))}
            </div>
            <textarea
              className="mt-2 min-h-20 w-full resize-none bg-transparent p-2 text-sm outline-none"
              aria-label="Compose"
              placeholder={note ? 'Add an internal note…' : 'Write a reply…'}
            />
            <div className="flex justify-end">
              <Button size="sm">
                {note ? (
                  'Add note'
                ) : (
                  <>
                    <SlackIcon className="size-4" />
                    Reply to #acme-support
                  </>
                )}
              </Button>
            </div>
          </QuietConversationComposer>
        </section>
        <aside className="space-y-2 border-l pl-5">
          <QuietSectionHeader title="Conversation details" />
          <QuietPropertyRow
            label="Status"
            value={<QuietStatusText tone="current">Open</QuietStatusText>}
          />
          <QuietPropertyRow label="Assignee" value="Sam Ellis" />
          <QuietPropertyRow
            label="Channel"
            value={
              <span className="inline-flex items-center gap-1">
                <SlackIcon className="size-3.5" />
                #acme-support
              </span>
            }
          />
          <div className="mt-6 border-t pt-4">
            <QuietSectionHeader title="Linked task" />
            <Button className="mt-1 px-0" variant="link" onClick={openTask}>
              PAY-142 · Fix checkout error
            </Button>
          </div>
        </aside>
      </div>
    </>
  )
}
function Task() {
  return (
    <>
      <QuietPageHeader
        title="Fix checkout error for discount codes"
        navigation={
          <span className="text-xs text-muted-foreground">Tasks / PAY-142</span>
        }
      />
      <div className="mt-6 grid gap-8 md:grid-cols-[minmax(0,1fr)_335px]">
        <div className="space-y-6">
          <p className="max-w-xl text-sm leading-6">
            Checkout fails when a percentage discount is applied to an order.
            Reproduced using the example from Acme’s support thread.
          </p>
          <div className="border-t pt-4">
            <QuietSectionHeader title="Slack discussion" icon={SlackIcon} />
            <p className="mt-1 text-xs text-muted-foreground">#payments</p>
            <Message
              who="Ana Bell"
              time="10:18"
              text="The discount calculation is fixed. The checkout regression check passed."
            />
            <QuietConversationComposer className="p-3">
              <textarea
                aria-label="Comment"
                className="min-h-14 w-full resize-none text-sm outline-none"
                placeholder="Reply in #payments…"
              />
              <div className="flex justify-end">
                <Button size="sm">Reply in thread</Button>
              </div>
            </QuietConversationComposer>
          </div>
        </div>
        <aside className="border-l pl-5">
          <QuietSectionHeader title="Properties" />
          {[
            ['Status', 'In review'],
            ['Team', 'Payments'],
            ['Epic', 'Checkout reliability'],
            ['Owner', 'Ana Bell'],
            ['Priority', 'High'],
          ].map(([k, v]) => (
            <QuietPropertyRow key={k} label={k} value={v} />
          ))}
        </aside>
      </div>
    </>
  )
}
function App() {
  const [screen, set] = useState<number>(
    Math.max(
      0,
      screens.indexOf(
        decodeURIComponent(location.hash.slice(1)) as (typeof screens)[number],
      ),
    ),
  )
  const select = (i: number) => {
    set(i)
    location.hash = screens[i]
  }
  return (
    <div className="flex min-h-screen bg-background text-foreground">
      <aside className="hidden w-52 shrink-0 border-r bg-muted/30 p-4 sm:block">
        <HelpinLogo className="mb-8 justify-start" />
        <p className="mb-2 px-2 text-xs font-medium text-muted-foreground">
          Northstar workspace
        </p>
        {screens.map((s, i) => {
          const Icon = [
            Settings02Icon,
            Message01Icon,
            CheckListIcon,
            Mail01Icon,
          ][i]
          return (
            <Button
              key={s}
              variant={i === screen ? 'secondary' : 'ghost'}
              className="mb-1 w-full justify-start"
              onClick={() => select(i)}
            >
              <Icon className="size-4" />
              {s}
            </Button>
          )
        })}
      </aside>
      <main className="min-w-0 flex-1">
        <div className="border-b px-6 py-3 text-xs text-muted-foreground">
          Northstar <span className="mx-2">/</span>
          {screens[screen]}
          <span className="float-right">Mock data</span>
        </div>
        <div className="flex gap-1 p-2 sm:hidden">
          {screens.map((s, i) => (
            <Button key={s} size="sm" variant="ghost" onClick={() => select(i)}>
              {s}
            </Button>
          ))}
        </div>
        <div className="mx-auto max-w-6xl p-6 md:p-10">
          {screen === 0 ? (
            <Channels />
          ) : screen === 1 ? (
            <Inbox openTask={() => select(2)} />
          ) : screen === 2 ? (
            <Task />
          ) : (
            <InboxList open={() => select(1)} />
          )}
        </div>
      </main>
    </div>
  )
}
createRoot(document.getElementById('root')!).render(
  <QueryClientProvider client={client}>
    <ConfirmProvider>
      <TooltipProvider>
        <App />
      </TooltipProvider>
    </ConfirmProvider>
  </QueryClientProvider>,
)
