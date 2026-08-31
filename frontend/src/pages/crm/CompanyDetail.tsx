import { useCallback, useEffect, useRef, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  ArrowLeft02Icon,
  Building03Icon,
  DollarCircleIcon,
  GlobeIcon,
  Loading01Icon,
  Delete01Icon,
  UserGroupIcon,
  LinkSquare01Icon,
  MapPinIcon,
  PlusSignIcon,
  LayoutTwoColumnIcon,
	ZapIcon,
	UserIcon,
} from '@/lib/icons';
import { QuietBreadcrumbs, QuietDetailAction, QuietDetailHeader, QuietEmptyState, QuietIconAction, QuietMetaLine, QuietTextAction, QuietTitleInput } from '@/components/design-system/quiet';
import { Skeleton } from '@/components/ui/skeleton';
import { Favicon } from '@/components/ui/favicon';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCompany, useUpdateCompany, useDeleteCompany, useCompanyTimeline, useCompanyAssociations } from '@/hooks/queries';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { LinkedTasksPanel } from '@/components/crm/LinkedTasksPanel';
import { AssociationsList } from '@/components/crm/AssociationsList';
import { EnrichmentRailCard } from '@/components/crm/contact-detail/EnrichmentRailCard';
import { CreateDealDialog } from '@/components/crm/CreateDealDialog';
import { DetailDescriptionEditorActions } from '@/components/pm/DetailDescriptionEditorActions';
import { DetailDescriptionEditButton } from '@/components/pm/DetailDescriptionEditButton';
import { RichTextMentionContent } from '@/components/pm/RichTextMentionContent';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet';
import { EmailTimeline } from '@/components/crm/EmailTimeline';
import { DesktopDetailRail } from '@/components/crm/DesktopDetailRail';
import { CompanyContactsView, CompanyDealsView, CompanyMeetingsView, CompanySupportView } from '@/components/crm/CompanyDetailCollections';
import { EntitySummaryCard } from '@/components/crm/EntitySummaryCard';
import { EntitySignals } from '@/components/crm/EntitySignals';
import { useTitle } from '@/hooks/useTitle';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { findAssignableMember } from '@/lib/assignableMembers';
import { cn } from '@/lib/utils';
import type { CRMCompanyTimelineFilter, CRMCompanyTimelineItem, UpdateCRMCompanyRequest } from '@/lib/crmTypes';
import type { CompanyDetailTab } from '@/lib/companyDetailTabs';

interface FormState {
  name: string;
  domain: string;
  industry: string;
  employee_count: string;
  annual_revenue: string;
  description: string;
  linkedin_url: string;
  headquarters: string;
	customer_success_owner_member_id: string;
}

const companyDetailTabLabels: Array<{
  value: CompanyDetailTab;
  label: string;
}> = [
  { value: 'overview', label: 'Overview' },
  { value: 'tasks', label: 'Tasks' },
  { value: 'emails', label: 'Emails' },
  { value: 'meetings', label: 'Meetings' },
  { value: 'calls', label: 'Calls' },
  { value: 'deals', label: 'Deals' },
  { value: 'support', label: 'Support' },
  { value: 'notes', label: 'Notes' },
];

function MetadataRow({ icon: Icon, label, children }: { icon: React.ElementType; label: string; children: React.ReactNode }) {
  return (
    <>
      <Icon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      <span className="mt-0.5 text-[12px] text-muted-foreground">{label}</span>
      <div className="min-w-0 text-[12px]">{children}</div>
    </>
  );
}

const railInputClassName = 'w-full rounded bg-transparent px-1.5 py-0.5 text-xs text-foreground outline-none transition-colors placeholder:text-muted-foreground hover:bg-accent focus:bg-accent';

