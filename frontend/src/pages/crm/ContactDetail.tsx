import { useEffect, useState } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  ArrowLeft02Icon,
  Briefcase01Icon,
  Building03Icon,
  Tick01Icon,
  ArrowRight01Icon,
  GlobeIcon,
  Loading01Icon,
  Mail01Icon,
  Message01Icon,
  TelephoneIcon,
  Tag01Icon,
  Delete01Icon,
  UserIcon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
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
  useContactEmails,
  useContactCalendar,
} from '@/hooks/queries';
import { ActivityTimeline } from '@/components/crm/ActivityTimeline';
import { EmailTimeline } from '@/components/crm/EmailTimeline';
import { CalendarEvents } from '@/components/crm/CalendarEvents';
import { BuyerSignals } from '@/components/crm/BuyerSignals';
import { EntitySummaryCard } from '@/components/crm/EntitySummaryCard';
import { EnrichmentCard } from '@/components/crm/EnrichmentCard';
import { AssociationsList } from '@/components/crm/AssociationsList';
import { useTitle } from '@/hooks/useTitle';
import { cn } from '@/lib/utils';
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

type ContactTab = 'overview' | 'emails' | 'meetings';

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
  value,
  options,
  onChange,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
}) {
  const [open, setOpen] = useState(false);
  const current = options.find((option) => option.value === value);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        >
          <span className="truncate">{current?.label ?? value}</span>
          <ArrowRight01Icon className="h-3 w-3 rotate-90 text-muted-foreground" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-40 p-0.5" align="start">
        <div className="flex max-h-64 flex-col overflow-y-auto">
          {options.map((option) => (
            <button
              key={option.value}
              type="button"
              className={cn(
                'flex items-center gap-2 rounded-md px-2.5 py-2 text-sm transition-colors',
                value === option.value
                  ? 'bg-accent font-medium text-foreground'
                  : 'text-muted-foreground hover:bg-accent hover:text-foreground',
              )}
              onClick={() => {
                onChange(option.value);
                setOpen(false);
              }}
            >
              <span className="truncate">{option.label}</span>
              {value === option.value ? <Tick01Icon className="ml-auto h-3.5 w-3.5 shrink-0" /> : null}
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
  const { data: emailsData } = useContactEmails(wsId, contactId);
  const { data: meetingsData } = useContactCalendar(wsId, contactId);
  const { data: activitiesData, refetch: refetchActivities } = useContactActivities(wsId, contactId);
  const { data: associations, refetch: refetchAssociations } = useContactAssociations(wsId, contactId);
  const { data: supportConversationsData } = useContactSupportConversations(wsId, contactId);
  const updateContact = useUpdateContact(wsId);
  const deleteContact = useDeleteContact(wsId);

  const [form, setForm] = useState<FormState | null>(null);
  const [activeTab, setActiveTab] = useState<ContactTab>('emails');
  const [pendingPatch, setPendingPatch] = useState<UpdateCRMContactRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  const fullName = `${form?.first_name ?? ''} ${form?.last_name ?? ''}`.trim() || 'Untitled contact';
  const primaryCompany = (associations ?? []).find(
    (association) => association.to_object_type === 'company' || association.from_object_type === 'company',
  );
  const companyName = primaryCompany?.linked_object_name;
  const emailCount = emailsData?.total ?? emailsData?.data?.length ?? 0;
  const meetingCount = meetingsData?.total ?? meetingsData?.data?.length ?? 0;
  const supportTicketCount = supportConversationsData?.data?.length ?? 0;

  useTitle(fullName);

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
    if (saving || Object.keys(pendingPatch).length === 0 || !wsId || !contactId) {
      return;
    }

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
  }, [contactId, pendingPatch, saving, updateContact, wsId]);

  const queuePatch = (patch: UpdateCRMContactRequest) => {
    setPendingPatch((current) => ({ ...current, ...patch }));
  };

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
        <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!contact || !form) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3">
        <p className="text-sm text-muted-foreground">Contact not found</p>
        <Button variant="outline" size="sm" onClick={goBack}>
          <ArrowLeft02Icon className="mr-1 h-3.5 w-3.5" />
          Back to contacts
        </Button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {/* Header bar */}
      <div className="flex items-center gap-2 border-b border-border/60 px-4 py-2.5">
        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={goBack}>
          <ArrowLeft02Icon className="h-4 w-4" />
        </Button>

        <div className="flex min-w-0 items-center gap-1 text-sm text-muted-foreground">
          <UserIcon className="h-3.5 w-3.5 shrink-0 text-sky-500" />
          <button type="button" className="shrink-0 transition-colors hover:text-foreground cursor-pointer" onClick={goBack}>
            Contacts
          </button>
          <ArrowRight01Icon className="h-3 w-3 shrink-0" />
          <span className="truncate font-medium text-foreground">{fullName}</span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <SaveIndicator saving={saving} error={saveError} />
          <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0 hover:text-destructive" onClick={() => setDeleteConfirmOpen(true)}>
            <Delete01Icon className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Tabs + content */}
      <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as ContactTab)} className="min-h-0 flex-1 flex flex-col">
        <div className="border-b border-border/60 px-4">
          <TabsList variant="line" className="h-auto gap-6 rounded-none border-none p-0">
            <TabsTrigger
              value="overview"
              className="h-auto rounded-none border-none px-0 pb-3 pt-2 text-sm data-[state=active]:bg-transparent data-[state=active]:shadow-none"
            >
              Overview
            </TabsTrigger>
            <TabsTrigger
              value="emails"
              className="h-auto rounded-none border-none px-0 pb-3 pt-2 text-sm data-[state=active]:bg-transparent data-[state=active]:shadow-none"
            >
              Emails
              <span className="text-xs text-muted-foreground">{emailCount}</span>
            </TabsTrigger>
            <TabsTrigger
              value="meetings"
              className="h-auto rounded-none border-none px-0 pb-3 pt-2 text-sm data-[state=active]:bg-transparent data-[state=active]:shadow-none"
            >
              Meetings
              <span className="text-xs text-muted-foreground">{meetingCount}</span>
            </TabsTrigger>
          </TabsList>
        </div>

        <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[1fr_300px]">
          {/* Left column */}
          <div className="min-h-0 overflow-y-auto">
            <TabsContent value="overview" className="mt-0 h-full overflow-y-auto px-8 py-6">
              {/* Name */}
              <div className="grid min-w-0 gap-2 sm:grid-cols-2">
                <input
                  className="min-w-0 bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
                  value={form.first_name}
                  onChange={(event) => updateField('first_name', event.target.value, { first_name: event.target.value })}
                  placeholder="First name"
                />
                <input
                  className="min-w-0 bg-transparent text-2xl font-bold text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
                  value={form.last_name}
                  onChange={(event) => updateField('last_name', event.target.value, { last_name: event.target.value })}
                  placeholder="Last name"
                />
              </div>
              <p className="mt-1 text-xs text-muted-foreground">{contact.display_id}</p>

              <Separator className="my-6" />

              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Summary</h3>
                <div className="mt-3">
                  <EntitySummaryCard workspaceId={wsId} contactId={contactId} />
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
                    contactId={contactId}
                    onActivityCreated={() => refetchActivities()}
                    onActivityDeleted={() => refetchActivities()}
                  />
                </div>
              </div>

              <Separator className="my-6" />

              {/* Signals + Enrichment */}
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Signals & enrichment</h3>
                <div className="mt-3 grid gap-6 2xl:grid-cols-2">
                  <BuyerSignals workspaceId={wsId} contactId={contactId} />
                  <EnrichmentCard workspaceId={wsId} objectType="contact" objectId={contactId} />
                </div>
              </div>

              {supportTicketCount > 0 && (
                <>
                  <Separator className="my-6" />

                  {/* Support Conversations */}
                  <div>
                    <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                      Support conversations ({supportTicketCount})
                    </h3>
                    <div className="mt-3 space-y-2">
                      {supportConversationsData!.data.map((conversation) => (
                        <Link
                          key={conversation.id}
                          to="/w/$slug/pm/support"
                          params={{ slug: wsSlug }}
                          className="flex items-center gap-3 rounded-md border border-border/60 px-4 py-3 text-sm transition-colors hover:bg-muted/40"
                        >
                          <Message01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="text-xs text-muted-foreground">#{conversation.display_id}</span>
                              <Badge variant="secondary" className="px-2 py-0 text-[10px]">
                                {conversation.status}
                              </Badge>
                              <Badge variant="secondary" className="px-2 py-0 text-[10px]">
                                {conversation.priority}
                              </Badge>
                            </div>
                            <p className="mt-1 truncate font-medium text-foreground">{conversation.subject}</p>
                          </div>
                          <span className="shrink-0 text-xs text-muted-foreground">
                            {new Date(conversation.created_at).toLocaleDateString()}
                          </span>
                        </Link>
                      ))}
                    </div>
                  </div>
                </>
              )}
            </TabsContent>

            <TabsContent value="emails" className="mt-0 h-full overflow-y-auto">
              <EmailTimeline workspaceId={wsId} contactId={contactId} />
            </TabsContent>

            <TabsContent value="meetings" className="mt-0 h-full overflow-y-auto px-8 py-6">
              <CalendarEvents workspaceId={wsId} contactId={contactId} />
            </TabsContent>
          </div>

          {/* Right sidebar */}
          <aside className="min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6">
            <h3 className="mb-4 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Details</h3>

            <div className="grid grid-cols-[16px_80px_1fr] gap-x-2 gap-y-3">
              <MetadataRow icon={Building03Icon} label="Company">
                {companyName ? (
                  <span className="text-xs text-foreground">{companyName}</span>
                ) : (
                  <span className="text-xs text-muted-foreground">—</span>
                )}
              </MetadataRow>

              <MetadataRow icon={Briefcase01Icon} label="Job title">
                <input
                  className="w-full bg-transparent text-xs outline-none"
                  value={form.job_title}
                  onChange={(event) => updateField('job_title', event.target.value, { job_title: event.target.value })}
                  placeholder="—"
                />
              </MetadataRow>

              <MetadataRow icon={Mail01Icon} label="Email">
                <input
                  className="w-full bg-transparent text-xs outline-none"
                  value={form.email}
                  onChange={(event) => updateField('email', event.target.value, { email: event.target.value })}
                  placeholder="—"
                />
              </MetadataRow>

              <MetadataRow icon={TelephoneIcon} label="TelephoneIcon">
                <input
                  className="w-full bg-transparent text-xs outline-none"
                  value={form.phone}
                  onChange={(event) => updateField('phone', event.target.value, { phone: event.target.value })}
                  placeholder="—"
                />
              </MetadataRow>

              <MetadataRow icon={GlobeIcon} label="Source">
                <input
                  className="w-full bg-transparent text-xs outline-none"
                  value={form.source}
                  onChange={(event) => updateField('source', event.target.value, { source: event.target.value })}
                  placeholder="—"
                />
              </MetadataRow>

              <MetadataRow icon={Tag01Icon} label="Display ID">
                <span className="text-xs text-muted-foreground">{contact.display_id}</span>
              </MetadataRow>
            </div>

            <Separator className="my-4" />

            <h3 className="mb-4 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Lifecycle</h3>
            <div className="grid grid-cols-[16px_80px_1fr] gap-x-2 gap-y-3">
              <MetadataRow icon={Tag01Icon} label="Stage">
                <SidebarPopoverSelect
                  value={form.lifecycle_stage}
                  options={lifecycleOptions}
                  onChange={(value) => updateField('lifecycle_stage', value, { lifecycle_stage: value })}
                />
              </MetadataRow>

              <MetadataRow icon={Tag01Icon} label="Status">
                <SidebarPopoverSelect
                  value={form.lead_status}
                  options={leadStatusOptions}
                  onChange={(value) => updateField('lead_status', value, { lead_status: value })}
                />
              </MetadataRow>
            </div>

            <Separator className="my-4" />

            <AssociationsList
              workspaceId={wsId}
              slug={wsSlug}
              associations={associations ?? []}
              currentObjectType="contact"
              currentObjectId={contactId}
              onAssociationRemoved={() => refetchAssociations()}
            />
          </aside>
        </div>
      </Tabs>

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
