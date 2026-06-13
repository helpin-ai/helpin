import { type FormEvent, type ReactNode, useState } from 'react';
import { ArrowDown01Icon, ArrowRight01Icon, Copy01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import {
  useCreateSupportEmailSender,
  useDisableSupportEmailSender,
  useSetDefaultSupportEmailSender,
  useSupportEmailSenders,
  useSupportMailboxes,
  useVerifySupportEmailSender,
} from '@/hooks/queries/useSupport';
import type { SupportEmailSender } from '@/lib/pmTypes';

export function buildSupportSenderEmailPreview({
  senderEmail,
  workspaceName,
  agentName,
  replyDomain,
}: {
  senderEmail: string;
  workspaceName?: string;
  agentName?: string;
  replyDomain?: string;
}) {
  const resolvedAgentName = agentName?.trim() || 'Agent';
  const resolvedWorkspaceName = workspaceName?.trim() || 'Workspace';
  const resolvedReplyDomain = replyDomain?.trim() || 'replies.helpin.email';

  return {
    from: `${resolvedAgentName} - ${resolvedWorkspaceName} <${senderEmail.trim()}>`,
    replyTo: `conv-{conversation_id}@${resolvedReplyDomain}`,
  };
}

export function SupportEmailSendersTab({ workspaceId }: { workspaceId: string }) {
  const [showHowItWorks, setShowHowItWorks] = useState(true);
  const [email, setEmail] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [mailboxId, setMailboxId] = useState('');
  const { data: senders = [], isLoading } = useSupportEmailSenders(workspaceId);
  const { data: mailboxes = [] } = useSupportMailboxes(workspaceId);
  const createSender = useCreateSupportEmailSender(workspaceId);
  const verifySender = useVerifySupportEmailSender(workspaceId);
  const setDefaultSender = useSetDefaultSupportEmailSender(workspaceId);
  const disableSender = useDisableSupportEmailSender(workspaceId);

  const busy = createSender.isPending || verifySender.isPending || setDefaultSender.isPending || disableSender.isPending;
  const workspaceDefault = senders.find((sender) => sender.active && sender.default_scope === 'workspace') ?? null;

  const addSender = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmedEmail = email.trim();
    if (!trimmedEmail) {
      toast.error('Enter a sender address');
      return;
    }
    await createSender.mutateAsync({
      email: trimmedEmail,
      display_name: displayName.trim(),
      mailbox_id: mailboxId || null,
    });
    setEmail('');
    setDisplayName('');
    setMailboxId('');
    toast.success('Sender address added');
  };

  const handleCopy = async (value: string, label = 'Value') => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(`${label} copied`);
    } catch {
      toast.error(`Could not copy ${label.toLowerCase()}`);
    }
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardContent className="pt-6">
          <button
            type="button"
            onClick={() => setShowHowItWorks((value) => !value)}
            className="flex items-center gap-1 text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            {showHowItWorks ? <ArrowDown01Icon className="h-3.5 w-3.5" /> : <ArrowRight01Icon className="h-3.5 w-3.5" />}
            How it works
          </button>
          {showHowItWorks && (
            <ol className="mt-3 space-y-2 text-sm text-muted-foreground">
              <li className="flex items-start gap-2.5">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground">1</span>
                <span>Add a verified sender address for replies from Helpin.</span>
              </li>
              <li className="flex items-start gap-2.5">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground">2</span>
                <span>Choose a specific inbox for an inbox sender, or choose workspace-wide for the shared fallback sender.</span>
              </li>
              <li className="flex items-start gap-2.5">
                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-medium text-foreground">3</span>
                <span>Verify the address, then set it as the workspace default or inbox default.</span>
              </li>
            </ol>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardContent className="space-y-4 pt-6">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-sm text-muted-foreground">
            {workspaceDefault ? `Workspace replies send from ${workspaceDefault.email}.` : 'No default sender email yet. Replies will use a Helpin email address.'}
          </p>
          {workspaceDefault && <StatusBadge tone="success">Workspace default</StatusBadge>}
        </div>

        <form onSubmit={addSender} className="rounded-lg border bg-muted/20 p-3">
          <div className="grid gap-2 lg:grid-cols-[1.2fr_1fr_1fr_auto]">
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">Sender address</label>
              <Input value={email} onChange={(event) => setEmail(event.target.value)} placeholder="support@example.com" disabled={busy} />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">Display name</label>
              <Input value={displayName} onChange={(event) => setDisplayName(event.target.value)} placeholder="Support" disabled={busy} />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">Team inbox</label>
              <select
                value={mailboxId}
                onChange={(event) => setMailboxId(event.target.value)}
                disabled={busy}
                className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"
              >
                <option value="">Workspace default</option>
                {mailboxes.map((mailbox) => (
                  <option key={mailbox.id} value={mailbox.id}>{mailbox.name}</option>
                ))}
              </select>
            </div>
            <div className="flex items-end">
              <Button type="submit" disabled={busy} className="w-full lg:w-auto">
                Add sender
              </Button>
            </div>
          </div>
        </form>

        {isLoading && <p className="text-sm text-muted-foreground">Loading sender addresses...</p>}

        {!isLoading && senders.length === 0 && (
          <div className="rounded-lg border border-dashed py-5 text-center text-sm text-muted-foreground">
            Add a sender address to get DNS records and a forwarding verification address.
          </div>
        )}

        {!isLoading && senders.length > 0 && (
          <div className="space-y-3">
            {senders.map((sender) => (
              <SenderRow
                key={sender.id}
                sender={sender}
                busy={busy}
                onCopy={handleCopy}
                onVerify={async () => {
                  await verifySender.mutateAsync(sender.id);
                  toast.success('DNS verification refreshed');
                }}
                onSetWorkspaceDefault={async () => {
                  await setDefaultSender.mutateAsync({ senderId: sender.id, payload: { default_scope: 'workspace' } });
                  toast.success('Workspace sender updated');
                }}
                onSetMailboxDefault={async () => {
                  await setDefaultSender.mutateAsync({ senderId: sender.id, payload: { default_scope: 'mailbox', mailbox_id: sender.mailbox_id ?? null } });
                  toast.success('Inbox sender updated');
                }}
                onDisable={async () => {
                  await disableSender.mutateAsync(sender.id);
                  toast.success('Sender disabled');
                }}
              />
            ))}
          </div>
        )}
      </CardContent>
      </Card>
    </div>
  );
}

