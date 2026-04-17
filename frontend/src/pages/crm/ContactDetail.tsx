import { useEffect, useMemo, useState } from 'react';
import { Link, useLocation, useNavigate } from '@tanstack/react-router';
import { format, formatDistanceToNow } from 'date-fns';
import { toast } from 'sonner';
import {
  ArrowLeft02Icon,
  ArrowRight01Icon,
  Calendar01Icon,
  Delete01Icon,
  DollarCircleIcon,
  GlobeIcon,
  Loading01Icon,
  Mail01Icon,
  Message01Icon,
  MoreVerticalIcon,
  PlusSignIcon,
  Search01Icon,
  Tag01Icon,
  TelephoneIcon,
  UserIcon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
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
  useCreateCRMActivity,
  useDeleteCRMActivity,
  useEmailAccounts,
  useTasks,
  useCreateAssociation,
  useDeleteAssociation,
} from '@/hooks/queries';
import { EmailTimeline } from '@/components/crm/EmailTimeline';
import { CalendarEvents } from '@/components/crm/CalendarEvents';
import { BuyerSignals } from '@/components/crm/BuyerSignals';
import { EntitySummaryCard } from '@/components/crm/EntitySummaryCard';
import { EnrichmentCard } from '@/components/crm/EnrichmentCard';
import { LinkedTasksPanel } from '@/components/crm/LinkedTasksPanel';
import { crmSearchService } from '@/lib/services/crmService';
import { supportService } from '@/lib/services/supportService';
import { useTitle } from '@/hooks/useTitle';
import { cn } from '@/lib/utils';
import type {
  CRMActivityType,
  CRMEmailProvider,
  CRMSearchResult,
  LifecycleStage,
  LeadStatus,
  UnifiedActivityItem,
  UpdateCRMContactRequest,
} from '@/lib/crmTypes';

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

type ContactTab = 'overview' | 'emails' | 'meetings' | 'tasks' | 'deals' | 'support';
type ContactSidebarSection = 'primary-company' | 'other-companies' | 'deals' | 'support' | 'tasks';

// ── Constants ──

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

const activityCreationTypes: { type: CRMActivityType; label: string; icon: typeof Message01Icon }[] = [
  { type: 'note', label: '+ Note', icon: Message01Icon },
  { type: 'call', label: 'Log call', icon: TelephoneIcon },
  { type: 'meeting', label: 'Log meeting', icon: Calendar01Icon },
];

const SIDEBAR_PREVIEW_LIMIT = 3;

// ── Source label for recent activity ──

function sourceLabel(source: string): string {
  switch (source) {
    case 'gmail': return 'Gmail';
    case 'microsoft': return 'Outlook';
    case 'manual': return 'manual';
    default: return source;
  }
}

// ── Small helpers ──

function MetadataRow({ icon: Icon, label, children }: { icon: React.ElementType; label: string; children: React.ReactNode }) {
  return (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0 self-center text-muted-foreground" />
      <span className="self-center text-xs text-muted-foreground">{label}</span>
      <div className="min-w-0 self-center">{children}</div>
    </>
  );
}

