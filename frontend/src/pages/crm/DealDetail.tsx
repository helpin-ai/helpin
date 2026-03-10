import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  ArrowLeft,
  Calendar,
  Check,
  ChevronRight,
  DollarSign,
  Gauge,
  Loader2,
  Tag,
  Trash2,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useDeal, useUpdateDeal, useDeleteDeal, useDealActivities, useDealAssociations, usePipelines } from '@/hooks/queries';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { DealHealthScore } from '@/components/crm/DealHealthScore';
import { BuyerSignals } from '@/components/crm/BuyerSignals';
import { EmailTimeline } from '@/components/crm/EmailTimeline';
import { AssociationsList } from '@/components/crm/AssociationsList';
import { useTitle } from '@/hooks/useTitle';
import type { UpdateCRMDealRequest } from '@/lib/crmTypes';

interface FormState {
  name: string;
  stage_id: string;
  amount: string;
  currency: string;
  close_date: string;
  probability: string;
}

function MetadataRow({ icon: Icon, label, children }: { icon: React.ElementType; label: string; children: React.ReactNode }) {
  return (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0 self-center text-muted-foreground" />
      <span className="self-center text-xs text-muted-foreground">{label}</span>
      <div className="min-w-0 self-center">{children}</div>
    </>
  );
}

