import { useState } from 'react';
import { toast } from 'sonner';
import { MoreHorizontalIcon, PlusSignIcon } from '@/lib/icons';
import { QuietIconAction, QuietPrimaryAction, QuietStatusText, QuietTextAction } from '@/components/design-system/quiet';
import { Card, CardContent } from '@/components/ui/card';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { useSupportEmailSenderDomains, useSupportEmailSenders, useSupportMailboxes, useUpdateSupportEmailSender } from '@/hooks/queries/useSupport';
import type { SupportEmailSender } from '@/lib/pmTypes';
import { senderHasVerifiedDNS, senderMailboxIDs } from './supportEmailSenderUtils';
import { SupportEmailSenderDialog } from './SupportEmailSenderDialog';

interface Props {
  workspaceId: string;
  workspaceName?: string;
  workspaceWebsiteUrl?: string;
  canManage?: boolean;
  createSender?: boolean;
}

export function SupportEmailSendersTab({ canManage = false, ...props }: Props) {
  if (!canManage) return <p className="py-6 text-sm text-muted-foreground">You need permission to manage support settings to view sender addresses.</p>;
  return <SenderAddresses key={props.workspaceId} {...props} />;
}

function SenderAddresses({ workspaceId, workspaceName, workspaceWebsiteUrl, createSender = false }: Omit<Props, 'canManage'>) {
  const domainsQuery = useSupportEmailSenderDomains(workspaceId);
  const sendersQuery = useSupportEmailSenders(workspaceId);
  const mailboxesQuery = useSupportMailboxes(workspaceId);
  const updateSender = useUpdateSupportEmailSender(workspaceId);
  const [dialog, setDialog] = useState<{ sender?: SupportEmailSender; manageDNS?: boolean } | null>(() => createSender ? {} : null);
  const [savingDefault, setSavingDefault] = useState(false);
  const domains = domainsQuery.data ?? [];
  const senders = sendersQuery.data ?? [];
  const mailboxes = mailboxesQuery.data ?? [];
  const loading = domainsQuery.isLoading || sendersQuery.isLoading || mailboxesQuery.isLoading;
  const failed = domainsQuery.isError || sendersQuery.isError || mailboxesQuery.isError;

  async function makeDefault(sender: SupportEmailSender) {
    if (savingDefault) return;
    setSavingDefault(true);
    try {
      await updateSender.mutateAsync({ senderId: sender.id, payload: { default_scope: 'workspace', mailbox_id: null, mailbox_ids: [] } });
      toast.success('Workspace default updated');
    } catch {
      // The existing mutation reports the API error and leaves the list retryable.
    } finally { setSavingDefault(false); }
  }

  function usage(sender: SupportEmailSender) {
    if (!sender.active || sender.default_scope === 'none') return <span className="text-quiet-text-tertiary">Not in use</span>;
    if (sender.default_scope === 'workspace') return <QuickTooltip label="Used by Shared Inbox and inboxes without their own sender."><span tabIndex={0}>Workspace default</span></QuickTooltip>;
    return senderMailboxIDs(sender).map(id => mailboxes.find(inbox => inbox.id === id)?.name ?? sender.mailbox_name ?? 'Unavailable inbox').join(', ');
  }

  return (
    <>
      <Card className="gap-0 overflow-hidden py-0">
        <CardContent className="p-0">
          <div className="flex flex-wrap items-center justify-between gap-4 p-6">
            <div>
              <h2 className="text-sm font-semibold text-quiet-text-primary">Sender addresses</h2>
              <p className="mt-1.5 text-sm text-quiet-text-tertiary">Send support replies from your own email addresses.</p>
            </div>
            <QuietPrimaryAction disabled={loading || failed || savingDefault} onClick={() => setDialog({})}><PlusSignIcon className="size-4" />Add address</QuietPrimaryAction>
          </div>
          {loading ? <p className="px-6 pb-6 text-sm text-muted-foreground" role="status">Loading sender addresses…</p> : failed ? (
            <div className="space-y-3 px-6 pb-6"><p role="alert" className="text-sm text-destructive">Could not load sender settings.</p><QuietTextAction onClick={() => { void domainsQuery.refetch(); void sendersQuery.refetch(); void mailboxesQuery.refetch(); }}>Try again</QuietTextAction></div>
          ) : senders.length === 0 ? (
            <div className="border-t border-quiet-divider-strong px-6 py-8">
              <p className="text-sm font-medium">Add your first sender address</p>
              <p className="mt-1.5 text-sm text-quiet-text-tertiary">We’ll guide you through any verification needed.</p>
            </div>
          ) : (
            <Table className="table-fixed min-w-[650px]">
              <TableHeader><TableRow className="border-y border-quiet-divider-strong hover:bg-transparent">
                <TableHead className="w-[34%] pl-6">Email address</TableHead>
                <TableHead className="w-[26%]">Used for inboxes</TableHead>
                <TableHead className="w-[18%]">Status</TableHead>
                <TableHead className="w-[22%] pr-6"><span className="sr-only">Actions</span></TableHead>
              </TableRow></TableHeader>
              <TableBody>{senders.map(sender => {
                const verified = senderHasVerifiedDNS(sender);
                return <TableRow key={sender.id} className="border-quiet-divider-light hover:bg-quiet-row-hover">
                  <TableCell className="py-4 pl-6">
                    <button type="button" className="block max-w-full truncate text-left font-medium hover:underline focus-visible:underline" title={sender.email} disabled={savingDefault} onClick={() => setDialog({ sender })}>{sender.email}</button>
                    {sender.display_name && <span className="mt-1 block truncate text-xs text-quiet-text-tertiary">{sender.display_name}</span>}
                  </TableCell>
                  <TableCell className="whitespace-normal break-words text-[12.5px]">{usage(sender)}</TableCell>
                  <TableCell><QuietStatusText tone={!verified ? 'blocker' : sender.active ? 'positive' : 'neutral'} className="whitespace-nowrap font-normal normal-case tracking-normal">{!verified ? 'Setup required' : sender.active ? 'Active' : 'Verified'}</QuietStatusText></TableCell>
                  <TableCell className="pr-6"><div className="flex items-center justify-end gap-3">
                    {!verified && <QuietTextAction disabled={savingDefault} className="whitespace-nowrap text-quiet-accent" onClick={() => setDialog({ sender })}>Finish setup</QuietTextAction>}
                    <DropdownMenu><DropdownMenuTrigger asChild><QuietIconAction disabled={savingDefault} aria-label={`More options for ${sender.email}`}><MoreHorizontalIcon /></QuietIconAction></DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onSelect={() => setDialog({ sender })}>Edit sender</DropdownMenuItem>
                        {verified && (!sender.active || sender.default_scope !== 'workspace') && <DropdownMenuItem onSelect={() => void makeDefault(sender)}>Make workspace default</DropdownMenuItem>}
                        <DropdownMenuSeparator />
                        <DropdownMenuItem onSelect={() => setDialog({ sender, manageDNS: true })}>Manage DNS</DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div></TableCell>
                </TableRow>;
              })}</TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      {dialog && !loading && !failed && <SupportEmailSenderDialog
        workspaceId={workspaceId} workspaceName={workspaceName} workspaceWebsiteUrl={workspaceWebsiteUrl}
        sender={dialog.sender} manageDNS={dialog.manageDNS} domains={domains} senders={senders}
        mailboxes={mailboxes.filter(inbox => inbox.active)} onClose={() => setDialog(null)}
      />}
    </>
  );
}
