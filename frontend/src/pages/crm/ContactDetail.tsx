import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  ArrowLeft,
  Briefcase,
  Check,
  Mail,
  MoreHorizontal,
  Phone,
  Signal,
  Tag,
  Trash2,
  User,
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
  useContact,
  useContactActivities,
  useContactAssociations,
  useUpdateContact,
  useDeleteContact,
} from '@/hooks/queries';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { EmailTimeline } from '@/components/crm/EmailTimeline';
import { BuyerSignals } from '@/components/crm/BuyerSignals';
import { EnrichmentCard } from '@/components/crm/EnrichmentCard';
import { useTitle } from '@/hooks/useTitle';
import type { LifecycleStage, LeadStatus, UpdateCRMContactRequest } from '@/lib/crmTypes';

// ── Types ──

interface FormState {
  first_name: string;
  last_name: string;
  email: string;
  phone: string;
  job_title: string;
  lifecycle_stage: LifecycleStage;
  lead_status: LeadStatus;
  source: string;
}

// ── Constants ──

const lifecycleStageOptions: { value: LifecycleStage; label: string }[] = [
  { value: 'subscriber', label: 'Subscriber' },
  { value: 'lead', label: 'Lead' },
  { value: 'marketing_qualified', label: 'Marketing Qualified' },
  { value: 'sales_qualified', label: 'Sales Qualified' },
  { value: 'opportunity', label: 'Opportunity' },
  { value: 'customer', label: 'Customer' },
  { value: 'evangelist', label: 'Evangelist' },
];

const leadStatusOptions: { value: LeadStatus; label: string }[] = [
  { value: 'new', label: 'New' },
  { value: 'open', label: 'Open' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'unqualified', label: 'Unqualified' },
];

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

export function ContactDetailPage({ contactId }: { contactId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();

  const { data: contact, isLoading } = useContact(wsId, contactId);
  const { data: activitiesData } = useContactActivities(wsId, contactId);
  const { data: associations } = useContactAssociations(wsId, contactId);
  const updateMutation = useUpdateContact(wsId);
  const deleteMutation = useDeleteContact(wsId);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateCRMContactRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  useTitle(contact ? `${contact.first_name} ${contact.last_name ?? ''}` : 'Contact');

  // Initialize form from contact data
  useEffect(() => {
    if (contact && !form) {
      setForm({
        first_name: contact.first_name,
        last_name: contact.last_name ?? '',
        email: contact.email ?? '',
        phone: contact.phone ?? '',
        job_title: contact.job_title ?? '',
        lifecycle_stage: contact.lifecycle_stage,
        lead_status: contact.lead_status,
        source: contact.source ?? '',
      });
    }
  }, [contact, form]);

  // Auto-save debounce
  useEffect(() => {
    if (saving || Object.keys(pendingPatch).length === 0 || !wsId || !contactId) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      try {
        await updateMutation.mutateAsync({ id: contactId, ...patch });
        setSaveError(null);
      } catch {
        setSaveError('Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [wsId, contactId, pendingPatch, saving]);

  const queuePatch = (patch: UpdateCRMContactRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateCRMContactRequest) => {
    setForm((current) => (current ? { ...current, [key]: value } : current));
    queuePatch(patch);
  };

  const handleDelete = async () => {
    try {
      await deleteMutation.mutateAsync(contactId);
      toast.success('Contact deleted');
      navigate({ to: '/w/$slug/crm/contacts', params: { slug: wsSlug } });
    } catch {
      toast.error('Failed to delete contact');
    }
  };

  const goBack = () => navigate({ to: '/w/$slug/crm/contacts', params: { slug: wsSlug } });

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

  if (!contact || !form) {
    return <div className="flex items-center justify-center p-8 text-muted-foreground">Contact not found</div>;
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
            <p className="text-xs text-muted-foreground">{contact.display_id}</p>
            <div className="flex items-center gap-1">
              <input
                className="text-2xl font-bold bg-transparent border-none outline-none focus:outline-none"
                value={form.first_name}
                onChange={(e) => updateField('first_name', e.target.value, { first_name: e.target.value })}
                placeholder="First name"
              />
              <input
                className="text-2xl font-bold bg-transparent border-none outline-none focus:outline-none"
                value={form.last_name}
                onChange={(e) => updateField('last_name', e.target.value, { last_name: e.target.value })}
                placeholder="Last name"
              />
            </div>
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
                Delete contact
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

          <EmailTimeline workspaceId={wsId} contactId={contactId} />
        </div>

        {/* Sidebar */}
        <div className="space-y-4">
          {/* Details */}
          <div className="rounded-lg border p-4">
            <h3 className="mb-3 text-sm font-medium">Details</h3>
            <div className="grid grid-cols-[16px_72px_1fr] gap-x-3 gap-y-2.5">
              <MetadataRow icon={Signal} label="Stage">
                <SidebarPopoverSelect
                  value={form.lifecycle_stage}
                  options={lifecycleStageOptions}
                  onChange={(v) => updateField('lifecycle_stage', v, { lifecycle_stage: v })}
                  renderTrigger={() => (
                    <span className="capitalize text-xs">{form.lifecycle_stage.replace(/_/g, ' ')}</span>
                  )}
                />
              </MetadataRow>

              <MetadataRow icon={Tag} label="Status">
                <SidebarPopoverSelect
                  value={form.lead_status}
                  options={leadStatusOptions}
                  onChange={(v) => updateField('lead_status', v, { lead_status: v })}
                  renderTrigger={() => (
                    <span className="capitalize text-xs">{form.lead_status.replace(/_/g, ' ')}</span>
                  )}
                />
              </MetadataRow>

              <MetadataRow icon={Mail} label="Email">
                <input
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.email}
                  onChange={(e) => updateField('email', e.target.value, { email: e.target.value })}
                  placeholder="--"
                />
              </MetadataRow>

              <MetadataRow icon={Phone} label="Phone">
                <input
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.phone}
                  onChange={(e) => updateField('phone', e.target.value, { phone: e.target.value })}
                  placeholder="--"
                />
              </MetadataRow>

              <MetadataRow icon={Briefcase} label="Job title">
                <input
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.job_title}
                  onChange={(e) => updateField('job_title', e.target.value, { job_title: e.target.value })}
                  placeholder="--"
                />
              </MetadataRow>

              <MetadataRow icon={User} label="Source">
                <input
                  className="w-full bg-transparent text-xs outline-none border-none focus:outline-none py-0.5"
                  value={form.source}
                  onChange={(e) => updateField('source', e.target.value, { source: e.target.value })}
                  placeholder="--"
                />
              </MetadataRow>
            </div>
          </div>

          {/* Enrichment */}
          <EnrichmentCard workspaceId={wsId} objectType="contact" objectId={contactId} />

          {/* Buyer Signals */}
          <BuyerSignals workspaceId={wsId} contactId={contactId} />

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
                    const targetType = assoc.from_object_type === 'contact' ? assoc.to_object_type : assoc.from_object_type;
                    const targetId = assoc.from_object_type === 'contact' ? assoc.to_object_id : assoc.from_object_id;
                    return (
                      <li key={assoc.id}>
                        <button
                          type="button"
                          className="text-sm hover:underline cursor-pointer text-left"
                          onClick={() => {
                            if (targetType === 'company') {
                              navigate({ to: '/w/$slug/crm/companies/$companyId', params: { slug: wsSlug, companyId: targetId } });
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
        title="Delete contact"
        description="Are you sure? This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
      />
    </div>
  );
}
