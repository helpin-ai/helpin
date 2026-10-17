import { ArrowDownLeft, ArrowUpRight } from 'lucide-react'
import { Tabs } from 'radix-ui'
import { useInboundEmails } from '@/hooks/queries/useInboundEmails'
import { IncomingEmailQueue } from './incoming-email-queue'
import { OutgoingEmailQueue } from './outgoing-email-queue'

export function EmailQueuePage() {
  const { data } = useInboundEmails()
  return (
    <div className="mx-auto max-w-6xl">
      <h1 className="text-xl font-semibold tracking-tight">Email Queue</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        Review incoming issues and pending outgoing mail.
      </p>
      <Tabs.Root defaultValue="incoming" className="mt-6">
        <Tabs.List aria-label="Email direction" className="mb-5 flex gap-6 border-b">
          <Tabs.Trigger
            value="incoming"
            className="flex items-center gap-2 border-b-2 border-transparent pb-3 text-sm text-muted-foreground outline-offset-4 data-[state=active]:border-primary data-[state=active]:text-foreground"
          >
            <ArrowDownLeft className="size-4" />
            Incoming
            {!!data?.attention_count && (
              <span className="rounded bg-primary/10 px-1.5 text-xs text-primary">
                {data.attention_count}
              </span>
            )}
          </Tabs.Trigger>
          <Tabs.Trigger
            value="outgoing"
            className="flex items-center gap-2 border-b-2 border-transparent pb-3 text-sm text-muted-foreground outline-offset-4 data-[state=active]:border-primary data-[state=active]:text-foreground"
          >
            <ArrowUpRight className="size-4" />
            Outgoing
          </Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="incoming">
          <IncomingEmailQueue />
        </Tabs.Content>
        <Tabs.Content value="outgoing">
          <OutgoingEmailQueue />
        </Tabs.Content>
      </Tabs.Root>
    </div>
  )
}
