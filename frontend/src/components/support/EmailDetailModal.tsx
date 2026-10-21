import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog';
import { InformationCircleIcon } from '@/lib/icons';
import { useMessageEmailDetail } from '@/hooks/queries/useSupport';
import type { SupportMessage } from '@/lib/pmTypes';
import { EmailBodyRenderer } from './EmailBodyRenderer';
import { SupportAttachmentGallery } from './SupportAttachmentGallery';

interface EmailDetailModalProps {
  workspaceId: string;
  message: SupportMessage;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function formatFullTimestamp(dateStr: string): string {
  return new Date(dateStr).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

function parseAddress(raw: string): { name: string; email: string } {
  const trimmed = (raw ?? '').trim();
  if (!trimmed) return { name: '', email: '' };
  const match = trimmed.match(/^(.*)<([^>]+)>\s*$/);
  if (match) {
    return { name: match[1].trim().replace(/^"|"$/g, ''), email: match[2].trim() };
  }
  return { name: '', email: trimmed };
}

function formatAddresses(addresses?: string[] | null): string {
  return (addresses ?? []).map((address) => address.trim()).filter(Boolean).join(', ');
}

export function EmailDetailModal({ workspaceId, message, open, onOpenChange }: EmailDetailModalProps) {
  const { data, isLoading, isError } = useMessageEmailDetail(workspaceId, open ? message.id : null, open);

  const attachments = message.attachments ?? [];
  const subject = data?.subject || '(no subject)';
  const from = parseAddress(data?.from_email ?? '');
  const replyTo = parseAddress(data?.reply_to ?? '');
  const to = parseAddress(data?.to_email ?? '');
  const cc = formatAddresses(data?.cc_emails);
  const bcc = formatAddresses(data?.bcc_emails);
  const htmlBody = (data?.html_body && data.html_body.trim()) || '';
  const textBody = (data?.stripped_text && data.stripped_text.trim()) || message.content || '';
  const timestamp = data?.created_at ?? message.created_at;
  const forwardedAttribution = data?.forwarded_attribution;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent aria-describedby={undefined} className="max-h-[85vh] max-w-3xl gap-0 overflow-hidden p-0 sm:max-w-3xl lg:max-w-4xl">
        <DialogTitle className="sr-only">{subject}</DialogTitle>

        <div className="flex max-h-[85vh] min-h-0 flex-col">
          {isLoading && (
            <div className="space-y-3 animate-pulse px-8 py-8">
              <div className="h-6 w-3/4 rounded bg-muted" />
              <div className="h-4 w-1/2 rounded bg-muted" />
              <div className="h-4 w-1/3 rounded bg-muted" />
              <div className="h-40 w-full rounded bg-muted" />
            </div>
          )}

          {isError && (
            <div className="px-8 py-8">
              <div className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
                <InformationCircleIcon className="h-4 w-4" />
                Failed to load email details.
              </div>
            </div>
          )}

          {!isLoading && !isError && data && (
            <div data-testid="email-detail-scroll" className="flex min-h-0 flex-1 flex-col overflow-y-auto px-8 py-8">
              <h2 className="pr-10 text-[17px] font-semibold leading-snug tracking-tight">{subject}</h2>

              <dl className="mt-5 grid grid-cols-[auto_1fr] gap-x-6 gap-y-1.5 text-sm">
                <dt className="text-muted-foreground">From</dt>
                <dd className="min-w-0 [overflow-wrap:anywhere]">
                  {from.name ? (
                    <>
                      <span className="font-medium text-foreground">{from.name}</span>
                      {from.email ? <span className="ml-1 text-muted-foreground">&lt;{from.email}&gt;</span> : null}
                    </>
                  ) : (
                    <span className="font-medium text-foreground">{from.email || '—'}</span>
                  )}
                </dd>

                {replyTo.email || replyTo.name ? (
                  <>
                    <dt className="text-muted-foreground">Reply-To</dt>
                    <dd className="min-w-0 [overflow-wrap:anywhere]">
                      {replyTo.name ? (
                        <>
                          <span className="font-medium text-foreground">{replyTo.name}</span>
                          {replyTo.email ? <span className="ml-1 text-muted-foreground">&lt;{replyTo.email}&gt;</span> : null}
                        </>
                      ) : (
                        <span className="font-medium text-foreground">{replyTo.email}</span>
                      )}
                    </dd>
                  </>
                ) : null}

                {to.email || to.name ? (
                  <>
                    <dt className="text-muted-foreground">To</dt>
                    <dd className="min-w-0 [overflow-wrap:anywhere]">
                      {to.name ? (
                        <>
                          <span className="font-medium text-foreground">{to.name}</span>
                          {to.email ? <span className="ml-1 text-muted-foreground">&lt;{to.email}&gt;</span> : null}
                        </>
                      ) : (
                        <span className="font-medium text-foreground">{to.email}</span>
                      )}
                    </dd>
                  </>
                ) : null}

                {cc ? (
                  <>
                    <dt className="text-muted-foreground">Cc</dt>
                    <dd className="min-w-0 [overflow-wrap:anywhere]">{cc}</dd>
                  </>
                ) : null}

                {bcc ? (
                  <>
                    <dt className="text-muted-foreground">Bcc</dt>
                    <dd className="min-w-0 [overflow-wrap:anywhere]">{bcc}</dd>
                  </>
                ) : null}

                <dt className="text-muted-foreground">Received at</dt>
                <dd className="text-foreground">{formatFullTimestamp(timestamp)}</dd>

                {data.delivered_at && <TechRow label="Delivered" value={formatFullTimestamp(data.delivered_at)} />}
                {data.opened_at && <TechRow label="Opened" value={formatFullTimestamp(data.opened_at)} />}
                {data.bounced_at && <TechRow label="Bounced" value={formatFullTimestamp(data.bounced_at)} />}
                {data.error_message && <TechRow label="Error" value={data.error_message} />}
                {data.rfc_message_id && <TechRow label="Message-ID" value={data.rfc_message_id} mono />}
                {data.in_reply_to && <TechRow label="In-Reply-To" value={data.in_reply_to} mono />}
              </dl>

              {forwardedAttribution && (
                <div className="mt-4 rounded-md border border-border/60 bg-muted/30 px-3 py-2.5 text-xs">
                  <div className="mb-2 font-medium text-foreground">Forwarded email</div>
                  <dl className="grid grid-cols-[7rem_1fr] gap-x-3 gap-y-1.5">
                    <dt className="text-muted-foreground">Original sender</dt>
                    <dd className="min-w-0 [overflow-wrap:anywhere]">
                      <span className="font-medium text-foreground">
                        {forwardedAttribution.original_sender_name || forwardedAttribution.original_sender_email}
                      </span>
                      {forwardedAttribution.original_sender_name ? (
                        <span className="ml-1 text-muted-foreground">&lt;{forwardedAttribution.original_sender_email}&gt;</span>
                      ) : null}
                    </dd>
                    <dt className="text-muted-foreground">Forwarded by</dt>
                    <dd className="min-w-0 [overflow-wrap:anywhere]">
                      <span className="font-medium text-foreground">
                        {forwardedAttribution.forwarded_by_name || forwardedAttribution.forwarded_by_email}
                      </span>
                      {forwardedAttribution.forwarded_by_name ? (
                        <span className="ml-1 text-muted-foreground">&lt;{forwardedAttribution.forwarded_by_email}&gt;</span>
                      ) : null}
                    </dd>
                  </dl>
                </div>
              )}

              <div className="mt-6 min-h-0 border-t border-border/60 pt-6">
                <div data-testid="email-body-scroll" className="max-h-[46vh] min-h-0 overflow-y-auto pb-8 pr-1">
                  {htmlBody ? (
                    <EmailBodyRenderer html={htmlBody} collapsedByDefault={false} constrainHeight={false} />
                  ) : textBody ? (
                    <div className="whitespace-pre-wrap text-[13.5px] leading-[1.7] text-foreground [overflow-wrap:anywhere]">
                      {textBody}
                    </div>
                  ) : (
                    <span className="italic text-muted-foreground">No message body.</span>
                  )}
                </div>
              </div>

              {attachments.length > 0 && (
                <div className="mt-6 border-t border-border/60 pt-5">
                  <div className="mb-2.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                    {attachments.length} attachment{attachments.length === 1 ? '' : 's'}
                  </div>
                  <SupportAttachmentGallery workspaceId={workspaceId} conversationId={message.conversation_id} attachments={attachments} thumbnailSize="md" />
                </div>
              )}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function TechRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className={`[overflow-wrap:anywhere] ${mono ? 'font-mono' : ''}`}>{value}</dd>
    </>
  );
}
