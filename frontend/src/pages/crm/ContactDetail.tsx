import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { Link } from '@tanstack/react-router';
import {
  ArrowLeft,
  Check,
  ChevronRight,
  Loader2,
  Mail,
  MessageSquare,
  Phone,
  Briefcase,
  Tag,
  Trash2,
  Globe,
  User,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  useContact,
  useUpdateContact,
  useDeleteContact,
  useContactActivities,
  useContactAssociations,
  useContactSupportConversations,
} from '@/hooks/queries';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { EmailTimeline } from '@/components/crm/EmailTimeline';
import { BuyerSignals } from '@/components/crm/BuyerSignals';
import { EnrichmentCard } from '@/components/crm/EnrichmentCard';
import { AssociationsList } from '@/components/crm/AssociationsList';
import { useTitle } from '@/hooks/useTitle';
import type { LifecycleStage, LeadStatus, UpdateCRMContactRequest } from '@/lib/crmTypes';

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

const lifecycleOptions: { value: LifecycleStage; label: string }[] = [
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

function MetadataRow({ icon: Icon, label, children }: { icon: React.ElementType; label: string; children: React.ReactNode }) {
  return (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0 self-center text-muted-foreground" />
      <span className="self-center text-xs text-muted-foreground">{label}</span>
      <div className="min-w-0 self-center">{children}</div>
    </>
  );
}

function SidebarPopoverSelect<T extends string>({
  value, options, onChange,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
}) {
  const [open, setOpen] = useState(false);
  const current = options.find((o) => o.value === value);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button type="button" className="inline-flex cursor-pointer items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent">
          {current?.label ?? value}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-40 p-0.5" align="start">
        <div className="flex max-h-60 flex-col overflow-y-auto">
          {options.map((option) => (
            <button key={option.value} type="button"
              className={`flex cursor-pointer items-center gap-1.5 rounded-sm px-2 py-1 text-xs transition-colors ${value === option.value ? 'bg-accent font-medium text-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'}`}
              onClick={() => { onChange(option.value); setOpen(false); }}>
              <span className="truncate">{option.label}</span>
              {value === option.value && <Check className="ml-auto h-3 w-3 shrink-0" />}
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}

export function ContactDetailPage({ contactId }: { contactId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();

  const { data: contact, isLoading } = useContact(wsId, contactId);
  const { data: activitiesData, refetch: refetchActivities } = useContactActivities(wsId, contactId);
  const { data: associations, refetch: refetchAssociations } = useContactAssociations(wsId, contactId);
  const { data: supportConversationsData } = useContactSupportConversations(wsId, contactId);
  const updateContact = useUpdateContact(wsId);
  const deleteContact = useDeleteContact(wsId);

  const [form, setForm] = useState<FormState | null>(null);
  const [pendingPatch, setPendingPatch] = useState<UpdateCRMContactRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  useTitle(form ? `${form.first_name} ${form.last_name}`.trim() : 'Contact');

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

  useEffect(() => {
    if (saving || Object.keys(pendingPatch).length === 0 || !wsId || !contactId) return;
    const timer = window.setTimeout(async () => {
      const patch = pendingPatch;
      setPendingPatch({});
      setSaving(true);
      try {
        await updateContact.mutateAsync({ id: contactId, ...patch });
        setSaveError(null);
      } catch {
        setSaveError('Failed to save');
        setPendingPatch((current) => ({ ...patch, ...current }));
      }
      setSaving(false);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [wsId, contactId, pendingPatch, saving]);

  const queuePatch = (patch: UpdateCRMContactRequest) => setPendingPatch((current) => ({ ...current, ...patch }));

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K], patch: UpdateCRMContactRequest) => {
    setForm((current) => (current ? { ...current, [key]: value } : current));
    queuePatch(patch);
  };

  const handleDelete = async () => {
    try {
      await deleteContact.mutateAsync(contactId);
      toast.success('Contact deleted');
      navigate({ to: '/w/$slug/crm/contacts', params: { slug: wsSlug } });
    } catch {
      toast.error('Failed to delete contact');
    }
  };

  const goBack = () => navigate({ to: '/w/$slug/crm/contacts', params: { slug: wsSlug } });

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!contact || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">Contact not found</p>
        <Button variant="outline" size="sm" onClick={goBack}>
          <ArrowLeft className="mr-1 h-3.5 w-3.5" />
          Back to Contacts
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
          <User className="h-3.5 w-3.5 shrink-0 text-blue-500" />
          <button type="button" className="shrink-0 hover:text-foreground transition-colors cursor-pointer" onClick={goBack}>
            Contacts
          </button>
          <ChevronRight className="h-3 w-3 shrink-0" />
          <span className="truncate font-medium text-foreground">
            {`${form.first_name} ${form.last_name}`.trim() || 'Untitled'}
          </span>
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
          {/* Name inputs */}
          <div className="flex gap-2">
            <input
              className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
              value={form.first_name}
              onChange={(e) => updateField('first_name', e.target.value, { first_name: e.target.value })}
              placeholder="First name"
            />
            <input
              className="w-full bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
              value={form.last_name}
              onChange={(e) => updateField('last_name', e.target.value, { last_name: e.target.value })}
              placeholder="Last name"
            />
          </div>
          <p className="mt-1 text-xs text-muted-foreground">{contact.display_id}</p>

          <Separator className="my-6" />

          {/* Activity */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Activity</h3>
            <div className="mt-3">
              <ActivityTimeline
                activities={activitiesData?.data ?? []}
                workspaceId={wsId}
                contactId={contactId}
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
              <EmailTimeline workspaceId={wsId} contactId={contactId} />
            </div>
          </div>

          <Separator className="my-6" />

          {/* Support Conversations */}
          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Support Conversations</h3>
            <div className="mt-3">
              {(!supportConversationsData?.data || supportConversationsData.data.length === 0) ? (
                <p className="text-sm text-muted-foreground">No support conversations linked to this contact.</p>
              ) : (
                <div className="space-y-2">
                  {supportConversationsData.data.map((conversation) => (
                    <Link
                      key={conversation.id}
                      to="/w/$slug/pm/support"
                      params={{ slug: wsSlug }}
                      className="flex items-center gap-3 rounded-md border border-border/60 px-3 py-2 text-sm transition-colors hover:bg-muted/50"
                    >
                      <MessageSquare className="h-4 w-4 shrink-0 text-muted-foreground" />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="text-xs text-muted-foreground">#{conversation.display_id}</span>
                          <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                            {conversation.status}
                          </Badge>
                          <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                            {conversation.priority}
                          </Badge>
                        </div>
                        <p className="mt-0.5 truncate font-medium">{conversation.subject}</p>
                      </div>
                      <span className="shrink-0 text-[10px] text-muted-foreground">
                        {new Date(conversation.created_at).toLocaleDateString()}
                      </span>
                    </Link>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Right sidebar */}
        <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6">
          <h3 className="mb-4 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Details</h3>

          <div className="grid grid-cols-[16px_80px_1fr] gap-x-2 gap-y-3">
            <MetadataRow icon={Tag} label="Stage">
              <SidebarPopoverSelect value={form.lifecycle_stage} options={lifecycleOptions}
                onChange={(v) => updateField('lifecycle_stage', v, { lifecycle_stage: v })} />
            </MetadataRow>
            <MetadataRow icon={Tag} label="Status">
              <SidebarPopoverSelect value={form.lead_status} options={leadStatusOptions}
                onChange={(v) => updateField('lead_status', v, { lead_status: v })} />
            </MetadataRow>
            <MetadataRow icon={Mail} label="Email">
              <input className="w-full bg-transparent text-xs outline-none" value={form.email}
                onChange={(e) => updateField('email', e.target.value, { email: e.target.value })} placeholder="—" />
            </MetadataRow>
            <MetadataRow icon={Phone} label="Phone">
              <input className="w-full bg-transparent text-xs outline-none" value={form.phone}
                onChange={(e) => updateField('phone', e.target.value, { phone: e.target.value })} placeholder="—" />
            </MetadataRow>
            <MetadataRow icon={Briefcase} label="Job Title">
              <input className="w-full bg-transparent text-xs outline-none" value={form.job_title}
                onChange={(e) => updateField('job_title', e.target.value, { job_title: e.target.value })} placeholder="—" />
            </MetadataRow>
            <MetadataRow icon={Globe} label="Source">
              <input className="w-full bg-transparent text-xs outline-none" value={form.source}
                onChange={(e) => updateField('source', e.target.value, { source: e.target.value })} placeholder="—" />
            </MetadataRow>
          </div>

          <Separator className="my-4" />
          <EnrichmentCard workspaceId={wsId} objectType="contact" objectId={contactId} />

          <Separator className="my-4" />
          <BuyerSignals workspaceId={wsId} contactId={contactId} />

          <Separator className="my-4" />
          <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Associations</h3>
          <AssociationsList workspaceId={wsId} slug={wsSlug} associations={associations ?? []}
            currentObjectType="contact" currentObjectId={contactId} onAssociationRemoved={() => refetchAssociations()} />
        </aside>
      </div>

      <ConfirmDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen} title="Delete contact"
        description="Are you sure? This action cannot be undone." confirmLabel="Delete" variant="destructive" onConfirm={handleDelete} />
    </div>
  );
}
