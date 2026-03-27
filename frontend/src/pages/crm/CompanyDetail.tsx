import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  ArrowLeft,
  Building2,
  ChevronRight,
  DollarSign,
  Globe,
  Loader2,
  Trash2,
  Users,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Favicon } from '@/components/ui/favicon';
import { Separator } from '@/components/ui/separator';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCompany, useUpdateCompany, useDeleteCompany, useCompanyActivities, useCompanyAssociations } from '@/hooks/queries';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { AssociationsList } from '@/components/crm/AssociationsList';
import { useTitle } from '@/hooks/useTitle';
import type { UpdateCRMCompanyRequest } from '@/lib/crmTypes';

interface FormState {
  name: string;
  domain: string;
  industry: string;
  employee_count: string;
  annual_revenue: string;
  description: string;
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

export function CompanyDetailPage({ companyId }: { companyId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();

  const { data: company, isLoading } = useCompany(wsId, companyId);
  const { data: activitiesData, refetch: refetchActivities } = useCompanyActivities(wsId, companyId);
  const { data: associations, refetch: refetchAssociations } = useCompanyAssociations(wsId, companyId);
  const updateCompany = useUpdateCompany(wsId);
  const deleteCompany = useDeleteCompany(wsId);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateCRMCompanyRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  useTitle(form?.name ?? 'Company');

  useEffect(() => {
    if (company && !form) {
      setForm({
        name: company.name,
        domain: company.domain ?? '',
        industry: company.industry ?? '',
        employee_count: company.employee_count != null ? String(company.employee_count) : '',
        annual_revenue: company.annual_revenue != null ? String(company.annual_revenue) : '',
        description: company.description ?? '',
      });
    }
  }, [company, form]);

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

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!company || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">Company not found</p>
        <Button variant="outline" size="sm" onClick={goBack}>
          <ArrowLeft className="mr-1 h-3.5 w-3.5" />
          Back to Companies
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
          <div className="flex items-start gap-3">
            <Favicon
              src={company.logo_url}
              url={form.domain}
              name={form.name}
              size={64}
              className="mt-0.5 h-10 w-10 rounded-xl"
              fallbackClassName="text-sm"
            />
            <input
              className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
              value={form.name}
              onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
              placeholder="Company name"
            />
          </div>
          <p className="mt-1 text-xs text-muted-foreground">{company.display_id}</p>

          <Separator className="my-6" />

          {/* Description */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Description</h3>
            <div className="mt-3">
              <TiptapEditor
                content={form.description}
                onChange={(html) => updateField('description', html, { description: html })}
                placeholder="Add a description..."
                className="border-transparent shadow-none"
              />
            </div>
          </div>

          <Separator className="my-6" />

          {/* Activity */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Activity</h3>
            <div className="mt-3">
              <ActivityTimeline
                activities={activitiesData?.data ?? []}
                workspaceId={wsId}
                companyId={companyId}
                onActivityCreated={() => refetchActivities()}
                onActivityDeleted={() => refetchActivities()}
              />
            </div>
          </div>
        </div>

        {/* Right sidebar */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6">
          <h3 className="mb-4 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Details</h3>

          <div className="grid grid-cols-[16px_80px_1fr] gap-x-2 gap-y-3">
            <MetadataRow icon={Globe} label="Domain">
              <div className="flex items-center gap-2">
                <Favicon
                  src={company.logo_url}
                  url={form.domain}
                  name={form.name}
                  size={16}
                  className="h-4 w-4 rounded-sm border-none bg-transparent"
                  fallbackClassName="text-[8px]"
                />
                <input className="w-full bg-transparent text-xs outline-none" value={form.domain}
                  onChange={(e) => updateField('domain', e.target.value, { domain: e.target.value })} placeholder="—" />
              </div>
            </MetadataRow>
            <MetadataRow icon={Building2} label="Industry">
              <input className="w-full bg-transparent text-xs outline-none" value={form.industry}
                onChange={(e) => updateField('industry', e.target.value, { industry: e.target.value })} placeholder="—" />
            </MetadataRow>
            <MetadataRow icon={Users} label="Employees">
              <input className="w-full bg-transparent text-xs outline-none" type="number" value={form.employee_count}
                onChange={(e) => updateField('employee_count', e.target.value, { employee_count: e.target.value ? parseInt(e.target.value) : undefined })} placeholder="—" />
            </MetadataRow>
            <MetadataRow icon={DollarSign} label="Revenue">
              <input className="w-full bg-transparent text-xs outline-none" type="number" value={form.annual_revenue}
                onChange={(e) => updateField('annual_revenue', e.target.value, { annual_revenue: e.target.value ? parseFloat(e.target.value) : undefined })} placeholder="—" />
            </MetadataRow>
          </div>

          <Separator className="my-4" />
          <AssociationsList workspaceId={wsId} slug={wsSlug} associations={associations ?? []}
            currentObjectType="company" currentObjectId={companyId} onAssociationRemoved={() => refetchAssociations()} />
        </aside>
      </div>

      <ConfirmDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen} title="Delete company"
        description="Are you sure? This action cannot be undone." confirmLabel="Delete" variant="destructive" onConfirm={handleDelete} />
    </div>
  );
}
