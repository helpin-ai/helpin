import { useDeferredValue, useState } from 'react';
import { toast } from 'sonner';
import { QuietEmptyState, QuietPrimaryAction, QuietSearchInput, QuietStatusText, QuietTextAction } from '@/components/design-system/quiet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useCRMPlaybookParticipants, useCRMPlaybookPreview, useCRMPlaybookWrite } from '@/hooks/queries/useCRMPlaybooks';
import { attentionLabels, createPlaybookIntentKey } from '@/lib/crmPlaybookPresentation';
import type { CRMPlaybookItem } from '@/lib/crmPlaybookTypes';
import type { CRMSituationItem } from '@/lib/crmSituationTypes';
import { PlaybookError, PlaybookLoading, PlaybookPagination, PlaybookSelect } from './PlaybookUI';
import { SignalDrawer } from '../signals/SignalDrawer';

function SignalTable({ signals, onOpen, preview = false, busy = false }: { signals: CRMSituationItem[]; onOpen: (signal: CRMSituationItem) => void; preview?: boolean; busy?: boolean }) {
  return <Table aria-label={preview ? 'Matching signals' : 'Playbook signals'} aria-busy={busy}><TableHeader><TableRow><TableHead>Customer / signal</TableHead><TableHead>Owner</TableHead>{!preview && <TableHead>Milestones</TableHead>}<TableHead>Next step</TableHead><TableHead>Status</TableHead></TableRow></TableHeader>
    <TableBody>{signals.map((item) => {
      const progress = item.situation.playbook_milestones ?? [];
      const achieved = progress.filter((entry) => entry.status === 'achieved').length;
      const skipped = progress.filter((entry) => entry.status === 'not_applicable').length;
      return <TableRow key={item.situation.id}>
        <TableCell className="min-w-60 max-w-sm whitespace-normal"><button type="button" onClick={() => onOpen(item)} className="text-left font-semibold text-quiet-text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-quiet-field">{item.company_name || item.contact_name || item.deal_name || 'Customer unavailable'}</button><p className="mt-1 text-xs leading-5 text-quiet-text-tertiary">{item.situation.title}</p></TableCell>
        <TableCell className="max-w-48 whitespace-normal">{item.owner_name || 'Unassigned'}{item.owner_name && !item.owner_available && <span className="block text-xs text-quiet-accent">Unavailable</span>}</TableCell>
        {!preview && <TableCell className="text-xs tabular-nums text-quiet-text-secondary">{achieved} / {progress.length} achieved{skipped > 0 && <span className="block">{skipped} not applicable</span>}</TableCell>}
        <TableCell className="min-w-48 max-w-sm whitespace-normal text-quiet-text-secondary">{item.situation.lifecycle === 'closed' ? item.situation.outcome_summary || 'Outcome recorded' : item.situation.next_step || 'Not set'}</TableCell>
        <TableCell><QuietStatusText tone={item.situation.lifecycle !== 'open' ? 'neutral' : ['needs_approval', 'automation_failed', 'follow_up_due'].includes(item.effective_attention) ? 'blocker' : 'neutral'}>{item.situation.lifecycle !== 'open' ? item.situation.lifecycle === 'paused' ? 'Paused' : 'Closed' : attentionLabels[item.effective_attention]}</QuietStatusText></TableCell>
      </TableRow>;
    })}</TableBody>
  </Table>;
}

export function PlaybookParticipants({ ws, slug, item, canEdit }: { ws: string; slug: string; item: CRMPlaybookItem; canEdit: boolean }) {
  const [search, setSearch] = useState('');
  const [state, setState] = useState('all');
  const [scope, setScope] = useState('all');
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<string>();
  const query = useDeferredValue(search.trim());
  const participants = useCRMPlaybookParticipants(ws, item.playbook.id, { q: query, state, scope, page });
  const clear = () => { setSearch(''); setState('all'); setScope('all'); setPage(1); };
  const filtered = !!search || state !== 'all' || scope !== 'all';
  return <>
    <div className="flex flex-wrap items-center gap-3 border-b border-quiet-divider-strong py-4">
      <QuietSearchInput aria-label="Search playbook signals" placeholder="Search customers or signals…" value={search} onChange={(event) => { setSearch(event.target.value); setPage(1); }} containerClassName="w-64 max-w-full" />
      <PlaybookSelect label="Signal status" value={state} onChange={(value) => { setState(value); setPage(1); }} options={[{ value: 'all', label: 'All statuses' }, { value: 'needs_attention', label: 'Needs attention' }, { value: 'open', label: 'Open' }, { value: 'waiting', label: 'Waiting' }, { value: 'paused', label: 'Paused' }, { value: 'closed', label: 'Closed' }]} />
      <PlaybookSelect label="Assignment" value={scope} onChange={(value) => { setScope(value); setPage(1); }} options={[{ value: 'all', label: 'Everyone' }, { value: 'mine', label: 'Assigned to me' }, { value: 'my_teams', label: 'My teams' }, { value: 'unassigned', label: 'Unassigned' }]} />
      {filtered && <QuietTextAction onClick={clear}>Clear filters</QuietTextAction>}
    </div>
    {participants.isPending ? <PlaybookLoading /> : participants.isError ? <PlaybookError error={participants.error} retry={() => void participants.refetch()} /> : <>
      {participants.data.data.length ? <SignalTable signals={participants.data.data} onOpen={(signal) => setSelected(signal.situation.id)} busy={participants.isFetching || search.trim() !== query} /> : <QuietEmptyState title={filtered ? 'No matching signals' : 'No signals in this playbook'} description={filtered ? 'Try another search, status, or assignment.' : item.playbook.accepting_customers ? 'Use Add signals to review eligible work and apply this playbook.' : 'Publish and allow enrollment before adding existing signals.'} action={filtered ? <QuietTextAction onClick={clear}>Clear filters</QuietTextAction> : undefined} />}
      <PlaybookPagination page={page} total={participants.data.total} pageSize={participants.data.page_size} onChange={setPage} busy={participants.isFetching} />
    </>}
    {selected && <SignalDrawer key={selected} ws={ws} slug={slug} playbookId={item.playbook.id} signalId={selected} canEdit={canEdit} onClose={() => setSelected(undefined)} />}
  </>;
}

