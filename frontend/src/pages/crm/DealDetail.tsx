import { SequenceEnrollmentDialog } from '@/components/crm/outreach/SequenceEnrollmentDialog';
import { CRMEmailComposerDialog } from '@/components/crm/CRMEmailComposerDialog';
import { useEmailAccounts, useContact } from '@/hooks/queries/useCRM';
import { recurringRevenue, revenueSuffix, type RevenueType } from '@/components/crm/dealCreationDefaults';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
	Cancel01Icon,
  Calendar01Icon,
  DashboardSpeed01Icon,
  Delete01Icon,
  DollarCircleIcon,
  Loading01Icon,
  Mail01Icon,
  Tag01Icon,
  UserIcon,
} from '@/lib/icons';
import {
	QuietBreadcrumbs,
	QuietDetailAction,
	QuietDetailHeader,
	QuietDetailLayout,
	QuietEmptyState,
	QuietMetaLine,
	QuietSection,
	QuietTextAction,
	QuietTitleInput,
} from '@/components/design-system/quiet';
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
import { useCRMOwnerMembers } from '@/hooks/useCRMOwnerMembers';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { DealHealthScore } from '@/components/crm/DealHealthScore';
import { EntitySignals } from '@/components/crm/EntitySignals';
import { EntitySummaryCard } from '@/components/crm/EntitySummaryCard';
import { AssociationsList } from '@/components/crm/AssociationsList';
import { LinkedTasksPanel } from '@/components/crm/LinkedTasksPanel';
import { DealEmailThreadPanel } from '@/components/crm/deal-detail/DealEmailThreadPanel';
import { DealStageSelect } from '@/components/crm/DealStageSelect';
import { DealRelationships } from '@/components/crm/deal-detail/DealRelationships';
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
  revenue_type: RevenueType;
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

function DealMetadataRow({ icon: Icon, label, children }: { icon: React.ElementType; label: string; children: React.ReactNode }) {
	return <>
		<Icon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
		<span className="mt-0.5 text-[12px] text-muted-foreground">{label}</span>
		<div className="min-w-0 text-[12px]">{children}</div>
	</>;
}

