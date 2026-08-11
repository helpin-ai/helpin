import { type FormEvent, type ReactNode, useMemo, useState } from 'react';
import { ArrowLeft02Icon, Copy01Icon, InformationCircleIcon, MailAdd01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { deriveSendingDomainDefaults, groupSupportEmailSendersByDomain } from './supportEmailSenderUtils';
import { cn } from '@/lib/utils';
import {
  useCreateSupportEmailSender,
  useCreateSupportEmailSenderDomain,
  useSupportEmailSenderDomains,
  useSupportEmailSenders,
  useSupportMailboxes,
  useUpdateSupportEmailSender,
  useVerifySupportEmailSenderDomain,
} from '@/hooks/queries/useSupport';
import type { SupportEmailSender, SupportEmailSenderDomain, UpdateSupportEmailSenderRequest } from '@/lib/pmTypes';

type SenderDomainGroup = {
  domain: string;
  senderDomain: SupportEmailSenderDomain | null;
  senders: SupportEmailSender[];
};


type SenderDraft = {
  localPart: string;
  displayName: string;
  mailboxId: string;
};

type DomainDraft = {
  domain: string;
  localPart: string;
  displayName: string;
};

export function SupportEmailSendersTab({
  workspaceId,
  workspaceName,
  workspaceWebsiteUrl,
}: {
  workspaceId: string;
  workspaceName?: string;
  workspaceWebsiteUrl?: string;
}) {
  const [addDomainOpen, setAddDomainOpen] = useState(false);
  const [addSenderDomain, setAddSenderDomain] = useState<string | null>(null);
  const [domainDraft, setDomainDraft] = useState<DomainDraft>({ domain: '', localPart: '', displayName: '' });
  const [senderDrafts, setSenderDrafts] = useState<Record<string, SenderDraft>>({});
  const [selectedDomain, setSelectedDomain] = useState<string | null>(null);
  const [editingSender, setEditingSender] = useState<SupportEmailSender | null>(null);

  const { data: senderDomains = [], isLoading: domainsLoading } = useSupportEmailSenderDomains(workspaceId);
  const { data: senders = [], isLoading: sendersLoading } = useSupportEmailSenders(workspaceId);
  const { data: mailboxes = [] } = useSupportMailboxes(workspaceId);
  const activeMailboxes = useMemo(() => mailboxes.filter((mailbox) => mailbox.active), [mailboxes]);
  const createSenderDomain = useCreateSupportEmailSenderDomain(workspaceId);
  const verifySenderDomain = useVerifySupportEmailSenderDomain(workspaceId);
  const createSender = useCreateSupportEmailSender(workspaceId);
  const updateSender = useUpdateSupportEmailSender(workspaceId);

  const busy = createSenderDomain.isPending || verifySenderDomain.isPending || createSender.isPending || updateSender.isPending;
  const isLoading = domainsLoading || sendersLoading;
  const groups = useMemo(() => groupSupportEmailSendersByDomain(senderDomains, senders), [senderDomains, senders]);
  const domainDefaults = useMemo(() => deriveSendingDomainDefaults(workspaceName, workspaceWebsiteUrl), [workspaceName, workspaceWebsiteUrl]);
  const selectedGroup = selectedDomain ? groups.find((group) => group.domain === selectedDomain) ?? null : null;
  const workspaceDefault = senders.find((sender) => sender.active && sender.default_scope === 'workspace') ?? null;
  const hasSendingDomains = groups.length > 0;

  const openAddDomainDialog = () => {
    setDomainDraft((current) => ({
      domain: current.domain || domainDefaults.domain,
      localPart: current.localPart || domainDefaults.localPart,
      displayName: current.displayName || domainDefaults.displayName,
    }));
    setAddDomainOpen(true);
  };

  const addDomain = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const domain = domainDraft.domain.trim().toLowerCase();
    if (!domain) {
      toast.error('Enter a sending domain');
      return;
    }
    const localPart = domainDraft.localPart.trim();
    if (!localPart) {
      toast.error('Enter a sender address');
      return;
    }

    await createSenderDomain.mutateAsync({ domain, from_local_part: domainDraft.localPart.trim() || 'support' });

    await createSender.mutateAsync({
      email: `${localPart}@${domain}`,
      display_name: domainDraft.displayName.trim(),
      mailbox_id: null,
    });

    setDomainDraft({ domain: '', localPart: '', displayName: '' });
    setSelectedDomain(domain);
    setAddDomainOpen(false);
    toast.success('Sending domain added');
  };

  const addSender = async (event: FormEvent<HTMLFormElement>, domain: string) => {
    event.preventDefault();
    const draft = senderDrafts[domain] ?? emptySenderDraft();
    const localPart = draft.localPart.trim();
    if (!localPart) {
      toast.error('Enter a sender address');
      return;
    }
    await createSender.mutateAsync({
      email: `${localPart}@${domain}`,
      display_name: draft.displayName.trim(),
      mailbox_id: draft.mailboxId || null,
    });
    setSenderDrafts((current) => ({ ...current, [domain]: emptySenderDraft() }));
    toast.success('Sender address added');
  };

  const updateSenderDraft = (domain: string, patch: Partial<SenderDraft>) => {
    setSenderDrafts((current) => ({
      ...current,
      [domain]: { ...emptySenderDraft(), ...current[domain], ...patch },
    }));
  };

  const handleCopy = async (value: string, label = 'Value') => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(`${label} copied`);
    } catch {
      toast.error(`Could not copy ${label.toLowerCase()}`);
    }
  };

  if (selectedGroup) {
    return (
      <>
        <DomainDetail
          key={selectedGroup.domain}
          group={selectedGroup as SenderDomainGroup}
          senderDraft={senderDrafts[selectedGroup.domain] ?? emptySenderDraft()}
          mailboxes={activeMailboxes}
          busy={busy}
          onBack={() => setSelectedDomain(null)}
          onCopy={handleCopy}
          onSenderDraftChange={(patch) => updateSenderDraft(selectedGroup.domain, patch)}
          onAddSender={(event) => addSender(event, selectedGroup.domain)}
          addSenderOpen={addSenderDomain === selectedGroup.domain}
          onAddSenderOpenChange={(open) => setAddSenderDomain(open ? selectedGroup.domain : null)}
          onVerifyDomain={async () => {
            if (!selectedGroup.senderDomain) return;
            await verifySenderDomain.mutateAsync(selectedGroup.senderDomain.id);
            toast.success('DNS verification refreshed');
          }}
          onEditSender={setEditingSender}
        />
        <EditSenderDialog
          open={Boolean(editingSender)}
          sender={editingSender}
          mailboxes={activeMailboxes}
          busy={busy}
          onOpenChange={(open) => {
            if (!open) setEditingSender(null);
          }}
          onSubmit={async (senderId, payload) => {
            await updateSender.mutateAsync({ senderId, payload });
            setEditingSender(null);
            toast.success('Sender address updated');
          }}
        />
      </>
    );
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardContent className="space-y-4 pt-6">
          {hasSendingDomains && (
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 className="text-base font-medium">Sending domains</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  Verify each domain once, then add the sender addresses you want to use for replies.
                </p>
              </div>
              <Button onClick={openAddDomainDialog}>Add domain</Button>
            </div>
          )}

          {workspaceDefault && (
            <p className="rounded-md bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
              Default sender: <span className="font-medium text-foreground">{workspaceDefault.email}</span>. Inboxes without their own sender use this address.
            </p>
          )}

          {!workspaceDefault && hasSendingDomains && senders.length > 0 && (
            <p className="rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:bg-amber-950/30 dark:text-amber-200">
              No workspace default sender. Edit a sender and choose <span className="font-medium">Workspace default - all inboxes</span>.
            </p>
          )}

          {isLoading && <p className="text-sm text-muted-foreground">Loading sending domains...</p>}

          {!isLoading && !hasSendingDomains && (
            <div className="flex min-h-64 flex-col items-center justify-center rounded-lg border border-dashed px-6 py-10 text-center">
              <MailAdd01Icon className="h-8 w-8 text-muted-foreground" />
              <h3 className="mt-4 text-base font-medium">Add a sending domain</h3>
              <p className="mt-2 max-w-lg text-sm text-muted-foreground">
                Add a domain before creating sender addresses like {domainDefaults.primaryExample} or {domainDefaults.secondaryExample}.
              </p>
              <Button className="mt-5" onClick={openAddDomainDialog}>Add sending domain</Button>
            </div>
          )}

          {!isLoading && hasSendingDomains && (
            <div className="rounded-lg border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Domain</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>From addresses</TableHead>
                    <TableHead className="w-[110px] text-right">Action</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {groups.map((group) => (
                    <DomainTableRow
                      key={group.domain}
                      group={group as SenderDomainGroup}
                      mailboxes={activeMailboxes}
                      onManage={() => setSelectedDomain(group.domain)}
                      onEditSender={setEditingSender}
                    />
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      <AddDomainDialog
        open={addDomainOpen}
        busy={busy}
        draft={domainDraft}
        defaults={domainDefaults}
        onOpenChange={setAddDomainOpen}
        onDraftChange={setDomainDraft}
        onSubmit={addDomain}
      />
      <EditSenderDialog
        open={Boolean(editingSender)}
        sender={editingSender}
        mailboxes={activeMailboxes}
        busy={busy}
        onOpenChange={(open) => {
          if (!open) setEditingSender(null);
        }}
        onSubmit={async (senderId, payload) => {
          await updateSender.mutateAsync({ senderId, payload });
          setEditingSender(null);
          toast.success('Sender address updated');
        }}
      />
    </div>
  );
}

function emptySenderDraft(): SenderDraft {
  return { localPart: '', displayName: '', mailboxId: '' };
}

function DomainTableRow({
  group,
  mailboxes,
  onManage,
  onEditSender,
}: {
  group: SenderDomainGroup;
  mailboxes: Array<{ id: string; name: string }>;
  onManage: () => void;
  onEditSender: (sender: SupportEmailSender) => void;
}) {
  const verified = isDomainVerified(group.senderDomain);
  const [expanded, setExpanded] = useState(true);
  const addressLabel = group.senders.length === 0
    ? 'No addresses'
    : `${group.senders.length} ${group.senders.length === 1 ? 'address' : 'addresses'}`;

  return (
    <>
      <TableRow>
        <TableCell>
          <button type="button" className="font-medium text-primary hover:underline" onClick={onManage}>
            {group.domain}
          </button>
        </TableCell>
        <TableCell>
          {verified ? <StatusBadge tone="success">Verified</StatusBadge> : <StatusBadge tone="warning">{group.senderDomain ? 'Unverified' : 'Domain not added'}</StatusBadge>}
        </TableCell>
        <TableCell>
          <button
            type="button"
            className={group.senders.length > 0 ? 'inline-flex items-center gap-1 text-sm text-primary underline-offset-4 hover:underline' : 'inline-flex items-center gap-1 text-sm text-muted-foreground'}
            onClick={() => setExpanded((current) => !current)}
            disabled={group.senders.length === 0}
          >
            <span>{addressLabel}</span>
          </button>
        </TableCell>
        <TableCell className="text-right">
          <Button size="sm" variant="outline" onClick={onManage}>Manage</Button>
        </TableCell>
      </TableRow>
      {expanded && group.senders.length > 0 && (
        <TableRow className="hover:bg-transparent">
          <TableCell colSpan={4} className="bg-muted/20 p-0">
            <div className="px-3 py-3">
              <CompactSenderTable senders={group.senders} mailboxes={mailboxes} onEditSender={onEditSender} />
            </div>
          </TableCell>
        </TableRow>
      )}
    </>
  );
}

function CompactSenderTable({
  senders,
  mailboxes,
  onEditSender,
}: {
  senders: SupportEmailSender[];
  mailboxes: Array<{ id: string; name: string }>;
  onEditSender: (sender: SupportEmailSender) => void;
}) {
  return (
    <div className="rounded-md bg-background">
      <Table>
        <TableHeader className="text-xs">
          <TableRow>
            <TableHead className="h-8 border-b-0 px-2 text-muted-foreground">Email address</TableHead>
            <TableHead className="h-8 border-b-0 px-2 text-muted-foreground">From name</TableHead>
            <TableHead className="h-8 border-b-0 px-2 text-muted-foreground">Status</TableHead>
            <TableHead className="h-8 border-b-0 px-2 text-muted-foreground">Use for inbox</TableHead>
            <TableHead className="h-8 border-b-0 px-2 text-right text-muted-foreground">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {senders.map((sender) => (
            <TableRow key={sender.id} className="hover:bg-transparent">
              <TableCell className="border-b-0 px-2 py-2 font-medium">{sender.email}</TableCell>
              <TableCell className="border-b-0 px-2 py-2 text-muted-foreground">{sender.display_name || 'Not set'}</TableCell>
              <TableCell className="border-b-0 px-2 py-2">
                {sender.dkim_verified && sender.return_path_domain_verified ? <StatusBadge tone="success">Verified</StatusBadge> : <StatusBadge tone="warning">DNS pending</StatusBadge>}
              </TableCell>
              <TableCell className="border-b-0 px-2 py-2">{senderUsageLabel(sender, mailboxes)}</TableCell>
              <TableCell className="border-b-0 px-2 py-2 text-right">
                <Button size="sm" variant="outline" onClick={() => onEditSender(sender)}>Edit</Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function senderUsageLabel(sender: SupportEmailSender, mailboxes: Array<{ id: string; name: string }> = []) {
  if (!sender.active || sender.default_scope === 'none') return <span className="text-muted-foreground">Not used for replies</span>;
  if (sender.default_scope === 'workspace') {
    return (
      <div className="flex flex-wrap gap-1.5">
        <DisplayPill>Workspace default - all inboxes</DisplayPill>
      </div>
    );
  }
  if (sender.default_scope === 'mailbox') {
    const mailboxIDs = senderMailboxIDs(sender);
    const names = mailboxIDs
      .map((mailboxID) => mailboxes.find((mailbox) => mailbox.id === mailboxID)?.name)
      .filter((name): name is string => Boolean(name));
    if (names.length === 0 && sender.mailbox_name) names.push(sender.mailbox_name);
    if (names.length === 0) names.push('Inbox');
    return (
      <div className="flex flex-wrap gap-1.5">
        {names.map((name) => (
          <DisplayPill key={name}>{name}</DisplayPill>
        ))}
      </div>
    );
  }
  return sender.default_scope;
}

function AddDomainDialog({
  open,
  busy,
  draft,
  defaults,
  onOpenChange,
  onDraftChange,
  onSubmit,
}: {
  open: boolean;
  busy: boolean;
  draft: DomainDraft;
  defaults: ReturnType<typeof deriveSendingDomainDefaults>;
  onOpenChange: (open: boolean) => void;
  onDraftChange: (draft: DomainDraft) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void | Promise<void>;
}) {
  const previewDomain = draft.domain.trim() || defaults.domain;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <form onSubmit={onSubmit}>
          <DialogHeader>
            <DialogTitle>Add a sending domain</DialogTitle>
            <DialogDescription>
              Verify a domain once, then add any sender addresses you want to use from it.
            </DialogDescription>
          </DialogHeader>

          <div className="mt-6 space-y-6">
            <section className="space-y-2">
              <h3 className="text-sm font-medium">1. Add your domain</h3>
              <div>
                <label className="mb-1 block text-xs font-medium text-muted-foreground">Domain</label>
                <Input
                  value={draft.domain}
                  onChange={(event) => onDraftChange({ ...draft, domain: event.target.value })}
                  placeholder={defaults.domain}
                  disabled={busy}
                />
              </div>
            </section>

            <section className="space-y-3">
              <div>
              <h3 className="text-sm font-medium">2. Add a sender address</h3>
                <p className="mt-1 text-sm text-muted-foreground">This address can be used as a From address after the domain is verified.</p>
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-muted-foreground">From name</label>
                <Input
                    value={draft.displayName}
                    onChange={(event) => onDraftChange({ ...draft, displayName: event.target.value })}
                    placeholder={defaults.displayName}
                    disabled={busy}
                  />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-muted-foreground">Sender email address</label>
                <div className="flex max-w-md rounded-md border border-input bg-background">
                  <Input
                    value={draft.localPart}
                    onChange={(event) => onDraftChange({ ...draft, localPart: event.target.value })}
                    placeholder={defaults.localPart}
                    disabled={busy}
                    className="border-0 shadow-none focus-visible:ring-0"
                  />
                  <span className="flex items-center border-l px-2 text-sm text-muted-foreground">@{previewDomain}</span>
                </div>
              </div>
            </section>
          </div>

          <DialogFooter className="mt-6">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>Cancel</Button>
            <Button type="submit" disabled={busy}>Add domain</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function DomainDetail({
  group,
  senderDraft,
  mailboxes,
  busy,
  onBack,
  onCopy,
  onSenderDraftChange,
  onAddSender,
  addSenderOpen,
  onAddSenderOpenChange,
  onVerifyDomain,
  onEditSender,
}: {
  group: SenderDomainGroup;
  senderDraft: SenderDraft;
  mailboxes: Array<{ id: string; name: string }>;
  busy: boolean;
  onBack: () => void;
  onCopy: (value: string, label?: string) => void | Promise<void>;
  onSenderDraftChange: (patch: Partial<SenderDraft>) => void;
  onAddSender: (event: FormEvent<HTMLFormElement>) => void | Promise<void>;
  addSenderOpen: boolean;
  onAddSenderOpenChange: (open: boolean) => void;
  onVerifyDomain: () => void | Promise<void>;
  onEditSender: (sender: SupportEmailSender) => void;
}) {
  const senderDomain = group.senderDomain;
  const verified = isDomainVerified(senderDomain);
  const dkimHost = senderDomain?.dkim_pending_host || senderDomain?.dkim_host || '';
  const dkimValue = senderDomain?.dkim_pending_text_value || senderDomain?.dkim_text_value || '';
  const [showVerificationRecords, setShowVerificationRecords] = useState(!verified);

  return (
    <div className="space-y-4">
      <Button variant="ghost" className="gap-2 px-0" onClick={onBack}>
        <ArrowLeft02Icon className="h-4 w-4" />
        Back to sending domains
      </Button>

      <Card>
        <CardContent className="space-y-4 pt-6">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="text-lg font-medium">{group.domain}</h2>
                {verified ? <StatusBadge tone="success">Verified</StatusBadge> : <StatusBadge tone="warning">{senderDomain ? 'Unverified' : 'Domain not added'}</StatusBadge>}
                {senderDomain?.active && <StatusBadge tone="success">Active</StatusBadge>}
              </div>
              <p className="mt-2 text-sm text-muted-foreground">
                {verified
                  ? 'This domain can send authenticated Helpin replies.'
                  : 'Set up the DNS records below to verify this domain before using its sender addresses.'}
              </p>
            </div>
            {senderDomain && (
              <div className="flex flex-wrap gap-2">
                <Button variant="outline" onClick={onVerifyDomain} disabled={busy}>Recheck DNS</Button>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {senderDomain && (
        <Card>
          <CardContent className="space-y-4 pt-6">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h3 className="text-base font-medium">Domain verification</h3>
                <p className="mt-1 text-sm text-muted-foreground">
                  {verified ? 'DKIM and Return-Path records are verified.' : 'Add these DNS records to verify the domain for outbound email.'}
                </p>
              </div>
              {verified && (
                <Button type="button" variant="outline" size="sm" onClick={() => setShowVerificationRecords((current) => !current)}>
                  {showVerificationRecords ? 'Hide records' : 'Show records'}
                </Button>
              )}
            </div>
            {showVerificationRecords && (
              <div className="space-y-4">
                <DNSRecordGroup
                  title="DKIM record"
                  description="Add this TXT record so mail providers can verify Helpin is allowed to send for your domain."
                  type="TXT"
                  host={dkimHost}
                  value={dkimValue}
                  verified={Boolean(senderDomain.dkim_verified)}
                  onCopy={onCopy}
                />
                <DNSRecordGroup
                  title="Return-Path record"
                  description="Add this CNAME record so bounce handling aligns with your domain."
                  type="CNAME"
                  host={senderDomain.return_path_domain}
                  value={senderDomain.return_path_domain_cname_value}
                  verified={Boolean(senderDomain.return_path_domain_verified)}
                  onCopy={onCopy}
                />
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardContent className="space-y-4 pt-6">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h3 className="text-base font-medium">Sender addresses</h3>
              <p className="mt-1 text-sm text-muted-foreground">Add the From addresses your team can use from this domain.</p>
            </div>
            <Button variant="outline" onClick={() => onAddSenderOpenChange(true)} disabled={busy}>Add sender</Button>
          </div>

          {group.senders.length === 0 ? (
            <p className="rounded-md bg-muted/40 px-3 py-3 text-sm text-muted-foreground">No sender addresses yet.</p>
          ) : (
            <div className="rounded-lg border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Email address</TableHead>
                    <TableHead>From name</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>
                      <LabelWithTooltip tooltip="Choose which inbox uses this email as the From address for replies. Inbox senders override the workspace sender.">
                        Use for inbox
                      </LabelWithTooltip>
                    </TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {group.senders.map((sender) => (
                    <SenderRow
                      key={sender.id}
                      sender={sender}
                      mailboxes={mailboxes}
                      busy={busy}
                      onEdit={() => onEditSender(sender)}
                    />
                  ))}
                </TableBody>
              </Table>
            </div>
          )}

          <AddSenderDialog
            open={addSenderOpen}
            domain={group.domain}
            verified={verified}
            busy={busy}
            senderDraft={senderDraft}
            mailboxes={mailboxes}
            onOpenChange={onAddSenderOpenChange}
            onSenderDraftChange={onSenderDraftChange}
            onSubmit={onAddSender}
          />
        </CardContent>
      </Card>
    </div>
  );
}

function AddSenderDialog({
  open,
  domain,
  verified,
  busy,
  senderDraft,
  mailboxes,
  onOpenChange,
  onSenderDraftChange,
  onSubmit,
}: {
  open: boolean;
  domain: string;
  verified: boolean;
  busy: boolean;
  senderDraft: SenderDraft;
  mailboxes: Array<{ id: string; name: string }>;
  onOpenChange: (open: boolean) => void;
  onSenderDraftChange: (patch: Partial<SenderDraft>) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void | Promise<void>;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <form onSubmit={onSubmit}>
          <DialogHeader>
            <DialogTitle>Add sender address</DialogTitle>
            <DialogDescription>
              Add a From address for {domain}.
            </DialogDescription>
          </DialogHeader>

          <div className="mt-6 space-y-4">
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">Sender email address</label>
              <div className="grid max-w-lg grid-cols-[minmax(0,220px)_auto] rounded-md border border-input bg-background">
                <Input
                  value={senderDraft.localPart}
                  onChange={(event) => onSenderDraftChange({ localPart: event.target.value })}
                  placeholder="support"
                  disabled={busy}
                  className="border-0 shadow-none focus-visible:ring-0"
                />
                <span className="flex min-w-0 items-center whitespace-nowrap border-l px-2 text-sm text-muted-foreground">@{domain}</span>
              </div>
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">From name</label>
              <Input value={senderDraft.displayName} onChange={(event) => onSenderDraftChange({ displayName: event.target.value })} placeholder="Support" disabled={busy} />
            </div>
            <div>
              <LabelWithTooltip
                className="mb-1"
                tooltip="This keeps the sender connected to an inbox. After DNS is verified, edit the sender to use it for replies."
              >
                Related inbox
              </LabelWithTooltip>
              <div className="flex flex-wrap gap-2">
                <SelectablePill
                  selected={!senderDraft.mailboxId}
                  disabled={busy}
                  onClick={() => onSenderDraftChange({ mailboxId: '' })}
                >
                  No specific inbox
                </SelectablePill>
                {mailboxes.map((mailbox) => (
                  <SelectablePill
                    key={mailbox.id}
                    selected={senderDraft.mailboxId === mailbox.id}
                    disabled={busy}
                    onClick={() => onSenderDraftChange({ mailboxId: mailbox.id })}
                  >
                    {mailbox.name}
                  </SelectablePill>
                ))}
              </div>
            </div>
            {!verified && <p className="text-xs text-muted-foreground">This address can be added now, but it cannot be used as a default until DNS is verified.</p>}
          </div>

          <DialogFooter className="mt-6">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>Cancel</Button>
            <Button type="submit" disabled={busy}>Add sender</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function SenderRow({
  sender,
  mailboxes,
  busy,
  onEdit,
}: {
  sender: SupportEmailSender;
  mailboxes: Array<{ id: string; name: string }>;
  busy: boolean;
  onEdit: () => void;
}) {
  const verified = sender.dkim_verified && sender.return_path_domain_verified;

  return (
    <TableRow>
      <TableCell>
        <div className="font-medium">{sender.email}</div>
        {sender.mailbox_name && <div className="mt-1 text-xs text-muted-foreground">Inbox: {sender.mailbox_name}</div>}
      </TableCell>
      <TableCell>{sender.display_name || <span className="text-muted-foreground">Not set</span>}</TableCell>
      <TableCell>
        {verified ? <StatusBadge tone="success">Verified</StatusBadge> : <StatusBadge tone="warning">DNS pending</StatusBadge>}
      </TableCell>
      <TableCell>{senderUsageLabel(sender, mailboxes)}</TableCell>
      <TableCell className="text-right">
        <Button size="sm" variant="outline" onClick={onEdit} disabled={busy}>Edit</Button>
      </TableCell>
    </TableRow>
  );
}

function EditSenderDialog({
  open,
  sender,
  mailboxes,
  busy,
  onOpenChange,
  onSubmit,
}: {
  open: boolean;
  sender: SupportEmailSender | null;
  mailboxes: Array<{ id: string; name: string }>;
  busy: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (senderId: string, payload: UpdateSupportEmailSenderRequest) => void | Promise<void>;
}) {
  if (!sender) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <EditSenderDialogContent
        key={sender.id}
        sender={sender}
        mailboxes={mailboxes}
        busy={busy}
        onOpenChange={onOpenChange}
        onSubmit={onSubmit}
      />
    </Dialog>
  );
}

function EditSenderDialogContent({
  sender,
  mailboxes,
  busy,
  onOpenChange,
  onSubmit,
}: {
  sender: SupportEmailSender;
  mailboxes: Array<{ id: string; name: string }>;
  busy: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (senderId: string, payload: UpdateSupportEmailSenderRequest) => void | Promise<void>;
}) {
  const [fromName, setFromName] = useState(sender.display_name || '');
  const [defaultFor, setDefaultFor] = useState(senderDefaultValue(sender));
  const [selectedInboxIds, setSelectedInboxIds] = useState<string[]>(senderMailboxIDs(sender));
  const verified = sender.dkim_verified && sender.return_path_domain_verified;

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (defaultFor === 'selected' && selectedInboxIds.length === 0) {
      toast.error('Select at least one inbox');
      return;
    }
    const payload: UpdateSupportEmailSenderRequest = {
      display_name: fromName.trim(),
      ...defaultValuePayload(defaultFor, selectedInboxIds),
    };
    void Promise.resolve(onSubmit(sender.id, payload)).catch(() => undefined);
  };

  return (
    <DialogContent className="sm:max-w-lg">
      <form onSubmit={submit}>
        <DialogHeader>
          <DialogTitle>Edit sender</DialogTitle>
          <DialogDescription>
            Update the From name and where Helpin uses this email for replies.
          </DialogDescription>
        </DialogHeader>

        <div className="mt-6 space-y-4">
          <div>
            <label className="mb-1 block text-xs font-medium text-muted-foreground">Email address</label>
            <Input value={sender.email} disabled />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-muted-foreground">From name</label>
            <Input value={fromName} onChange={(event) => setFromName(event.target.value)} placeholder="Support" disabled={busy} />
          </div>
          <div>
            <LabelWithTooltip
              className="mb-1"
              tooltip="Choose which inbox uses this email as the From address for replies. Inbox senders override the workspace sender."
            >
              Use for inbox
            </LabelWithTooltip>
            <div className="flex flex-wrap gap-2">
              <SelectablePill
                selected={defaultFor === 'none'}
                disabled={busy}
                onClick={() => setDefaultFor('none')}
              >
                Do not use
              </SelectablePill>
              <SelectablePill
                selected={defaultFor === 'workspace'}
                disabled={busy || (!verified && defaultFor !== 'workspace')}
                onClick={() => setDefaultFor('workspace')}
                tooltip="Acts as the fallback sender for inboxes without their own selected sender."
              >
                Workspace default - all inboxes
              </SelectablePill>
              <SelectablePill
                selected={defaultFor === 'selected'}
                disabled={busy || (!verified && defaultFor !== 'selected')}
                onClick={() => setDefaultFor('selected')}
              >
                Selected inboxes
              </SelectablePill>
            </div>
            {defaultFor === 'selected' && (
              <div className="mt-3 flex flex-wrap gap-2">
                {mailboxes.map((mailbox) => {
                  const selected = selectedInboxIds.includes(mailbox.id);
                  return (
                    <SelectablePill
                      key={mailbox.id}
                      selected={selected}
                      disabled={busy || !verified}
                      onClick={() => {
                        setSelectedInboxIds((current) => selected
                          ? current.filter((id) => id !== mailbox.id)
                          : [...current, mailbox.id]);
                      }}
                    >
                      {mailbox.name}
                    </SelectablePill>
                  );
                })}
              </div>
            )}
            {!verified && <p className="mt-2 text-xs text-muted-foreground">Verify DNS before using this sender as a default.</p>}
            {verified && defaultFor === 'selected' && selectedInboxIds.length === 0 && (
              <p className="mt-2 text-xs text-muted-foreground">Select one or more inboxes for this sender.</p>
            )}
          </div>
        </div>

        <DialogFooter className="mt-6">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>Cancel</Button>
          <Button type="submit" disabled={busy}>Save changes</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  );
}

function senderDefaultValue(sender: SupportEmailSender) {
  if (sender.default_scope === 'workspace') return 'workspace';
  if (sender.default_scope === 'mailbox') return 'selected';
  return 'none';
}

function senderMailboxIDs(sender: SupportEmailSender) {
  const ids = [...(sender.mailbox_ids ?? [])];
  if (sender.mailbox_id && !ids.includes(sender.mailbox_id)) ids.push(sender.mailbox_id);
  return ids;
}

function defaultValuePayload(value: string, mailboxIds: string[]): Pick<UpdateSupportEmailSenderRequest, 'default_scope' | 'mailbox_id' | 'mailbox_ids'> {
  if (value === 'workspace') {
    return { default_scope: 'workspace', mailbox_id: null, mailbox_ids: [] };
  }
  if (value === 'selected') {
    return { default_scope: 'mailbox', mailbox_id: mailboxIds[0] ?? null, mailbox_ids: mailboxIds };
  }
  return { default_scope: 'none', mailbox_id: null, mailbox_ids: [] };
}

function DNSRecordGroup({
  title,
  description,
  type,
  host,
  value,
  verified,
  onCopy,
}: {
  title: string;
  description: string;
  type: 'TXT' | 'CNAME';
  host: string;
  value: string;
  verified: boolean;
  onCopy: (value: string, label?: string) => void | Promise<void>;
}) {
  return (
    <div className="rounded-md border p-3">
      <div className="flex items-start gap-3">
        <RecordStatus verified={verified} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <h4 className="text-sm font-medium">{title}</h4>
              <p className="mt-1 text-sm text-muted-foreground">{description}</p>
            </div>
            {verified && <StatusBadge tone="success">Verified</StatusBadge>}
          </div>
          <div className="mt-3 grid gap-2 text-xs md:grid-cols-[90px_1fr_1fr] md:items-start">
            <div>
              <label className="mb-1 block text-muted-foreground">Type</label>
              <code className="block rounded bg-muted px-2 py-2">{type}</code>
            </div>
            <div>
              <label className="mb-1 block text-muted-foreground">Host</label>
              <div className="flex items-start gap-1">
                <code className="min-w-0 flex-1 truncate rounded bg-muted px-2 py-2">{host || 'Waiting for Postmark'}</code>
                <CopyButton label={`${type} host`} value={host} onCopy={onCopy} />
              </div>
            </div>
            <div>
              <label className="mb-1 block text-muted-foreground">Value</label>
              <div className="flex items-start gap-1">
                <code className="min-w-0 flex-1 max-h-28 overflow-auto whitespace-pre-wrap break-all rounded bg-muted px-2 py-2">{value || 'Waiting for Postmark'}</code>
                <CopyButton label={`${type} value`} value={value} onCopy={onCopy} />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function RecordStatus({ verified }: { verified: boolean }) {
  return (
    <span className={verified ? 'mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300' : 'mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300'}>
      {verified ? '✓' : '!'}
    </span>
  );
}

function LabelWithTooltip({
  children,
  tooltip,
  className,
}: {
  children: ReactNode;
  tooltip: string;
  className?: string;
}) {
  return (
    <div className={`flex items-center gap-1 text-xs font-medium text-muted-foreground ${className ?? ''}`}>
      <span>{children}</span>
      <Tooltip>
        <TooltipTrigger asChild>
          <button type="button" className="text-muted-foreground hover:text-foreground">
            <InformationCircleIcon className="h-3.5 w-3.5" />
          </button>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-xs">
          {tooltip}
        </TooltipContent>
      </Tooltip>
    </div>
  );
}

function SelectablePill({
  selected,
  disabled,
  onClick,
  tooltip,
  children,
}: {
  selected: boolean;
  disabled?: boolean;
  onClick: () => void;
  tooltip?: string;
  children: ReactNode;
}) {
  const button = (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={cn(
        'inline-flex h-8 max-w-full items-center rounded-full border px-3 text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50',
        selected
          ? 'border-primary bg-primary/10 text-primary'
          : 'border-border bg-background text-muted-foreground hover:border-primary/40 hover:text-foreground',
      )}
    >
      <span className="truncate">{children}</span>
    </button>
  );

  if (!tooltip) return button;

  return (
    <Tooltip>
      <TooltipTrigger asChild>{button}</TooltipTrigger>
      <TooltipContent side="top" className="max-w-xs">
        {tooltip}
      </TooltipContent>
    </Tooltip>
  );
}

function DisplayPill({ children }: { children: ReactNode }) {
  return (
    <span className="inline-flex h-6 max-w-full items-center rounded-full border border-border bg-muted/40 px-2 text-xs text-foreground">
      <span className="truncate">{children}</span>
    </span>
  );
}

function CopyButton({ label, value, onCopy }: { label: string; value: string; onCopy: (value: string, label?: string) => void | Promise<void> }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => onCopy(value, label)} disabled={!value}>
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
    : 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-300';
  return <Badge variant="outline" className={className}>{children}</Badge>;
}

function isDomainVerified(senderDomain: SupportEmailSenderDomain | null) {
  return Boolean(senderDomain?.dkim_verified && senderDomain.return_path_domain_verified);
}
