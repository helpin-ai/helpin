import { useEffect, useState } from 'react'
import {
  CheckCheck,
  ChevronLeft,
  ChevronRight,
  HelpCircle,
  LoaderCircle,
  RotateCcw,
  Search,
} from 'lucide-react'
import { Dialog, Tooltip } from 'radix-ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useInboundEmails } from '@/hooks/queries/useInboundEmails'
import type { IncomingStatus } from '@/lib/inboundEmailTypes'
import { cn } from '@/lib/utils'
import { InboundEmailDrawer } from './inbound-email-drawer'
import { emailDate, emailStatus, recoveryInProgress } from './inbound-email-utils'

export function IncomingEmailQueue() {
  const [filter, setFilter] = useState<IncomingStatus>('attention')
  const [search, setSearch] = useState('')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<string | null>(null)
  useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(search.trim())
      setPage(1)
    }, 250)
    return () => clearTimeout(timer)
  }, [search])
  const emails = useInboundEmails(filter, query, page)
  const entries = emails.data?.entries ?? []
  const total = emails.data?.total ?? 0

  return (
    <>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Select
            value={filter}
            onValueChange={(value) => {
              setFilter(value as IncomingStatus)
              setPage(1)
            }}
          >
            <SelectTrigger aria-label="Email status" className="w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {[
                ['attention', 'Needs attention'],
                ['processing', 'Processing'],
                ['recovered', 'Recovered'],
                ['all', 'All recovery emails'],
              ].map(([value, label]) => (
                <SelectItem key={value} value={value}>
                  {label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Tooltip.Provider>
            <Tooltip.Root>
              <Tooltip.Trigger asChild>
                <button aria-label="About incoming emails" className="p-1 text-muted-foreground">
                  <HelpCircle className="size-4" />
                </button>
              </Tooltip.Trigger>
              <Tooltip.Portal>
                <Tooltip.Content
                  sideOffset={6}
                  className="z-50 max-w-64 rounded-md border bg-popover p-3 text-xs text-popover-foreground shadow-md"
                >
                  Emails stay here until their destination is confirmed or processing succeeds. Only
                  Helpin admins can review them.
                </Tooltip.Content>
              </Tooltip.Portal>
            </Tooltip.Root>
          </Tooltip.Provider>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-2.5 size-4 text-muted-foreground" />
            <Input
              aria-label="Search emails"
              placeholder="Search emails…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-52 pl-9 sm:w-64"
            />
          </div>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Refresh incoming emails"
            disabled={emails.isFetching}
            onClick={() => void emails.refetch()}
          >
            <RotateCcw className={cn('size-4', emails.isFetching && 'animate-spin')} />
          </Button>
        </div>
      </div>
      <div className="overflow-x-auto rounded-md border bg-background">
        {emails.isError ? (
          <div className="py-14 text-center text-sm" role="alert">
            <p>Could not load incoming emails.</p>
            <Button variant="outline" className="mt-3" onClick={() => void emails.refetch()}>
              Try again
            </Button>
          </div>
        ) : emails.isLoading ? (
          <div
            role="status"
            className="flex justify-center gap-2 py-16 text-sm text-muted-foreground"
          >
            <LoaderCircle className="size-4 animate-spin" />
            Loading incoming emails…
          </div>
        ) : entries.length === 0 ? (
          <div className="flex flex-col items-center gap-2 px-4 py-16 text-center">
            <CheckCheck className="mb-1 size-7 text-muted-foreground" />
            <h2 className="text-sm font-medium">
              {query
                ? 'No matching emails'
                : page > 1
                  ? 'No more emails'
                  : filter === 'attention'
                    ? 'All caught up'
                    : 'No emails here yet'}
            </h2>
            <p className="text-sm text-muted-foreground">
              {query
                ? 'Try another sender or subject.'
                : filter === 'attention'
                  ? 'Incoming emails that need review will appear here.'
                  : 'Recovery updates will appear here.'}
            </p>
          </div>
        ) : (
          <table className="w-full min-w-[620px] text-left text-sm">
            <thead className="border-b bg-muted/20 text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-3 font-medium">Email</th>
                <th className="px-4 py-3 font-medium">Workspace</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium">Received</th>
                <th>
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {entries.map((email) => (
                <tr
                  key={email.id}
                  onClick={() => setSelected(email.id)}
                  className="cursor-pointer border-b last:border-0 hover:bg-muted/20"
                >
                  <td className="max-w-80 px-4 py-4">
                    <button
                      onClick={() => setSelected(email.id)}
                      className="block max-w-full truncate text-left font-medium"
                    >
                      {email.subject || '(No subject)'}
                    </button>
                    <p className="mt-1 truncate text-xs text-muted-foreground">
                      {email.sender_name ? `${email.sender_name} · ` : ''}
                      {email.sender_email}
                    </p>
                  </td>
                  <td className="max-w-40 truncate px-4 py-4 text-xs">
                    {email.workspace_name || (
                      <span className="text-muted-foreground">Not identified</span>
                    )}
                  </td>
                  <td className="px-4 py-4">
                    <span
                      className={cn(
                        'inline-flex items-center gap-1.5 whitespace-nowrap text-xs',
                        email.status === 'completed'
                          ? 'text-emerald-700 dark:text-emerald-400'
                          : 'text-muted-foreground',
                      )}
                    >
                      {recoveryInProgress(email) ? (
                        <LoaderCircle className="size-3 animate-spin" />
                      ) : (
                        <span
                          className={cn(
                            'size-1.5 rounded-full',
                            email.status === 'completed' ? 'bg-emerald-500' : 'bg-amber-500',
                          )}
                        />
                      )}
                      {emailStatus(email)}
                    </span>
                  </td>
                  <td className="whitespace-nowrap px-4 py-4 text-xs text-muted-foreground">
                    {emailDate(email.received_at)}
                  </td>
                  <td className="pr-4">
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label={`Review ${email.subject || 'email'}`}
                      onClick={() => setSelected(email.id)}
                    >
                      Review
                      <ChevronRight className="size-3.5" />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      {emails.data && !emails.isError && (
        <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
          <span>
            {total} {total === 1 ? 'email' : 'emails'}
          </span>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="icon"
              aria-label="Previous page"
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
            >
              <ChevronLeft className="size-4" />
            </Button>
            <span>Page {page}</span>
            <Button
              variant="ghost"
              size="icon"
              aria-label="Next page"
              disabled={page * emails.data.per_page >= total}
              onClick={() => setPage(page + 1)}
            >
              <ChevronRight className="size-4" />
            </Button>
          </div>
        </div>
      )}
      <Dialog.Root
        open={!!selected}
        onOpenChange={(open) => {
          if (!open) setSelected(null)
        }}
      >
        {selected && (
          <InboundEmailDrawer key={selected} id={selected} close={() => setSelected(null)} />
        )}
      </Dialog.Root>
    </>
  )
}
