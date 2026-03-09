import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  ArrowLeft,
  Calendar,
  Check,
  DollarSign,
  Gauge,
  Layers,
  MoreHorizontal,
  Percent,
  Trash2,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  useDeal,
  useDealActivities,
  useDealAssociations,
  useUpdateDeal,
  useDeleteDeal,
  usePipelines,
} from '@/hooks/queries';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { DealHealthScore } from '@/components/crm/DealHealthScore';
import { BuyerSignals } from '@/components/crm/BuyerSignals';
import { EmailTimeline } from '@/components/crm/EmailTimeline';
import { LinkedPMItems } from '@/components/crm/LinkedPMItems';
import { useTitle } from '@/hooks/useTitle';
import type { UpdateCRMDealRequest, CRMPipelineStage, CRMAssociationEnriched } from '@/lib/crmTypes';

// ── Types ──

interface FormState {
  name: string;
  pipeline_id: string;
  stage_id: string;
  amount: string;
  currency: string;
  close_date: string;
  probability: string;
}

// ── Helpers ──

function MetadataRow({ icon: Icon, label, children }: { icon: React.ElementType; label: string; children: React.ReactNode }) {
  return (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
      <span className="text-xs text-muted-foreground self-center">{label}</span>
      <div className="min-w-0 self-center">{children}</div>
    </>
  );
}

