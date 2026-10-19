import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useState } from 'react';
import { toast } from 'sonner';
import { useChatSettings, useSupportEmailSenders, useSetDefaultSupportEmailSender, useUpdateMailbox } from '@/hooks/queries/useSupport';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { ArrowDown01Icon, PlusSignIcon } from '@/lib/icons';
import type { SupportEmailSender, SupportMailbox } from '@/lib/pmTypes';
import { inboxAIReplyStatus } from './inboxAIReplyStatus';

export function InboxAIRepliesCell({ workspaceId, mailbox }: { workspaceId: string; mailbox?: SupportMailbox }) {
  const { data, isError } = useChatSettings(workspaceId);
  const update = useUpdateMailbox(workspaceId);
  const [open, setOpen] = useState(false);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const canManage = usePermissions(access).has("support.admin");
  const status = isError ? 'Unavailable' : inboxAIReplyStatus(data?.settings, mailbox?.ai_replies_disabled);
  if (!mailbox) return <span className="text-xs">{status}</span>;
  if (!mailbox.active) return <span className="text-xs text-muted-foreground">Disabled</span>;
  return <Popover open={open} onOpenChange={setOpen}>
    <PopoverTrigger asChild><Button variant="ghost" size="sm" className="h-7 max-w-full gap-1 px-1 text-xs" disabled={!canManage || !data || update.isPending} aria-label={`AI replies for ${mailbox.name}: ${status}`}>{status}<ArrowDown01Icon className="size-3 shrink-0" /></Button></PopoverTrigger>
    <PopoverContent className="w-56 p-1" align="start">
      {[{ disabled: false, label: inboxAIReplyStatus(data?.settings) }, { disabled: true, label: 'Off for this inbox' }].map(option => <Button key={String(option.disabled)} variant="ghost" size="sm" className="w-full justify-start text-xs" disabled={update.isPending} onClick={async () => {
        try { await update.mutateAsync({ mailboxId: mailbox.id, payload: { ai_replies_disabled: option.disabled } }); setOpen(false); toast.success('AI replies updated'); } catch { /* mutation reports failure */ }
      }}>{option.disabled ? option.label : `${option.label} · follow workspace settings`}</Button>)}
    </PopoverContent>
  </Popover>;
}

export function InboxSenderCell({ workspaceId, mailbox, sender }: { workspaceId: string; mailbox?: SupportMailbox; sender?: SupportEmailSender | null }) {
  const query = useSupportEmailSenders(workspaceId);
  const update = useSetDefaultSupportEmailSender(workspaceId);
  const [open, setOpen] = useState(false);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const canManage = usePermissions(access).has("support.admin");
  if (mailbox && !mailbox.active) return <span className="text-xs text-muted-foreground">Disabled</span>;
  const eligible = (query.data ?? []).filter(item => item.dkim_verified && item.return_path_domain_verified);
  return <Popover open={open} onOpenChange={setOpen}>
    <PopoverTrigger asChild><Button variant="ghost" size="sm" className="h-7 max-w-full gap-1 px-1 text-xs" disabled={!canManage || query.isLoading || update.isPending} aria-label={`Change sender address for ${mailbox?.name ?? 'Shared inbox'}`} title={sender?.email}><span className="truncate">{sender?.email ?? 'Choose sender'}</span><ArrowDown01Icon className="size-3 shrink-0" /></Button></PopoverTrigger>
    <PopoverContent className="w-72 p-1" align="start">
      {query.isError && <p className="p-2 text-xs text-destructive">Could not load sender addresses.</p>}
      {eligible.map(item => <Button key={item.id} variant="ghost" size="sm" className="w-full justify-start text-xs" disabled={update.isPending} onClick={async () => {
        try { await update.mutateAsync({ senderId: item.id, payload: mailbox ? { default_scope: 'mailbox', mailbox_ids: [mailbox.id], single_mailbox: true } : { default_scope: 'workspace' } }); setOpen(false); toast.success('Sender address updated'); } catch { /* mutation reports failure */ }
      }}><span className="truncate">{item.email}</span>{item.id === sender?.id && <span className="ml-auto text-muted-foreground">Current</span>}</Button>)}
      {!query.isError && !query.isLoading && eligible.length === 0 && <p className="p-2 text-xs text-muted-foreground">No verified sender addresses yet.</p>}
      <Button asChild variant="ghost" size="sm" className="w-full justify-start gap-2 border-t text-xs"><a href="./inboxes-routing?tab=senders&create_sender=true"><PlusSignIcon className="size-3" />New sender address</a></Button>
    </PopoverContent>
  </Popover>;
}