function SenderRow({
  sender,
  busy,
  onCopy,
  onVerify,
  onSetWorkspaceDefault,
  onSetMailboxDefault,
  onDisable,
}: {
  sender: SupportEmailSender;
  busy: boolean;
  onCopy: (value: string, label?: string) => void | Promise<void>;
  onVerify: () => void | Promise<void>;
  onSetWorkspaceDefault: () => void | Promise<void>;
  onSetMailboxDefault: () => void | Promise<void>;
  onDisable: () => void | Promise<void>;
}) {
  const verified = sender.dkim_verified && sender.return_path_domain_verified;
  const dkimHost = sender.dkim_pending_host || sender.dkim_host;
  const dkimValue = sender.dkim_pending_text_value || sender.dkim_text_value;
  const lastChecked = sender.last_checked_at ? new Date(sender.last_checked_at).toLocaleString() : null;
  const forwardingVerified = sender.forwarding_status === 'verified';
  const canUseForMailbox = Boolean(sender.mailbox_id) && forwardingVerified;
  const preview = buildSupportSenderEmailPreview({ senderEmail: sender.email });
  const selectedAsDefault = sender.active && (sender.default_scope === 'workspace' || sender.default_scope === 'mailbox');

  return (
    <div className="rounded-lg border p-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <code className="rounded bg-muted px-2 py-0.5 text-sm">{sender.email}</code>
            {sender.default_scope === 'workspace' && sender.active && <StatusBadge tone="success">Workspace default</StatusBadge>}
            {sender.default_scope === 'mailbox' && sender.active && <StatusBadge tone="success">Inbox default</StatusBadge>}
            {verified ? <StatusBadge tone="success">DNS verified</StatusBadge> : <StatusBadge tone="warning">DNS pending</StatusBadge>}
            {forwardingVerified ? <StatusBadge tone="success">Forwarding verified</StatusBadge> : <StatusBadge tone="warning">Forwarding pending</StatusBadge>}
          </div>
          <div className="mt-2 flex flex-wrap gap-2 text-xs text-muted-foreground">
            <span>DKIM: {sender.dkim_verified ? 'verified' : 'pending'}</span>
            <span>Return-Path: {sender.return_path_domain_verified ? 'verified' : 'pending'}</span>
            <span>Forwarding: {forwardingVerified ? 'verified' : 'pending'}</span>
            {sender.dmarc_policy && <span>DMARC: {sender.dmarc_policy}</span>}
            {sender.mailbox_name && <span>Inbox: {sender.mailbox_name}</span>}
            {lastChecked && <span>Checked: {lastChecked}</span>}
          </div>
          {sender.last_error && <p className="mt-2 text-xs text-destructive">{sender.last_error}</p>}
          {sender.forwarding_last_error && <p className="mt-2 text-xs text-destructive">{sender.forwarding_last_error}</p>}
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          <Button size="sm" variant="outline" onClick={onVerify} disabled={busy}>Verify DNS</Button>
          {verified && sender.default_scope !== 'workspace' && <Button size="sm" variant="outline" onClick={onSetWorkspaceDefault} disabled={busy}>Set workspace default</Button>}
          {verified && canUseForMailbox && sender.default_scope !== 'mailbox' && <Button size="sm" variant="outline" onClick={onSetMailboxDefault} disabled={busy}>Set inbox default</Button>}
          {sender.active && <Button size="sm" variant="outline" onClick={onDisable} disabled={busy}>Disable</Button>}
        </div>
      </div>

      <div className="mt-3 grid gap-2 rounded-md border bg-background p-2 text-xs md:grid-cols-[90px_1fr]">
        <span className="font-medium text-muted-foreground">From</span>
        <code className="min-w-0 truncate rounded bg-muted px-2 py-1">{preview.from}</code>
        <span className="font-medium text-muted-foreground">Reply-To</span>
        <code className="min-w-0 truncate rounded bg-muted px-2 py-1">{preview.replyTo}</code>
        {!selectedAsDefault && (
          <>
            <span className="font-medium text-muted-foreground">Fallback</span>
            <span className="text-muted-foreground">Helpin verified sender until this address is verified and selected as a default.</span>
          </>
        )}
      </div>

      <div className="mt-3 divide-y rounded-md border bg-muted/20">
        <DNSRecordRow type="TXT" host={dkimHost} value={dkimValue} onCopy={onCopy} />
        <DNSRecordRow type="CNAME" host={sender.return_path_domain} value={sender.return_path_domain_cname_value} onCopy={onCopy} />
        <ForwardingRecordRow sender={sender} onCopy={onCopy} />
      </div>
    </div>
  );
}