export function DealDetailPage({ dealId, onRequestClose, registerBeforeClose }: DealDetailPageProps) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const location = useLocation();

  const { data: deal, isLoading } = useDeal(wsId, dealId);
  const { data: associations, refetch: refetchAssociations } = useDealAssociations(wsId, dealId);
  const [sequenceOpen, setSequenceOpen] = useState(false);
  const [emailOpen, setEmailOpen] = useState(false);
  const emailAccounts = useEmailAccounts(wsId);
  const contactAssociation = associations?.find((association) => association.from_object_type === 'contact' || association.to_object_type === 'contact');
  const emailContactId = contactAssociation ? (contactAssociation.from_object_type === 'contact' ? contactAssociation.from_object_id : contactAssociation.to_object_id) : '';
  const emailContact = useContact(wsId, emailContactId, Boolean(emailContactId));
  const { data: pipelines } = usePipelines(wsId);
  const updateDeal = useUpdateDeal(wsId);
  const deleteDeal = useDeleteDeal(wsId);
  const { members: assignableMembers } = useCRMOwnerMembers(wsId);

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
        revenue_type: deal.revenue_type ?? 'one_time',
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

  const goToDeals = () => void closeThen(() => navigate({ to: '/w/$slug/crm/deals', params: { slug: wsSlug } } as never));

  if (isLoading) {
    return (
		<div className="flex h-full flex-col overflow-hidden">
			<QuietDetailHeader
				breadcrumbs={<QuietBreadcrumbs items={[{ id: 'deals', label: 'Deals', onClick: goToDeals }]} onBack={goToDeals} backLabel="Back to deals" />}
				title={<div className="h-6 w-64 max-w-full animate-pulse bg-quiet-icon-well" />}
				meta={<div className="h-3 w-40 animate-pulse bg-quiet-icon-well" />}
			/>
			<div className="flex flex-1 items-center justify-center"><Loading01Icon className="h-6 w-6 animate-spin text-quiet-muted" /></div>
		</div>
    );
  }

  if (!deal || !form) {
    return (
		<div className="flex h-full flex-col overflow-hidden">
			<QuietDetailHeader breadcrumbs={<QuietBreadcrumbs items={[{ id: 'deals', label: 'Deals', onClick: goToDeals }]} onBack={goToDeals} backLabel="Back to deals" />} title="Deal" />
			<div className="flex-1 overflow-auto p-4 sm:p-6">
				<QuietEmptyState title="Deal not found" description="This deal may have been deleted or is no longer available to you." action={<QuietTextAction onClick={goToDeals}>Back to deals</QuietTextAction>} />
			</div>
		</div>
    );
  }

  const recurring = recurringRevenue(form.amount === '' ? undefined : Number(form.amount), form.revenue_type);
  const selectedOwner = findAssignableMember(assignableMembers, form.owner_member_id);
  const amountValue = Number.parseFloat(form.amount);
	const customerAssociation = associations?.find((association) => association.association_label === 'deal_customer');
	const customerType = customerAssociation
		? (customerAssociation.from_object_type === 'deal' ? customerAssociation.to_object_type : customerAssociation.from_object_type)
		: null;
	const customerId = customerAssociation
		? (customerAssociation.from_object_type === 'deal' ? customerAssociation.to_object_id : customerAssociation.from_object_id)
		: null;
	const customerName = customerAssociation?.linked_object_name || 'Customer needed';
	const openCustomer = () => {
		if (!customerId || (customerType !== 'contact' && customerType !== 'company')) return;
		void closeThen(() => navigate(customerType === 'company'
			? { to: '/w/$slug/crm/companies/$companyId', params: { slug: wsSlug, companyId: customerId } }
			: { to: '/w/$slug/crm/contacts/$contactId', params: { slug: wsSlug, contactId: customerId } } as never));
	};

  return (
		<div className="flex h-full min-h-0 flex-col bg-background">
			<QuietDetailHeader
				breadcrumbs={<QuietBreadcrumbs items={[
					{ id: 'deals', label: 'Deals', onClick: goToDeals },
					{ id: 'customer', label: customerName, onClick: customerId ? openCustomer : undefined },
				]} onBack={goToDeals} backLabel="Back to deals" />}
				title={<QuietTitleInput aria-label="Deal name" presentation="header" className="max-w-[42rem] border-b-transparent hover:border-quiet-field focus-visible:border-quiet-text-primary" value={form.name} onChange={(event) => updateField('name', event.target.value, { name: event.target.value })} onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); event.currentTarget.blur(); } }} placeholder="Deal name" />}
				meta={<QuietMetaLine items={[<span className="font-mono" key="id">{deal.display_id}</span>, currentPipeline?.name, form.amount ? new Intl.NumberFormat('en-US', { style: 'currency', currency: form.currency || 'USD', maximumFractionDigits: 0 }).format(amountValue || 0) + revenueSuffix(form.revenue_type) : null, form.probability ? `${form.probability}% probability` : null]} />}
				state={<SaveIndicator saving={saving} error={saveError} presentation="quiet" />}
				actions={<>
          <QuietDetailAction icon={<Mail01Icon className="h-4 w-4" />} label="Add to sequence" onClick={() => setSequenceOpen(true)} />
          <QuietDetailAction icon={<Mail01Icon className="h-4 w-4" />} label="Send email" onClick={() => setEmailOpen(true)} />
					<QuietDetailAction tone="danger" icon={<Delete01Icon className="h-4 w-4" />} label="Delete deal" onClick={() => setDeleteConfirmOpen(true)} />
					{onRequestClose ? <QuietDetailAction iconOnly icon={<Cancel01Icon className="h-4 w-4" />} label="Close deal" onClick={() => void onRequestClose()} /> : null}
				</>}
			/>



			<QuietDetailLayout
				className="min-h-0 flex-1 overflow-y-auto lg:overflow-hidden"
				main={<main className="min-h-0 lg:overflow-y-auto">
					<QuietSection title="Summary" className="lg:px-10"><EntitySummaryCard workspaceId={wsId} dealId={dealId} presentation="compact" /></QuietSection>
					<QuietSection title="Signals" action={<DealHealthScore workspaceId={wsId} dealId={dealId} compact />} className="lg:px-10">
						<EntitySignals workspaceId={wsId} dealId={dealId} presentation="compact" onOpenSource={(signal) => {
							if (signal.source_type === 'email' && signal.source_thread_id) setEmailThreadId(signal.source_thread_id);
							else if (signal.source_type === 'meeting' && signal.source_id) void closeThen(() => navigate({ to: '/w/$slug/crm/meetings/$meetingId', params: { slug: wsSlug, meetingId: signal.source_id } } as never));
							else if (signal.source_type === 'support' && signal.source_thread_id) void closeThen(() => navigate({ to: '/w/$slug/support/$conversationId', params: { slug: wsSlug, conversationId: signal.source_thread_id } } as never));
						}} />
					</QuietSection>
					<LinkedTasksPanel workspaceId={wsId} workspaceSlug={wsSlug} dealId={dealId} presentation="borderless" onTaskActivityChange={() => void timeline.refetch()} />
					<ActivityTimeline timelineItems={timelineItems} timelineFilter={timelineFilter} onTimelineFilterChange={setTimelineFilter} onTimelineItemOpen={openTimelineSource} hasNextPage={timeline.hasNextPage} isFetchingNextPage={timeline.isFetchingNextPage} isTimelineLoading={timeline.isLoading} onLoadMore={() => void timeline.fetchNextPage()} workspaceId={wsId} dealId={dealId} onActivityCreated={() => void timeline.refetch()} onActivityDeleted={() => void timeline.refetch()} presentation="borderless" filterControl="dropdown" heading="Activity" />
				</main>}
				railClassName="px-4 py-5 pb-16 sm:px-6 lg:px-5 lg:pb-40"
				rail={<>
					<DealRelationships workspaceId={wsId} dealId={dealId} associations={associations ?? []} onChanged={() => void refetchAssociations()} onNavigate={(type, id) => void closeThen(() => navigate(type === 'company' ? { to: '/w/$slug/crm/companies/$companyId', params: { slug: wsSlug, companyId: id } } : { to: '/w/$slug/crm/contacts/$contactId', params: { slug: wsSlug, contactId: id } } as never))} />
					<section className="-mx-4 mt-4 border-t border-border/60 px-4 pt-4">
						<h2 className="mb-3 text-xs font-semibold uppercase tracking-wide text-foreground/70">Details</h2>
						<div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
							<DealMetadataRow icon={Tag01Icon} label="Stage"><DealStageSelect stages={sortedStages} value={form.stage_id} disabled={saving} onChange={(stageId) => updateField('stage_id', stageId, { stage_id: stageId })} /></DealMetadataRow>
                            <DealMetadataRow icon={Tag01Icon} label="Pipeline"><SidebarPopoverSelect value={form.pipeline_id} options={(pipelines ?? []).map((pipeline) => ({ value: pipeline.id, label: pipeline.name }))} onChange={(pipelineId) => { const pipeline = pipelines?.find((item) => item.id === pipelineId); const firstStageId = [...(pipeline?.stages ?? [])].sort((a, b) => a.position - b.position)[0]?.id ?? ''; setForm((current) => current ? { ...current, pipeline_id: pipelineId, stage_id: firstStageId } : current); queuePatch({ pipeline_id: pipelineId, ...(firstStageId ? { stage_id: firstStageId } : {}) }); }} renderTrigger={() => <span className="truncate">{currentPipeline?.name ?? 'Select pipeline'}</span>} /></DealMetadataRow>
							<DealMetadataRow icon={DollarCircleIcon} label="Amount"><div className="flex min-w-0 items-center gap-1.5"><input className="min-w-0 flex-1 bg-transparent px-1.5 py-0.5 text-xs outline-none placeholder:text-muted-foreground" type="number" step="0.01" value={form.amount} onChange={(event) => updateField('amount', event.target.value, { amount: event.target.value ? Number.parseFloat(event.target.value) : undefined })} placeholder="None" /><SidebarPopoverSelect value={form.currency} options={['USD', 'EUR', 'GBP', 'CAD', 'AUD'].map((value) => ({ value, label: value }))} onChange={(value) => updateField('currency', value, { currency: value })} renderTrigger={() => <span className="text-muted-foreground">{form.currency}</span>} triggerClassName="px-1" width="w-28" /></div></DealMetadataRow>
              <DealMetadataRow icon={DollarCircleIcon} label="Revenue type"><SidebarPopoverSelect renderTrigger={() => <span>{form.revenue_type === 'one_time' ? 'One-time' : form.revenue_type === 'monthly' ? 'Monthly' : 'Annual'}</span>} value={form.revenue_type} options={[{value:'one_time',label:'One-time'},{value:'monthly',label:'Monthly'},{value:'annual',label:'Annual'}]} onChange={(value) => updateField('revenue_type', value as RevenueType, { revenue_type: value as RevenueType })} /></DealMetadataRow>
              {recurring && <DealMetadataRow icon={DollarCircleIcon} label={deal.stage?.stage_type === 'open' ? 'Potential MRR / ARR' : 'MRR / ARR'}><span>{form.currency} {new Intl.NumberFormat(undefined, {maximumFractionDigits:2}).format(recurring.mrr)} / {new Intl.NumberFormat(undefined, {maximumFractionDigits:2}).format(recurring.arr)}</span></DealMetadataRow>}

							<DealMetadataRow icon={Tag01Icon} label="Deal type"><SidebarPopoverSelect value={form.commercial_motion} options={[{ value: 'inherit', label: `Inherit ${currentPipeline?.default_commercial_motion?.replace('_', ' ') ?? 'pipeline default'}` }, { value: 'new_business', label: 'New business' }, { value: 'existing_business', label: 'Existing business' }, { value: 'expansion', label: 'Expansion' }, { value: 'renewal', label: 'Renewal' }]} onChange={(value) => updateField('commercial_motion', value as FormState['commercial_motion'], value === 'inherit' ? { clear_commercial_motion: true } : { commercial_motion: value as CRMDealCommercialMotion })} renderTrigger={() => <span className="capitalize">{form.commercial_motion === 'inherit' ? `Inherit ${currentPipeline?.default_commercial_motion?.replace('_', ' ') ?? ''}` : form.commercial_motion.replace('_', ' ')}</span>} /></DealMetadataRow>
							<div className="col-span-3 my-1 h-px bg-border/40" />
							<DealMetadataRow icon={Calendar01Icon} label="Close date"><input className="w-full bg-transparent px-1.5 py-0.5 text-xs outline-none" type="date" value={form.close_date} onChange={(event) => updateField('close_date', event.target.value, { close_date: event.target.value ? `${event.target.value}T00:00:00Z` : undefined })} /></DealMetadataRow>
							<DealMetadataRow icon={DashboardSpeed01Icon} label="Probability"><div className="flex items-center gap-1"><input className="w-12 bg-transparent px-1.5 py-0.5 text-xs outline-none placeholder:text-muted-foreground" type="number" min="0" max="100" value={form.probability} onChange={(event) => updateField('probability', event.target.value, { probability: event.target.value ? Number.parseInt(event.target.value, 10) : undefined, clear_probability: event.target.value === '' })} placeholder={String(sortedStages.find(stage => stage.id === form.stage_id)?.probability ?? 0)} /><span className="text-muted-foreground">%</span></div></DealMetadataRow>
							<DealMetadataRow icon={UserIcon} label="Owner"><MemberPickerPopover value={form.owner_member_id || '__none__'} members={assignableMembers} noneLabel="Unassigned" onChange={(value) => updateField('owner_member_id', value === '__none__' ? '' : value, { owner_member_id: value === '__none__' ? '' : value })} renderTrigger={() => selectedOwner ? <><UserAvatar name={selectedOwner.display_name || selectedOwner.email} avatarUrl={selectedOwner.avatar_url} avatarStyle={selectedOwner.avatar_style} avatarSeed={selectedOwner.avatar_seed} avatarBackgroundMode={selectedOwner.avatar_background_mode} avatarBackgroundColor={selectedOwner.avatar_background_color} className="h-4 w-4" fallbackClassName="text-[7px]" /><span className="truncate">{selectedOwner.display_name || selectedOwner.email}</span></> : <span className="text-muted-foreground">Unassigned</span>} /></DealMetadataRow>
						</div>
					</section>
					<details className="-mx-4 mt-4 border-t border-border/60 px-4 pt-4" open><summary className="cursor-pointer select-none text-xs font-semibold uppercase tracking-wide text-foreground/70">Related</summary><AssociationsList workspaceId={wsId} slug={wsSlug} associations={associations ?? []} currentObjectType="deal" currentObjectId={dealId} excludeTypes={['task', 'contact', 'company']} onAssociationRemoved={() => void refetchAssociations()} /></details>
				</>}
			/>

      {sequenceOpen && <SequenceEnrollmentDialog workspaceId={wsId} dealId={dealId} onClose={() => setSequenceOpen(false)} />}
      {emailOpen && <CRMEmailComposerDialog workspaceId={wsId} accounts={emailAccounts.data ?? []} open draft={{ dealId, dealName: deal.name, to: emailContact.data?.email ? [emailContact.data.email] : [] }} onOpenChange={setEmailOpen} />}
      <DealEmailThreadPanel open={!!emailThreadId} onOpenChange={(open) => { if (!open) setEmailThreadId(undefined); }} workspaceId={wsId} dealId={dealId} threadId={emailThreadId} />
      <ConfirmDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen} title="Delete deal" description="Are you sure? This action cannot be undone." confirmLabel="Delete" variant="destructive" onConfirm={handleDelete} />
    </div>
  );
}
