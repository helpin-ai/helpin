import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  ArrowRight01Icon,
  Calendar01Icon,
  DashboardSpeed01Icon,
  Delete01Icon,
  DollarCircleIcon,
  Loading01Icon,
  Tag01Icon,
  UserIcon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  useDeal,
  useDealAssociations,
  useDealTimeline,
  useDeleteDeal,
  usePipelines,
  useUpdateDeal,
} from '@/hooks/queries';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { DealHealthScore } from '@/components/crm/DealHealthScore';
import { EntitySignals } from '@/components/crm/EntitySignals';
import { EntitySummaryCard } from '@/components/crm/EntitySummaryCard';
import { AssociationsList } from '@/components/crm/AssociationsList';
import { LinkedTasksPanel } from '@/components/crm/LinkedTasksPanel';
import { DealEmailThreadPanel } from '@/components/crm/deal-detail/DealEmailThreadPanel';
import { DealStagePath } from '@/components/crm/deal-detail/DealStagePath';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { useRegisterPageContext } from '@/components/command-bar/pageContext';
import { useTitle } from '@/hooks/useTitle';
import { findAssignableMember } from '@/lib/assignableMembers';
import type { CRMDealCommercialMotion, CRMTimelineFilter, CRMTimelineItem, UpdateCRMDealRequest } from '@/lib/crmTypes';

interface FormState {
  name: string;
  pipeline_id: string;
  stage_id: string;
  amount: string;
  currency: string;
  close_date: string;
  probability: string;
  owner_member_id: string;
	commercial_motion: CRMDealCommercialMotion | 'inherit';
}

interface DealDetailPageProps {
  dealId: string;
  onRequestClose?: () => Promise<void> | void;
  registerBeforeClose?: (handler: (() => Promise<void>) | null) => void;
}

function MetadataRow({ icon: Icon, label, children }: { icon: React.ElementType; label: string; children: React.ReactNode }) {
  return (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0 self-center text-muted-foreground" />
      <span className="self-center text-ui text-muted-foreground">{label}</span>
      <div className="min-w-0 self-center text-ui">{children}</div>
    </>
  );
}

