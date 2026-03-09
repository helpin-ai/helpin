import { Mail, ArrowDownLeft, ArrowUpRight } from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';
import { useContactEmails, useDealEmails } from '@/hooks/queries/useCRM';
import type { CRMEmailMessage } from '@/lib/crmTypes';

interface EmailTimelineProps {
  workspaceId: string;
  contactId?: string;
  dealId?: string;
}

export function EmailTimeline({ workspaceId, contactId, dealId }: EmailTimelineProps) {
  const contactQuery = useContactEmails(workspaceId, contactId ?? '');
  const dealQuery = useDealEmails(workspaceId, dealId ?? '');

  const query = contactId ? contactQuery : dealQuery;
  const messages = (query.data?.data ?? []) as CRMEmailMessage[];

  if (messages.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
        <Mail className="mb-2 h-8 w-8 opacity-40" />
        <p className="text-sm">No emails tracked</p>
        <p className="mt-1 text-xs text-muted-foreground/70">Connect your email to see conversations here</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {messages.map((msg) => (
        <div key={msg.id} className="flex gap-3 rounded-md border p-3">
          <div className="mt-0.5">
            {msg.direction === 'inbound' ? (
              <ArrowDownLeft className="h-4 w-4 text-blue-500" />
            ) : (
              <ArrowUpRight className="h-4 w-4 text-green-500" />
            )}
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center justify-between">
              <p className="truncate text-sm font-medium">{msg.subject || '(no subject)'}</p>
              <span className="ml-2 shrink-0 text-xs text-muted-foreground">
                {formatDistanceToNow(new Date(msg.sent_at), { addSuffix: true })}
              </span>
            </div>
            <p className="text-xs text-muted-foreground">
              {msg.direction === 'inbound' ? 'From' : 'To'}: {msg.from_name || msg.from_address}
            </p>
            {msg.body_text && (
              <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{msg.body_text}</p>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