function ForwardingRecordRow({
  sender,
  onCopy,
}: {
  sender: SupportEmailSender;
  onCopy: (value: string, label?: string) => void | Promise<void>;
}) {
  const value = sender.forwarding_address || 'Waiting for Helpin';
  return (
    <div className="grid gap-2 p-2 text-xs md:grid-cols-[70px_1fr_1fr_auto] md:items-center">
      <Badge variant="secondary" className="w-fit rounded-md">Forward</Badge>
      <code className="min-w-0 truncate rounded bg-background px-2 py-1">{sender.email}</code>
      <code className="min-w-0 truncate rounded bg-background px-2 py-1">{value}</code>
      <CopyButton label="forwarding address" value={sender.forwarding_address} onCopy={onCopy} />
    </div>
  );
}

function DNSRecordRow({
  type,
  host,
  value,
  onCopy,
}: {
  type: 'TXT' | 'CNAME';
  host: string;
  value: string;
  onCopy: (value: string, label?: string) => void | Promise<void>;
}) {
  return (
    <div className="grid gap-2 p-2 text-xs md:grid-cols-[70px_1fr_1fr_auto] md:items-center">
      <Badge variant="secondary" className="w-fit rounded-md">{type}</Badge>
      <code className="min-w-0 truncate rounded bg-background px-2 py-1">{host || 'Waiting for Postmark'}</code>
      <code className="min-w-0 truncate rounded bg-background px-2 py-1">{value || 'Waiting for Postmark'}</code>
      <div className="flex gap-1">
        <CopyButton label={`${type} host`} value={host} onCopy={onCopy} />
        <CopyButton label={`${type} value`} value={value} onCopy={onCopy} />
      </div>
    </div>
  );
}

function CopyButton({ label, value, onCopy }: { label: string; value: string; onCopy: (value: string, label?: string) => void | Promise<void> }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onCopy(value, label)} disabled={!value}>
          <Copy01Icon className="h-3.5 w-3.5" />
        </Button>
      </TooltipTrigger>
      <TooltipContent>Copy {label.toLowerCase()}</TooltipContent>
    </Tooltip>
  );
}

function StatusBadge({ tone, children }: { tone: 'success' | 'warning'; children: ReactNode }) {
  const className = tone === 'success'
    ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-950/40 dark:text-emerald-300'
    : 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-300';
  return <Badge variant="outline" className={className}>{children}</Badge>;
}
