import { useCallback, useEffect, useRef, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  ArrowLeft02Icon,
  Building03Icon,
  ArrowRight01Icon,
  DollarCircleIcon,
  GlobeIcon,
  Loading01Icon,
  Delete01Icon,
  UserGroupIcon,
  LinkSquare01Icon,
  MapPinIcon,
  PlusSignIcon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Favicon } from '@/components/ui/favicon';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
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
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useTitle } from '@/hooks/useTitle';
import type { CRMCompanyTimelineFilter, CRMCompanyTimelineItem, UpdateCRMCompanyRequest } from '@/lib/crmTypes';

interface FormState {
  name: string;
  domain: string;
  industry: string;
  employee_count: string;
  annual_revenue: string;
  description: string;
  linkedin_url: string;
  headquarters: string;
}

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

export function CompanyDetailPage({ companyId }: { companyId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const location = useLocation();

  const { data: company, isLoading } = useCompany(wsId, companyId);
  const [activityFilter, setActivityFilter] = useState<CRMCompanyTimelineFilter>('all');
  const timeline = useCompanyTimeline(wsId, companyId, activityFilter);
  const { data: associations, refetch: refetchAssociations } = useCompanyAssociations(wsId, companyId);
  const updateCompany = useUpdateCompany(wsId);
  const deleteCompany = useDeleteCompany(wsId);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateCRMCompanyRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [createDealOpen, setCreateDealOpen] = useState(false);
  const [editingDescription, setEditingDescription] = useState(false);
  const [timelinePreview, setTimelinePreview] = useState<CRMCompanyTimelineItem | null>(null);
  const descriptionEditStartRef = useRef('');

  useTitle(form?.name ?? 'Company');

  useEffect(() => {
    if (company) {
      setForm({
        name: company.name,
        domain: company.domain ?? '',
        industry: company.industry ?? '',
        employee_count: company.employee_count != null ? String(company.employee_count) : '',
        annual_revenue: company.annual_revenue != null ? String(company.annual_revenue) : '',
        description: company.description ?? '',
        linkedin_url: company.linkedin_url ?? '',
        headquarters: company.headquarters ?? '',
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
      updateField('description', initialDescription, { description: initialDescription });
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
    const routes: Record<string, { to: string; params: Record<string, string> }> = {
      deal: { to: '/w/$slug/crm/deals/$dealId', params: { slug: wsSlug, dealId: entity.id } },
      meeting: { to: '/w/$slug/crm/meetings/$meetingId', params: { slug: wsSlug, meetingId: entity.id } },
      support_conversation: { to: '/w/$slug/support/$conversationId', params: { slug: wsSlug, conversationId: entity.id } },
    };
    const route = routes[entity.type];
    if (route) navigate(route as never);
    else setTimelinePreview(item);
  };

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!company || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">Company not found</p>
        <Button variant="outline" size="sm" onClick={goBack}>
          <ArrowLeft02Icon className="mr-1 h-3.5 w-3.5" />
          Back to Companies
        </Button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header bar */}
      <div className="ui-divider-bottom-fade flex items-center gap-2 px-4 py-2.5">
        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={goBack}>
          <ArrowLeft02Icon className="h-4 w-4" />
        </Button>

        <div className="flex min-w-0 items-center gap-1 text-xs text-muted-foreground">
          <Favicon
            src={company.logo_url}
            url={form.domain}
            name={form.name}
            size={32}
            className="h-4 w-4 rounded-sm border-none bg-transparent"
            fallbackClassName="text-[8px]"
          />
          <button type="button" className="shrink-0 hover:text-foreground transition-colors cursor-pointer" onClick={goBack}>
            Companies
          </button>
          <ArrowRight01Icon className="h-3 w-3 shrink-0" />
          <span className="truncate font-medium text-foreground">{form.name || 'Untitled'}</span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <SaveIndicator saving={saving} error={saveError} />
          <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0 hover:text-destructive" onClick={() => setDeleteConfirmOpen(true)}>
            <Delete01Icon className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Two-column layout */}
      <div className="relative grid min-h-0 flex-1 grid-cols-1 overflow-y-auto lg:grid-cols-[minmax(0,1fr)_300px] lg:overflow-hidden">
        {/* Left column */}
        <div className="flex min-h-0 min-w-0 flex-col lg:overflow-y-auto">
          {/* Identity */}
          <section className="border-b border-border/60 px-4 pb-3 pt-5 sm:px-6 lg:px-10">
            <div className="flex flex-wrap items-start gap-3">
              <Favicon
                src={company.logo_url}
                url={form.domain}
                name={form.name}
                size={64}
                className="mt-0.5 h-10 w-10 rounded-xl"
                fallbackClassName="text-sm"
              />
              <div className="min-w-[12rem] flex-1">
                <input
                  aria-label="Company name"
                  className="w-full border-b border-transparent bg-transparent pb-1 text-2xl font-bold tracking-tight text-foreground transition-colors placeholder:text-muted-foreground/50 focus:border-foreground/70 focus:outline-none"
                  value={form.name}
                  onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
                  placeholder="Company name"
                />
                <p className="mt-1.5 text-xs text-muted-foreground">{company.display_id}</p>
              </div>
              <Button size="sm" className="h-8 shrink-0 gap-1.5" onClick={() => setCreateDealOpen(true)}>
                <PlusSignIcon className="h-3.5 w-3.5" />
                New deal
              </Button>
            </div>
          </section>

          {/* Description */}
          <section aria-label="Description" className="border-b border-border/60 px-4 py-5 sm:px-6 lg:px-10">
            <div className="group/desc relative">
              {editingDescription ? (
                <div className="group/description-editor">
                  <TiptapEditor
                    content={form.description}
                    onChange={(html) => updateField('description', html, { description: html })}
                    placeholder="Add a description..."
                    variant="divider"
                    contentVariant="pm"
                    className="min-h-[320px] [&_.tiptap]:min-h-[250px] [&_.tiptap]:p-0"
                  />
                  <DetailDescriptionEditorActions
                    onCancel={cancelDescriptionEditing}
                    onDone={() => setEditingDescription(false)}
                  />
                </div>
              ) : (
                <div className="relative min-h-9 pr-12">
                  {form.description ? (
                    <RichTextMentionContent
                      html={form.description}
                      variant="pm"
                      className="[&_p:empty]:my-0 [&_p:empty]:h-1"
                      onHtmlChange={(html) => updateField('description', html, { description: html })}
                    />
                  ) : (
                    <p className="text-sm text-muted-foreground">No description yet</p>
                  )}
                  <DetailDescriptionEditButton onClick={beginDescriptionEditing} />
                </div>
              )}
            </div>
          </section>

          <LinkedTasksPanel
            workspaceId={wsId}
            workspaceSlug={wsSlug}
            companyId={companyId}
            presentation="borderless"
            onTaskActivityChange={() => void timeline.refetch()}
          />

          <ActivityTimeline
            timelineItems={timeline.data?.pages.flatMap((page) => page.data) ?? []}
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
          />

          <div className="h-20 shrink-0 lg:h-32" aria-hidden="true" />
        </div>

        {/* Right sidebar */}
        <aside className="border-t border-border/60 px-4 py-5 pb-16 sm:px-6 lg:min-h-0 lg:overflow-y-auto lg:border-l lg:border-t-0 lg:px-5 lg:pb-40">
          <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-foreground/70">Details</h3>

          <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
            <MetadataRow icon={GlobeIcon} label="Domain">
              <input className={railInputClassName} value={form.domain}
                onChange={(e) => updateField('domain', e.target.value, { domain: e.target.value })} placeholder="None" />
            </MetadataRow>
            <MetadataRow icon={Building03Icon} label="Industry">
              <input className={railInputClassName} value={form.industry}
                onChange={(e) => updateField('industry', e.target.value, { industry: e.target.value })} placeholder="None" />
            </MetadataRow>
            <MetadataRow icon={UserGroupIcon} label="Employees">
              <input className={railInputClassName} type="number" value={form.employee_count}
                onChange={(e) => updateField('employee_count', e.target.value, { employee_count: e.target.value ? parseInt(e.target.value) : undefined })} placeholder="None" />
            </MetadataRow>
            <MetadataRow icon={DollarCircleIcon} label="Revenue">
              <input className={railInputClassName} type="number" value={form.annual_revenue}
                onChange={(e) => updateField('annual_revenue', e.target.value, { annual_revenue: e.target.value ? parseFloat(e.target.value) : undefined })} placeholder="None" />
            </MetadataRow>
            <MetadataRow icon={LinkSquare01Icon} label="LinkedIn">
              <input className={railInputClassName} value={form.linkedin_url}
                onChange={(e) => updateField('linkedin_url', e.target.value, { linkedin_url: e.target.value })} placeholder="None" />
            </MetadataRow>
            <MetadataRow icon={MapPinIcon} label="HQ">
              <input className={railInputClassName} value={form.headquarters}
                onChange={(e) => updateField('headquarters', e.target.value, { headquarters: e.target.value })} placeholder="None" />
            </MetadataRow>
          </div>

          <EnrichmentRailCard workspaceId={wsId} objectType="company" objectId={companyId} presentation="borderless" />

          <AssociationsList workspaceId={wsId} slug={wsSlug} associations={associations ?? []}
            currentObjectType="company" currentObjectId={companyId} onAssociationRemoved={() => refetchAssociations()} />
        </aside>
      </div>

      <CreateDealDialog
        open={createDealOpen}
        onOpenChange={setCreateDealOpen}
        companyContext={{ id: companyId, name: form.name || 'this company' }}
        onDealCreated={() => {
          void timeline.refetch();
          void refetchAssociations();
        }}
      />
      <ConfirmDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen} title="Delete company"
        description="Are you sure? This action cannot be undone." confirmLabel="Delete" variant="destructive" onConfirm={handleDelete} />
      <Dialog open={!!timelinePreview} onOpenChange={(open) => !open && setTimelinePreview(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{timelinePreview?.title}</DialogTitle>
            <DialogDescription>
              {timelinePreview ? new Date(timelinePreview.occurred_at).toLocaleString() : ''}
            </DialogDescription>
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
