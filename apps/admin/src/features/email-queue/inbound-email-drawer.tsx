import { useEffect, useRef, useState } from 'react'
import {
  CheckCheck,
  ChevronDown,
  ChevronRight,
  Download,
  ExternalLink,
  LoaderCircle,
  Search,
  X,
} from 'lucide-react'
import { Dialog } from 'radix-ui'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  useInboundConversations,
  useInboundEmail,
  useRecoverInboundEmail,
} from '@/hooks/queries/useInboundEmails'
import type { InboundConversation, InboundEmailDetail } from '@/lib/inboundEmailTypes'
import { inboundEmailService } from '@/lib/services/inboundEmailService'
import { unwrap } from '@/lib/queryUtils'
import { emailDate, recoveryInProgress } from './inbound-email-utils'

export function InboundEmailDrawer({ id, close }: { id: string; close: () => void }) {
  const email = useInboundEmail(id)
  const closeButton = useRef<HTMLButtonElement>(null)
  return (
    <Dialog.Portal>
      <Dialog.Overlay className="fixed inset-0 z-40 bg-black/30" />
      <Dialog.Content
        className="fixed inset-y-0 right-0 z-50 flex w-full max-w-[560px] flex-col border-l bg-background shadow-xl outline-none"
        onOpenAutoFocus={(event) => {
          event.preventDefault()
          closeButton.current?.focus()
        }}
      >
        <header className="flex items-start justify-between border-b px-6 py-5">
          <div>
            <Dialog.Title className="text-base font-semibold">
              {email.data?.status === 'completed' ? 'Recovered email' : 'Recover email'}
            </Dialog.Title>
            <Dialog.Description className="mt-1 text-xs text-muted-foreground">
              {email.data
                ? `Received ${emailDate(email.data.received_at)}`
                : 'Review this incoming email.'}
            </Dialog.Description>
          </div>
          <Dialog.Close asChild>
            <Button ref={closeButton} variant="ghost" size="icon-sm" aria-label="Close review">
              <X />
            </Button>
          </Dialog.Close>
        </header>
        {email.isLoading ? (
          <p role="status" className="px-6 py-10 text-sm text-muted-foreground">
            Loading email…
          </p>
        ) : email.data && !email.isError ? (
          <RecoveryReview email={email.data} close={close} />
        ) : (
          <div role="alert" className="space-y-4 px-6 py-10 text-sm">
            <p>{email.error?.message || 'Could not load this email.'}</p>
            <Button variant="outline" onClick={() => void email.refetch()}>
              Try again
            </Button>
          </div>
        )}
      </Dialog.Content>
    </Dialog.Portal>
  )
}

