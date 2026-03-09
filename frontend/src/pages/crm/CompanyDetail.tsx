import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  ArrowLeft,
  Building2,
  Check,
  DollarSign,
  Globe,
  MoreHorizontal,
  Trash2,
  User,
  Users,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
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
  useCompany,
  useCompanyActivities,
  useCompanyAssociations,
  useUpdateCompany,
  useDeleteCompany,
} from '@/hooks/queries';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { useTitle } from '@/hooks/useTitle';
import type { UpdateCRMCompanyRequest } from '@/lib/crmTypes';

// ── Types ──

interface FormState {
  name: string;
  domain: string;
  industry: string;
  employee_count: string;
  annual_revenue: string;
  description: string;
  owner_member_id: string;
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

// ── Main Page ──

export function CompanyDetailPage({ companyId }: { companyId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();

  const { data: company, isLoading } = useCompany(wsId, companyId);
  const { data: activitiesData } = useCompanyActivities(wsId, companyId);
  const { data: associations } = useCompanyAssociations(wsId, companyId);
  const updateMutation = useUpdateCompany(wsId);
  const deleteMutation = useDeleteCompany(wsId);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateCRMCompanyRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  useTitle(company?.name ?? 'Company');

  // Initialize form from company data
  useEffect(() => {
    if (company && !form) {
      setForm({
        name: company.name,
        domain: company.domain ?? '',
        industry: company.industry ?? '',
        employee_count: company.employee_count != null ? String(company.employee_count) : '',
        annual_revenue: company.annual_revenue != null ? String(company.annual_revenue) : '',
        description: company.description ?? '',
        owner_member_id: company.owner_member_id ?? '',
      });
    }
  }, [company, form]);

  // Auto-save debounce
  useEffect(() => {
    if (saving || Object.keys(pendingPatch).length === 0 || !wsId || !companyId) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      try {
        await updateMutation.mutateAsync({ id: companyId, ...patch });
        setSaveError(null);
      } catch {
        setSaveError('Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [wsId, companyId, pendingPatch, saving]);

  const queuePatch = (patch: UpdateCRMCompanyRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateCRMCompanyRequest) => {
    setForm((current) => (current ? { ...current, [key]: value } : current));
    queuePatch(patch);
  };

  const handleDelete = async () => {
    try {
      await deleteMutation.mutateAsync(companyId);
      toast.success('Company deleted');
      navigate({ to: '/w/$slug/crm/companies', params: { slug: wsSlug } });
    } catch {
      toast.error('Failed to delete company');
    }
  };

  const goBack = () => navigate({ to: '/w/$slug/crm/companies', params: { slug: wsSlug } });

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

  if (!company || !form) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Company not found</div>;
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
            <p className="text-xs text-muted-foreground">{company.display_id}</p>
            <input
              className="text-2xl font-bold bg-transparent border-none outline-none focus:outline-none w-full"
              value={form.name}
              onChange={(e) => updateField('name', e.target.value, { name: e.target.value })}
              placeholder="Company name"
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
                Delete company
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* Two-column layout */}
      <div className="grid gap-6 lg:grid-cols-[1fr_280px]">
        {/* Main content */}
        <div className="space-y-6">
          {form.description !== undefined && (
            <div className="rounded-lg border p-4">
              <h3 className="mb-2 text-sm font-medium">Description</h3>
              <textarea
                className="w-full bg-transparent text-sm outline-none border-none focus:outline-none resize-none min-h-[60px]"
                value={form.description}
                onChange={(e) => updateField('description', e.target.value, { description: e.target.value })}
                placeholder="Add a description..."
              />
            </div>
          )}

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Activity</CardTitle>
            </CardHeader>
            <CardContent>
              <ActivityTimeline activities={activitiesData?.data ?? []} />
            </CardContent>
          </Card>
        </div>

        {/* Sidebar */}
        <div className="space-y-4">
          {/* Details */}
          <div className="rounded-lg border p-4">
            <h3 className="mb-3 text-sm font-medium">Details</h3>
            <div className="grid grid-cols-[16px_72px_1fr] gap-x-3 gap-y-2.5">
              <MetadataRow icon={Globe} label="Domain">
                <input
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.domain}
                  onChange={(e) => updateField('domain', e.target.value, { domain: e.target.value })}
                  placeholder="--"
                />
              </MetadataRow>

              <MetadataRow icon={Building2} label="Industry">
                <input
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.industry}
                  onChange={(e) => updateField('industry', e.target.value, { industry: e.target.value })}
                  placeholder="--"
                />
              </MetadataRow>

              <MetadataRow icon={Users} label="Employees">
                <input
                  type="number"
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.employee_count}
                  onChange={(e) => {
                    const val = e.target.value;
                    setForm((current) => (current ? { ...current, employee_count: val } : current));
                    queuePatch({ employee_count: val ? Number(val) : undefined });
                  }}
                  placeholder="--"
                />
              </MetadataRow>

              <MetadataRow icon={DollarSign} label="Revenue">
                <input
                  type="number"
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.annual_revenue}
                  onChange={(e) => {
                    const val = e.target.value;
                    setForm((current) => (current ? { ...current, annual_revenue: val } : current));
                    queuePatch({ annual_revenue: val ? Number(val) : undefined });
                  }}
                  placeholder="--"
                />
              </MetadataRow>

              <MetadataRow icon={User} label="Owner">
                <input
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.owner_member_id}
                  onChange={(e) => updateField('owner_member_id', e.target.value, { owner_member_id: e.target.value })}
                  placeholder="--"
                />
              </MetadataRow>
            </div>
          </div>

          {/* Associations */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Associations</CardTitle>
            </CardHeader>
            <CardContent>
              {(!associations || associations.length === 0) ? (
                <p className="text-sm text-muted-foreground">No associations yet</p>
              ) : (
                <ul className="space-y-2">
                  {associations.map((assoc) => {
                    const targetType = assoc.from_object_type === 'company' ? assoc.to_object_type : assoc.from_object_type;
                    const targetId = assoc.from_object_type === 'company' ? assoc.to_object_id : assoc.from_object_id;
                    return (
                      <li key={assoc.id}>
                        <button
                          type="button"
                          className="text-sm hover:underline cursor-pointer text-left"
                          onClick={() => {
                            if (targetType === 'contact') {
                              navigate({ to: '/w/$slug/crm/contacts/$contactId', params: { slug: wsSlug, contactId: targetId } });
                            } else if (targetType === 'deal') {
                              navigate({ to: '/w/$slug/crm/deals/$dealId', params: { slug: wsSlug, dealId: targetId } });
                            }
                          }}
                        >
                          <span className="capitalize">{targetType}</span>
                          {assoc.association_label && <span className="text-muted-foreground"> ({assoc.association_label})</span>}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Delete company"
        description="Are you sure? This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
      />
    </div>
  );
}