function SidebarPopoverSelect<T extends string>({
  value,
  options,
  onChange,
  renderTrigger,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
  renderTrigger: () => React.ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        >
          {renderTrigger()}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-40 p-0.5" align="start">
        <div className="flex max-h-60 flex-col overflow-y-auto">
          {options.map((option) => (
            <button
              key={option.value}
              type="button"
              className={`flex items-center gap-1.5 rounded-sm px-2 py-1 text-xs transition-colors cursor-pointer ${
                value === option.value
                  ? 'bg-accent text-foreground font-medium'
                  : 'text-muted-foreground hover:bg-accent hover:text-foreground'
              }`}
              onClick={() => {
                onChange(option.value);
                setOpen(false);
              }}
            >
              <span className="truncate">{option.label}</span>
              {value === option.value && <Check className="ml-auto h-3 w-3 shrink-0" />}
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}

// ── Main Page ──

export function DealDetailPage({ dealId }: { dealId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();

  const { data: deal, isLoading } = useDeal(wsId, dealId);
  const { data: activitiesData } = useDealActivities(wsId, dealId);
  const { data: associations } = useDealAssociations(wsId, dealId);
  const { data: pipelines } = usePipelines(wsId);
  const updateMutation = useUpdateDeal(wsId);
  const deleteMutation = useDeleteDeal(wsId);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateCRMDealRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  useTitle(deal?.name ?? 'Deal');

  // Derive stages from selected pipeline
  const currentPipeline = useMemo(
    () => pipelines?.find((p) => p.id === form?.pipeline_id),
    [pipelines, form?.pipeline_id],
  );

  const stageOptions = useMemo<{ value: string; label: string }[]>(() => {
    if (!currentPipeline?.stages) return [];
    return currentPipeline.stages
      .sort((a: CRMPipelineStage, b: CRMPipelineStage) => a.position - b.position)
      .map((s: CRMPipelineStage) => ({ value: s.id, label: s.name }));
  }, [currentPipeline]);

  const currentStageName = useMemo(() => {
    if (!form?.stage_id || !currentPipeline?.stages) return '--';
    return currentPipeline.stages.find((s: CRMPipelineStage) => s.id === form.stage_id)?.name ?? '--';
  }, [form?.stage_id, currentPipeline]);

  // Initialize form from deal data
  useEffect(() => {
    if (deal && !form) {
      setForm({
        name: deal.name,
        pipeline_id: deal.pipeline_id,
        stage_id: deal.stage_id,
        amount: deal.amount != null ? String(deal.amount) : '',
        currency: deal.currency ?? 'USD',
        close_date: deal.close_date ? deal.close_date.slice(0, 10) : '',
        probability: deal.probability != null ? String(deal.probability) : '',
      });
    }
  }, [deal, form]);

  // Auto-save debounce
  useEffect(() => {
    if (saving || Object.keys(pendingPatch).length === 0 || !wsId || !dealId) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      try {
        await updateMutation.mutateAsync({ id: dealId, ...patch });
        setSaveError(null);
      } catch {
        setSaveError('Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [wsId, dealId, pendingPatch, saving]);

  const queuePatch = (patch: UpdateCRMDealRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateCRMDealRequest) => {
    setForm((current) => (current ? { ...current, [key]: value } : current));
    queuePatch(patch);
  };

  const handleDelete = async () => {
    try {
      await deleteMutation.mutateAsync(dealId);
      toast.success('Deal deleted');
      navigate({ to: '/w/$slug/crm/deals', params: { slug: wsSlug } });
    } catch {
      toast.error('Failed to delete deal');
    }
  };

  const goBack = () => navigate({ to: '/w/$slug/crm/deals', params: { slug: wsSlug } });

  // Split associations into CRM vs PM
  const crmAssocs = useMemo(() => {
    return (associations ?? []).filter((a: CRMAssociationEnriched) => {
      const otherType = a.from_object_type === 'deal' ? a.to_object_type : a.from_object_type;
      return otherType === 'contact' || otherType === 'company';
    });
  }, [associations]);

  if (isLoading) {
    return (
      <div className="mx-auto max-w-5xl px-4 md:px-6">
        <Skeleton className="mb-4 h-8 w-48" />
        <div className="grid gap-6 lg:grid-cols-[1fr_280px]">
          <div className="space-y-4">
            <Skeleton className="h-10 w-3/4" />
            <Skeleton className="h-32 w-full" />
          </div>
          <div className="space-y-3">
            <Skeleton className="h-48 w-full" />
          </div>
        </div>
      </div>
    );
  }

  if (!deal || !form) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Deal not found</div>;
  }

  return (
    <div className="mx-auto max-w-5xl px-4 md:px-6">
      {/* Header */}
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="icon" onClick={goBack}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <p className="text-xs text-muted-foreground">{deal.display_id}</p>
            <input
              className="text-2xl font-bold bg-transparent border-none outline-none focus:outline-none w-full"
              value={form.name}
              onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
              placeholder="Deal name"
            />
          </div>
        </div>
        <div className="flex items-center gap-2">
          <SaveIndicator saving={saving} error={saveError} />
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                className="text-destructive focus:text-destructive"
                onClick={() => setDeleteConfirmOpen(true)}
              >
                <Trash2 className="mr-2 h-4 w-4" />
                Delete deal
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* Two-column layout */}
      <div className="grid gap-6 lg:grid-cols-[1fr_280px]">
        {/* Main content */}
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Activity</CardTitle>
            </CardHeader>
            <CardContent>
              <ActivityTimeline activities={activitiesData?.data ?? []} />
            </CardContent>
          </Card>

          <EmailTimeline workspaceId={wsId} dealId={dealId} />
        </div>

        {/* Sidebar */}
        <div className="space-y-4">
          {/* Details */}
          <div className="rounded-lg border p-4">
            <h3 className="mb-3 text-sm font-medium">Details</h3>
            <div className="grid grid-cols-[16px_72px_1fr] gap-x-3 gap-y-2.5">
              <MetadataRow icon={Layers} label="Pipeline">
                <span className="text-xs">{currentPipeline?.name ?? '--'}</span>
              </MetadataRow>

              <MetadataRow icon={Gauge} label="Stage">
                <SidebarPopoverSelect
                  value={form.stage_id}
                  options={stageOptions}
                  onChange={(v) => updateField('stage_id', v, { stage_id: v })}
                  renderTrigger={() => (
                    <span className="text-xs">{currentStageName}</span>
                  )}
                />
              </MetadataRow>

              <MetadataRow icon={DollarSign} label="Amount">
                <input
                  type="number"
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.amount}
                  onChange={(e) => {
                    const val = e.target.value;
                    setForm((current) => (current ? { ...current, amount: val } : current));
                    queuePatch({ amount: val ? Number(val) : undefined });
                  }}
                  placeholder="--"
                />
              </MetadataRow>

              <MetadataRow icon={DollarSign} label="Currency">
                <input
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.currency}
                  onChange={(e) => updateField('currency', e.target.value, { currency: e.target.value })}
                  placeholder="USD"
                />
              </MetadataRow>

              <MetadataRow icon={Calendar} label="Close date">
                <input
                  type="date"
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.close_date}
                  onChange={(e) => updateField('close_date', e.target.value, { close_date: e.target.value || undefined })}
                />
              </MetadataRow>

              <MetadataRow icon={Percent} label="Probability">
                <input
                  type="number"
                  min="0"
                  max="100"
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.probability}
                  onChange={(e) => {
                    const val = e.target.value;
                    setForm((current) => (current ? { ...current, probability: val } : current));
                    queuePatch({ probability: val ? Number(val) : undefined });
                  }}
                  placeholder="--"
                />
              </MetadataRow>
            </div>
          </div>

          {/* Health Score */}
          <DealHealthScore workspaceId={wsId} dealId={dealId} />

          {/* Buyer Signals */}
          <BuyerSignals workspaceId={wsId} dealId={dealId} />

          {/* Associations */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Associations</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {crmAssocs.length === 0 ? (
                <p className="text-sm text-muted-foreground">No CRM associations yet</p>
              ) : (
                <ul className="space-y-2">
                  {crmAssocs.map((assoc: CRMAssociationEnriched) => {
                    const targetType = assoc.from_object_type === 'deal' ? assoc.to_object_type : assoc.from_object_type;
                    const targetId = assoc.from_object_type === 'deal' ? assoc.to_object_id : assoc.from_object_id;
                    return (
                      <li key={assoc.id}>
                        <button
                          type="button"
                          className="text-sm hover:underline cursor-pointer text-left"
                          onClick={() => {
                            if (targetType === 'contact') {
                              navigate({ to: '/w/$slug/crm/contacts/$contactId', params: { slug: wsSlug, contactId: targetId } });
                            } else if (targetType === 'company') {
                              navigate({ to: '/w/$slug/crm/companies/$companyId', params: { slug: wsSlug, companyId: targetId } });
                            }
                          }}
                        >
                          <span className="capitalize">{targetType}</span>
                          {assoc.linked_object_name && <span className="font-medium ml-1">{assoc.linked_object_name}</span>}
                          {assoc.linked_object_display_id && <span className="text-muted-foreground ml-1">({assoc.linked_object_display_id})</span>}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              )}

              <LinkedPMItems
                dealId={dealId}
                workspaceId={wsId}
                workspaceSlug={wsSlug}
                associations={(associations ?? []) as CRMAssociationEnriched[]}
              />
            </CardContent>
          </Card>
        </div>
      </div>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Delete deal"
        description="Are you sure? This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
      />
    </div>
  );
}