export function CompanyDetailPage({ companyId, activeTab = 'overview', onTabChange, emailThreadId, onEmailThreadChange }: { companyId: string; activeTab?: CompanyDetailTab; onTabChange?: (tab: CompanyDetailTab) => void; emailThreadId?: string; onEmailThreadChange?: (threadId?: string) => void }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const location = useLocation();

  const { data: company, isLoading } = useCompany(wsId, companyId);
  const [activityFilter, setActivityFilter] = useState<CRMCompanyTimelineFilter>('all');
  const timeline = useCompanyTimeline(wsId, companyId, activityFilter);
  const notesTimeline = useCompanyTimeline(wsId, companyId, 'note', activeTab === 'notes');
  const callsTimeline = useCompanyTimeline(wsId, companyId, 'call', activeTab === 'calls');
  const { data: associations, refetch: refetchAssociations } = useCompanyAssociations(wsId, companyId);
  const updateCompany = useUpdateCompany(wsId);
	const { members: assignableMembers } = useAssignableWorkspaceMembers(wsId);
  const deleteCompany = useDeleteCompany(wsId);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateCRMCompanyRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [createDealOpen, setCreateDealOpen] = useState(false);
  const [editingDescription, setEditingDescription] = useState(false);
  const [timelinePreview, setTimelinePreview] = useState<CRMCompanyTimelineItem | null>(null);
  const [mobileDetailsOpen, setMobileDetailsOpen] = useState(false);
  const [desktopDetailsCollapsed, setDesktopDetailsCollapsed] = useState(activeTab !== 'overview');
  const descriptionEditStartRef = useRef('');

  useTitle(form?.name ?? 'Company');

  useEffect(() => {
    const timer = window.setTimeout(() => setDesktopDetailsCollapsed(activeTab !== 'overview'), 0);
    return () => window.clearTimeout(timer);
  }, [activeTab]);

  useEffect(() => {
    if (company) {
      // The detail form is intentionally reset when navigating between company records.
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setForm({
        name: company.name,
        domain: company.domain ?? '',
        industry: company.industry ?? '',
        employee_count: company.employee_count != null ? String(company.employee_count) : '',
        annual_revenue: company.annual_revenue != null ? String(company.annual_revenue) : '',
        description: company.description ?? '',
        linkedin_url: company.linkedin_url ?? '',
        headquarters: company.headquarters ?? '',
		customer_success_owner_member_id: company.customer_success_owner_member_id ?? '',
      });
    }
  }, [company]);

  useEffect(() => {
    if (saving || Object.keys(pendingPatch).length === 0 || !wsId || !companyId) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      try {
        await updateCompany.mutateAsync({ id: companyId, ...patch });
        setSaveError(null);
      } catch {
        setSaveError('Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
    // updateCompany is a mutation wrapper; its mutateAsync function is stable for this workspace.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wsId, companyId, pendingPatch, saving]);

  const queuePatch = (patch: UpdateCRMCompanyRequest) => setPendingPatch((current) => ({ ...current, ...patch }));

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateCRMCompanyRequest) => {
    setForm((current) => (current ? { ...current, [key]: value } : current));
    queuePatch(patch);
  };

  const beginDescriptionEditing = useCallback(() => {
    descriptionEditStartRef.current = form?.description ?? '';
    setEditingDescription(true);
  }, [form?.description]);

  const cancelDescriptionEditing = () => {
    if (!form) return;
    const initialDescription = descriptionEditStartRef.current;
    if (form.description !== initialDescription) {
      updateField('description', initialDescription, {
        description: initialDescription,
      });
    }
    setEditingDescription(false);
  };

  const handleDelete = async () => {
    try {
      await deleteCompany.mutateAsync(companyId);
      toast.success('Company deleted');
      navigate({ to: '/w/$slug/crm/companies', params: { slug: wsSlug } });
    } catch {
      toast.error('Failed to delete company');
    }
  };

  const goBack = () => navigate({ to: '/w/$slug/crm/companies', params: { slug: wsSlug } });

  const openTimelineSource = (item: CRMCompanyTimelineItem) => {
    const entity = item.entity;
    if (!entity) {
      setTimelinePreview(item);
      return;
    }
    if (entity.type === 'task') {
      openTaskRoute(navigate as never, location as never, wsSlug, entity.id);
      return;
    }
    if (entity.type === 'deal') {
      openDealRoute(navigate as never, location, wsSlug, entity.id);
      return;
    }
    const routes: Record<string, { to: string; params: Record<string, string> }> = {
      meeting: {
        to: '/w/$slug/crm/meetings/$meetingId',
        params: { slug: wsSlug, meetingId: entity.id },
      },
      support_conversation: {
        to: '/w/$slug/support/$conversationId',
        params: { slug: wsSlug, conversationId: entity.id },
      },
    };
    const route = routes[entity.type];
    if (route) navigate(route as never);
    else setTimelinePreview(item);
  };

  if (isLoading) {
    return (
      <div className="flex h-full flex-col overflow-hidden">
        <QuietDetailHeader
          className="lg:px-10"
          breadcrumbs={<QuietBreadcrumbs items={[{ id: 'companies', label: 'Companies', onClick: goBack }]} onBack={goBack} backLabel="Back to companies" />}
          avatar={<Skeleton className="h-10 w-10 rounded-[10px]" />}
          title={<Skeleton className="h-8 w-64 max-w-full rounded-none" />}
          meta={<Skeleton className="h-3 w-48 rounded-none" />}
        />
        <div className="flex flex-1 items-center justify-center">
          <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      </div>
    );
  }

  if (!company || !form) {
    return (
      <div className="flex h-full flex-col overflow-hidden">
        <QuietDetailHeader
          breadcrumbs={<QuietBreadcrumbs items={[{ id: 'companies', label: 'Companies', onClick: goBack }]} onBack={goBack} backLabel="Back to companies" />}
          title="Company"
        />
        <div className="flex-1 overflow-auto p-4 sm:p-6">
          <QuietEmptyState
            title="Company not found"
            description="This company may have been deleted or you may no longer have access to it."
            action={<QuietTextAction onClick={goBack}><ArrowLeft02Icon className="h-3.5 w-3.5" />Back to companies</QuietTextAction>}
          />
        </div>
      </div>
    );
  }

  const renderRailContent = () => (
    <>
      <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-foreground/70">Details</h3>
      <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
        <MetadataRow icon={GlobeIcon} label="Domain">
          <input className={railInputClassName} value={form.domain} onChange={(e) => updateField('domain', e.target.value, { domain: e.target.value })} placeholder="None" />
        </MetadataRow>
        <MetadataRow icon={Building03Icon} label="Industry">
          <input
            className={railInputClassName}
            value={form.industry}
            onChange={(e) =>
              updateField('industry', e.target.value, {
                industry: e.target.value,
              })
            }
            placeholder="None"
          />
        </MetadataRow>
        <MetadataRow icon={UserGroupIcon} label="Employees">
          <input
            className={railInputClassName}
            type="number"
            value={form.employee_count}
            onChange={(e) =>
              updateField('employee_count', e.target.value, {
                employee_count: e.target.value ? parseInt(e.target.value) : undefined,
              })
            }
            placeholder="None"
          />
        </MetadataRow>
        <MetadataRow icon={DollarCircleIcon} label="Revenue">
          <input
            className={railInputClassName}
            type="number"
            value={form.annual_revenue}
            onChange={(e) =>
              updateField('annual_revenue', e.target.value, {
                annual_revenue: e.target.value ? parseFloat(e.target.value) : undefined,
              })
            }
            placeholder="None"
          />
        </MetadataRow>
        <MetadataRow icon={LinkSquare01Icon} label="LinkedIn">
          <input
            className={railInputClassName}
            value={form.linkedin_url}
            onChange={(e) =>
              updateField('linkedin_url', e.target.value, {
                linkedin_url: e.target.value,
              })
            }
            placeholder="None"
          />
        </MetadataRow>
        <MetadataRow icon={MapPinIcon} label="HQ">
          <input
            className={railInputClassName}
            value={form.headquarters}
            onChange={(e) =>
              updateField('headquarters', e.target.value, {
                headquarters: e.target.value,
              })
            }
            placeholder="None"
          />
        </MetadataRow>
		<MetadataRow icon={UserIcon} label="CS owner">
			<MemberPickerPopover
				value={form.customer_success_owner_member_id || '__none__'}
				members={assignableMembers}
				noneLabel="Unassigned"
				onChange={(value) => updateField('customer_success_owner_member_id', value === '__none__' ? '' : value,
					value === '__none__' ? { clear_customer_success_owner: true } : { customer_success_owner_member_id: value })}
				renderTrigger={() => {
					const owner = findAssignableMember(assignableMembers, form.customer_success_owner_member_id);
					return owner ? <><UserAvatar name={owner.display_name || owner.email} avatarUrl={owner.avatar_url} avatarStyle={owner.avatar_style} avatarSeed={owner.avatar_seed} avatarBackgroundMode={owner.avatar_background_mode} avatarBackgroundColor={owner.avatar_background_color} className="h-4 w-4" fallbackClassName="text-[7px]" /><span className="truncate">{owner.display_name || owner.email}</span></> : <span className="text-muted-foreground">Unassigned</span>;
				}}
			/>
		</MetadataRow>
      </div>
		{company.commercial_state_health ? (
			<div className="mt-5 rounded-md border border-border/70 bg-muted/20 p-3">
				<div className="flex items-center gap-2">
					<ZapIcon className="h-3.5 w-3.5 text-muted-foreground" />
					<p className="text-xs font-semibold">Commercial-state integration</p>
				</div>
				<p className="mt-2 text-xs text-muted-foreground">
					{company.commercial_state_health.last_accepted_at
						? `Last accepted ${new Date(company.commercial_state_health.last_accepted_at).toLocaleString()}`
						: 'No accepted state update yet'}
				</p>
				{company.commercial_state_health.rejected_update_count > 0 ? (
					<p className="mt-1 text-xs text-destructive">
						{company.commercial_state_health.rejected_update_count} rejected update{company.commercial_state_health.rejected_update_count === 1 ? '' : 's'}
						{company.commercial_state_health.last_rejection_reason ? ` · ${company.commercial_state_health.last_rejection_reason}` : ''}
					</p>
				) : null}
			</div>
		) : null}
      <EnrichmentRailCard workspaceId={wsId} objectType="company" objectId={companyId} presentation="borderless" />
      <AssociationsList workspaceId={wsId} slug={wsSlug} associations={associations ?? []} currentObjectType="company" currentObjectId={companyId} onAssociationRemoved={() => refetchAssociations()} />
    </>
  );

  return (
    <div className="flex h-full flex-col">
      <QuietDetailHeader
        className="lg:px-10"
        breadcrumbs={<QuietBreadcrumbs items={[{ id: 'companies', label: 'Companies', onClick: goBack }]} onBack={goBack} backLabel="Back to companies" />}
        avatar={<Favicon src={company.logo_url} url={form.domain} name={form.name} size={64} className="h-10 w-10 rounded-[10px]" fallbackClassName="text-sm" />}
        title={(
          <QuietTitleInput
            aria-label="Company name"
            presentation="header"
            className="max-w-[32rem] border-b-transparent pb-0.5 hover:border-quiet-field focus-visible:border-quiet-text-primary"
            value={form.name}
            onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
            placeholder="Company name"
          />
        )}
        meta={<QuietMetaLine items={[<span className="font-mono" key="id">{company.display_id}</span>, form.domain || null]} />}
        state={<SaveIndicator saving={saving} error={saveError} presentation="quiet" />}
        actions={(
          <>
            <QuietDetailAction
              tone="danger"
              icon={<Delete01Icon className="h-4 w-4" />}
              label="Delete company"
              onClick={() => setDeleteConfirmOpen(true)}
            />
            <QuietDetailAction
              tone="primary"
              icon={<PlusSignIcon className="h-3.5 w-3.5" />}
              label="New deal"
              onClick={() => setCreateDealOpen(true)}
            />
          </>
        )}
      />

      <div className={cn(
        'relative grid min-h-0 flex-1 grid-cols-1 overflow-hidden',
        desktopDetailsCollapsed
          ? 'lg:grid-cols-[minmax(0,1fr)_40px]'
          : 'lg:grid-cols-[minmax(0,1fr)_300px]',
      )}>
        <div className="flex min-h-0 min-w-0 flex-col overflow-hidden">
          <div className="flex items-end border-b border-quiet-divider-strong px-3 sm:px-5 lg:px-8">
            <div className="min-w-0 flex-1 overflow-x-auto px-1 pt-1">
              <Tabs value={activeTab} onValueChange={(value) => onTabChange?.(value as CompanyDetailTab)} className="min-w-max gap-0">
                <TabsList variant="quiet" aria-label="Company detail views" className="border-b-0">
                {companyDetailTabLabels.map((tab) => (
                  <TabsTrigger
                    key={tab.value}
                    value={tab.value}
                  >
                    {tab.label}
                  </TabsTrigger>
                ))}
                </TabsList>
              </Tabs>
            </div>
            <QuietIconAction className="mb-1 ml-2 shrink-0 lg:hidden" onClick={() => setMobileDetailsOpen(true)} aria-label="Open company details">
              <LayoutTwoColumnIcon className="h-4 w-4" />
            </QuietIconAction>
          </div>

          {activeTab === 'overview' && (
            <div className="min-h-0 flex-1 overflow-y-auto">
              <section aria-label="Description" className="border-b border-border/60 px-4 py-5 sm:px-6 lg:px-10">
                <div className="group/desc relative">
                  {editingDescription ? (
                    <div className="group/description-editor">
                      <TiptapEditor
                        content={form.description}
                        onChange={(html) =>
                          updateField('description', html, {
                            description: html,
                          })
                        }
                        placeholder="Add a description..."
                        variant="divider"
                        contentVariant="pm"
                        className="min-h-[320px] [&_.tiptap]:min-h-[250px] [&_.tiptap]:p-0"
                      />
                      <DetailDescriptionEditorActions onCancel={cancelDescriptionEditing} onDone={() => setEditingDescription(false)} />
                    </div>
                  ) : (
                    <div className="relative min-h-9 pr-12">
                      {form.description ? (
                        <RichTextMentionContent
                          html={form.description}
                          variant="pm"
                          className="[&_p:empty]:my-0 [&_p:empty]:h-1"
                          onHtmlChange={(html) =>
                            updateField('description', html, {
                              description: html,
                            })
                          }
                        />
                      ) : (
                        <p className="text-sm text-muted-foreground">No description yet</p>
                      )}
                      <DetailDescriptionEditButton onClick={beginDescriptionEditing} />
                    </div>
                  )}
                </div>
              </section>
              <EntitySummaryCard
                workspaceId={wsId}
                companyId={companyId}
                presentation="overview"
                onOpenTab={(tab, threadId) => {
                  if (tab === 'emails' && threadId) onEmailThreadChange?.(threadId);
                  else onTabChange?.(tab);
                }}
                onTaskCreated={() => void timeline.refetch()}
              />
              <section aria-label="CRM signals" className="border-b border-border/60">
                <div className="flex items-center gap-2 px-4 pb-2 pt-5 text-[12px] font-semibold uppercase tracking-[0.06em] text-muted-foreground/70 sm:px-6 lg:px-10">
                  <ZapIcon className="h-[15px] w-[15px] text-muted-foreground" />
                  CRM signals
                </div>
                <EntitySignals
                  workspaceId={wsId}
                  companyId={companyId}
                  presentation="overview"
                  onOpenSource={(signal) => {
                    if (signal.source_type === 'email' && signal.source_thread_id) onEmailThreadChange?.(signal.source_thread_id);
                    else if (signal.source_type === 'meeting' && signal.source_id) navigate({ to: '/w/$slug/crm/meetings/$meetingId', params: { slug: wsSlug, meetingId: signal.source_id } } as never);
                    else if (signal.source_type === 'support' && signal.source_thread_id) navigate({ to: '/w/$slug/support/$conversationId', params: { slug: wsSlug, conversationId: signal.source_thread_id } } as never);
                    else if (signal.source_type === 'meeting') onTabChange?.('meetings');
                    else if (signal.source_type === 'support') onTabChange?.('support');
                    else if (signal.source_type === 'note') onTabChange?.('notes');
                    else if (signal.source_type === 'call') onTabChange?.('calls');
                  }}
                />
              </section>
              <section aria-label="Contacts" className="border-b border-border/60">
                <CompanyContactsView
                  workspaceId={wsId}
                  workspaceSlug={wsSlug}
                  companyId={companyId}
                />
              </section>
              <ActivityTimeline
                timelineItems={timeline.data?.pages.flatMap((page) => page?.data ?? []) ?? []}
                timelineFilter={activityFilter}
                onTimelineFilterChange={setActivityFilter}
                onTimelineItemOpen={openTimelineSource}
                hasNextPage={timeline.hasNextPage}
                isFetchingNextPage={timeline.isFetchingNextPage}
                isTimelineLoading={timeline.isLoading}
                onLoadMore={() => void timeline.fetchNextPage()}
                workspaceId={wsId}
                companyId={companyId}
                onActivityCreated={() => void timeline.refetch()}
                onActivityDeleted={() => void timeline.refetch()}
                presentation="borderless"
                filterControl="dropdown"
              />
              <div className="h-20" aria-hidden="true" />
            </div>
          )}
          {activeTab === 'notes' && (
            <div className="min-h-0 flex-1 overflow-y-auto">
              <ActivityTimeline
                timelineItems={notesTimeline.data?.pages.flatMap((page) => page?.data ?? []) ?? []}
                timelineFilter="note"
                onTimelineItemOpen={openTimelineSource}
                hasNextPage={notesTimeline.hasNextPage}
                isFetchingNextPage={notesTimeline.isFetchingNextPage}
                isTimelineLoading={notesTimeline.isLoading}
                onLoadMore={() => void notesTimeline.fetchNextPage()}
                workspaceId={wsId}
                companyId={companyId}
                onActivityCreated={() => void notesTimeline.refetch()}
                onActivityDeleted={() => void notesTimeline.refetch()}
                presentation="borderless"
                filterControl="hidden"
                actionTypes={['note']}
                heading="Notes"
              />
            </div>
          )}
          {activeTab === 'emails' && (
            <div className="min-h-0 flex-1 overflow-hidden">
              <EmailTimeline workspaceId={wsId} companyId={companyId} selectedThreadId={emailThreadId} onSelectedThreadChange={onEmailThreadChange} />
            </div>
          )}
          {activeTab === 'calls' && (
            <div className="min-h-0 flex-1 overflow-y-auto">
              <ActivityTimeline
                timelineItems={callsTimeline.data?.pages.flatMap((page) => page?.data ?? []) ?? []}
                timelineFilter="call"
                onTimelineItemOpen={openTimelineSource}
                hasNextPage={callsTimeline.hasNextPage}
                isFetchingNextPage={callsTimeline.isFetchingNextPage}
                isTimelineLoading={callsTimeline.isLoading}
                onLoadMore={() => void callsTimeline.fetchNextPage()}
                workspaceId={wsId}
                companyId={companyId}
                onActivityCreated={() => void callsTimeline.refetch()}
                onActivityDeleted={() => void callsTimeline.refetch()}
                presentation="borderless"
                filterControl="hidden"
                actionTypes={['call']}
                heading="Calls"
              />
            </div>
          )}
          {activeTab === 'meetings' && <CompanyMeetingsView workspaceId={wsId} workspaceSlug={wsSlug} companyId={companyId} />}
          {activeTab === 'tasks' && (
            <div className="min-h-0 flex-1">
              <LinkedTasksPanel workspaceId={wsId} workspaceSlug={wsSlug} companyId={companyId} presentation="borderless" fullHeight onTaskActivityChange={() => void timeline.refetch()} />
            </div>
          )}
          {activeTab === 'deals' && <CompanyDealsView workspaceId={wsId} workspaceSlug={wsSlug} companyId={companyId} companyName={form.name || 'this company'} />}
          {activeTab === 'support' && <CompanySupportView workspaceId={wsId} workspaceSlug={wsSlug} companyId={companyId} />}
        </div>

        <DesktopDetailRail
          collapsed={desktopDetailsCollapsed}
          onCollapsedChange={setDesktopDetailsCollapsed}
          contentClassName="px-5 py-5 pb-40"
          label="Company details"
        >
          {renderRailContent()}
        </DesktopDetailRail>
      </div>

      <Sheet open={mobileDetailsOpen} onOpenChange={setMobileDetailsOpen}>
        <SheetContent side="right" className="overflow-y-auto px-5 py-6">
          <SheetTitle className="sr-only">Company details</SheetTitle>
          {renderRailContent()}
        </SheetContent>
      </Sheet>

      <CreateDealDialog
        open={createDealOpen}
        onOpenChange={setCreateDealOpen}
        companyContext={{ id: companyId, name: form.name || 'this company' }}
        onDealCreated={() => {
          void timeline.refetch();
          void refetchAssociations();
        }}
      />
      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Delete company"
        description="Are you sure? This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
      />
      <Dialog open={!!timelinePreview} onOpenChange={(open) => !open && setTimelinePreview(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{timelinePreview?.title}</DialogTitle>
            <DialogDescription>{timelinePreview ? new Date(timelinePreview.occurred_at).toLocaleString() : ''}</DialogDescription>
          </DialogHeader>
          {timelinePreview?.description ? (
            <p className="whitespace-pre-wrap text-sm leading-6 text-muted-foreground">{timelinePreview.description}</p>
          ) : (
            <p className="text-sm text-muted-foreground">No additional details are available.</p>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
