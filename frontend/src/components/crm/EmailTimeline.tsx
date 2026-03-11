import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { differenceInCalendarDays, format, isToday, isYesterday } from 'date-fns';
import {
  ArrowDownLeft,
  ArrowUpRight,
  Forward,
  Mail,
  Reply,
  ReplyAll,
  Trash2,
} from 'lucide-react';
// ArrowDownLeft/ArrowUpRight kept for email detail dialog direction badge
import { useContactEmails, useDealEmails, useEmailAccounts } from '@/hooks/queries/useCRM';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { cn } from '@/lib/utils';
import type { CRMEmailMessage } from '@/lib/crmTypes';

interface EmailTimelineProps {
  workspaceId: string;
  contactId?: string;
  dealId?: string;
}

type EmailBucket = 'Today' | 'Yesterday' | 'Last Week' | 'Older';

interface AddressParts {
  primary: string;
  secondary?: string;
}

const bucketOrder: EmailBucket[] = ['Today', 'Yesterday', 'Last Week', 'Older'];

function humanizeLocalPart(email: string) {
  const localPart = email.split('@')[0] ?? email;
  return localPart
    .replace(/[._-]+/g, ' ')
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function parseAddress(value: string, fallbackName?: string): AddressParts {
  const trimmed = value.trim();
  const match = trimmed.match(/^(?:"?([^"]*)"?\s*)?<([^>]+)>$/);
  if (match) {
    const name = match[1]?.trim();
    const email = match[2]?.trim();
    if (email) {
      return {
        primary: name || fallbackName || humanizeLocalPart(email),
        secondary: email,
      };
    }
  }

  if (fallbackName && trimmed.includes('@')) {
    return { primary: fallbackName, secondary: trimmed };
  }

  if (trimmed.includes('@')) {
    return {
      primary: humanizeLocalPart(trimmed),
      secondary: trimmed,
    };
  }

  return { primary: trimmed || fallbackName || 'Unknown sender' };
}

function getMessageText(message: CRMEmailMessage) {
  if (message.body_text?.trim()) {
    return message.body_text.trim();
  }

  if (!message.body_html?.trim()) {
    return '';
  }

  if (typeof window === 'undefined') {
    return message.body_html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
  }

  const doc = new window.DOMParser().parseFromString(message.body_html, 'text/html');
  return doc.body.textContent?.replace(/\u00a0/g, ' ').trim() ?? '';
}

function getPreview(message: CRMEmailMessage) {
  return getMessageText(message).replace(/\s+/g, ' ').trim();
}

function splitQuotedReply(text: string) {
  const normalized = text.replace(/\r\n/g, '\n').trim();
  if (!normalized) {
    return { mainBody: '', quotedBody: '' };
  }

  const markers = [
    /^On .+wrote:$/m,
    /^From:\s.+$/m,
    /^-{2,}\s*Original Message\s*-{2,}$/m,
  ];

  for (const marker of markers) {
    const match = normalized.match(marker);
    if (!match?.index) {
      continue;
    }

    return {
      mainBody: normalized.slice(0, match.index).trim(),
      quotedBody: normalized.slice(match.index).trim(),
    };
  }

  return { mainBody: normalized, quotedBody: '' };
}

function getBucket(sentAt: string): EmailBucket {
  const date = new Date(sentAt);
  if (isToday(date)) {
    return 'Today';
  }
  if (isYesterday(date)) {
    return 'Yesterday';
  }
  if (differenceInCalendarDays(new Date(), date) <= 7) {
    return 'Last Week';
  }
  return 'Older';
}

function formatListTimestamp(sentAt: string) {
  return format(new Date(sentAt), 'MMM d, yyyy, h:mm a');
}

function formatDetailTimestamp(sentAt: string) {
  return format(new Date(sentAt), "EEE, MMM d, yyyy 'at' h:mm a");
}

function EmailHeaderLine({ label, addresses }: { label: string; addresses: AddressParts[] }) {
  if (addresses.length === 0) {
    return null;
  }

  return (
    <div className="grid grid-cols-[40px_1fr] gap-3 text-sm">
      <span className="pt-0.5 text-muted-foreground">{label}</span>
      <div className="min-w-0 text-foreground">
        {addresses.map((address, index) => (
          <span key={`${address.primary}-${address.secondary ?? index}`} className="inline">
            <span className="font-medium">{address.primary}</span>
            {address.secondary ? (
              <span className="text-muted-foreground">{` <${address.secondary}>`}</span>
            ) : null}
            {index < addresses.length - 1 ? <span className="text-muted-foreground">, </span> : null}
          </span>
        ))}
      </div>
    </div>
  );
}

function LoadingState() {
  return (
    <div>
      {Array.from({ length: 4 }).map((_, index) => (
        <div
          key={index}
          className="flex animate-pulse items-center gap-2.5 border-b border-border/60 px-5 py-2"
        >
          <div className="h-7 w-7 shrink-0 rounded-full bg-muted" />
          <div className="h-2.5 w-24 rounded bg-muted" />
          <div className="h-3 flex-1 rounded bg-muted" />
          <div className="h-3 w-32 rounded bg-muted" />
        </div>
      ))}
    </div>
  );
}

export function EmailTimeline({ workspaceId, contactId, dealId }: EmailTimelineProps) {
  const contactQuery = useContactEmails(workspaceId, contactId ?? '');
  const dealQuery = useDealEmails(workspaceId, dealId ?? '');
  const accountsQuery = useEmailAccounts(workspaceId);
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();

  const query = contactId ? contactQuery : dealQuery;
  const sourceMessages = (query.data?.data ?? []) as CRMEmailMessage[];
  const messages = [...sourceMessages].sort(
    (left, right) => new Date(right.sent_at).getTime() - new Date(left.sent_at).getTime(),
  );
  const groupedMessages = bucketOrder
    .map((bucket) => ({
      bucket,
      messages: messages.filter((message) => getBucket(message.sent_at) === bucket),
    }))
    .filter((group) => group.messages.length > 0);
  const [selectedMessage, setSelectedMessage] = useState<CRMEmailMessage | null>(null);

  const hasConnectedAccounts = (accountsQuery.data?.length ?? 0) > 0;
  const openEmailSettings = () => {
    if (!currentWorkspace?.slug) {
      return;
    }

    navigate({
      to: '/w/$slug/settings/$section',
      params: {
        slug: currentWorkspace.slug,
        section: 'crm-email',
      },
    });
  };

  if (query.isLoading) {
    return <LoadingState />;
  }

  if (messages.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center px-6 py-14 text-center">
        <div className="rounded-full bg-muted p-3">
          <Mail className="h-7 w-7 text-muted-foreground" />
        </div>
        <p className="mt-4 text-base font-medium text-foreground">No emails tracked</p>
        <p className="mt-2 max-w-sm text-sm text-muted-foreground">
          {hasConnectedAccounts
            ? 'This contact does not have any synced messages yet.'
            : 'Connect Gmail to sync conversations, show all participants, and open full message threads here.'}
        </p>
        <Button variant="outline" className="mt-5" onClick={openEmailSettings}>
          {hasConnectedAccounts ? 'Manage email accounts' : 'Set up email sending'}
        </Button>
      </div>
    );
  }

  return (
    <>
      <div>
        {groupedMessages.map((group) => (
          <section key={group.bucket}>
            <div className="px-5 py-2 text-sm font-medium text-muted-foreground">
              {group.bucket}
            </div>
            {group.messages.map((message) => {
              const sender = parseAddress(message.from_address, message.from_name);
              const preview = getPreview(message);

              return (
                <button
                  key={message.id}
                  type="button"
                  onClick={() => setSelectedMessage(message)}
                  className="flex w-full items-center gap-2.5 border-b border-border/60 px-5 py-2 text-left transition-colors hover:bg-muted/30"
                >
                  <UserAvatar
                    name={sender.primary}
                    className="h-7 w-7 shrink-0"
                    fallbackClassName="text-[10px]"
                  />
                  <span className="w-24 shrink-0 truncate text-xs font-medium text-foreground">
                    {sender.primary}
                  </span>
                  <p className="min-w-0 flex-1 truncate text-xs">
                    <span className="text-foreground">{message.subject || '(no subject)'}</span>
                    <span className="ml-1 font-normal text-muted-foreground">{preview}</span>
                  </p>
                  <span className="shrink-0 text-[11px] text-muted-foreground whitespace-nowrap">
                    {formatListTimestamp(message.sent_at)}
                  </span>
                </button>
              );
            })}
          </section>
        ))}
      </div>

      <Dialog open={!!selectedMessage} onOpenChange={(open) => !open && setSelectedMessage(null)}>
        {selectedMessage ? (
          <DialogContent
            className="max-h-[92vh] w-[min(960px,calc(100vw-2rem))] max-w-none sm:max-w-none gap-0 overflow-hidden p-0"
          >
            {(() => {
              const fromAddress = parseAddress(selectedMessage.from_address, selectedMessage.from_name);
              const toAddresses = (selectedMessage.to_addresses ?? []).map((address) => parseAddress(address));
              const ccAddresses = (selectedMessage.cc_addresses ?? []).map((address) => parseAddress(address));
              const { mainBody, quotedBody } = splitQuotedReply(getMessageText(selectedMessage));
              const DirectionIcon = selectedMessage.direction === 'inbound' ? ArrowDownLeft : ArrowUpRight;

              return (
                <>
                  <div className="border-b border-border/60 px-6 py-5">
                    <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground">
                      <span
                        className={cn(
                          'inline-flex h-7 w-7 items-center justify-center rounded-full border',
                          selectedMessage.direction === 'inbound'
                            ? 'border-sky-200 bg-sky-50 text-sky-600'
                            : 'border-emerald-200 bg-emerald-50 text-emerald-600',
                        )}
                      >
                        <DirectionIcon className="h-4 w-4" />
                      </span>
                      {selectedMessage.direction === 'inbound' ? 'Inbound message' : 'Sent message'}
                    </div>
                    <DialogTitle className="mt-4 pr-10 text-xl font-semibold text-foreground">
                      {selectedMessage.subject || '(no subject)'}
                    </DialogTitle>
                    <p className="mt-2 text-sm text-muted-foreground">
                      {formatDetailTimestamp(selectedMessage.sent_at)}
                    </p>
                    <div className="mt-5 space-y-2 border-t border-border/60 pt-4">
                      <EmailHeaderLine label="From" addresses={[fromAddress]} />
                      <EmailHeaderLine label="To" addresses={toAddresses} />
                      <EmailHeaderLine label="CC" addresses={ccAddresses} />
                    </div>
                  </div>

                  <ScrollArea className="max-h-[58vh] bg-background">
                    <div className="space-y-8 px-6 py-6">
                      {mainBody ? (
                        <div className="whitespace-pre-wrap text-[15px] leading-7 text-foreground">
                          {mainBody}
                        </div>
                      ) : (
                        <p className="text-sm text-muted-foreground">No message body available.</p>
                      )}

                      {quotedBody ? (
                        <div className="border-l-2 border-border/80 pl-5 text-sm leading-7 text-muted-foreground">
                          <div className="whitespace-pre-wrap">{quotedBody}</div>
                        </div>
                      ) : null}
                    </div>
                  </ScrollArea>

                  <div className="flex flex-col gap-3 border-t border-border/60 bg-muted/20 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
                    <div className="flex flex-wrap gap-2">
                      <Button variant="outline" size="sm" disabled>
                        <Reply className="mr-2 h-3.5 w-3.5" />
                        Reply
                      </Button>
                      <Button variant="outline" size="sm" disabled>
                        <ReplyAll className="mr-2 h-3.5 w-3.5" />
                        Reply all
                      </Button>
                      <Button variant="outline" size="sm" disabled>
                        <Forward className="mr-2 h-3.5 w-3.5" />
                        Forward
                      </Button>
                    </div>

                    <div className="flex flex-wrap items-center gap-2">
                      <Button variant="ghost" size="sm" className="text-primary hover:text-primary" onClick={openEmailSettings}>
                        Set up email sending
                      </Button>
                      <Button variant="outline" size="sm" disabled>
                        <Trash2 className="mr-2 h-3.5 w-3.5" />
                        Delete
                      </Button>
                    </div>
                  </div>
                </>
              );
            })()}
          </DialogContent>
        ) : null}
      </Dialog>
    </>
  );
}
