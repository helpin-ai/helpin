import { useState } from 'react';
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog';
import { AttachmentIcon, Download04Icon, ArrowDown01Icon, ArrowUp01Icon, InformationCircleIcon } from '@/lib/icons';
import { useMessageEmailDetail } from '@/hooks/queries/useSupport';
import type { SupportMessage } from '@/lib/pmTypes';

interface EmailDetailModalProps {
  workspaceId: string;
  message: SupportMessage;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
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

export function EmailDetailModal({ workspaceId, message, open, onOpenChange }: EmailDetailModalProps) {
  const { data, isLoading, isError } = useMessageEmailDetail(workspaceId, open ? message.id : null, open);
  const [showTech, setShowTech] = useState(false);

  const attachments = message.attachments ?? [];
  const subject = data?.subject || '(no subject)';
  const from = parseAddress(data?.from_email ?? '');
  const to = parseAddress(data?.to_email ?? '');
  const body = (data?.stripped_text && data.stripped_text.trim()) || message.content || '';
  const timestamp = data?.created_at ?? message.created_at;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl gap-0 overflow-hidden p-0 sm:max-w-3xl lg:max-w-4xl">
        <DialogTitle className="sr-only">{subject}</DialogTitle>

        <div className="max-h-[80vh] overflow-y-auto">
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
            <div className="px-8 py-8">
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

                <dt className="text-muted-foreground">Date</dt>
                <dd className="text-foreground">{formatFullTimestamp(timestamp)}</dd>
              </dl>

              <div className="mt-6 border-t border-border/60 pt-6">
                <div className="whitespace-pre-wrap text-[13.5px] leading-[1.7] text-foreground [overflow-wrap:anywhere]">
                  {body || <span className="italic text-muted-foreground">No message body.</span>}
                </div>
              </div>

              {attachments.length > 0 && (
                <div className="mt-6 border-t border-border/60 pt-5">
                  <div className="mb-2.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                    {attachments.length} attachment{attachments.length === 1 ? '' : 's'}
                  </div>
                  <div className="grid gap-1.5 sm:grid-cols-2">
                    {attachments.map((att) => (
                      <a
                        key={att.id}
                        href={att.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="group flex items-center gap-2 rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-xs transition-colors hover:border-border hover:bg-muted/60"
                      >
                        <AttachmentIcon className="h-3.5 w-3.5 shrink-0 opacity-60" />
                        <span className="min-w-0 flex-1 truncate font-medium">{att.file_name}</span>
                        <span className="shrink-0 text-muted-foreground">{formatFileSize(att.file_size)}</span>
                        <Download04Icon className="h-3.5 w-3.5 shrink-0 opacity-60 transition-opacity group-hover:opacity-100" />
                      </a>
                    ))}
                  </div>
                </div>
              )}

              <div className="mt-6 border-t border-border/60 pt-4">
                <button
                  type="button"
                  onClick={() => setShowTech(!showTech)}
                  className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
                >
                  {showTech ? <ArrowUp01Icon className="h-3 w-3" /> : <ArrowDown01Icon className="h-3 w-3" />}
                  {showTech ? 'Hide technical details' : 'Show technical details'}
                </button>

                {showTech && (
                  <dl className="mt-3 space-y-1.5 rounded-md bg-muted/40 p-3 text-xs">
                    <TechRow label="Direction" value={data.direction} />
                    <TechRow label="Status" value={data.status} />
                    {data.opened_at && <TechRow label="Opened" value={formatFullTimestamp(data.opened_at)} />}
                    {data.rfc_message_id && <TechRow label="Message-ID" value={data.rfc_message_id} mono />}
                    {data.in_reply_to && <TechRow label="In-Reply-To" value={data.in_reply_to} mono />}
                    {data.references_header && <TechRow label="References" value={data.references_header} mono />}
                  </dl>
                )}
              </div>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function TechRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[7rem_1fr] gap-3">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className={`[overflow-wrap:anywhere] ${mono ? 'font-mono' : ''}`}>{value}</dd>
    </div>
  );
}
