import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Building03Icon, Search01Icon, UserIcon } from '@/lib/icons';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCreateDeal, usePipelines, useContacts } from '@/hooks/queries';
import { crmSearchService } from '@/lib/services/crmService';
import { entityCreatedToastIcons, showEntityCreatedToast } from '@/components/ui/entity-created-toast';
import type { CRMDeal, CRMObjectType, CRMSearchResult } from '@/lib/crmTypes';
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation';

interface CreateDealDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  companyContext?: { id: string; name: string };
  contactContext?: { id: string; name: string };
  onDealCreated?: (deal: CRMDeal) => void;
}

interface SelectedCustomer {
  id: string;
  type: Extract<CRMObjectType, 'contact' | 'company'>;
  name: string;
  detail?: string;
}

const currencyOptions = ['USD', 'EUR', 'GBP', 'CAD', 'AUD'];

export function CreateDealDialog({ open, onOpenChange, companyContext, contactContext, onDealCreated }: CreateDealDialogProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const createDeal = useCreateDeal(wsId);
  const { data: pipelines } = usePipelines(wsId);
  const { data: contacts } = useContacts(wsId);

  const contextualCustomer = useMemo<SelectedCustomer | null>(() => {
    if (companyContext) return { id: companyContext.id, type: 'company', name: companyContext.name };
    if (contactContext) return { id: contactContext.id, type: 'contact', name: contactContext.name };
    return null;
  }, [companyContext, contactContext]);

  const [name, setName] = useState('');
  const [customer, setCustomer] = useState<SelectedCustomer | null>(contextualCustomer);
  const [customerQuery, setCustomerQuery] = useState('');
  const [customerResults, setCustomerResults] = useState<CRMSearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [primaryContactId, setPrimaryContactId] = useState('');
  const [pipelineId, setPipelineId] = useState('');
  const [stageId, setStageId] = useState('');
  const [amount, setAmount] = useState('');
  const [currency, setCurrency] = useState('USD');
  const [closeDate, setCloseDate] = useState('');
  const [probability, setProbability] = useState('');

	const effectivePipelineId = pipelineId || pipelines?.[0]?.id || '';
	const selectedPipeline = pipelines?.find((pipeline) => pipeline.id === effectivePipelineId);
	const stages = selectedPipeline?.stages ?? [];
	const effectiveStageId = stageId || [...stages].sort((a, b) => a.position - b.position)[0]?.id || '';

	useEffect(() => {
		if (!open || contextualCustomer || customer || customerQuery.trim().length < 2) {
			return;
    }
    let cancelled = false;
    const timer = window.setTimeout(async () => {
      setSearching(true);
      const response = await crmSearchService.search(wsId, customerQuery.trim());
      if (cancelled) return;
      setCustomerResults((response.data ?? []).filter((item) => item.type === 'contact' || item.type === 'company'));
      setSearching(false);
    }, 220);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [contextualCustomer, customer, customerQuery, open, wsId]);

  const resetForm = () => {
    setName('');
    setCustomer(contextualCustomer);
    setCustomerQuery('');
    setCustomerResults([]);
    setPrimaryContactId('');
    setAmount('');
    setCurrency('USD');
    setCloseDate('');
    setProbability('');
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
		if (!name.trim() || !customer || !effectivePipelineId || !effectiveStageId) return;

    let deal: CRMDeal;
    try {
      deal = await createDeal.mutateAsync({
        workspace_id: wsId,
        name: name.trim(),
        ...(customer.type === 'company'
          ? { company_id: customer.id, contact_id: primaryContactId || undefined }
          : { contact_id: customer.id }),
			pipeline_id: effectivePipelineId,
			stage_id: effectiveStageId,
        amount: amount ? Number.parseFloat(amount) : undefined,
        currency,
        close_date: closeDate ? `${closeDate}T00:00:00Z` : undefined,
        probability: probability ? Number.parseInt(probability, 10) : undefined,
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to create deal');
      return;
    }

    const openDeal = () => {
      if (!currentWorkspace?.slug) return;
      openDealRoute(navigate as never, location, currentWorkspace.slug, deal.id);
    };
    onDealCreated?.(deal);
    showEntityCreatedToast({
      entityLabel: 'Deal',
      title: deal.name,
      subtitle: deal.amount ? `${deal.currency} ${deal.amount.toLocaleString()}` : undefined,
      tone: 'crm',
      icon: entityCreatedToastIcons.deal,
      onOpen: currentWorkspace?.slug ? openDeal : undefined,
    });
    onOpenChange(false);
    resetForm();
  };

  const selectCustomer = (result: CRMSearchResult) => {
    if (result.type !== 'contact' && result.type !== 'company') return;
    setCustomer({ id: result.id, type: result.type, name: result.name, detail: result.detail });
    setCustomerQuery('');
    setCustomerResults([]);
    setPrimaryContactId('');
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader className="border-b border-quiet-divider-strong pb-3">
          <DialogTitle className="text-[20px] font-semibold tracking-[-0.018em]">Create deal</DialogTitle>
          <DialogDescription className="text-sm text-quiet-text-tertiary">Every deal belongs to one company or independent contact.</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-5">
          <div className="space-y-1">
            <Label htmlFor="dealName" className="text-sm font-medium text-quiet-text-secondary">Deal name</Label>
            <QuietUnderlineInput id="dealName" value={name} onChange={(event) => setName(event.target.value)} placeholder="Name the opportunity" required />
          </div>

          <div className="space-y-1.5">
            <Label className="text-sm font-medium text-quiet-text-secondary">Customer</Label>
            {customer ? (
              <div className="flex items-center gap-2 border-b border-quiet-field py-2 text-sm">
                {customer.type === 'company' ? <Building03Icon className="h-4 w-4 text-quiet-muted" /> : <UserIcon className="h-4 w-4 text-quiet-muted" />}
                <span className="min-w-0 flex-1 truncate font-medium text-quiet-text-primary">{customer.name}</span>
                <span className="text-[11.5px] uppercase tracking-[0.03em] text-quiet-muted">{customer.type}</span>
                {!contextualCustomer ? <QuietTextAction type="button" onClick={() => setCustomer(null)}>Change</QuietTextAction> : null}
              </div>
            ) : (
              <div>
                <div className="relative">
                  <Search01Icon className="pointer-events-none absolute left-0.5 top-1/2 h-4 w-4 -translate-y-1/2 text-quiet-muted" />
				  <QuietUnderlineInput className="pl-6" value={customerQuery} onChange={(event) => { const value = event.target.value; setCustomerQuery(value); if (value.trim().length < 2) { setCustomerResults([]); setSearching(false); } }} placeholder="Search contacts or companies" autoFocus />
                </div>
                {customerQuery.trim().length >= 2 ? (
                  <div className="max-h-52 overflow-y-auto border-b border-quiet-divider-strong">
                    {searching ? <p className="py-3 text-sm text-quiet-muted">Searching…</p> : null}
                    {!searching && customerResults.length === 0 ? <p className="py-3 text-sm text-quiet-muted">No matching customer</p> : null}
                    {customerResults.map((result) => (
                      <button key={`${result.type}:${result.id}`} type="button" onClick={() => selectCustomer(result)} className="flex w-full items-center gap-2 border-t border-quiet-divider-light py-2.5 text-left hover:bg-quiet-row-hover">
                        {result.type === 'company' ? <Building03Icon className="h-4 w-4 text-quiet-muted" /> : <UserIcon className="h-4 w-4 text-quiet-muted" />}
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm font-medium text-quiet-text-primary">{result.name}</span>
                          <span className="block truncate text-[11.5px] text-quiet-muted">{result.detail}</span>
                        </span>
                      </button>
                    ))}
                  </div>
                ) : null}
              </div>
            )}
            {customer?.type === 'contact' ? <p className="text-[11.5px] text-quiet-muted">If this contact has a primary company, that company becomes the customer.</p> : null}
          </div>

          {customer?.type === 'company' ? (
            <div className="space-y-1">
              <Label className="text-sm font-medium text-quiet-text-secondary">Primary contact <span className="font-normal text-quiet-muted">optional</span></Label>
              <Select value={primaryContactId || '__none__'} onValueChange={(value) => setPrimaryContactId(value === '__none__' ? '' : value)}>
                <SelectTrigger variant="underline" className="w-full px-0.5"><SelectValue placeholder="No primary contact" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">No primary contact</SelectItem>
                  {contacts?.data?.map((contact) => <SelectItem key={contact.id} value={contact.id}>{contact.first_name} {contact.last_name}{contact.email ? ` · ${contact.email}` : ''}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
          ) : null}

          <div className="grid gap-4 sm:grid-cols-2">
			<div className="space-y-1"><Label className="text-sm font-medium text-quiet-text-secondary">Pipeline</Label><Select value={effectivePipelineId} onValueChange={(value) => { setPipelineId(value); setStageId(''); }}><SelectTrigger variant="underline" className="w-full px-0.5"><SelectValue placeholder="Select pipeline" /></SelectTrigger><SelectContent>{pipelines?.map((pipeline) => <SelectItem key={pipeline.id} value={pipeline.id}>{pipeline.name}</SelectItem>)}</SelectContent></Select></div>
			<div className="space-y-1"><Label className="text-sm font-medium text-quiet-text-secondary">Stage</Label><Select value={effectiveStageId} onValueChange={setStageId}><SelectTrigger variant="underline" className="w-full px-0.5"><SelectValue placeholder="Select stage" /></SelectTrigger><SelectContent>{stages.map((stage) => <SelectItem key={stage.id} value={stage.id}>{stage.name}</SelectItem>)}</SelectContent></Select></div>
          </div>

          <div className="grid grid-cols-[minmax(0,1fr)_90px] gap-4">
            <div className="space-y-1"><Label htmlFor="dealAmount" className="text-sm font-medium text-quiet-text-secondary">Amount</Label><QuietUnderlineInput id="dealAmount" type="number" step="0.01" value={amount} onChange={(event) => setAmount(event.target.value)} placeholder="0.00" /></div>
            <div className="space-y-1"><Label className="text-sm font-medium text-quiet-text-secondary">Currency</Label><Select value={currency} onValueChange={setCurrency}><SelectTrigger variant="underline" className="w-full px-0.5"><SelectValue /></SelectTrigger><SelectContent>{currencyOptions.map((option) => <SelectItem key={option} value={option}>{option}</SelectItem>)}</SelectContent></Select></div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1"><Label htmlFor="closeDate" className="text-sm font-medium text-quiet-text-secondary">Close date</Label><QuietUnderlineInput id="closeDate" type="date" value={closeDate} onChange={(event) => setCloseDate(event.target.value)} /></div>
            <div className="space-y-1"><Label htmlFor="probability" className="text-sm font-medium text-quiet-text-secondary">Probability</Label><QuietUnderlineInput id="probability" type="number" min="0" max="100" value={probability} onChange={(event) => setProbability(event.target.value)} placeholder="Percent" /></div>
          </div>

          <DialogFooter className="border-t border-quiet-divider-strong pt-3">
            <QuietTextAction type="button" onClick={() => onOpenChange(false)}>Cancel</QuietTextAction>
			<QuietPrimaryAction type="submit" disabled={createDeal.isPending || !name.trim() || !customer || !effectivePipelineId || !effectiveStageId}>{createDeal.isPending ? 'Creating…' : 'Create deal'}</QuietPrimaryAction>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