function RecoveryReview({ email, close }: { email: InboundEmailDetail; close: () => void }) {
  const [selected, setSelected] = useState<InboundConversation | null>(null)
  const [search, setSearch] = useState('')
  const [query, setQuery] = useState('')
  const [downloading, setDownloading] = useState<string | null>(null)
  const recovery = useRecoverInboundEmail(email.id)
  const completed = email.status === 'completed'
  const processing = recovery.isPending || recoveryInProgress(email)
  const destination = email.destination ?? selected
  const candidates = useInboundConversations(
    email.id,
    query,
    email.requires_destination && !selected && !processing && !completed,
  )
  useEffect(() => {
    const timer = setTimeout(() => setQuery(search.trim()), 250)
    return () => clearTimeout(timer)
  }, [search])

  async function download(attachment: InboundEmailDetail['attachments'][number]) {
    setDownloading(attachment.id)
    try {
      const blob = unwrap(await inboundEmailService.attachment(email.id, attachment.id))
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = attachment.name || 'attachment'
      link.click()
      setTimeout(() => URL.revokeObjectURL(url), 1_000)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not download this attachment.')
    } finally {
      setDownloading(null)
    }
  }

  return (
    <>
      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-6">
        {completed ? (
          <section aria-live="polite" className="pb-6">
            <div className="flex items-center gap-2">
              <CheckCheck className="size-5 text-emerald-600 dark:text-emerald-400" />
              <h2 className="text-base font-medium">Email recovered</h2>
            </div>
            {destination && (
              <>
                <p className="mt-2 text-sm text-muted-foreground">
                  Added to {destination.workspace_name} · #{destination.display_id}.
                </p>
                <DestinationCard destination={destination} />
              </>
            )}
            <p className="mt-3 text-xs text-muted-foreground">
              {email.recovery_requested_at
                ? `Requested by ${email.recovery_actor_name || 'a Helpin admin'} · ${emailDate(email.recovery_requested_at)}`
                : 'Recovered automatically.'}
            </p>
            {destination?.workspace_slug && (
              <Button variant="link" className="mt-2 px-0" asChild>
                <a
                  href={`${(import.meta.env.VITE_MAIN_APP_URL || 'https://app.helpin.ai').replace(/\/$/, '')}/w/${encodeURIComponent(destination.workspace_slug)}/support/${encodeURIComponent(destination.id)}`}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Open conversation
                  <ExternalLink className="size-3.5" />
                </a>
              </Button>
            )}
          </section>
        ) : (
          <section className="pb-6">
            <h2 className="text-base font-medium">
              {processing
                ? 'Processing email'
                : !email.requires_destination
                  ? 'Retry this email'
                  : destination
                    ? 'Confirm the destination'
                    : 'Choose a conversation'}
            </h2>
            <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
              {processing
                ? 'You can leave this open or check back in the queue.'
                : !email.requires_destination
                  ? email.status === 'pending'
                    ? 'An automatic retry is scheduled. You can retry now.'
                    : 'The destination is known. Retry adding the email.'
                  : destination
                    ? 'Check the destination, then add this email.'
                    : 'This email hasn’t reached an inbox. Choose its original thread.'}
            </p>
            {processing && (
              <p role="status" className="mt-5 flex items-center gap-2 text-sm">
                <LoaderCircle className="size-4 animate-spin" />
                Waiting for processing to finish…
              </p>
            )}
            {destination ? (
              <DestinationCard
                destination={destination}
                change={
                  email.requires_destination && !processing
                    ? () => {
                        setSelected(null)
                        recovery.reset()
                      }
                    : undefined
                }
              />
            ) : !processing && email.requires_destination ? (
              <>
                <div className="relative mt-5">
                  <Search className="pointer-events-none absolute left-3 top-2.5 size-4 text-muted-foreground" />
                  <Input
                    aria-label="Find conversation"
                    placeholder="Search or paste a conversation link…"
                    value={search}
                    onChange={(event) => setSearch(event.target.value)}
                    className="pl-9"
                  />
                </div>
                <p className="mb-2 mt-4 text-xs text-muted-foreground">
                  Conversations with this sender
                </p>
                {candidates.isError ? (
                  <div role="alert" className="py-3 text-sm">
                    <p>Could not find conversations.</p>
                    <Button variant="link" onClick={() => void candidates.refetch()}>
                      Try again
                    </Button>
                  </div>
                ) : candidates.isLoading || search.trim() !== query ? (
                  <p className="py-4 text-sm text-muted-foreground">Finding conversations…</p>
                ) : candidates.data?.length ? (
                  <div className="divide-y rounded-md border">
                    {candidates.data.map((candidate) => (
                      <button
                        key={candidate.id}
                        onClick={() => {
                          setSelected(candidate)
                          recovery.reset()
                        }}
                        className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left hover:bg-muted/30 focus-visible:bg-muted/30"
                      >
                        <span className="min-w-0">
                          <span className="block truncate text-sm font-medium">
                            {candidate.subject || '(No subject)'}
                          </span>
                          <span className="mt-1 block truncate text-xs text-muted-foreground">
                            {candidate.workspace_name} · {candidate.inbox_name} · #
                            {candidate.display_id}
                          </span>
                        </span>
                        <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
                      </button>
                    ))}
                  </div>
                ) : (
                  <p className="py-3 text-sm text-muted-foreground">
                    No matching conversations. Try a conversation link.
                  </p>
                )}
              </>
            ) : (
              !processing && (
                <p className="mt-4 text-sm">
                  {email.workspace_name || 'Known workspace'} · Original email route
                </p>
              )
            )}
            {email.status === 'failed' && email.recovery_requested_at && (
              <p role="alert" className="mt-4 text-sm text-destructive">
                Recovery failed. The email is saved for another attempt.
              </p>
            )}
            {recovery.isError && (
              <p role="alert" className="mt-4 text-sm text-destructive">
                {recovery.error.message}
              </p>
            )}
            <div className="mt-5 flex justify-end gap-2">
              <Button variant="ghost" onClick={close}>
                {processing ? 'Close' : 'Keep for later'}
              </Button>
              <Button
                disabled={processing || (email.requires_destination && !destination)}
                onClick={() =>
                  recovery.mutate(email.requires_destination ? selected?.id : undefined)
                }
              >
                {processing && <LoaderCircle className="animate-spin" />}
                {processing
                  ? 'Processing…'
                  : email.requires_destination
                    ? 'Add to conversation'
                    : 'Retry processing'}
              </Button>
            </div>
          </section>
        )}

        <details className="group border-y py-5">
          <summary className="flex cursor-pointer list-none items-center justify-between gap-4">
            <span className="min-w-0">
              <span className="block text-xs text-muted-foreground">Original email</span>
              <strong className="mt-2 block truncate text-sm font-medium">
                {email.subject || '(No subject)'}
              </strong>
              <span className="mt-1 block truncate text-xs text-muted-foreground">
                {email.sender_name ? `${email.sender_name} · ` : ''}
                {email.sender_email}
              </span>
            </span>
            <ChevronDown className="size-4 shrink-0 text-muted-foreground group-open:rotate-180" />
          </summary>
          <div className="pt-5">
            <p className="mb-4 break-all text-xs text-muted-foreground">To: {email.recipient}</p>
            <div className="whitespace-pre-wrap break-words text-sm leading-relaxed">
              {email.body || '(No text content)'}
            </div>
            {!!email.attachments.length && (
              <div className="mt-5 space-y-2">
                {email.attachments.map((attachment) => (
                  <button
                    key={attachment.id}
                    disabled={!!downloading}
                    onClick={() => void download(attachment)}
                    className="flex w-full items-center gap-3 rounded-md border px-3 py-2 text-left hover:bg-muted/30 disabled:opacity-50"
                  >
                    {downloading === attachment.id ? (
                      <LoaderCircle className="size-4 shrink-0 animate-spin" />
                    ) : (
                      <Download className="size-4 shrink-0 text-muted-foreground" />
                    )}
                    <span className="min-w-0">
                      <span className="block truncate text-sm">{attachment.name}</span>
                      <span className="text-xs text-muted-foreground">
                        {Math.max(1, Math.round(attachment.size / 1024))} KB
                      </span>
                    </span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </details>
        <details className="group py-5 text-xs text-muted-foreground">
          <summary className="flex cursor-pointer list-none items-center gap-1.5">
            <ChevronRight className="size-3.5 group-open:rotate-90" />
            Technical details
          </summary>
          <dl className="mt-4 space-y-3">
            {[
              ['Receipt ID', email.id],
              ['Provider message ID', email.provider_id],
              ['Reply reference', email.in_reply_to || 'Missing'],
              ['Processing', `${email.status} · ${email.attempts} attempts`],
              ['Last error', email.last_error || 'None'],
            ].map(([label, value]) => (
              <div key={label} className="grid grid-cols-[120px_1fr] gap-3">
                <dt>{label}</dt>
                <dd className="min-w-0 break-all text-foreground">{value}</dd>
              </div>
            ))}
          </dl>
          <Button
            variant="link"
            size="sm"
            className="mt-3 px-0"
            onClick={async () => {
              try {
                await navigator.clipboard.writeText(
                  `Receipt: ${email.id}\nProvider message: ${email.provider_id}\nStatus: ${email.status}\nAttempts: ${email.attempts}\nLast error: ${email.last_error || 'None'}`,
                )
                toast.success('Diagnostics copied')
              } catch {
                toast.error('Could not copy. Select the receipt ID to copy it manually.')
              }
            }}
          >
            Copy diagnostics
          </Button>
        </details>
      </div>
      {completed && (
        <footer className="flex justify-end border-t px-6 py-4">
          <Button onClick={close}>Done</Button>
        </footer>
      )}
    </>
  )
}

function DestinationCard({
  destination,
  change,
}: {
  destination: InboundConversation
  change?: () => void
}) {
  return (
    <div className="mt-5 rounded-md border px-4 py-4">
      <div className="flex items-center justify-between gap-2">
        <p className="min-w-0 truncate text-xs text-muted-foreground">
          {destination.workspace_name} · {destination.inbox_name}
        </p>
        {change && (
          <button onClick={change} className="text-xs text-primary hover:underline">
            Change
          </button>
        )}
      </div>
      <h3 className="mt-3 break-words text-sm font-medium">
        {destination.subject || '(No subject)'}{' '}
        <span className="font-normal text-muted-foreground">#{destination.display_id}</span>
      </h3>
      <p className="mt-1 break-words text-xs text-muted-foreground">
        {destination.customer_name ? `${destination.customer_name} · ` : ''}
        {destination.customer_email}
      </p>
      {destination.last_team_reply && (
        <details className="group mt-4 border-t pt-3">
          <summary className="flex cursor-pointer list-none items-center gap-1 text-xs text-muted-foreground">
            <ChevronRight className="size-3 group-open:rotate-90" />
            Last team reply
          </summary>
          <p className="mt-3 whitespace-pre-wrap break-words text-sm leading-relaxed">
            {destination.last_team_reply}
          </p>
        </details>
      )}
    </div>
  )
}
