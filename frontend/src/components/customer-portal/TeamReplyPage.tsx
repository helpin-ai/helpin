import { useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { QuietPageHeader, QuietPrimaryAction, QuietUnderlineTextarea } from '@/components/design-system/quiet'
import { Label } from '@/components/ui/label'
import { CheckmarkCircle02Icon } from '@/lib/icons'
import { queryKeys } from '@/lib/queryKeys'
import {
  SupportTeamReplyLinkError,
  supportTeamReplyService,
  type SupportTeamReplyPage,
} from '@/lib/services/supportTeamReplyService'
import { PortalFrame } from './PortalFrame'

const NOTE_MAX_LENGTH = 2000

/**
 * TeamReplyPage is where the "Ask our team to reply" link in an AI email
 * lands. Opening it changes nothing, because mail scanners open links too;
 * only the button hands the conversation to the team.
 */
export function TeamReplyPage({ token }: { token: string }) {
  const queryClient = useQueryClient()
  const [note, setNote] = useState('')
  const page = useQuery({
    queryKey: queryKeys.support.teamReply(token),
    queryFn: () => supportTeamReplyService.get(token),
    retry: (count, error) => !(error instanceof SupportTeamReplyLinkError) && count < 1,
    refetchOnWindowFocus: false,
  })
  const request = useMutation({
    mutationFn: () => supportTeamReplyService.request(token, note),
    onSuccess: (data) => queryClient.setQueryData(queryKeys.support.teamReply(token), data),
  })

  if (page.isPending) return <div className="min-h-svh bg-background" />
  if (page.isError) {
    const invalid = page.error instanceof SupportTeamReplyLinkError
    return (
      <PortalFrame name="Customer support" width="form">
        <QuietPageHeader
          title={invalid ? 'This link is not valid' : 'Something went wrong'}
          description={invalid ? 'Reply to the email and ask for a person. Our team will pick it up.' : 'Please try again, or reply to the email and ask for a person.'}
        />
        {invalid ? null : (
          <QuietPrimaryAction className="mt-6" onClick={() => void page.refetch()}>
            Try again
          </QuietPrimaryAction>
        )}
      </PortalFrame>
    )
  }

  const data = page.data
  const send = (event: FormEvent) => {
    event.preventDefault()
    if (!request.isPending) request.mutate()
  }
  return (
    <PortalFrame name={data.branding.name} logoUrl={data.branding.logo_url} color={data.branding.brand_color} width="form">
      {data.state === 'open' ? (
        <>
          <QuietPageHeader
            title="Want a person to reply?"
            description={`A person from ${data.branding.name} will read your conversation and reply by email.`}
          />
          <form className="mt-6 space-y-5" onSubmit={send}>
            {data.subject || data.customer_email ? (
              <p className="text-[12.5px] leading-[1.6] text-quiet-text-tertiary">
                {data.subject ? <span className="block text-sm font-medium text-quiet-text-primary">{data.subject}</span> : null}
                {data.customer_email ? (
                  <>
                    Reply goes to <span className="text-quiet-text-secondary">{data.customer_email}</span>
                  </>
                ) : null}
              </p>
            ) : null}
            <div className="min-w-0 space-y-1">
              <Label htmlFor="team-reply-note" className="text-sm font-medium text-quiet-text-secondary">
                Anything to add? <span className="font-normal text-quiet-muted">Optional</span>
              </Label>
              <QuietUnderlineTextarea
                id="team-reply-note"
                rows={4}
                maxLength={NOTE_MAX_LENGTH}
                className="max-h-none min-h-24"
                placeholder="For example, what didn't work"
                value={note}
                onChange={(event) => setNote(event.target.value)}
              />
            </div>
            {request.isError ? (
              <p role="alert" className="text-sm text-destructive">
                We couldn't send this. Please try again.
              </p>
            ) : null}
            <div>
              <QuietPrimaryAction type="submit" disabled={request.isPending}>
                {request.isPending ? 'Sending…' : 'Ask our team to reply'}
              </QuietPrimaryAction>
              <p className="mt-2.5 text-[12.5px] text-quiet-text-tertiary">{data.reply_time_text}</p>
            </div>
          </form>
        </>
      ) : (
        <TeamReplyOutcome page={data} />
      )}
    </PortalFrame>
  )
}

function TeamReplyOutcome({ page }: { page: SupportTeamReplyPage }) {
  if (page.state === 'done') {
    return (
      <>
        <CheckmarkCircle02Icon aria-hidden="true" className="mb-3 size-7 text-quiet-positive" />
        <QuietPageHeader title="Our team has your conversation" description={`${page.reply_time_text} You can close this page.`} />
        <p className="mt-2.5 text-[12.5px] text-quiet-text-tertiary">To add anything else, just reply to the email.</p>
      </>
    )
  }
  if (page.state === 'already') {
    return (
      <QuietPageHeader
        title="Our team is already on it"
        description={`A person from ${page.branding.name} is handling this conversation and will reply by email.`}
      />
    )
  }
  return <QuietPageHeader title="This link has expired" description="Reply to the email and ask for a person. Our team will pick it up." />
}
