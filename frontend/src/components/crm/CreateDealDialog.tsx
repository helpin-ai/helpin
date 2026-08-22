import { useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCreateAssociation, useCreateDeal, usePipelines, useContacts } from '@/hooks/queries';
import { entityCreatedToastIcons, showEntityCreatedToast } from '@/components/ui/entity-created-toast';
import type { CRMDeal } from '@/lib/crmTypes';
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation';

interface CreateDealDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  companyContext?: { id: string; name: string };
  contactContext?: { id: string; name: string };
  onDealCreated?: (deal: CRMDeal) => void;
}

const currencyOptions = ['USD', 'EUR', 'GBP', 'CAD', 'AUD'];

export function CreateDealDialog({ open, onOpenChange, companyContext, contactContext, onDealCreated }: CreateDealDialogProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const createDeal = useCreateDeal(wsId);
  const createAssociation = useCreateAssociation(wsId);
  const { data: pipelines } = usePipelines(wsId);
  const { data: contacts } = useContacts(wsId);

  const [name, setName] = useState('');
  const [contactId, setContactId] = useState('');
  const [pipelineId, setPipelineId] = useState('');
  const [stageId, setStageId] = useState('');
  const [amount, setAmount] = useState('');
  const [currency, setCurrency] = useState('USD');
  const [closeDate, setCloseDate] = useState('');
  const [probability, setProbability] = useState('');

  const selectedContactId = contactContext?.id ?? contactId;

  const selectedPipeline = pipelines?.find((p) => p.id === pipelineId);
  const stages = selectedPipeline?.stages ?? [];

  // Auto-select first pipeline
  if (pipelines && pipelines.length > 0 && !pipelineId) {
    setPipelineId(pipelines[0].id);
    if (pipelines[0].stages && pipelines[0].stages.length > 0) {
      setStageId(pipelines[0].stages[0].id);
    }
  }

  const resetForm = () => {
    setName('');
    setContactId(contactContext?.id ?? '');
    setAmount('');
    setCurrency('USD');
    setCloseDate('');
    setProbability('');
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !selectedContactId || !pipelineId || !stageId) return;

    let deal: CRMDeal;
    try {
      deal = await createDeal.mutateAsync({
        workspace_id: wsId,
        name: name.trim(),
        contact_id: selectedContactId,
        pipeline_id: pipelineId,
        stage_id: stageId,
        amount: amount ? parseFloat(amount) : undefined,
        currency,
        close_date: closeDate ? `${closeDate}T00:00:00Z` : undefined,
        probability: probability ? parseInt(probability) : undefined,
      });
    } catch {
      toast.error('Failed to create deal');
      return;
    }

    const openDeal = () => {
      if (!currentWorkspace?.slug) return;
      openDealRoute(navigate as never, location, currentWorkspace.slug, deal.id);
    };

    if (companyContext) {
      try {
        await createAssociation.mutateAsync({
          workspace_id: wsId,
          from_object_type: 'deal',
          from_object_id: deal.id,
          to_object_type: 'company',
          to_object_id: companyContext.id,
        });
      } catch {
        onOpenChange(false);
        resetForm();
        toast.warning(`Deal created, but it could not be linked to ${companyContext.name}.`, {
          action: { label: 'Open deal', onClick: openDeal },
        });
        return;
      }
    }

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

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Deal</DialogTitle>
          <DialogDescription className="sr-only">
            Create a deal linked to the selected CRM contact.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="dealName">Deal Name *</Label>
            <Input id="dealName" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-2">
            <Label>Contact *</Label>
            <Select value={selectedContactId} onValueChange={setContactId} disabled={!!contactContext}>
              <SelectTrigger><SelectValue placeholder="Select contact" /></SelectTrigger>
              <SelectContent>
                {contacts?.data?.map((c) => (
                  <SelectItem key={c.id} value={c.id}>
                    {c.first_name} {c.last_name}{c.email ? ` (${c.email})` : ''}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {pipelines && pipelines.length > 0 && (
            <div className="space-y-2">
              <Label>Pipeline</Label>
              <Select value={pipelineId} onValueChange={(v) => { setPipelineId(v); setStageId(''); }}>
                <SelectTrigger><SelectValue placeholder="Select pipeline" /></SelectTrigger>
                <SelectContent>
                  {pipelines.map((p) => (
                    <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          {stages.length > 0 && (
            <div className="space-y-2">
              <Label>Stage</Label>
              <Select value={stageId} onValueChange={setStageId}>
                <SelectTrigger><SelectValue placeholder="Select stage" /></SelectTrigger>
                <SelectContent>
                  {stages.map((s) => (
                    <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          <div className="grid grid-cols-[1fr_100px] gap-2">
            <div className="space-y-2">
              <Label htmlFor="dealAmount">Amount</Label>
              <Input id="dealAmount" type="number" step="0.01" value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="0.00" />
            </div>
            <div className="space-y-2">
              <Label>Currency</Label>
              <Select value={currency} onValueChange={setCurrency}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {currencyOptions.map((c) => (
                    <SelectItem key={c} value={c}>{c}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="closeDate">Close Date</Label>
              <Input id="closeDate" type="date" value={closeDate} onChange={(e) => setCloseDate(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="probability">Probability %</Label>
              <Input id="probability" type="number" min="0" max="100" value={probability} onChange={(e) => setProbability(e.target.value)} placeholder="0" />
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={createDeal.isPending || createAssociation.isPending || !name.trim() || !selectedContactId || !pipelineId || !stageId}>
              {createDeal.isPending || createAssociation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
