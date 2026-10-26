import type { ReactNode } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { useSupportEmailRoutes, useSupportEmailSenders } from '@/hooks/queries/useSupport';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { getInboxForwardingRoutes, getInboxSenderAddress, isSenderVerified } from './inboxEmailRouting';

function StatusBadge({ tone, children }: { tone: 'success' | 'warning'; children: ReactNode }) {
  const className = tone === 'success'
    ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-950/40 dark:text-emerald-300'
    : 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-300';
  return <Badge variant="outline" className={`shrink-0 ${className}`}>{children}</Badge>;
}

function EmailSetupSection({
  title,
  children,
  action,
}: {
  title: string;
  children: ReactNode;
  action?: { label: string; href: string } | null;
}) {
  return (
    <div className="rounded-lg border p-4">
      <h3 className="text-sm font-medium">{title}</h3>
      <div className="mt-1 text-sm text-muted-foreground">{children}</div>
      {action && (
        <Button asChild variant="link" size="sm" className="mt-2 h-auto px-0">
          <a href={action.href} target="_blank" rel="noreferrer">
            {action.label}
          </a>
        </Button>
      )}
    </div>
  );
}

function AddressRow({ primary, secondary, badge }: { primary: string; secondary?: string | null; badge: ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <p className="truncate text-foreground" title={primary}>{primary}</p>
        {secondary && <p className="truncate text-xs" title={secondary}>{secondary}</p>}
      </div>
      {badge}
    </div>
  );
}

// Shows the inbox's current forwarding and sender address, or the action that sets each up.
// Settings open in a new tab; the queries refetch on window focus, so status updates on return.
export function TeamInboxEmailSetup({ workspaceId, mailboxId }: { workspaceId: string; mailboxId?: string | null }) {
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug);
  const routesQuery = useSupportEmailRoutes(workspaceId);
  const sendersQuery = useSupportEmailSenders(workspaceId);
  const settingsHref = (search: string) =>
    workspaceSlug ? `/w/${workspaceSlug}/settings/inboxes-routing?${search}` : '/workspaces';

  const forwardingRoutes = getInboxForwardingRoutes(routesQuery.data ?? [], mailboxId);
  const unverifiedRoutes = forwardingRoutes.filter((route) => !route.forwarding_verified_at);
  const senderAddress = getInboxSenderAddress(sendersQuery.data ?? [], mailboxId);
  const senderVerified = senderAddress ? isSenderVerified(senderAddress.sender) : false;

  let forwarding: { body: ReactNode; action: { label: string; href: string } | null };
  if (routesQuery.isPending) {
    forwarding = { body: <p>Checking email forwarding…</p>, action: null };
  } else if (routesQuery.isError) {
    forwarding = { body: <p>Could not load email forwarding.</p>, action: { label: 'Open email settings', href: settingsHref('tab=email') } };
  } else if (forwardingRoutes.length === 0) {
    forwarding = {
      body: <p>Send messages received at your existing email address to this Helpin inbox.</p>,
      action: { label: 'Set up forwarding', href: settingsHref('tab=email') },
    };
  } else {
    forwarding = {
      body: (
        <div className="space-y-2">
          {forwardingRoutes.map((route) => (
            <AddressRow
              key={route.id}
              primary={route.source_address || route.inbound_address}
              secondary={route.source_address ? `Forwards to ${route.inbound_address}` : 'Helpin forwarding address'}
              badge={route.forwarding_verified_at
                ? <StatusBadge tone="success">Verified</StatusBadge>
                : <StatusBadge tone="warning">Setup incomplete</StatusBadge>}
            />
          ))}
          {unverifiedRoutes.length > 0 && (
            <p className="text-xs">Confirm forwarding in your email provider, then run an end-to-end test.</p>
          )}
        </div>
      ),
      action: unverifiedRoutes.length > 0
        ? { label: 'Finish setup', href: settingsHref('tab=email') }
        : { label: 'Manage forwarding', href: settingsHref('tab=email') },
    };
  }

  let sender: { body: ReactNode; action: { label: string; href: string } | null };
  if (sendersQuery.isPending) {
    sender = { body: <p>Checking sender address…</p>, action: null };
  } else if (sendersQuery.isError) {
    sender = { body: <p>Could not load sender addresses.</p>, action: { label: 'Open sender settings', href: settingsHref('tab=senders') } };
  } else if (!senderAddress) {
    sender = {
      body: <p>Choose and verify the email address customers see when your team replies.</p>,
      action: { label: 'Add sender address', href: settingsHref('tab=senders&create_sender=true') },
    };
  } else {
    const { sender: address, source } = senderAddress;
    const hasDisplayName = Boolean(address.display_name) && address.display_name !== address.email;
    sender = {
      body: (
        <div className="space-y-2">
          <AddressRow
            primary={hasDisplayName ? address.display_name : address.email}
            secondary={hasDisplayName ? address.email : null}
            badge={senderVerified
              ? <StatusBadge tone="success">Verified</StatusBadge>
              : <StatusBadge tone="warning">DNS pending</StatusBadge>}
          />
          {source === 'workspace' && (
            <p className="text-xs">Workspace default. This inbox has no sender address of its own.</p>
          )}
          {!senderVerified && (
            <p className="text-xs">Add the DNS records in sender settings to finish verification.</p>
          )}
        </div>
      ),
      action: senderVerified
        ? { label: 'Manage sender addresses', href: settingsHref('tab=senders') }
        : { label: 'Finish verification', href: settingsHref('tab=senders') },
    };
  }

  return (
    <div className="space-y-3">
      <EmailSetupSection title="Email forwarding" action={forwarding.action}>
        {forwarding.body}
      </EmailSetupSection>
      <EmailSetupSection title="Sender address" action={sender.action}>
        {sender.body}
      </EmailSetupSection>
    </div>
  );
}