export function DealDetailPage({ dealId }: { dealId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();

  const { data: deal, isLoading } = useDeal(wsId, dealId);
  const { data: activitiesData, refetch: refetchActivities } = useDealActivities(wsId, dealId);
  const { data: associations, refetch: refetchAssociations } = useDealAssociations(wsId, dealId);
  const { data: pipelines } = usePipelines(wsId);
  const updateDeal = useUpdateDeal(wsId);
  const deleteDeal = useDeleteDeal(wsId);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateCRMDealRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [stageOpen, setStageOpen] = useState(false);

  useTitle(form?.name ?? 'Deal');

  const currentPipeline = useMemo(() => pipelines?.find((p) => p.id === deal?.pipeline_id), [pipelines, deal?.pipeline_id]);
  const stageOptions = useMemo(
    () => (currentPipeline?.stages ?? []).sort((a, b) => a.position - b.position).map((s) => ({ value: s.id, label: s.name })),
    [currentPipeline],
  );

  useEffect(() => {
    if (deal && !form) {
      setForm({
        name: deal.name,
        stage_id: deal.stage_id,
        amount: deal.amount != null ? String(deal.amount) : '',
        currency: deal.currency ?? 'USD',
        close_date: deal.close_date ? deal.close_date.slice(0, 10) : '',
        probability: deal.probability != null ? String(deal.probability) : '',
      });
    }
  }, [deal, form]);

  useEffect(() => {
    if (saving || Object.keys(pendingPatch).length === 0 || !wsId || !dealId) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      try {
        await updateDeal.mutateAsync({ id: dealId, ...patch });
        setSaveError(null);
      } catch {
        setSaveError('Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [wsId, dealId, pendingPatch, saving]);

  const queuePatch = (patch: UpdateCRMDealRequest) => setPendingPatch((current) => ({ ...current, ...patch }));

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateCRMDealRequest) => {
    setForm((current) => (current ? { ...current, [key]: value } : current));
    queuePatch(patch);
  };

  const handleDelete = async () => {
    try {
      await deleteDeal.mutateAsync(dealId);
      toast.success('Deal deleted');
      navigate({ to: '/w/$slug/crm/deals', params: { slug: wsSlug } });
    } catch {
      toast.error('Failed to delete deal');
    }
  };

  const goBack = () => navigate({ to: '/w/$slug/crm/deals', params: { slug: wsSlug } });

  const sortedStages = useMemo(
    () => (currentPipeline?.stages ?? []).sort((a, b) => a.position - b.position),
    [currentPipeline],
  );
  const stageProgressValue = useMemo(() => {
    if (sortedStages.length === 0) return 0;
    const idx = sortedStages.findIndex((s) => s.id === form?.stage_id);
    if (idx < 0) return 0;
    // Position 0 → some progress, last stage → 100%
    return Math.round(((idx + 1) / sortedStages.length) * 100);
  }, [sortedStages, form?.stage_id]);
  const probabilityValue = form?.probability ? parseInt(form.probability) : null;
  const currentStageName = stageOptions.find((s) => s.value === form?.stage_id)?.label ?? deal?.stage?.name ?? '—';
  const formattedAmount = form?.amount
    ? new Intl.NumberFormat('en-US', { style: 'currency', currency: form.currency || 'USD', minimumFractionDigits: 0 }).format(parseFloat(form.amount))
    : '';

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!deal || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">Deal not found</p>
        <Button variant="outline" size="sm" onClick={goBack}>
          <ArrowLeft className="mr-1 h-3.5 w-3.5" />
          Back to Deals
        </Button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header bar */}
      <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={goBack}>
          <ArrowLeft className="h-4 w-4" />
        </Button>

        <div className="flex min-w-0 items-center gap-1 text-sm text-muted-foreground">
          <DollarSign className="h-3.5 w-3.5 shrink-0 text-green-500" />
          <button type="button" className="shrink-0 hover:text-foreground transition-colors cursor-pointer" onClick={goBack}>
            Deals
          </button>
          <ChevronRight className="h-3 w-3 shrink-0" />
          <span className="truncate font-medium text-foreground">{form.name || 'Untitled'}</span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <SaveIndicator saving={saving} error={saveError} />
          <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0 hover:text-destructive" onClick={() => setDeleteConfirmOpen(true)}>
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Two-column layout */}
      <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_300px]">
        {/* Left column */}
        <div className="min-h-0 overflow-y-auto px-8 py-6">
          {/* Name */}
          <input
            className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
            value={form.name}
            onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
            placeholder="Deal name"
          />
          <p className="mt-1 text-xs text-muted-foreground">{deal.display_id}</p>

          <Separator className="my-6" />

          {/* Stage Progress */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Stage Progress</h3>
              <span className="text-xs text-muted-foreground">{stageProgressValue}%</span>
            </div>
            <Progress value={stageProgressValue} />
            <p className="text-xs text-muted-foreground">
              {currentStageName}{probabilityValue != null ? ` · ${probabilityValue}% prob.` : ''}{formattedAmount ? ` · ${formattedAmount}` : ''}
            </p>
          </div>

          <Separator className="my-6" />

          {/* Activity */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Activity</h3>
            <div className="mt-3">
              <ActivityTimeline
                activities={activitiesData?.data ?? []}
                workspaceId={wsId}
                dealId={dealId}
                onActivityCreated={() => refetchActivities()}
                onActivityDeleted={() => refetchActivities()}
              />
            </div>
          </div>

          <Separator className="my-6" />

          {/* Email Timeline */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Emails</h3>
            <div className="mt-3">
              <EmailTimeline workspaceId={wsId} dealId={dealId} />
            </div>
          </div>
        </div>

        {/* Right sidebar */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6">
          <h3 className="mb-4 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Details</h3>

          <div className="grid grid-cols-[16px_80px_1fr] gap-x-2 gap-y-3">
            <MetadataRow icon={Tag} label="Stage">
              <Popover open={stageOpen} onOpenChange={setStageOpen}>
                <PopoverTrigger asChild>
                  <button type="button" className="inline-flex cursor-pointer items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent">
                    {currentStageName}
                  </button>
                </PopoverTrigger>
                <PopoverContent className="w-40 p-0.5" align="start">
                  <div className="flex max-h-60 flex-col overflow-y-auto">
                    {stageOptions.map((option) => (
                      <button key={option.value} type="button"
                        className={`flex cursor-pointer items-center gap-1.5 rounded-sm px-2 py-1 text-xs transition-colors ${form.stage_id === option.value ? 'bg-accent font-medium text-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'}`}
                        onClick={() => { updateField('stage_id', option.value, { stage_id: option.value }); setStageOpen(false); }}>
                        <span className="truncate">{option.label}</span>
                        {form.stage_id === option.value && <Check className="ml-auto h-3 w-3 shrink-0" />}
                      </button>
                    ))}
                  </div>
                </PopoverContent>
              </Popover>
            </MetadataRow>
            <MetadataRow icon={DollarSign} label="Amount">
              <div className="flex items-center gap-1">
                <input className="w-full bg-transparent text-xs outline-none" type="number" step="0.01" value={form.amount}
                  onChange={(e) => updateField('amount', e.target.value, { amount: e.target.value ? parseFloat(e.target.value) : undefined })} placeholder="—" />
                <span className="text-xs text-muted-foreground">{form.currency}</span>
              </div>
            </MetadataRow>
            <MetadataRow icon={Calendar} label="Close Date">
              <input className="w-full bg-transparent text-xs outline-none" type="date" value={form.close_date}
                onChange={(e) => updateField('close_date', e.target.value, { close_date: e.target.value ? `${e.target.value}T00:00:00Z` : undefined })} />
            </MetadataRow>
            <MetadataRow icon={Gauge} label="Probability">
              <div className="flex items-center gap-1">
                <input className="w-16 bg-transparent text-xs outline-none" type="number" min="0" max="100" value={form.probability}
                  onChange={(e) => updateField('probability', e.target.value, { probability: e.target.value ? parseInt(e.target.value) : undefined })} placeholder="—" />
                <span className="text-xs text-muted-foreground">%</span>
              </div>
            </MetadataRow>
          </div>

          <Separator className="my-4" />
          <DealHealthScore workspaceId={wsId} dealId={dealId} />

          <Separator className="my-4" />
          <BuyerSignals workspaceId={wsId} dealId={dealId} />

          <Separator className="my-4" />
          <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Associations</h3>
          <AssociationsList workspaceId={wsId} slug={wsSlug} associations={associations ?? []}
            currentObjectType="deal" currentObjectId={dealId} onAssociationRemoved={() => refetchAssociations()} />
        </aside>
      </div>

      <ConfirmDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen} title="Delete deal"
        description="Are you sure? This action cannot be undone." confirmLabel="Delete" variant="destructive" onConfirm={handleDelete} />
    </div>
  );
}
