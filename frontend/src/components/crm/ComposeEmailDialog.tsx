import { useMemo, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { Loading01Icon, Mail01Icon } from '@/lib/icons';
import type { CRMEmailAccount } from '@/lib/crmTypes';
import { crmEmailService } from '@/lib/services/crmService';
import { unwrap } from '@/lib/queryUtils';

export interface EmailDraft {
  title?: string;
  to?: string[];
  cc?: string[];
  subject?: string;
  body?: string;
}

interface ComposeEmailDialogProps {
  workspaceId: string;
  accounts: CRMEmailAccount[];
  open: boolean;
  draft?: EmailDraft;
  onOpenChange: (open: boolean) => void;
}

function parseRecipients(value: string) {
  return value.split(/[;,\n]/).map((item) => item.trim()).filter(Boolean);
}

function bodyToHTML(body: string) {
  const escaped = body
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
  return escaped.replace(/\n/g, '<br>');
}

export function ComposeEmailDialog({ workspaceId, accounts, open, draft, onOpenChange }: ComposeEmailDialogProps) {
  const queryClient = useQueryClient();
  const availableAccounts = useMemo(
    () => accounts.filter((account) => account.is_active && account.status !== 'pending_oauth' && account.status !== 'disconnected'),
    [accounts],
  );
  const [accountId, setAccountId] = useState(availableAccounts[0]?.id ?? '');
  const [to, setTo] = useState(draft?.to?.join(', ') ?? '');
  const [cc, setCC] = useState(draft?.cc?.join(', ') ?? '');
  const [subject, setSubject] = useState(draft?.subject ?? '');
  const [body, setBody] = useState(draft?.body ?? '');
  const [sending, setSending] = useState(false);

  const send = async () => {
    const recipients = parseRecipients(to);
    if (!accountId || recipients.length === 0 || !body.trim()) {
      toast.error('Choose an account and add a recipient and message');
      return;
    }
    setSending(true);
    try {
      await unwrap(await crmEmailService.sendEmail(workspaceId, {
        account_id: accountId,
        to: recipients,
        cc: parseRecipients(cc),
        subject: subject.trim(),
        body_html: bodyToHTML(body.trim()),
      }));
      await queryClient.invalidateQueries({ queryKey: ['crm', workspaceId] });
      toast.success('Email sent');
      onOpenChange(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Email could not be sent');
    } finally {
      setSending(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !sending && onOpenChange(nextOpen)}>
      <DialogContent className="max-w-2xl gap-0 overflow-hidden p-0">
        <DialogHeader className="border-b border-border/60 px-6 py-5 text-left">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Mail01Icon className="h-5 w-5" />
            </div>
            <div>
              <DialogTitle>{draft?.title ?? 'New email'}</DialogTitle>
              <DialogDescription className="mt-1">Send from a connected Google account.</DialogDescription>
            </div>
          </div>
        </DialogHeader>
        <div className="space-y-4 px-6 py-5">
          <div className="space-y-2">
            <Label>From</Label>
            <Select value={accountId} onValueChange={setAccountId} disabled={sending}>
              <SelectTrigger><SelectValue placeholder="Choose a Google account" /></SelectTrigger>
              <SelectContent>
                {availableAccounts.map((account) => <SelectItem key={account.id} value={account.id}>{account.email_address}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="crm-email-to">To</Label>
            <Input id="crm-email-to" value={to} onChange={(event) => setTo(event.target.value)} placeholder="name@company.com" disabled={sending} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="crm-email-cc">CC <span className="font-normal text-muted-foreground">(optional)</span></Label>
            <Input id="crm-email-cc" value={cc} onChange={(event) => setCC(event.target.value)} placeholder="Separate addresses with commas" disabled={sending} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="crm-email-subject">Subject</Label>
            <Input id="crm-email-subject" value={subject} onChange={(event) => setSubject(event.target.value)} disabled={sending} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="crm-email-body">Message</Label>
            <Textarea id="crm-email-body" value={body} onChange={(event) => setBody(event.target.value)} className="min-h-52 resize-y" disabled={sending} autoFocus />
          </div>
        </div>
        <DialogFooter className="border-t border-border/60 bg-muted/20 px-6 py-4">
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={sending}>Cancel</Button>
          <Button onClick={send} disabled={sending || availableAccounts.length === 0}>
            {sending && <Loading01Icon className="mr-2 h-4 w-4 animate-spin" />}
            Send email
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
