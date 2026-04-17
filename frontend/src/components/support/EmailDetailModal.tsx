import { useState } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Mail01Icon, AttachmentIcon, Download04Icon, ArrowDown01Icon, ArrowUp01Icon, InformationCircleIcon } from '@/lib/icons';
import { useMessageEmailDetail } from '@/hooks/queries/useSupport';
import { getAvatarColor, getInitial } from './helpers';
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
  // "Name <email@example.com>" form
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
  const fromInitial = getInitial(from.name || from.email || '?');
  const avatarSeed = from.email || from.name || message.id;
  const body = (data?.stripped_text && data.stripped_text.trim()) || message.content || '';
  const timestamp = data?.created_at ?? message.created_at;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl gap-0 overflow-hidden p-0">
        <DialogHeader className="border-b px-6 py-4">
          <DialogTitle className="flex items-center gap-2 text-base">
            <Mail01Icon className="h-4 w-4 text-muted-foreground" />
            Email details
          </DialogTitle>
        </DialogHeader>

        <div className="max-h-[75vh] overflow-y-auto px-6 py-4">
          {isLoading && (
            <div className="space-y-3 animate-pulse">
              <div className="h-5 w-3/4 rounded bg-muted" />
              <div className="h-10 w-full rounded bg-muted" />
              <div className="h-40 w-full rounded bg-muted" />
            </div>
          )}

          {isError && (
            <div className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
              <InformationCircleIcon className="h-4 w-4" />
              Failed to load email details.
            </div>
          )}

          {!isLoading && !isError && data && (
            <>
              <h2 className="text-base font-semibold leading-snug">{subject}</h2>

              <div className="mt-3 flex items-start gap-3">
                <div className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-xs font-semibold ${getAvatarColor(avatarSeed)}`}>
                  {fromInitial}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-baseline gap-x-2 text-sm">
                    <span className="font-medium">{from.name || from.email}</span>
                    {from.name && from.email ? (
                      <span className="text-muted-foreground">&lt;{from.email}&gt;</span>
                    ) : null}
                  </div>
                  {to.email ? (
                    <div className="text-xs text-muted-foreground">
                      to {to.email}
                    </div>
                  ) : null}
                </div>
              </div>

              <div className="mt-2 text-xs text-muted-foreground">
                {formatFullTimestamp(timestamp)}
              </div>

              <div className="mt-4 border-t pt-4">
                <div className="whitespace-pre-wrap text-sm leading-relaxed text-foreground [overflow-wrap:anywhere]">
                  {body || <span className="italic text-muted-foreground">No message body.</span>}
                </div>
              </div>

              {attachments.length > 0 && (
                <div className="mt-5 border-t pt-4">
                  <div className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Attachments ({attachments.length})
                  </div>
                  <div className="space-y-1.5">
                    {attachments.map((att) => (
                      <a
                        key={att.id}
                        href={att.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center gap-2 rounded-lg border px-3 py-2 text-xs transition-colors hover:bg-muted/50"
                      >
                        <AttachmentIcon className="h-3.5 w-3.5 shrink-0 opacity-60" />
                        <span className="truncate font-medium">{att.file_name}</span>
                        <span className="shrink-0 text-muted-foreground">{formatFileSize(att.file_size)}</span>
                        <Download04Icon className="ml-auto h-3.5 w-3.5 shrink-0 opacity-60" />
                      </a>
                    ))}
                  </div>
                </div>
              )}

              <div className="mt-5 border-t pt-3">
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
            </>
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