export function DealDetailPage({ dealId, onRequestClose, registerBeforeClose }: DealDetailPageProps) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const location = useLocation();

  const { data: deal, isLoading } = useDeal(wsId, dealId);
  const { data: associations, refetch: refetchAssociations } = useDealAssociations(wsId, dealId);
  const { data: pipelines } = usePipelines(wsId);
  const updateDeal = useUpdateDeal(wsId);
  const deleteDeal = useDeleteDeal(wsId);
  const { members: assignableMembers } = useAssignableWorkspaceMembers(wsId);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingVersion, setPendingVersion] = useState(0);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [timelineFilter, setTimelineFilter] = useState<CRMTimelineFilter>('all');
  const [emailThreadId, setEmailThreadId] = useState<string | undefined>();
  const pendingPatchRef = useRef<UpdateCRMDealRequest>({});
  const saveTimerRef = useRef<number | null>(null);

  const timeline = useDealTimeline(wsId, dealId, timelineFilter);
  const timelineItems = useMemo(
    () => timeline.data?.pages.flatMap((page) => page.data ?? []) ?? [],
    [timeline.data?.pages],
  );

  useRegisterPageContext(deal ? {
    entity_type: 'crm_deal',
    entity_id: deal.id,
    display_id: deal.display_id,
    display_title: deal.name,
  } : null, 30);
  useTitle(form?.name ?? deal?.name ?? 'Deal');

  const currentPipeline = useMemo(
    () => pipelines?.find((pipeline) => pipeline.id === form?.pipeline_id),
    [form?.pipeline_id, pipelines],
  );
  const sortedStages = useMemo(
    () => [...(currentPipeline?.stages ?? [])].sort((a, b) => a.position - b.position),
    [currentPipeline?.stages],
  );

  useEffect(() => {
    if (!deal) return;
    let cancelled = false;
    queueMicrotask(() => {
      if (cancelled) return;
      pendingPatchRef.current = {};
      setPendingVersion(0);
      setSaveError(null);
      setTimelineFilter('all');
      setEmailThreadId(undefined);
      setForm({
        name: deal.name,
        pipeline_id: deal.pipeline_id,
        stage_id: deal.stage_id,
        amount: deal.amount != null ? String(deal.amount) : '',
        currency: deal.currency ?? 'USD',
        close_date: deal.close_date ? deal.close_date.slice(0, 10) : '',
        probability: deal.probability != null ? String(deal.probability) : '',
        owner_member_id: deal.owner_member_id ?? '',
		commercial_motion: deal.commercial_motion ?? 'inherit',
      });
    });
    return () => { cancelled = true; };
  }, [deal]);

  const flushPendingPatch = useCallback(async () => {
    if (saveTimerRef.current !== null) {
      window.clearTimeout(saveTimerRef.current);
      saveTimerRef.current = null;
    }
    const patch = pendingPatchRef.current;
    if (!wsId || !dealId || Object.keys(patch).length === 0) return;
    pendingPatchRef.current = {};
    setSaving(true);
    try {
      await updateDeal.mutateAsync({ id: dealId, ...patch });
      setSaveError(null);
    } catch {
      pendingPatchRef.current = { ...patch, ...pendingPatchRef.current };
      setPendingVersion((version) => version + 1);
      setSaveError('Failed to save');
      throw new Error('Failed to save deal');
    } finally {
      setSaving(false);
    }
  }, [dealId, updateDeal, wsId]);

  useEffect(() => {
    registerBeforeClose?.(flushPendingPatch);
    return () => registerBeforeClose?.(null);
  }, [flushPendingPatch, registerBeforeClose]);

  useEffect(() => {
    if (pendingVersion === 0 || saving || Object.keys(pendingPatchRef.current).length === 0) return;
    saveTimerRef.current = window.setTimeout(() => {
      void flushPendingPatch().catch(() => undefined);
    }, 650);
    return () => {
      if (saveTimerRef.current !== null) window.clearTimeout(saveTimerRef.current);
    };
  }, [flushPendingPatch, pendingVersion, saving]);

  const queuePatch = (patch: UpdateCRMDealRequest) => {
    pendingPatchRef.current = { ...pendingPatchRef.current, ...patch };
    setPendingVersion((version) => version + 1);
  };

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateCRMDealRequest) => {
    setForm((current) => (current ? { ...current, [key]: value } : current));
    queuePatch(patch);
  };

  const closeThen = async (action: () => void) => {
    try {
      await onRequestClose?.();
      action();
    } catch {
      toast.error('Save the deal before leaving');
    }
  };

  const openTimelineSource = (item: CRMTimelineItem) => {
    const entity = item.entity;
    if (!entity) return;
    if (entity.type === 'email_thread') {
      setEmailThreadId(entity.id);
      return;
    }
    if (entity.type === 'task') {
      void closeThen(() => openTaskRoute(navigate as never, location as never, wsSlug, entity.id));
      return;
    }
    if (entity.type === 'meeting') {
      void closeThen(() => navigate({ to: '/w/$slug/crm/meetings/$meetingId', params: { slug: wsSlug, meetingId: entity.id } } as never));
      return;
    }
    if (entity.type === 'support_conversation') {
      void closeThen(() => navigate({ to: '/w/$slug/support/$conversationId', params: { slug: wsSlug, conversationId: entity.id } } as never));
    }
  };

  const handleDelete = async () => {
    try {
      pendingPatchRef.current = {};
      await deleteDeal.mutateAsync(dealId);
      toast.success('Deal deleted');
      await onRequestClose?.();
    } catch {
      toast.error('Failed to delete deal');
    }
  };

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!deal || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">Deal not found</p>
        <Button variant="outline" size="sm" onClick={() => void onRequestClose?.()}>Close</Button>
      </div>
    );
  }

  const selectedOwner = findAssignableMember(assignableMembers, form.owner_member_id);
  const amountValue = Number.parseFloat(form.amount);

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <header className="flex h-11 shrink-0 items-center gap-2 border-b border-border/60 px-4 pr-12">
        <DollarCircleIcon className="h-4 w-4 shrink-0 text-emerald-600" />
        <span className="text-xs text-muted-foreground">Deals</span>
        <ArrowRight01Icon className="h-3 w-3 text-muted-foreground/60" />
        <span className="min-w-0 truncate text-xs font-medium text-foreground">{deal.display_id}</span>
        <div className="ml-auto flex items-center gap-1">
          <SaveIndicator saving={saving} error={saveError} />
          <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground hover:text-destructive" aria-label="Delete deal" onClick={() => setDeleteConfirmOpen(true)}>
            <Delete01Icon className="h-3.5 w-3.5" />
          </Button>
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto lg:grid lg:grid-cols-[minmax(0,1fr)_300px] lg:overflow-hidden">
        <main className="min-h-0 lg:overflow-y-auto">
          <section className="border-b border-border/60 px-4 pb-5 pt-6 sm:px-6 lg:px-10 lg:pt-8">
            <input className="w-full bg-transparent font-heading text-2xl font-semibold leading-tight text-foreground outline-none placeholder:text-muted-foreground/50 sm:text-[1.75rem]" value={form.name} onChange={(event) => updateField('name', event.target.value, { name: event.target.value })} placeholder="Deal name" />
            <div className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
              <span>{deal.display_id}</span>
              {form.amount ? <><span>·</span><span>{new Intl.NumberFormat('en-US', { style: 'currency', currency: form.currency || 'USD', maximumFractionDigits: 0 }).format(amountValue || 0)}</span></> : null}
              {form.probability ? <><span>·</span><span>{form.probability}% probability</span></> : null}
            </div>
          </section>

          <DealStagePath stages={sortedStages} value={form.stage_id} disabled={saving} onChange={(stageId) => updateField('stage_id', stageId, { stage_id: stageId })} />

          <section className="grid gap-6 border-b border-border/60 px-4 py-6 sm:px-6 lg:grid-cols-[minmax(0,1.35fr)_minmax(220px,0.65fr)] lg:px-10">
            <EntitySummaryCard workspaceId={wsId} dealId={dealId} presentation="compact" />
            <div className="min-w-0">
              <div className="mb-2 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">Deal health <DealHealthScore workspaceId={wsId} dealId={dealId} compact /></div>
              <EntitySignals
                workspaceId={wsId}
                dealId={dealId}
                presentation="compact"
                onOpenSource={(signal) => {
                  if (signal.source_type === 'email' && signal.source_thread_id) setEmailThreadId(signal.source_thread_id);
                  else if (signal.source_type === 'meeting' && signal.source_id) void closeThen(() => navigate({ to: '/w/$slug/crm/meetings/$meetingId', params: { slug: wsSlug, meetingId: signal.source_id } } as never));
                  else if (signal.source_type === 'support' && signal.source_thread_id) void closeThen(() => navigate({ to: '/w/$slug/support/$conversationId', params: { slug: wsSlug, conversationId: signal.source_thread_id } } as never));
                }}
              />
            </div>
          </section>

          <LinkedTasksPanel workspaceId={wsId} workspaceSlug={wsSlug} dealId={dealId} presentation="borderless" onTaskActivityChange={() => void timeline.refetch()} />

          <ActivityTimeline
            timelineItems={timelineItems}
            timelineFilter={timelineFilter}
            onTimelineFilterChange={setTimelineFilter}
            onTimelineItemOpen={openTimelineSource}
            hasNextPage={timeline.hasNextPage}
            isFetchingNextPage={timeline.isFetchingNextPage}
            isTimelineLoading={timeline.isLoading}
            onLoadMore={() => void timeline.fetchNextPage()}
            workspaceId={wsId}
            dealId={dealId}
            onActivityCreated={() => void timeline.refetch()}
            onActivityDeleted={() => void timeline.refetch()}
            presentation="borderless"
            filterControl="dropdown"
            heading="Activity"
          />
        </main>

        <aside className="min-h-0 border-t border-border/60 bg-muted/[0.12] px-4 py-5 lg:overflow-y-auto lg:border-l lg:border-t-0">
          <h2 className="mb-4 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">Details</h2>
          <div className="grid grid-cols-[16px_72px_minmax(0,1fr)] gap-x-2 gap-y-3">
            <MetadataRow icon={Tag01Icon} label="Pipeline">
              <SidebarPopoverSelect
                value={form.pipeline_id}
                options={(pipelines ?? []).map((pipeline) => ({ value: pipeline.id, label: pipeline.name }))}
                onChange={(pipelineId) => {
                  const pipeline = pipelines?.find((item) => item.id === pipelineId);
                  const firstStageId = [...(pipeline?.stages ?? [])].sort((a, b) => a.position - b.position)[0]?.id ?? '';
                  setForm((current) => current ? { ...current, pipeline_id: pipelineId, stage_id: firstStageId } : current);
                  queuePatch({ pipeline_id: pipelineId, ...(firstStageId ? { stage_id: firstStageId } : {}) });
                }}
                renderTrigger={() => <span className="truncate">{currentPipeline?.name ?? 'Select pipeline'}</span>}
                triggerClassName="-ml-1.5"
              />
            </MetadataRow>
            <MetadataRow icon={DollarCircleIcon} label="Amount">
              <div className="flex min-w-0 items-center gap-1.5">
                <input className="min-w-0 flex-1 bg-transparent outline-none" type="number" step="0.01" value={form.amount} onChange={(event) => updateField('amount', event.target.value, { amount: event.target.value ? Number.parseFloat(event.target.value) : undefined })} placeholder="—" />
                <SidebarPopoverSelect value={form.currency} options={['USD', 'EUR', 'GBP', 'CAD', 'AUD'].map((currency) => ({ value: currency, label: currency }))} onChange={(currency) => updateField('currency', currency, { currency })} renderTrigger={() => <span className="text-muted-foreground">{form.currency}</span>} triggerClassName="px-1" width="w-28" />
              </div>
            </MetadataRow>
			<MetadataRow icon={Tag01Icon} label="Motion">
				<SidebarPopoverSelect
					value={form.commercial_motion}
					options={[
						{ value: 'inherit', label: `Inherit ${currentPipeline?.default_commercial_motion?.replace('_', ' ') ?? 'pipeline default'}` },
						{ value: 'new_business', label: 'New business' },
						{ value: 'expansion', label: 'Expansion' },
						{ value: 'renewal', label: 'Renewal' },
					]}
					onChange={(value) => updateField('commercial_motion', value as FormState['commercial_motion'],
						value === 'inherit' ? { clear_commercial_motion: true } : { commercial_motion: value as CRMDealCommercialMotion })}
					renderTrigger={() => <span className="capitalize">{form.commercial_motion === 'inherit' ? `Inherit ${currentPipeline?.default_commercial_motion?.replace('_', ' ') ?? ''}` : form.commercial_motion.replace('_', ' ')}</span>}
					triggerClassName="-ml-1.5"
				/>
			</MetadataRow>
            <MetadataRow icon={Calendar01Icon} label="Close date">
              <input className="w-full bg-transparent outline-none" type="date" value={form.close_date} onChange={(event) => updateField('close_date', event.target.value, { close_date: event.target.value ? `${event.target.value}T00:00:00Z` : undefined })} />
            </MetadataRow>
            <MetadataRow icon={DashboardSpeed01Icon} label="Probability">
              <div className="flex items-center gap-1">
                <input className="w-12 bg-transparent outline-none" type="number" min="0" max="100" value={form.probability} onChange={(event) => updateField('probability', event.target.value, { probability: event.target.value ? Number.parseInt(event.target.value, 10) : undefined })} placeholder="—" />
                <span className="text-muted-foreground">%</span>
              </div>
            </MetadataRow>
            <MetadataRow icon={UserIcon} label="Owner">
              <MemberPickerPopover
                value={form.owner_member_id || '__none__'}
                members={assignableMembers}
                noneLabel="Unassigned"
                onChange={(value) => updateField('owner_member_id', value === '__none__' ? '' : value, { owner_member_id: value === '__none__' ? '' : value })}
                renderTrigger={() => selectedOwner ? (
                  <>
                    <UserAvatar name={selectedOwner.display_name || selectedOwner.email} avatarUrl={selectedOwner.avatar_url} avatarStyle={selectedOwner.avatar_style} avatarSeed={selectedOwner.avatar_seed} avatarBackgroundMode={selectedOwner.avatar_background_mode} avatarBackgroundColor={selectedOwner.avatar_background_color} className="h-4 w-4" fallbackClassName="text-[7px]" />
                    <span className="truncate">{selectedOwner.display_name || selectedOwner.email}</span>
                  </>
                ) : <span className="text-muted-foreground">Unassigned</span>}
              />
            </MetadataRow>
          </div>

          <Separator className="my-5" />
          <AssociationsList workspaceId={wsId} slug={wsSlug} associations={associations ?? []} currentObjectType="deal" currentObjectId={dealId} excludeTypes={['task']} onAssociationRemoved={() => void refetchAssociations()} />
        </aside>
      </div>

      <DealEmailThreadPanel open={!!emailThreadId} onOpenChange={(open) => { if (!open) setEmailThreadId(undefined); }} workspaceId={wsId} dealId={dealId} threadId={emailThreadId} />
      <ConfirmDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen} title="Delete deal" description="Are you sure? This action cannot be undone." confirmLabel="Delete" variant="destructive" onConfirm={handleDelete} />
    </div>
  );
}
