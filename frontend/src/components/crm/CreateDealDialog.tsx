import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useCreateDeal, usePipelines } from '@/hooks/queries';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { crmContactService } from '@/lib/services/crmService';
import { unwrapRequired } from '@/lib/queryUtils';
import { entityCreatedToastIcons, showEntityCreatedToast } from '@/components/ui/entity-created-toast';
import type { CRMDeal } from '@/lib/crmTypes';
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation';
import { DealEntityPicker, type DealEntity } from './DealEntityPicker';
import { generatedDealName, resolveDealStage, type RevenueType } from './dealCreationDefaults';

interface CreateDealDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  companyContext?: DealEntity;
  contactContext?: DealEntity;
  initialPipelineId?: string;
  initialStageId?: string;
  onDealCreated?: (deal: CRMDeal) => void;
}
const currencies = ['USD', 'EUR', 'GBP', 'CAD', 'AUD'];
function Field({ label, id, children }: { label: string; id?: string; children: ReactNode }) {
  return <div className="min-w-0 space-y-1"><Label htmlFor={id} className="text-sm font-medium text-quiet-text-secondary">{label}</Label>{children}</div>;
}
function Choice({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: { value: string; label: string }[] }) {
  return <Select value={value} onValueChange={onChange}><SelectTrigger aria-label={label} variant="underline" className="w-full px-0.5"><SelectValue placeholder={`Select ${label.toLowerCase()}`} /></SelectTrigger><SelectContent>{options.map(o => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}</SelectContent></Select>;
}
export function CreateDealDialog(props: CreateDealDialogProps) {
  return props.open ? <CreateDealForm {...props} /> : null;
}
function CreateDealForm({ onOpenChange, companyContext, contactContext, initialPipelineId, initialStageId, onDealCreated }: CreateDealDialogProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const user = useAuthStore(s => s.user);
  const createDeal = useCreateDeal(wsId);
  const { data: pipelines = [] } = usePipelines(wsId);
  const { members } = useAssignableWorkspaceMembers(wsId);
  const [company, setCompany] = useState<DealEntity | null>(companyContext ?? null);
  const [people, setPeople] = useState<DealEntity[]>(contactContext ? [contactContext] : []);
  const [editedName, setEditedName] = useState<string | null>(null);
  const [pipelineId, setPipelineId] = useState(initialPipelineId);
  const [stageId, setStageId] = useState(initialStageId);
  const [amount, setAmount] = useState('');
  const [currency, setCurrency] = useState(() => { try { return localStorage.getItem(`crm-deal-currency:${wsId}`) || 'USD'; } catch { return 'USD'; } });
  const [revenueType, setRevenueType] = useState<RevenueType>('one_time');
  const [closeDate, setCloseDate] = useState('');
  const [owner, setOwner] = useState<string | null>(null);
  const [probability, setProbability] = useState<string | null>(null);
  const [pendingPerson, setPendingPerson] = useState<{ person: DealEntity; company: DealEntity } | null>(null);
  const [resolving, setResolving] = useState(!!contactContext);
  const [entityDraft, setEntityDraft] = useState(false);
  const selectionVersion = useRef(0);
  const companyRef = useRef(company);
  const name = editedName ?? generatedDealName(company?.name, people[0]?.name);
  const effective = resolveDealStage(pipelines, pipelineId, stageId);
  const stages = [...(pipelines.find(p => p.id === effective.pipelineId)?.stages ?? [])].sort((a,b) => a.position - b.position);
  const ownerId = owner ?? members.find(m => m.user_id === user?.id)?.id ?? '';
  const selectCompany = (next: DealEntity | null) => { companyRef.current = next; setCompany(next); };
  const addPerson = (person: DealEntity) => setPeople(current => current.some(p => p.id === person.id) ? current : [...current, person]);
  const selectPerson = async (person: DealEntity) => {
    const version = ++selectionVersion.current;
    setResolving(true);
    try {
      const associations = unwrapRequired(await crmContactService.listAssociations(wsId, person.id), 'Contact company');
      if (version !== selectionVersion.current) return;
      const primary = associations.find(a => a.association_label === 'primary' && (a.from_object_type === 'company' || a.to_object_type === 'company'));
      const linkedCompany = primary ? { id: primary.from_object_type === 'company' ? primary.from_object_id : primary.to_object_id, name: primary.linked_object_name } : null;
      if (linkedCompany && companyRef.current && companyRef.current.id !== linkedCompany.id) { setPendingPerson({ person, company: linkedCompany }); return; }
      if (linkedCompany && !companyRef.current) selectCompany(linkedCompany);
      addPerson(person);
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Could not load contact company'); }
    finally { if (version === selectionVersion.current) setResolving(false); }
  };
  useEffect(() => {
    let active = true;
    const sequence = selectionVersion;
    queueMicrotask(() => { if (active && contactContext) void selectPerson(contactContext); });
    return () => { active = false; sequence.current++; };
    // Context is fixed for this mounted form. Async selection has a stale-response guard.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!name.trim() || (!company && !people.length) || !effective.pipelineId || !effective.stageId || resolving || pendingPerson || entityDraft) return;
    try {
      const deal = await createDeal.mutateAsync({
        workspace_id: wsId, name: name.trim(), company_id: company?.id,
        contact_id: people[0]?.id, contact_ids: people.slice(1).map(p => p.id),
        pipeline_id: effective.pipelineId, stage_id: effective.stageId,
        amount: amount === '' ? undefined : Number(amount), currency, revenue_type: revenueType,
        owner_member_id: ownerId || undefined,
        close_date: closeDate ? `${closeDate}T00:00:00Z` : undefined,
        probability: probability === null || probability === '' ? undefined : Number(probability),
      });
      try { localStorage.setItem(`crm-deal-currency:${wsId}`, currency); } catch { /* Storage is optional. */ }
      onDealCreated?.(deal);
      showEntityCreatedToast({ entityLabel: 'Deal', title: deal.name, identifier: deal.display_id ? { label: 'Deal ID', value: deal.display_id } : undefined, tone: 'crm', icon: entityCreatedToastIcons.deal,
        onOpen: currentWorkspace?.slug ? () => openDealRoute(navigate as never, location, currentWorkspace.slug, deal.id) : undefined });
      onOpenChange(false);
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Failed to create deal'); }
  };
  return <Dialog open onOpenChange={next => { if (!createDeal.isPending) onOpenChange(next); }}>
    <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl" aria-describedby={undefined}>
      <DialogHeader className="border-b border-quiet-divider-strong pb-3"><DialogTitle className="text-[20px] font-semibold tracking-[-0.018em]">Create deal</DialogTitle></DialogHeader>
      <form onSubmit={handleSubmit} className="space-y-4">
        <fieldset disabled={createDeal.isPending} className="min-w-0 space-y-4">
          <Field label="Company">{company ? <div className="flex items-center gap-2 border-b border-quiet-field py-2 text-sm"><span className="min-w-0 flex-1 truncate">{company.name}</span><QuietTextAction type="button" disabled={entityDraft || resolving || !!pendingPerson} onClick={() => selectCompany(null)}>Change</QuietTextAction></div> : <DealEntityPicker workspaceId={wsId} kind="company" onDraftChange={setEntityDraft} onSelect={selectCompany} disabled={entityDraft || resolving || !!pendingPerson} />}</Field>
          <Field label="Contacts">
            {people.map((person,index) => <div key={person.id} className="flex items-center gap-2 py-1 text-sm"><span className="min-w-0 flex-1 truncate">{person.name}</span>{index === 0 ? <span className="text-xs text-quiet-muted">Primary</span> : <QuietTextAction type="button" onClick={() => setPeople([person, ...people.filter(p => p.id !== person.id)])}>Make primary</QuietTextAction>}<QuietTextAction type="button" aria-label={`Remove ${person.name}`} onClick={() => setPeople(people.filter(p => p.id !== person.id))}>Remove</QuietTextAction></div>)}
            <DealEntityPicker workspaceId={wsId} kind="contact" onDraftChange={setEntityDraft} company={company} excludedIds={people.map(p => p.id)} onSelect={person => void selectPerson(person)} disabled={entityDraft || resolving || !!pendingPerson} />
            {resolving && <p role="status" className="text-xs text-quiet-muted">Checking company…</p>}
            {pendingPerson && <div className="space-y-2 rounded-md border border-quiet-divider-strong p-3 text-sm"><p>{pendingPerson.person.name} is associated with {pendingPerson.company.name}.</p><div className="flex flex-wrap gap-3"><QuietTextAction type="button" onClick={() => { addPerson(pendingPerson.person); setPendingPerson(null); }}>Keep {company?.name}</QuietTextAction><QuietTextAction type="button" onClick={() => { selectCompany(pendingPerson.company); addPerson(pendingPerson.person); setPendingPerson(null); }}>Switch company</QuietTextAction><QuietTextAction type="button" onClick={() => setPendingPerson(null)}>Cancel</QuietTextAction></div></div>}
          </Field>
          <Field label="Deal name" id="dealName"><QuietUnderlineInput id="dealName" value={name} onChange={e => setEditedName(e.target.value)} placeholder="Name the opportunity" required /></Field>
          <div className="grid grid-cols-[minmax(0,1fr)_80px_minmax(0,1fr)] gap-3">
            <Field label="Amount" id="dealAmount"><QuietUnderlineInput id="dealAmount" type="number" min="0" step="0.01" value={amount} onChange={e => setAmount(e.target.value)} placeholder="0.00" /></Field>
            <Field label="Currency"><Choice label="Currency" value={currency} onChange={setCurrency} options={currencies.map(value => ({ value, label: value }))} /></Field>
            <Field label="Revenue type"><Choice label="Revenue type" value={revenueType} onChange={value => setRevenueType(value as RevenueType)} options={[{value:'one_time',label:'One-time'},{value:'monthly',label:'Monthly'},{value:'annual',label:'Annual'}]} /></Field>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <Field label="Pipeline"><Choice label="Pipeline" value={effective.pipelineId} onChange={value => { setPipelineId(value); setStageId(undefined); }} options={pipelines.map(p => ({value:p.id,label:p.name}))} /></Field>
            <Field label="Stage"><Choice label="Stage" value={effective.stageId} onChange={setStageId} options={stages.map(s => ({value:s.id,label:s.name}))} /></Field>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <Field label="Owner"><Choice label="Owner" value={ownerId || '__none__'} onChange={value => setOwner(value === '__none__' ? '' : value)} options={[{value:'__none__',label:'Unassigned'},...members.map(m => ({value:m.id,label:m.display_name || m.email}))]} /></Field>
            <Field label="Expected close date" id="closeDate"><QuietUnderlineInput id="closeDate" type="date" value={closeDate} onChange={e => setCloseDate(e.target.value)} /></Field>
          </div>
          <Field label="Probability (%)" id="probability"><QuietUnderlineInput id="probability" type="number" min="0" max="100" step="1" value={probability ?? String(effective.probability)} onChange={e => setProbability(e.target.value === '' ? null : e.target.value)} /></Field>
        </fieldset>
        <DialogFooter className="border-t border-quiet-divider-strong pt-3"><QuietTextAction type="button" disabled={createDeal.isPending} onClick={() => onOpenChange(false)}>Cancel</QuietTextAction><QuietPrimaryAction type="submit" disabled={createDeal.isPending || entityDraft || resolving || !!pendingPerson || !name.trim() || (!company && !people.length) || !effective.pipelineId || !effective.stageId}>{createDeal.isPending ? 'Creating…' : 'Create deal'}</QuietPrimaryAction></DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
}