function SidebarAssociationSection({
  title,
  count,
  expanded,
  onToggle,
  onAdd,
  addDisabled = false,
  children,
}: {
  title: string;
  count: number;
  expanded: boolean;
  onToggle: () => void;
  onAdd: () => void;
  addDisabled?: boolean;
  children: React.ReactNode;
}) {
  const canToggle = count > SIDEBAR_PREVIEW_LIMIT;
  const hiddenCount = Math.max(count - SIDEBAR_PREVIEW_LIMIT, 0);

  return (
    <>
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {title}
          {count > 0 && <span className="ml-1.5 font-normal">{count}</span>}
        </h3>
        <button
          type="button"
          className="rounded-md p-0.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:pointer-events-none disabled:opacity-40"
          onClick={onAdd}
          disabled={addDisabled}
          aria-label={`Add ${title.toLowerCase()}`}
        >
          <PlusSignIcon className="h-3.5 w-3.5" />
        </button>
      </div>

      {count > 0 && (
        <>
          <div className="mt-2 space-y-1">{children}</div>
          {canToggle && (
            <button
              type="button"
              className="mt-1 inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-medium uppercase tracking-wide text-muted-foreground transition-colors hover:bg-muted/40 hover:text-foreground"
              onClick={onToggle}
            >
              <ArrowRight01Icon className={cn('h-3 w-3 transition-transform', expanded && 'rotate-90')} />
              {expanded ? 'Show less' : `Show ${hiddenCount} more`}
            </button>
          )}
        </>
      )}
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
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}

// ── Status badge colors for support ──

function statusColor(status: string) {
  switch (status) {
    case 'open': return 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400';
    case 'waiting_on_customer': return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400';
    case 'resolved': return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-400';
    default: return 'bg-muted text-muted-foreground';
  }
}

// ── Main component ──

export function ContactDetailPage({ contactId }: { contactId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const location = useLocation();

  // ── Data hooks ──
  const { data: contact, isLoading } = useContact(wsId, contactId);
  const { data: emailsData } = useContactEmails(wsId, contactId);
  const { data: meetingsData } = useContactCalendar(wsId, contactId);
  const { data: activitiesData, refetch: refetchActivities } = useContactActivities(wsId, contactId);
  const { data: associations, refetch: refetchAssociations } = useContactAssociations(wsId, contactId);
  const { data: supportConversationsData, refetch: refetchSupportConversations } = useContactSupportConversations(wsId, contactId);
  const { data: emailAccounts } = useEmailAccounts(wsId);
  const { data: tasksData } = useTasks(wsId, { contact_id: contactId });

  const updateContact = useUpdateContact(wsId);
  const deleteContact = useDeleteContact(wsId);
  const createActivity = useCreateCRMActivity(wsId);
  const deleteActivityMutation = useDeleteCRMActivity(wsId);
  const createAssociation = useCreateAssociation(wsId);
  const deleteAssociation = useDeleteAssociation(wsId);

  // ── Form state ──
  const [form, setForm] = useState<FormState | null>(null);
  const [activeTab, setActiveTab] = useState<ContactTab>('overview');
  const [pendingPatch, setPendingPatch] = useState<UpdateCRMContactRequest>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  // ── Activity creation state (moved from ActivityTimeline) ──
  const [creatingType, setCreatingType] = useState<CRMActivityType | null>(null);
  const [newSubject, setNewSubject] = useState('');
  const [newBody, setNewBody] = useState('');

  // ── Activity delete state ──
  const [deleteActivityId, setDeleteActivityId] = useState<string | null>(null);

  // ── Deal picker state ──
  const [dealPickerOpen, setDealPickerOpen] = useState(false);
  const [dealQuery, setDealQuery] = useState('');
  const [dealSearching, setDealSearching] = useState(false);
  const [dealResults, setDealResults] = useState<CRMSearchResult[]>([]);
  const [companyPickerOpen, setCompanyPickerOpen] = useState(false);
  const [companyQuery, setCompanyQuery] = useState('');
  const [companySearching, setCompanySearching] = useState(false);
  const [companyResults, setCompanyResults] = useState<CRMSearchResult[]>([]);
  const [companyPickerMode, setCompanyPickerMode] = useState<'primary' | 'secondary'>('primary');
  const [removeAssocId, setRemoveAssocId] = useState<string | null>(null);
  const [expandedSections, setExpandedSections] = useState<Record<ContactSidebarSection, boolean>>({
    'primary-company': false,
    'other-companies': false,
    deals: false,
    support: false,
    tasks: false,
  });

  // ── Provider map for source badges ──
  const accountProviderMap = useMemo(() => {
    const map = new Map<string, CRMEmailProvider>();
    for (const account of emailAccounts ?? []) {
      map.set(account.id, account.provider);
    }
    return map;
  }, [emailAccounts]);

  // ── Unified activity merge (for Overview recent activity) ──
  const unifiedItems = useMemo<UnifiedActivityItem[]>(() => {
    const items: UnifiedActivityItem[] = [];

    for (const a of activitiesData?.data ?? []) {
      items.push({ kind: 'activity', data: a, timestamp: a.occurred_at, source: 'manual' });
    }

    for (const e of emailsData?.data ?? []) {
      const provider = accountProviderMap.get(e.email_account_id) ?? 'unknown';
      items.push({ kind: 'email', data: e, timestamp: e.sent_at, source: provider });
    }

    for (const c of meetingsData?.data ?? []) {
      const provider = accountProviderMap.get(c.email_account_id) ?? 'unknown';
      items.push({ kind: 'calendar', data: c, timestamp: c.start_time, source: provider });
    }

    return items.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());
  }, [activitiesData, emailsData, meetingsData, accountProviderMap]);

  // ── Derived counts ──
  const tasks = tasksData?.data ?? [];
  const taskCount = tasks.length;
  const emailCount = emailsData?.total ?? emailsData?.data?.length ?? 0;
  const syncedMeetingCount = meetingsData?.total ?? meetingsData?.data?.length ?? 0;
  const manualMeetings = (activitiesData?.data ?? []).filter(
    (a) => a.activity_type === 'meeting',
  );
  const meetingCount = syncedMeetingCount + manualMeetings.length;
  const notesAndCalls = (activitiesData?.data ?? []).filter(
    (a) => a.activity_type === 'note' || a.activity_type === 'call',
  );

  const linkedAssociations = useMemo(() => {
    return (associations ?? []).map((a) => {
      const isFrom = a.from_object_type === 'contact' && a.from_object_id === contactId;
      return { ...a, linkedType: isFrom ? a.to_object_type : a.from_object_type, linkedId: isFrom ? a.to_object_id : a.from_object_id };
    });
  }, [associations, contactId]);
  const companyAssociations = useMemo(
    () => linkedAssociations.filter((a) => a.linkedType === 'company'),
    [linkedAssociations],
  );
  const primaryCompanyAssociation = useMemo(
    () => companyAssociations.find((assoc) => assoc.association_label === 'primary') ?? companyAssociations[0] ?? null,
    [companyAssociations],
  );
  const otherCompanyAssociations = useMemo(
    () => companyAssociations.filter((assoc) => assoc.id !== primaryCompanyAssociation?.id),
    [companyAssociations, primaryCompanyAssociation],
  );
  const dealAssociations = useMemo(
    () => linkedAssociations.filter((a) => a.linkedType === 'deal'),
    [linkedAssociations],
  );
  const taskAssociations = useMemo(
    () => linkedAssociations.filter((a) => a.linkedType === 'task'),
    [linkedAssociations],
  );
  const taskAssociationIdByTaskId = useMemo(
    () => new Map(taskAssociations.map((assoc) => [assoc.linkedId, assoc.id])),
    [taskAssociations],
  );
  const otherCompanyCount = otherCompanyAssociations.length;
  const linkedCompanyIds = useMemo(
    () => new Set(companyAssociations.map((assoc) => assoc.linkedId)),
    [companyAssociations],
  );
  const dealCount = dealAssociations.length;

  const supportConvos = supportConversationsData?.data ?? [];
  const supportCount = supportConvos.length;
  const openSupportConvos = supportConvos.filter((c) => c.status === 'open');
  const openSupportCount = openSupportConvos.length;
  const mostRecentSupportDate = openSupportConvos.length > 0
    ? openSupportConvos.reduce((latest, c) => {
        const d = new Date(c.updated_at);
        return d > latest ? d : latest;
      }, new Date(0))
    : null;

  const fullName = `${form?.first_name ?? ''} ${form?.last_name ?? ''}`.trim() || 'Untitled contact';
  const visibleOtherCompanyAssociations = expandedSections['other-companies']
    ? otherCompanyAssociations
    : otherCompanyAssociations.slice(0, SIDEBAR_PREVIEW_LIMIT);
  const visibleDealAssociations = expandedSections.deals ? dealAssociations : dealAssociations.slice(0, SIDEBAR_PREVIEW_LIMIT);
  const visibleSupportConvos = expandedSections.support ? supportConvos : supportConvos.slice(0, SIDEBAR_PREVIEW_LIMIT);
  const visibleTasks = expandedSections.tasks ? tasks : tasks.slice(0, SIDEBAR_PREVIEW_LIMIT);

  useTitle(fullName);

  // ── Effects ──
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

  // Deal search effect
  useEffect(() => {
    if (!companyPickerOpen) {
      setCompanyQuery('');
      setCompanyResults([]);
      setCompanySearching(false);
      return;
    }
    if (companyQuery.trim().length < 2) {
      setCompanyResults([]);
      return;
    }
    const handle = window.setTimeout(async () => {
      setCompanySearching(true);
      const response = await crmSearchService.search(wsId, companyQuery.trim());
      setCompanyResults((response.data ?? []).filter((r) => r.type === 'company' && !linkedCompanyIds.has(r.id)));
      setCompanySearching(false);
    }, 250);
    return () => window.clearTimeout(handle);
  }, [companyPickerOpen, companyQuery, wsId, linkedCompanyIds]);

  useEffect(() => {
    if (!dealPickerOpen) {
      setDealQuery('');
      setDealResults([]);
      setDealSearching(false);
      return;
    }
    if (dealQuery.trim().length < 2) {
      setDealResults([]);
      return;
    }
    const handle = window.setTimeout(async () => {
      setDealSearching(true);
      const response = await crmSearchService.search(wsId, dealQuery.trim());
      setDealResults((response.data ?? []).filter((r) => r.type === 'deal'));
      setDealSearching(false);
    }, 250);
    return () => window.clearTimeout(handle);
  }, [dealPickerOpen, dealQuery, wsId]);

  // ── Handlers ──
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

  const handleDeleteActivity = async () => {
    if (!deleteActivityId) return;
    try {
      await deleteActivityMutation.mutateAsync(deleteActivityId);
      toast.success('Activity deleted');
      setDeleteActivityId(null);
      refetchActivities();
    } catch {
      toast.error('Failed to delete activity');
    }
  };

  const handleCreateActivity = async () => {
    if (!wsId || !newSubject.trim() || !creatingType) return;
    try {
      await createActivity.mutateAsync({
        workspace_id: wsId,
        activity_type: creatingType,
        contact_id: contactId,
        subject: newSubject.trim(),
        body: newBody.trim() || undefined,
      });
      toast.success('Activity logged');
      setCreatingType(null);
      setNewSubject('');
      setNewBody('');
      refetchActivities();
    } catch {
      toast.error('Failed to log activity');
    }
  };

  const handleAddDeal = async (dealId: string) => {
    try {
      await createAssociation.mutateAsync({
        workspace_id: wsId,
        from_object_type: 'contact',
        from_object_id: contactId,
        to_object_type: 'deal',
        to_object_id: dealId,
      });
      toast.success('Deal linked');
      setDealPickerOpen(false);
      refetchAssociations();
    } catch {
      toast.error('Failed to link deal');
    }
  };

  const handleAddCompany = async (companyId: string) => {
    try {
      await createAssociation.mutateAsync({
        workspace_id: wsId,
        from_object_type: 'contact',
        from_object_id: contactId,
        to_object_type: 'company',
        to_object_id: companyId,
        association_label: companyPickerMode === 'primary' ? 'primary' : undefined,
      });
      toast.success(companyPickerMode === 'primary' ? 'Primary company set' : 'Company linked');
      setCompanyPickerOpen(false);
      refetchAssociations();
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to link company';
      toast.error(message);
    }
  };

  const handleMakePrimaryCompany = async (companyId: string) => {
    try {
      await createAssociation.mutateAsync({
        workspace_id: wsId,
        from_object_type: 'contact',
        from_object_id: contactId,
        to_object_type: 'company',
        to_object_id: companyId,
        association_label: 'primary',
      });
      toast.success('Primary company updated');
      refetchAssociations();
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to set primary company';
      toast.error(message);
    }
  };

  const handleRemoveAssociation = async () => {
    if (!removeAssocId) return;
    try {
      await deleteAssociation.mutateAsync(removeAssocId);
      toast.success('Association removed');
      setRemoveAssocId(null);
      refetchAssociations();
    } catch {
      toast.error('Failed to remove association');
    }
  };

  const handleUnlinkSupportConversation = async (conversationId: string) => {
    try {
      const response = await supportService.updateConversationCRMContact(wsId, conversationId, { crm_contact_id: null });
      if (response.error) {
        throw new Error(response.error);
      }
      toast.success('Support conversation unlinked');
      refetchSupportConversations();
      refetchAssociations();
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to unlink support conversation';
      toast.error(message);
    }
  };

  const goBack = () => navigate({ to: '/w/$slug/crm/contacts', params: { slug: wsSlug } });
  const toggleExpandedSection = (section: ContactSidebarSection) => {
    setExpandedSections((current) => ({ ...current, [section]: !current[section] }));
  };

  // ── Loading / Not found ──
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
      {/* ── Header bar ── */}
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
          <span className="ml-1 text-xs text-muted-foreground">{contact.display_id}</span>
        </div>

        {/* Activity creation buttons */}
        <div className="ml-auto flex items-center gap-1.5">
          {activityCreationTypes.map(({ type, label, icon: Icon }) => (
            <Button
              key={type}
              variant="outline"
              size="sm"
              className="h-7 gap-1.5 text-xs"
              onClick={() => {
                setCreatingType(creatingType === type ? null : type);
                setNewSubject('');
                setNewBody('');
              }}
            >
              <Icon className="h-3.5 w-3.5" />
              {label}
            </Button>
          ))}

          <Separator orientation="vertical" className="mx-1 h-5" />
          <SaveIndicator saving={saving} error={saveError} />

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0">
                <MoreVerticalIcon className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                className="text-destructive focus:text-destructive"
                onClick={() => setDeleteConfirmOpen(true)}
              >
                <Delete01Icon className="mr-2 h-3.5 w-3.5" />
                Delete contact
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* ── Activity creation dialog ── */}
      <Dialog open={!!creatingType} onOpenChange={(open) => { if (!open) setCreatingType(null); }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-sm capitalize">
              {creatingType === 'note' ? 'Add note' : creatingType === 'call' ? 'Log call' : 'Schedule meeting'}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <Input
              placeholder="Subject"
              value={newSubject}
              onChange={(e) => setNewSubject(e.target.value)}
              className="text-sm"
              autoFocus
            />
            <Textarea
              placeholder="Details..."
              value={newBody}
              onChange={(e) => setNewBody(e.target.value)}
              className="min-h-[80px] text-sm"
              rows={3}
            />
            <div className="flex justify-end gap-2">
              <Button variant="ghost" size="sm" onClick={() => setCreatingType(null)}>
                Cancel
              </Button>
              <Button
                size="sm"
                disabled={!newSubject.trim() || createActivity.isPending}
                onClick={handleCreateActivity}
              >
                {createActivity.isPending ? 'Saving...' : 'Save'}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* ── Tabs + content ── */}
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
              {emailCount > 0 && <span className="ml-1 text-xs text-muted-foreground">{emailCount}</span>}
            </TabsTrigger>
            <TabsTrigger
              value="meetings"
              className="h-auto rounded-none border-none px-0 pb-3 pt-2 text-sm data-[state=active]:bg-transparent data-[state=active]:shadow-none"
            >
              Meetings
              {meetingCount > 0 && <span className="ml-1 text-xs text-muted-foreground">{meetingCount}</span>}
            </TabsTrigger>
            <TabsTrigger
              value="tasks"
              className="h-auto rounded-none border-none px-0 pb-3 pt-2 text-sm data-[state=active]:bg-transparent data-[state=active]:shadow-none"
            >
              Tasks
              {taskCount > 0 && <span className="ml-1 text-xs text-muted-foreground">{taskCount}</span>}
            </TabsTrigger>
            <TabsTrigger
              value="deals"
              className="h-auto rounded-none border-none px-0 pb-3 pt-2 text-sm data-[state=active]:bg-transparent data-[state=active]:shadow-none"
            >
              Deals
              {dealCount > 0 && <span className="ml-1 text-xs text-muted-foreground">{dealCount}</span>}
            </TabsTrigger>
            <TabsTrigger
              value="support"
              className="h-auto rounded-none border-none px-0 pb-3 pt-2 text-sm data-[state=active]:bg-transparent data-[state=active]:shadow-none"
            >
              Support
              {supportCount > 0 && <span className="ml-1 text-xs text-muted-foreground">{supportCount}</span>}
            </TabsTrigger>
          </TabsList>
        </div>

        <div className={cn(
          'grid min-h-0 flex-1 overflow-hidden',
          activeTab === 'overview' ? 'grid-cols-1 lg:grid-cols-[1fr_300px]' : 'grid-cols-1',
        )}>
          {/* ── Left column ── */}
          <div className="min-h-0 overflow-y-auto">
            {/* ──────── OVERVIEW TAB ──────── */}
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

              {/* Support callout banner */}
              {openSupportCount > 0 && mostRecentSupportDate && (
                <>
                  <div className="mt-5 flex items-center gap-3 rounded-lg border border-amber-200/60 bg-amber-50/50 px-4 py-3 dark:border-amber-800/40 dark:bg-amber-950/20">
                    <Message01Icon className="h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
                    <span className="text-sm">
                      {openSupportCount} open support conversation{openSupportCount > 1 ? 's' : ''}
                      {' · '}last activity {format(mostRecentSupportDate, 'dd/MM/yyyy')}
                    </span>
                    <button
                      type="button"
                      onClick={() => setActiveTab('support')}
                      className="ml-auto text-sm font-medium text-amber-700 transition-colors hover:underline dark:text-amber-400"
                    >
                      Open ↗
                    </button>
                  </div>
                </>
              )}

              <Separator className="my-6" />

              {/* AI Summary */}
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">AI Summary</h3>
                <div className="mt-3">
                  <EntitySummaryCard workspaceId={wsId} contactId={contactId} />
                </div>
              </div>

              <Separator className="my-6" />

              {/* Recent activity (3 items across all types) */}
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Recent activity</h3>
                <div className="mt-3">
                  {unifiedItems.length === 0 ? (
                    <p className="py-4 text-center text-sm text-muted-foreground">No activities yet</p>
                  ) : (
                    <div className="space-y-1">
                      {unifiedItems.slice(0, 3).map((item) => {
                        const key = item.kind === 'activity' ? `a-${item.data.id}` : item.kind === 'email' ? `e-${item.data.id}` : `c-${item.data.id}`;
                        let icon = Message01Icon;
                        let typeLabel = '';
                        let title = '';

                        if (item.kind === 'activity') {
                          icon = item.data.activity_type === 'call' ? TelephoneIcon : item.data.activity_type === 'meeting' ? Calendar01Icon : Message01Icon;
                          typeLabel = item.data.activity_type;
                          title = item.data.subject ?? '';
                        } else if (item.kind === 'email') {
                          icon = Mail01Icon;
                          typeLabel = 'Email';
                          title = item.data.subject || '(no subject)';
                        } else if (item.kind === 'calendar') {
                          icon = Calendar01Icon;
                          typeLabel = 'Meeting';
                          title = item.data.title;
                        }

                        const Icon = icon;
                        return (
                          <div key={key} className="flex items-center gap-3 rounded-md px-2 py-2 transition-colors hover:bg-muted/30">
                            <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-muted">
                              <Icon className="h-3.5 w-3.5 text-muted-foreground" />
                            </div>
                            <div className="min-w-0 flex-1">
                              <div className="flex items-center gap-1.5">
                                <span className="text-xs font-medium capitalize">{typeLabel}</span>
                                <span className="text-[10px] text-muted-foreground">· {sourceLabel(item.source)}</span>
                              </div>
                              <p className="truncate text-sm">{title}</p>
                            </div>
                            <span className="shrink-0 text-xs text-muted-foreground">
                              {formatDistanceToNow(new Date(item.timestamp), { addSuffix: true })}
                            </span>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>

              <Separator className="my-6" />

              {/* Notes & Calls */}
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Notes & calls</h3>
                <div className="mt-3">
                  {notesAndCalls.length === 0 ? (
                    <p className="py-4 text-center text-sm text-muted-foreground">No notes or calls logged yet</p>
                  ) : (
                    <div className="space-y-1">
                      {notesAndCalls.map((activity) => {
                        const Icon = activity.activity_type === 'call' ? TelephoneIcon : Message01Icon;
                        return (
                          <div key={activity.id} className="group flex gap-3 rounded-md px-2 py-2.5 transition-colors hover:bg-muted/30">
                            <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-muted">
                              <Icon className="h-3.5 w-3.5 text-muted-foreground" />
                            </div>
                            <div className="min-w-0 flex-1">
                              <div className="flex items-center gap-2">
                                <span className="text-xs font-medium capitalize">{activity.activity_type}</span>
                                <span className="text-xs text-muted-foreground">
                                  {formatDistanceToNow(new Date(activity.occurred_at), { addSuffix: true })}
                                </span>
                                <div className="ml-auto flex gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                                  <Button
                                    variant="ghost"
                                    size="icon"
                                    className="h-6 w-6"
                                    onClick={() => setDeleteActivityId(activity.id)}
                                  >
                                    <Delete01Icon className="h-3 w-3" />
                                  </Button>
                                </div>
                              </div>
                              {activity.subject && <p className="mt-0.5 text-sm">{activity.subject}</p>}
                              {activity.body && (
                                <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{activity.body}</p>
                              )}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>

              <Separator className="my-6" />

              {/* Signals & Enrichment */}
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Signals & enrichment</h3>
                <div className="mt-3 grid gap-6 2xl:grid-cols-2">
                  <BuyerSignals workspaceId={wsId} contactId={contactId} />
                  <EnrichmentCard workspaceId={wsId} objectType="contact" objectId={contactId} />
                </div>
              </div>
            </TabsContent>

            {/* ──────── EMAILS TAB ──────── */}
            <TabsContent value="emails" className="mt-0 h-full overflow-y-auto">
              <EmailTimeline workspaceId={wsId} contactId={contactId} />
            </TabsContent>

            {/* ──────── MEETINGS TAB ──────── */}
            <TabsContent value="meetings" className="mt-0 h-full overflow-y-auto px-8 py-6">
              {/* Manual meetings */}
              {manualMeetings.length > 0 && (
                <div className="mb-6">
                  <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Logged meetings</h3>
                  <div className="space-y-2">
                    {manualMeetings.map((activity) => (
                      <div key={activity.id} className="group flex gap-3 rounded-lg border border-border/60 bg-card px-4 py-3">
                        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted">
                          <Calendar01Icon className="h-4 w-4 text-muted-foreground" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center justify-between">
                            <p className="text-sm font-medium">{activity.subject || 'Untitled meeting'}</p>
                            <div className="flex items-center gap-2">
                              <span className="text-xs text-muted-foreground">
                                {format(new Date(activity.occurred_at), 'MMM d, yyyy')}
                              </span>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6 opacity-0 transition-opacity group-hover:opacity-100"
                                onClick={() => setDeleteActivityId(activity.id)}
                              >
                                <Delete01Icon className="h-3 w-3" />
                              </Button>
                            </div>
                          </div>
                          {activity.body && (
                            <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{activity.body}</p>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Synced calendar events */}
              <CalendarEvents workspaceId={wsId} contactId={contactId} />
            </TabsContent>

            {/* ──────── TASKS TAB ──────── */}
            <TabsContent value="tasks" className="mt-0 h-full overflow-y-auto px-8 py-6">
              <LinkedTasksPanel
                workspaceId={wsId}
                workspaceSlug={wsSlug}
                contactId={contactId}
              />
            </TabsContent>

            {/* ──────── DEALS TAB ──────── */}
            <TabsContent value="deals" className="mt-0 h-full overflow-y-auto px-8 py-6">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-medium">Deals</h3>
                <Button variant="outline" size="sm" className="h-7 gap-1.5 text-xs" onClick={() => setDealPickerOpen(true)}>
                  <PlusSignIcon className="h-3 w-3" />
                  Link deal
                </Button>
              </div>

              {dealAssociations.length === 0 ? (
                <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border/60 px-6 py-12 text-center mt-4">
                  <div className="rounded-full bg-muted p-3">
                    <DollarCircleIcon className="h-7 w-7 text-muted-foreground" />
                  </div>
                  <p className="mt-4 text-base font-medium">No deals linked yet</p>
                  <p className="mt-2 max-w-sm text-sm text-muted-foreground">
                    Link deals to track revenue opportunities for this contact.
                  </p>
                  <Button variant="outline" className="mt-5" onClick={() => setDealPickerOpen(true)}>
                    Link deal
                  </Button>
                </div>
              ) : (
                <div className="mt-4 divide-y divide-border/60 rounded-md border border-border/60">
                  {dealAssociations.map((assoc) => (
                    <div
                      key={assoc.id}
                      className="group flex items-center gap-3 px-4 py-3 transition-colors hover:bg-muted/30"
                    >
                      <button
                        type="button"
                        className="min-w-0 flex-1 text-left"
                        onClick={() => navigate({ to: '/w/$slug/crm/deals/$dealId', params: { slug: wsSlug, dealId: assoc.linkedId } } as never)}
                      >
                        <span className="text-sm font-medium">{assoc.linked_object_name || 'Untitled deal'}</span>
                        {assoc.linked_object_display_id && (
                          <span className="ml-2 text-xs text-muted-foreground">{assoc.linked_object_display_id}</span>
                        )}
                      </button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6 opacity-0 transition-opacity group-hover:opacity-100"
                        onClick={() => setRemoveAssocId(assoc.id)}
                      >
                        <Delete01Icon className="h-3 w-3" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </TabsContent>

            {/* ──────── SUPPORT TAB ──────── */}
            <TabsContent value="support" className="mt-0 h-full overflow-y-auto px-8 py-6">
              <h3 className="text-sm font-medium">Support conversations</h3>

              {supportConvos.length === 0 ? (
                <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border/60 px-6 py-12 text-center mt-4">
                  <div className="rounded-full bg-muted p-3">
                    <Message01Icon className="h-7 w-7 text-muted-foreground" />
                  </div>
                  <p className="mt-4 text-base font-medium">No support conversations</p>
                  <p className="mt-2 max-w-sm text-sm text-muted-foreground">
                    Support conversations linked to this contact will appear here.
                  </p>
                </div>
              ) : (
                <div className="mt-4 space-y-2">
                  {supportConvos.map((conversation) => (
                    <Link
                      key={conversation.id}
                      to="/w/$slug/support/$conversationId"
                      params={{ slug: wsSlug, conversationId: conversation.id }}
                      className="flex items-center gap-3 rounded-md border border-border/60 px-4 py-3 text-sm transition-colors hover:bg-muted/40"
                    >
                      <Message01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-xs text-muted-foreground">#{conversation.display_id}</span>
                          <span className={cn('rounded-full px-2 py-0.5 text-[10px] font-medium', statusColor(conversation.status))}>
                            {conversation.status.replace(/_/g, ' ')}
                          </span>
                          <Badge variant="secondary" className="px-2 py-0 text-[10px]">
                            {conversation.priority}
                          </Badge>
                          {conversation.source && (
                            <span className="text-[10px] text-muted-foreground">{conversation.source}</span>
                          )}
                        </div>
                        <p className="mt-1 truncate font-medium text-foreground">{conversation.subject}</p>
                      </div>
                      <span className="shrink-0 text-xs text-muted-foreground">
                        {format(new Date(conversation.created_at), 'dd/MM/yyyy')}
                      </span>
                    </Link>
                  ))}
                </div>
              )}
            </TabsContent>
          </div>

          {/* ── Right sidebar (Overview tab only) ── */}
          {activeTab === 'overview' && (
            <aside className="hidden min-h-0 overflow-y-auto border-l border-border/60 px-4 py-6 lg:block">
              {/* DETAILS */}
              <h3 className="mb-4 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Details</h3>
              <div className="grid grid-cols-[16px_80px_1fr] gap-x-2 gap-y-3">
                <MetadataRow icon={Mail01Icon} label="Email">
                  <input
                    className="w-full bg-transparent text-xs outline-none"
                    value={form.email}
                    onChange={(event) => updateField('email', event.target.value, { email: event.target.value })}
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

              {/* PRIMARY COMPANY */}
              <SidebarAssociationSection
                title="Primary company"
                count={primaryCompanyAssociation ? 1 : 0}
                expanded={expandedSections['primary-company']}
                onToggle={() => toggleExpandedSection('primary-company')}
                onAdd={() => {
                  setCompanyPickerMode('primary');
                  setCompanyPickerOpen(true);
                }}
                addDisabled={!!primaryCompanyAssociation}
              >
                {primaryCompanyAssociation ? (
                  <div className="group flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40">
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 items-center gap-2 text-left"
                      onClick={() => navigate({ to: '/w/$slug/crm/companies/$companyId', params: { slug: wsSlug, companyId: primaryCompanyAssociation.linkedId } } as never)}
                    >
                      <GlobeIcon className="h-3 w-3 shrink-0 text-muted-foreground" />
                      <span className="truncate font-medium">{primaryCompanyAssociation.linked_object_name || 'Untitled'}</span>
                      {primaryCompanyAssociation.linked_object_display_id && (
                        <span className="ml-auto shrink-0 text-muted-foreground">{primaryCompanyAssociation.linked_object_display_id}</span>
                      )}
                    </button>
                    <Badge variant="secondary" className="h-5 shrink-0 px-1.5 text-[10px] uppercase">
                      Primary
                    </Badge>
                    <button
                      type="button"
                      className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                      onClick={() => setRemoveAssocId(primaryCompanyAssociation.id)}
                      aria-label="Remove primary company association"
                    >
                      <Delete01Icon className="h-3 w-3" />
                    </button>
                  </div>
                ) : null}
              </SidebarAssociationSection>

              <Separator className="my-4" />

              {/* OTHER COMPANIES */}
              <SidebarAssociationSection
                title="Other companies"
                count={otherCompanyCount}
                expanded={expandedSections['other-companies']}
                onToggle={() => toggleExpandedSection('other-companies')}
                onAdd={() => {
                  setCompanyPickerMode('secondary');
                  setCompanyPickerOpen(true);
                }}
              >
                {visibleOtherCompanyAssociations.map((assoc) => (
                  <div
                    key={assoc.id}
                    className="group flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40"
                  >
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 items-center gap-2 text-left"
                      onClick={() => navigate({ to: '/w/$slug/crm/companies/$companyId', params: { slug: wsSlug, companyId: assoc.linkedId } } as never)}
                    >
                      <GlobeIcon className="h-3 w-3 shrink-0 text-muted-foreground" />
                      <span className="truncate font-medium">{assoc.linked_object_name || 'Untitled'}</span>
                      {assoc.linked_object_display_id && (
                        <span className="ml-auto shrink-0 text-muted-foreground">{assoc.linked_object_display_id}</span>
                      )}
                    </button>
                    <button
                      type="button"
                      className="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-foreground group-hover:opacity-100"
                      onClick={() => void handleMakePrimaryCompany(assoc.linkedId)}
                    >
                      Make primary
                    </button>
                    <button
                      type="button"
                      className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                      onClick={() => setRemoveAssocId(assoc.id)}
                      aria-label="Remove company association"
                    >
                      <Delete01Icon className="h-3 w-3" />
                    </button>
                  </div>
                ))}
              </SidebarAssociationSection>

              <Separator className="my-4" />

              {/* DEALS */}
              <SidebarAssociationSection
                title="Deals"
                count={dealCount}
                expanded={expandedSections.deals}
                onToggle={() => toggleExpandedSection('deals')}
                onAdd={() => setDealPickerOpen(true)}
              >
                {visibleDealAssociations.map((assoc) => (
                  <div
                    key={assoc.id}
                    className="group flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40"
                  >
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 items-center gap-2 text-left"
                      onClick={() => navigate({ to: '/w/$slug/crm/deals/$dealId', params: { slug: wsSlug, dealId: assoc.linkedId } } as never)}
                    >
                      <DollarCircleIcon className="h-3 w-3 shrink-0 text-muted-foreground" />
                      <span className="truncate font-medium">{assoc.linked_object_name || 'Untitled'}</span>
                      {assoc.linked_object_display_id && (
                        <span className="ml-auto shrink-0 text-muted-foreground">{assoc.linked_object_display_id}</span>
                      )}
                    </button>
                    <button
                      type="button"
                      className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                      onClick={() => setRemoveAssocId(assoc.id)}
                      aria-label="Remove deal association"
                    >
                      <Delete01Icon className="h-3 w-3" />
                    </button>
                  </div>
                ))}
              </SidebarAssociationSection>

              <Separator className="my-4" />

              {/* SUPPORT */}
              <SidebarAssociationSection
                title="Support"
                count={supportCount}
                expanded={expandedSections.support}
                onToggle={() => toggleExpandedSection('support')}
                onAdd={() => setActiveTab('support')}
              >
                {visibleSupportConvos.map((c) => (
                  <div
                    key={c.id}
                    className="group flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40"
                  >
                    <Link
                      to="/w/$slug/support/$conversationId"
                      params={{ slug: wsSlug, conversationId: c.id }}
                      className="flex min-w-0 flex-1 items-center gap-2"
                    >
                      <span className={cn('rounded-full px-1.5 py-0.5 text-[9px] font-medium leading-none', statusColor(c.status))}>
                        {c.status.replace(/_/g, ' ')}
                      </span>
                      <span className="truncate">{c.subject}</span>
                    </Link>
                    <button
                      type="button"
                      className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                      onClick={() => void handleUnlinkSupportConversation(c.id)}
                      aria-label="Remove support association"
                    >
                      <Delete01Icon className="h-3 w-3" />
                    </button>
                  </div>
                ))}
              </SidebarAssociationSection>

              <Separator className="my-4" />

              {/* TASKS */}
              <SidebarAssociationSection
                title="Tasks"
                count={taskCount}
                expanded={expandedSections.tasks}
                onToggle={() => toggleExpandedSection('tasks')}
                onAdd={() => setActiveTab('tasks')}
              >
                {visibleTasks.map((task) => {
                  const removableAssociationId = taskAssociationIdByTaskId.get(task.id);

                  return (
                    <div
                      key={task.id}
                      className="group flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40"
                    >
                      <button
                        type="button"
                        className="flex min-w-0 flex-1 items-center gap-2 text-left"
                        onClick={() => openTaskRoute(navigate as never, location as never, wsSlug, task.id)}
                      >
                        <span className="shrink-0 text-muted-foreground">{task.task_key}</span>
                        <span className="truncate">{task.name}</span>
                        {task.state_name && (
                          <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">{task.state_name}</span>
                        )}
                      </button>
                      {removableAssociationId ? (
                        <button
                          type="button"
                          className="shrink-0 rounded p-0.5 text-muted-foreground opacity-0 transition-all hover:bg-background hover:text-destructive group-hover:opacity-100"
                          onClick={() => setRemoveAssocId(removableAssociationId)}
                          aria-label="Remove task association"
                        >
                          <Delete01Icon className="h-3 w-3" />
                        </button>
                      ) : null}
                    </div>
                  );
                })}
              </SidebarAssociationSection>

              <Separator className="my-4" />

              {/* SIGNALS */}
              <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Signals</h3>
              <BuyerSignals workspaceId={wsId} contactId={contactId} />
            </aside>
          )}
        </div>
      </Tabs>

      {/* ── Company picker dialog ── */}
      <Dialog open={companyPickerOpen} onOpenChange={setCompanyPickerOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-sm">
              {companyPickerMode === 'primary' ? 'Set primary company' : 'Link company'}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="relative">
              <Search01Icon className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                value={companyQuery}
                onChange={(e) => setCompanyQuery(e.target.value)}
                placeholder={companyPickerMode === 'primary' ? 'Search companies to set as primary' : 'Search companies by name'}
                className="pl-9"
                autoFocus
              />
            </div>
            <div className="max-h-64 space-y-1 overflow-y-auto">
              {companySearching && (
                <div className="flex items-center gap-2 py-4 justify-center text-sm text-muted-foreground">
                  <Loading01Icon className="h-4 w-4 animate-spin" /> Searching...
                </div>
              )}
              {!companySearching && companyResults.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  className="w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent"
                  onClick={() => handleAddCompany(r.id)}
                >
                  <div className="flex items-center gap-2">
                    <GlobeIcon className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    <span className="font-medium truncate">{r.name}</span>
                  </div>
                </button>
              ))}
              {!companySearching && companyQuery.trim().length >= 2 && companyResults.length === 0 && (
                <p className="py-4 text-sm text-muted-foreground text-center">No companies found</p>
              )}
              {!companySearching && companyQuery.trim().length < 2 && (
                <p className="py-4 text-sm text-muted-foreground text-center">Type at least 2 characters to search</p>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* ── Deal picker dialog ── */}
      <Dialog open={dealPickerOpen} onOpenChange={setDealPickerOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-sm">Link deal</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="relative">
              <Search01Icon className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                value={dealQuery}
                onChange={(e) => setDealQuery(e.target.value)}
                placeholder="Search deals by name"
                className="pl-9"
                autoFocus
              />
            </div>
            <div className="max-h-64 space-y-1 overflow-y-auto">
              {dealSearching && (
                <div className="flex items-center gap-2 py-4 justify-center text-sm text-muted-foreground">
                  <Loading01Icon className="h-4 w-4 animate-spin" /> Searching...
                </div>
              )}
              {!dealSearching && dealResults.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  className="w-full rounded-md border px-3 py-2 text-left text-sm transition hover:bg-accent"
                  onClick={() => handleAddDeal(r.id)}
                >
                  <div className="flex items-center gap-2">
                    <DollarCircleIcon className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                    <span className="font-medium truncate">{r.name}</span>
                  </div>
                </button>
              ))}
              {!dealSearching && dealQuery.trim().length >= 2 && dealResults.length === 0 && (
                <p className="py-4 text-sm text-muted-foreground text-center">No deals found</p>
              )}
              {!dealSearching && dealQuery.trim().length < 2 && (
                <p className="py-4 text-sm text-muted-foreground text-center">Type at least 2 characters to search</p>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* ── Delete contact confirm ── */}
      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Delete contact"
        description="Are you sure? This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
      />

      {/* ── Remove association confirm ── */}
      <ConfirmDialog
        open={!!removeAssocId}
        onOpenChange={(open) => !open && setRemoveAssocId(null)}
        title="Remove association"
        description="Are you sure you want to remove this association?"
        confirmLabel="Remove"
        variant="destructive"
        onConfirm={handleRemoveAssociation}
      />

      {/* ── Delete activity confirm ── */}
      <ConfirmDialog
        open={!!deleteActivityId}
        onOpenChange={(open) => !open && setDeleteActivityId(null)}
        title="Delete activity"
        description="Are you sure you want to delete this activity?"
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDeleteActivity}
      />
    </div>
  );
}
