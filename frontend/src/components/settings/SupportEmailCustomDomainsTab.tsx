import { type FormEvent, type ReactNode, useState } from 'react';
import { Copy01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import {
  useActivateSupportEmailSenderDomain,
  useCreateSupportEmailSenderDomain,
  useDeactivateSupportEmailSenderDomain,
  useSupportEmailSenderDomains,
  useVerifySupportEmailSenderDomain,
} from '@/hooks/queries/useSupport';
import type { SupportEmailSenderDomain } from '@/lib/pmTypes';

export function SupportEmailCustomDomainsTab({ workspaceId }: { workspaceId: string }) {
  const [senderLocalPart, setSenderLocalPart] = useState('support');
  const [senderDomain, setSenderDomain] = useState('');
  const { data: senderDomains = [], isLoading } = useSupportEmailSenderDomains(workspaceId);
  const createSenderDomain = useCreateSupportEmailSenderDomain(workspaceId);
  const verifySenderDomain = useVerifySupportEmailSenderDomain(workspaceId);
  const activateSenderDomain = useActivateSupportEmailSenderDomain(workspaceId);
  const deactivateSenderDomain = useDeactivateSupportEmailSenderDomain(workspaceId);

  const busy = createSenderDomain.isPending || verifySenderDomain.isPending || activateSenderDomain.isPending || deactivateSenderDomain.isPending;

  const handleCopy = async (value: string, label = 'Value') => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(`${label} copied`);
    } catch {
      toast.error(`Could not copy ${label.toLowerCase()}`);
    }
  };

  const addSenderDomain = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const domain = senderDomain.trim();
    if (!domain) {
      toast.error('Enter a sender domain');
      return;
    }
    await createSenderDomain.mutateAsync({ domain, from_local_part: senderLocalPart.trim() || 'support' });
    setSenderDomain('');
    toast.success('Sender domain added');
  };

  const verifyDomain = async (domain: SupportEmailSenderDomain) => {
    await verifySenderDomain.mutateAsync(domain.id);
    toast.success('DNS verification refreshed');
  };

  const activateDomain = async (domain: SupportEmailSenderDomain) => {
    await activateSenderDomain.mutateAsync(domain.id);
    toast.success('Sender domain activated');
  };

  const deactivateDomain = async (domain: SupportEmailSenderDomain) => {
    await deactivateSenderDomain.mutateAsync(domain.id);
    toast.success('Sender domain deactivated');
  };

  const activeDomain = senderDomains.find((item) => item.active) ?? null;
  const previewDomain = senderDomain.trim() || 'example.com';
  const previewLocalPart = senderLocalPart.trim() || 'support';

  return (
    <Card>
      <CardContent className="space-y-4 pt-6">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-sm text-muted-foreground">
            {activeDomain ? `Replies currently send from ${activeDomain.from_local_part}@${activeDomain.domain}.` : 'Replies use the verified Helpin sender until a custom domain is active.'}
          </p>
          {activeDomain && <StatusBadge tone="success">Active</StatusBadge>}
        </div>

        <form onSubmit={addSenderDomain} className="rounded-lg border bg-muted/20 p-3">
          <div className="grid gap-2 md:grid-cols-[140px_1fr_auto]">
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">Mailbox</label>
              <Input value={senderLocalPart} onChange={(event) => setSenderLocalPart(event.target.value)} placeholder="support" disabled={busy} />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">Domain</label>
              <Input value={senderDomain} onChange={(event) => setSenderDomain(event.target.value)} placeholder="example.com" disabled={busy} />
            </div>
            <div className="flex items-end">
              <Button type="submit" disabled={busy} className="w-full md:w-auto">
                Add domain
              </Button>
            </div>
          </div>
          <p className="mt-2 text-xs text-muted-foreground">
            Preview: <code className="rounded bg-background px-1 py-0.5">{previewLocalPart}@{previewDomain}</code>
          </p>
        </form>

        {isLoading && <p className="text-sm text-muted-foreground">Loading custom domains...</p>}

        {!isLoading && senderDomains.length === 0 && (
          <div className="rounded-lg border border-dashed py-5 text-center text-sm text-muted-foreground">
            Add a domain to get DKIM TXT and Return-Path CNAME records.
          </div>
        )}

        {!isLoading && senderDomains.length > 0 && (
          <div className="space-y-3">
            {senderDomains.map((item) => (
              <SenderDomainRow
                key={item.id}
                domain={item}
                busy={busy}
                onCopy={handleCopy}
                onVerify={() => verifyDomain(item)}
                onActivate={() => activateDomain(item)}
                onDeactivate={() => deactivateDomain(item)}
              />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function SenderDomainRow({
  domain,
  busy,
  onCopy,
  onVerify,
  onActivate,
  onDeactivate,
}: {
  domain: SupportEmailSenderDomain;
  busy: boolean;
  onCopy: (value: string, label?: string) => void | Promise<void>;
  onVerify: () => void | Promise<void>;
  onActivate: () => void | Promise<void>;
  onDeactivate: () => void | Promise<void>;
}) {
  const fromAddress = `${domain.from_local_part}@${domain.domain}`;
  const canActivate = domain.dkim_verified && domain.return_path_domain_verified && !domain.active;
  const dkimHost = domain.dkim_pending_host || domain.dkim_host;
  const dkimValue = domain.dkim_pending_text_value || domain.dkim_text_value;
  const lastChecked = domain.last_checked_at ? new Date(domain.last_checked_at).toLocaleString() : null;

  return (
    <div className="rounded-lg border p-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <code className="rounded bg-muted px-2 py-0.5 text-sm">{fromAddress}</code>
            {domain.active && <StatusBadge tone="success">Active</StatusBadge>}
            {!domain.active && domain.status === 'verified' && <StatusBadge tone="success">Verified</StatusBadge>}
            {domain.status !== 'verified' && <StatusBadge tone="warning">DNS pending</StatusBadge>}
          </div>
          <div className="mt-2 flex flex-wrap gap-2 text-xs text-muted-foreground">
            <span>DKIM: {domain.dkim_verified ? 'verified' : 'pending'}</span>
            <span>Return-Path: {domain.return_path_domain_verified ? 'verified' : 'pending'}</span>
            {lastChecked && <span>Checked: {lastChecked}</span>}
          </div>
          {domain.last_error && <p className="mt-2 text-xs text-destructive">{domain.last_error}</p>}
        </div>
        <div className="flex shrink-0 flex-wrap gap-2">
          <Button size="sm" variant="outline" onClick={onVerify} disabled={busy}>Verify DNS</Button>
          {canActivate && <Button size="sm" onClick={onActivate} disabled={busy}>Activate</Button>}
          {domain.active && <Button size="sm" variant="outline" onClick={onDeactivate} disabled={busy}>Deactivate</Button>}
        </div>
      </div>

      <div className="mt-3 divide-y rounded-md border bg-muted/20">
        <DNSRecordRow type="TXT" host={dkimHost} value={dkimValue} onCopy={onCopy} />
        <DNSRecordRow type="CNAME" host={domain.return_path_domain} value={domain.return_path_domain_cname_value} onCopy={onCopy} />
      </div>
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
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onCopy(host, `${type} host`)} disabled={!host}>
              <Copy01Icon className="h-3.5 w-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Copy host</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onCopy(value, `${type} value`)} disabled={!value}>
              <Copy01Icon className="h-3.5 w-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Copy value</TooltipContent>
        </Tooltip>
      </div>
    </div>
  );
}

function StatusBadge({ tone, children }: { tone: 'success' | 'warning'; children: ReactNode }) {
  const className = tone === 'success'
    ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-950/40 dark:text-emerald-300'
    : 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-300';
  return <Badge variant="outline" className={className}>{children}</Badge>;
}
