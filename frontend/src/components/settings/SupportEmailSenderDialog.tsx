import { useEffect, useRef, useState, type FormEvent } from 'react';
import { toast } from 'sonner';
import { ArrowDown01Icon, Cancel01Icon, InformationCircleIcon, Loading01Icon } from '@/lib/icons';
import { QuietDropdown, QuietIconAction, QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { useCreateSupportEmailSender, useCreateSupportEmailSenderDomain, useUpdateSupportEmailSender, useVerifySupportEmailSender, useVerifySupportEmailSenderDomain } from '@/hooks/queries/useSupport';
import type { SupportEmailSender, SupportEmailSenderDomain, UpdateSupportEmailSenderRequest } from '@/lib/pmTypes';
import { deriveSendingDomainDefaults, senderHasVerifiedDNS, senderMailboxIDs } from './supportEmailSenderUtils';
import { SupportEmailSenderDNSDialog } from './SupportEmailSenderDNSDialog';

type Scope = 'none' | 'workspace' | 'mailbox';
interface Props {
  workspaceId: string;
  workspaceName?: string;
  workspaceWebsiteUrl?: string;
  sender?: SupportEmailSender;
  manageDNS?: boolean;
  domains: SupportEmailSenderDomain[];
  senders: SupportEmailSender[];
  mailboxes: { id: string; name: string }[];
  onClose: () => void;
}

export function SupportEmailSenderDialog({ workspaceId, workspaceName, workspaceWebsiteUrl, sender: initialSender, manageDNS = false, domains, senders, mailboxes, onClose }: Props) {
  const defaults = deriveSendingDomainDefaults(workspaceName, workspaceWebsiteUrl);
  const [sender, setSender] = useState(initialSender);
  const [email, setEmail] = useState(initialSender?.email ?? defaults.primaryExample);
  const [label, setLabel] = useState(initialSender?.display_name ?? defaults.displayName);
  const [scope, setScope] = useState<Scope>(initialSender ? initialSender.active ? initialSender.default_scope as Scope : 'none' : senders.some(row => row.active && row.default_scope === 'workspace') ? mailboxes.length ? 'mailbox' : 'none' : 'workspace');
  const [inboxIds, setInboxIds] = useState(initialSender?.active ? senderMailboxIDs(initialSender) : []);
  const [domain, setDomain] = useState(domains.find(row => row.domain === initialSender?.domain));
  const [view, setView] = useState<'form' | 'dns' | 'complete'>(manageDNS ? 'dns' : 'form');
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);
  const [retrySave, setRetrySave] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const closed = useRef(false);
  const inFlight = useRef(false);
  useEffect(() => { closed.current = false; return () => { closed.current = true; }; }, []);
  const createDomain = useCreateSupportEmailSenderDomain(workspaceId);
  const createSender = useCreateSupportEmailSender(workspaceId);
  const updateSender = useUpdateSupportEmailSender(workspaceId);
  const verifyDomain = useVerifySupportEmailSenderDomain(workspaceId);
  const verifySender = useVerifySupportEmailSender(workspaceId);
  const domainName = email.trim().split('@').at(-1)?.toLowerCase() ?? '';
  const knownDomain = domain?.domain === domainName ? domain : domains.find(row => row.domain === domainName);
  const knownDNS = knownDomain ?? senders.find(row => row.domain === domainName);
  const ready = sender ? senderHasVerifiedDNS(sender) : senderHasVerifiedDNS(knownDNS);

  function close() { closed.current = true; onClose(); }
  function start() {
    if (inFlight.current) return false;
    inFlight.current = true; setBusy(true); setError(''); setNotice(''); return true;
  }
  function finish() { inFlight.current = false; if (!closed.current) { setBusy(false); setSaving(false); } }

  async function saveUsage(current: SupportEmailSender) {
    const payload: UpdateSupportEmailSenderRequest = {};
    if (label.trim() !== current.display_name) payload.display_name = label.trim();
    const currentScope = current.active ? current.default_scope : 'none';
    if (scope !== currentScope || (scope === 'mailbox' && [...inboxIds].sort().join(',') !== senderMailboxIDs(current).sort().join(','))) {
      payload.default_scope = scope;
      payload.mailbox_id = scope === 'mailbox' ? inboxIds[0] : null;
      payload.mailbox_ids = scope === 'mailbox' ? inboxIds : [];
    }
    if (Object.keys(payload).length === 0) return current;
    return updateSender.mutateAsync({ senderId: current.id, payload });
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    const normalized = email.trim().toLowerCase();
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(normalized)) { setError('Enter a valid email address.'); return; }
    if (!sender && senders.some(row => row.email.toLowerCase() === normalized)) { setError('This address is already added. Edit it from the list.'); return; }
    if (scope === 'mailbox' && (!inboxIds.length || inboxIds.some(id => !mailboxes.some(inbox => inbox.id === id)))) { setError('Select one or more available inboxes.'); return; }
    if (!start()) return;
    try {
      let current = sender;
      if (!current) {
        // Reuse existing domain records, including a domain saved by an earlier partial attempt.
        if (!knownDNS) {
          const created = await createDomain.mutateAsync({ domain: domainName, from_local_part: normalized.split('@')[0] });
          if (closed.current) return;
          setDomain(created);
        } else if (knownDomain) setDomain(knownDomain);
        current = await createSender.mutateAsync({ email: normalized, display_name: label.trim(), mailbox_id: null });
        if (closed.current) return;
        setSender(current); setEmail(current.email);
      }
      if (senderHasVerifiedDNS(current) || (current.active && scope === 'none')) {
        setSaving(true);
        await saveUsage(current);
        if (closed.current) return;
        toast.success(initialSender ? 'Sender settings saved' : 'Sender address added');
        close();
      } else {
        if (label.trim() !== current.display_name) {
          current = await updateSender.mutateAsync({ senderId: current.id, payload: { display_name: label.trim() } });
          if (closed.current) return;
          setSender(current);
        }
        setView('dns');
      }
    } catch (err) {
      if (!closed.current) setError(err instanceof Error ? err.message : 'Could not save this address. Try again.');
    } finally { finish(); }
  }

  async function checkDNS() {
    if (!sender || !start()) return;
    let applyingUsage = false;
    try {
      let current = sender;
      if (!retrySave) {
        const result = domain ? await verifyDomain.mutateAsync(domain.id) : await verifySender.mutateAsync(sender.id);
        if (closed.current) return;
        if (domain) {
          setDomain(result as SupportEmailSenderDomain);
          // Domain verification synchronizes these fields to all addresses on the server.
          current = { ...sender, dkim_verified: result.dkim_verified, return_path_domain_verified: result.return_path_domain_verified };
        } else current = result as SupportEmailSender;
        setSender(current);
        if (result.last_error) { setError(`Could not finish checking DNS. ${result.last_error}`); return; }
        if (!senderHasVerifiedDNS(result)) { setNotice('Not verified yet. DNS updates can take time. Check the records and try again.'); return; }
      }
      if (manageDNS) { setNotice('DNS verification refreshed.'); return; }
      applyingUsage = true;
      setRetrySave(true); setSaving(true);
      const updated = await saveUsage(current);
      if (closed.current) return;
      setSender(updated); setRetrySave(false); setView('complete');
    } catch (err) {
      if (!closed.current) setError(`${applyingUsage ? 'Address verified, but inbox selection could not be saved. ' : ''}${err instanceof Error ? err.message : 'The check failed. Try again.'}`);
    } finally { finish(); }
  }

  return <Dialog open onOpenChange={open => { if (!open) close(); }}>
    <DialogContent variant="flush" showCloseButton={false} className="flex max-h-[calc(100dvh-3rem)] flex-col overflow-hidden sm:max-w-[560px]" onOpenAutoFocus={event => {
      event.preventDefault();
      const input = document.getElementById('sender-email') as HTMLInputElement | null;
      if (input && !input.disabled) input.focus();
      else document.querySelector<HTMLElement>('[role="dialog"] [data-slot="dialog-title"]')?.focus();
    }} onInteractOutside={event => event.preventDefault()}>
      <QuietIconAction className="absolute right-4 top-4 z-10" aria-label="Close sender dialog" onClick={close}><Cancel01Icon /></QuietIconAction>
      {view === 'form' ? <form onSubmit={submit} noValidate className="flex min-h-0 flex-col">
        <div className="shrink-0 space-y-2 px-6 pb-5 pt-7 pr-14"><DialogTitle tabIndex={-1}>{initialSender ? 'Edit sender' : 'Add address'}</DialogTitle><DialogDescription>Choose the address customers see when you reply.</DialogDescription></div>
        <div className="min-h-0 space-y-6 overflow-y-auto px-6 pb-6">
          <div><label htmlFor="sender-email" className="mb-1 block text-xs font-medium">Email address</label>
            <QuietUnderlineInput id="sender-email" type="email" autoComplete="off" disabled={Boolean(sender) || busy} value={email} onChange={event => { setEmail(event.target.value); setError(''); }} />
            {ready && <p className="mt-2 text-xs text-quiet-positive">Domain verified. No DNS setup needed.</p>}
          </div>
          <div><div className="mb-1 flex items-center gap-1"><label htmlFor="sender-label" className="text-xs font-medium">Sender label <span className="font-normal text-muted-foreground">(optional)</span></label><QuickTooltip label="Identifies this address in settings. Replies use the teammate and workspace name."><QuietIconAction type="button" aria-label="About sender labels"><InformationCircleIcon /></QuietIconAction></QuickTooltip></div>
            <QuietUnderlineInput id="sender-label" value={label} disabled={busy} onChange={event => setLabel(event.target.value)} />
          </div>
          <div><div className="mb-1 flex items-center gap-1"><label htmlFor="sender-usage" className="text-xs font-medium">Use for inboxes</label><QuickTooltip label="The workspace default is used by Shared Inbox and inboxes without their own sender."><QuietIconAction type="button" aria-label="About inbox usage"><InformationCircleIcon /></QuietIconAction></QuickTooltip></div>
            <Select value={scope} disabled={busy} onValueChange={value => { setScope(value as Scope); setError(''); }}><SelectTrigger id="sender-usage" aria-label="Use for inboxes" variant="underline" className="w-full px-0.5"><SelectValue /></SelectTrigger><SelectContent>
              <SelectItem value="workspace">Workspace default</SelectItem><SelectItem value="mailbox">Selected inboxes</SelectItem><SelectItem value="none">{initialSender ? 'Do not use' : 'Set up only'}</SelectItem>
            </SelectContent></Select>
            {scope === 'mailbox' && <div className="mt-4">
              <QuietDropdown label="Inboxes" multiple disabled={busy} selected={inboxIds}
                options={mailboxes.map(inbox => ({ value: inbox.id, label: inbox.name }))}
                trigger={<button type="button" disabled={busy} className="flex w-full items-center justify-between gap-3 border-b border-quiet-field px-0.5 py-2 text-left text-sm focus-visible:border-quiet-text-primary focus-visible:outline-none" aria-label="Select inboxes"><span className="min-w-0 truncate">{inboxIds.length ? inboxIds.map(id => mailboxes.find(inbox => inbox.id === id)?.name ?? 'Unavailable inbox').join(', ') : 'Select inboxes'} </span><ArrowDown01Icon className="size-4 shrink-0 text-muted-foreground" /></button>}
                onSelect={id => { setInboxIds(current => current.includes(id) ? current.filter(value => value !== id) : [...current, id]); setError(''); }} empty="No available inboxes" />
            </div>}
          </div>
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        </div>
        <div className="flex shrink-0 items-center justify-between gap-4 border-t border-quiet-divider-strong px-6 py-4"><QuietTextAction type="button" onClick={close}>Cancel</QuietTextAction><QuietPrimaryAction type="submit" disabled={busy}>{busy && <Loading01Icon className="size-4 motion-safe:animate-spin" />}{busy ? 'Saving…' : ready || (sender?.active && scope === 'none') ? initialSender ? 'Save changes' : scope === 'none' ? 'Add address' : 'Add & use address' : 'Continue'}</QuietPrimaryAction></div>
      </form> : sender && <SupportEmailSenderDNSDialog email={sender.email} details={domain ?? sender} busy={busy} saving={saving} complete={view === 'complete'} manageDNS={manageDNS} error={error} notice={notice} retrySave={retrySave} onCheck={() => void checkDNS()} onClose={close} />}
    </DialogContent>
  </Dialog>;
}