export function PlaybookPreview({ ws, item, mode, canEdit, onClose, onReload }: { ws: string; item: CRMPlaybookItem; mode: 'draft' | 'published'; canEdit: boolean; onClose: () => void; onReload: () => void }) {
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<CRMSituationItem>();
  const [intentKey] = useState(createPlaybookIntentKey);
  const versionId = mode === 'published' ? item.playbook.published_version_id ?? undefined : undefined;
  const preview = useCRMPlaybookPreview(ws, item.playbook.id, item.playbook.revision, versionId, page);
  const write = useCRMPlaybookWrite(ws);
  const canApply = mode === 'published' && !!versionId && item.playbook.accepting_customers && canEdit;
  const apply = async () => {
    if (!selected || !versionId || !preview.data || !canApply) return;
    const intent = { situation_id: selected.situation.id, expected_situation_revision: selected.situation.revision, version_id: versionId, expected_playbook_revision: preview.data.playbook_revision, confirmed: true as const };
    try {
      await write.mutateAsync({ kind: 'apply', id: item.playbook.id, body: { ...intent, command_key: intentKey(intent) } });
      setSelected(undefined);
      toast.success('Playbook applied. Ownership and next steps are unchanged.');
    } catch { /* Keep the exact confirmation and retry key. */ }
  };
  return <Sheet open onOpenChange={(open) => { if (!open && !write.isPending) onClose(); }}><SheetContent className="overflow-y-auto data-[side=right]:w-full data-[side=right]:sm:max-w-5xl">
    <SheetHeader className="pr-14"><SheetTitle>{mode === 'draft' ? 'Preview matches' : 'Add signals'}</SheetTitle><SheetDescription>{mode === 'draft' ? 'Existing open signals that match the saved draft. Previewing does not change anything.' : 'Existing open signals that match the published playbook and are not already in another playbook.'}</SheetDescription></SheetHeader>
    <div className="px-6 pb-6">
      {preview.isPending ? <PlaybookLoading /> : preview.isError ? <PlaybookError error={preview.error} retry={() => { onReload(); void preview.refetch(); }} /> : <>
        <p className="py-3 text-xs text-quiet-text-tertiary">{preview.data.signals.total} matching signals · This is not a scan of every CRM customer.</p>
        {preview.data.signals.data.length ? <SignalTable signals={preview.data.signals.data} preview onOpen={setSelected} busy={preview.isFetching} /> : <QuietEmptyState title="No matching signals" description="Only open signals with available CRM records can qualify. Signals already in a playbook are excluded." />}
        <PlaybookPagination page={page} total={preview.data.signals.total} pageSize={preview.data.signals.page_size} onChange={setPage} busy={preview.isFetching} />
      </>}
    </div>
    <Dialog open={!!selected} onOpenChange={(open) => { if (!open && !write.isPending) { setSelected(undefined); write.reset(); } }}><DialogContent>
      <DialogHeader><DialogTitle>{canApply ? 'Apply playbook?' : selected?.situation.title}</DialogTitle><DialogDescription>{canApply ? `Apply “${item.published_version?.definition.name || item.playbook.draft.name}” to this signal. Its owner and next step stay unchanged. No messages, tasks, Flows, or Agents will start.` : 'This signal matches the selected playbook definition.'}</DialogDescription></DialogHeader>
      <div className="space-y-2 text-sm"><p className="font-medium">{selected?.company_name || selected?.contact_name || selected?.deal_name}</p>{canApply && <p>{selected?.situation.title}</p>}<p className="text-quiet-text-secondary">{selected?.situation.objective}</p><p className="text-quiet-text-tertiary">{selected?.situation.next_step || 'No next step set'}</p></div>
      {write.isError && <PlaybookError error={write.error} />}
      <DialogFooter><QuietTextAction disabled={write.isPending} onClick={() => { setSelected(undefined); write.reset(); }}>{canApply ? 'Cancel' : 'Close'}</QuietTextAction>{canApply && <QuietPrimaryAction disabled={write.isPending || preview.isFetching || preview.isError} onClick={() => void apply()}>{write.isPending ? 'Applying…' : 'Apply playbook'}</QuietPrimaryAction>}</DialogFooter>
    </DialogContent></Dialog>
  </SheetContent></Sheet>;
}
