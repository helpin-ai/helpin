import { memo, useEffect, useMemo, useState, type JSX, type SVGProps } from 'react';
import { Link, useLocation, useNavigate } from '@tanstack/react-router';
import { format, formatDistanceToNow } from 'date-fns';
import * as Flags from 'country-flag-icons/react/3x2';
import { toast } from 'sonner';
import {
  ArrowLeft02Icon,
  ArrowRight01Icon,
  Calendar01Icon,
  Clock01Icon,
  Copy01Icon,
  Delete01Icon,
  DollarCircleIcon,
  GlobeIcon,
  InformationCircleIcon,
  LinkSquare01Icon,
  Loading01Icon,
  Mail01Icon,
  MapPinIcon,
  Message01Icon,
  MoreVerticalIcon,
  PlusSignIcon,
  Search01Icon,
  StickyNote01Icon,
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
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsContent } from '@/components/ui/tabs';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { usePageHeaderStore } from '@/stores/pageHeaderStore';
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
import { EnrichmentRailCard } from '@/components/crm/contact-detail/EnrichmentRailCard';
import { LinkedTasksPanel } from '@/components/crm/LinkedTasksPanel';
import { ContactHeader } from '@/components/crm/contact-detail/ContactHeader';
import { ContactComposer } from '@/components/crm/contact-detail/ContactComposer';
import { RailSection } from '@/components/crm/contact-detail/RailSection';
import { CompanyRailCard } from '@/components/crm/contact-detail/CompanyRailCard';
import { SupportTagBadge } from '@/components/support/SupportTagPicker';
import { SocialPlatformIcon } from '@/components/docs/helpcenter/SocialPlatformIcon';
import { useRegisterPageContext } from '@/components/command-bar/pageContext';
import { crmSearchService } from '@/lib/services/crmService';
import { supportService } from '@/lib/services/supportService';
import { BILLING_CHOOSE_PLAN_SEARCH } from '@/lib/billingNavigation';
import { useTitle } from '@/hooks/useTitle';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { cn } from '@/lib/utils';
import type {
  HelpcenterSocialPlatform,
} from '@/lib/docsTypes';
import type {
  CRMEmailProvider,
  CRMSearchResult,
  LifecycleStage,
  LeadStatus,
  CRMContact,
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
  description: string;
  labels: string;
  primary_location: string;
  country_code: string;
  country_name: string;
  linkedin_url: string;
  facebook_url: string;
  instagram_url: string;
  angellist_url: string;
  x_url: string;
  lifecycle_stage: LifecycleStage;
  lead_status: LeadStatus;
  source: string;
}

type ContactTab = 'overview' | 'emails' | 'meetings' | 'tasks' | 'deals' | 'support';
type ContactSidebarSection = 'primary-company' | 'other-companies' | 'deals' | 'support' | 'tasks';
export const contactDetailOverviewGridClassName = 'grid-cols-1 lg:grid-cols-[1fr_360px]';
export const contactDetailOverviewContentClassName = 'mt-0 h-full overflow-y-auto px-8 pb-28 pt-6';
export const buyerSignalsSectionClassName = '';
export const contactDetailOverviewSectionOrder = ['summary', 'composer', 'notes_calls', 'buyer_signals', 'recent_activity'] as const;
export const contactDetailSidebarSectionTitles = ['Details', 'Enrichment', 'Company', 'Deals', 'Open tasks', 'Support'] as const;
export const contactDetailDefaultFieldKeys = [
  'email',
  'phone',
  'job_title',
  'linkedin_url',
  'primary_location',
  'labels',
  'description',
  'last_interaction',
  'lifecycle_stage',
  'lead_status',
] as const;
export const contactDetailExpandedFieldKeys = [
  'source',
  'facebook_url',
  'instagram_url',
  'x_url',
  'angellist_url',
  'first_email',
  'last_email',
  'first_calendar',
  'last_calendar',
  'next_calendar',
  'first_interaction',
  'created_at',
  'updated_at',
] as const;
const isContactLimitError = (error: unknown) =>
  error instanceof Error && error.message.includes('5,000 contacts');
type EnrichedDetailRow = {
  key: string;
  label: string;
  value: string;
  href?: string;
  icon: React.ElementType;
};

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

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function nonEmptyString(value: unknown): string | null {
  if (typeof value !== 'string') return null;
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}

function isHttpUrl(value: string): boolean {
  return /^https?:\/\//i.test(value);
}

function externalHref(value: string): string | undefined {
  if (isHttpUrl(value)) return value;
  if (/^[a-z0-9.-]+\.[a-z]{2,}(?:\/.*)?$/i.test(value)) return `https://${value}`;
  return undefined;
}

function shortUrlLabel(value: string): string {
  try {
    const url = new URL(value);
    return `${url.hostname}${url.pathname === '/' ? '' : url.pathname}`.replace(/^www\./, '');
  } catch {
    return value.replace(/^https?:\/\//i, '').replace(/^www\./i, '');
  }
}

function normalizeCountryCode(code?: string | null): keyof typeof Flags | null {
  const normalized = code?.trim().toUpperCase().replace(/-/g, '_');
  if (!normalized || !/^[A-Z]{2,3}(?:_[A-Z]{2,3})?$/.test(normalized)) {
    return null;
  }
  return normalized as keyof typeof Flags;
}

const ContactCountryFlag = memo(function ContactCountryFlag({
  countryCode,
  countryName,
}: {
  countryCode?: string | null;
  countryName?: string | null;
}) {
  const flagKey = normalizeCountryCode(countryCode);
  if (!flagKey) return null;

  const Flag = Flags[flagKey] as ((props: SVGProps<SVGSVGElement>) => JSX.Element) | undefined;
  if (!Flag) return null;

  const label = countryName?.trim() || countryCode?.trim()?.toUpperCase() || 'Contact country';

  return (
    <QuickTooltip label={label}>
      <span className="inline-flex h-3.5 w-5 shrink-0 items-center justify-center overflow-hidden rounded-[3px] border border-border/70 shadow-sm">
        <Flag aria-label={label} className="h-full w-full object-cover" />
      </span>
    </QuickTooltip>
  );
});

export function formatCRMContactLocation(contact: Pick<CRMContact, 'primary_location' | 'country_name'>): string {
  const primaryLocation = nonEmptyString(contact.primary_location);
  const countryName = nonEmptyString(contact.country_name);
  if (!primaryLocation) return countryName ?? '';
  if (!countryName) return primaryLocation;
  if (primaryLocation.toLowerCase().includes(countryName.toLowerCase())) return primaryLocation;
  return `${primaryLocation}, ${countryName}`;
}

export function normalizeCRMContactLabels(value: string | string[] | null | undefined): string[] {
  const rawLabels = Array.isArray(value) ? value : (value ?? '').split(',');
  const seen = new Set<string>();
  const labels: string[] = [];

  for (const rawLabel of rawLabels) {
    const label = rawLabel.trim();
    const key = label.toLowerCase();
    if (!label || seen.has(key)) continue;
    seen.add(key);
    labels.push(label);
  }

  return labels;
}

type ContactInteractionKind = 'email' | 'calendar' | 'activity' | 'support';
type ContactInteractionPoint = {
  kind: ContactInteractionKind;
  timestamp: string;
};

export function deriveCRMContactInteractionSummary({
  emails = [],
  meetings = [],
  activities = [],
  supportConversations = [],
}: {
  emails?: Array<{ sent_at?: string | null }>;
  meetings?: Array<{ start_time?: string | null }>;
  activities?: Array<{ occurred_at?: string | null }>;
  supportConversations?: Array<{ updated_at?: string | null; created_at?: string | null }>;
}) {
  const makePoint = (kind: ContactInteractionKind, value?: string | null): ContactInteractionPoint[] => {
    const timestamp = nonEmptyString(value);
    return timestamp ? [{ kind, timestamp }] : [];
  };
  const emailPoints = emails.flatMap((email) => makePoint('email', email.sent_at));
  const calendarPoints = meetings.flatMap((meeting) => makePoint('calendar', meeting.start_time));
  const activityPoints = activities.flatMap((activity) => makePoint('activity', activity.occurred_at));
  const supportPoints = supportConversations.flatMap((conversation) => {
    const timestamp = nonEmptyString(conversation.updated_at) ?? nonEmptyString(conversation.created_at);
    return makePoint('support', timestamp);
  });

  const validDate = (point: ContactInteractionPoint) => !Number.isNaN(new Date(point.timestamp).getTime());
  const sortAsc = (left: ContactInteractionPoint, right: ContactInteractionPoint) =>
    new Date(left.timestamp).getTime() - new Date(right.timestamp).getTime();
  const allPoints = [...emailPoints, ...calendarPoints, ...activityPoints, ...supportPoints]
    .filter(validDate)
    .sort(sortAsc);
  const sortedEmails = emailPoints.filter(validDate).sort(sortAsc);
  const sortedCalendar = calendarPoints.filter(validDate).sort(sortAsc);
  const now = Date.now();
  const futureCalendar = sortedCalendar.filter((point) => new Date(point.timestamp).getTime() >= now);

  return {
    firstInteraction: allPoints[0],
    lastInteraction: allPoints.at(-1),
    firstEmail: sortedEmails[0],
    lastEmail: sortedEmails.at(-1),
    firstCalendar: sortedCalendar[0],
    lastCalendar: sortedCalendar.at(-1),
    nextCalendar: futureCalendar[0],
  };
}

function formatRailTimestamp(timestamp?: string | null): string {
  if (!timestamp) return '—';
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return '—';
  return formatDistanceToNow(date, { addSuffix: true });
}

function formatRailTimestampTitle(timestamp?: string | null): string | undefined {
  if (!timestamp) return undefined;
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return undefined;
  return format(date, 'MMM d, yyyy h:mm a');
}

function isBlankEditableValue(value: string | null | undefined): boolean {
  return !value || value.trim().length === 0;
}

export function editableRailFieldClassName(
  value: string | null | undefined,
  options: { mono?: boolean; link?: boolean } = {},
): string {
  const empty = isBlankEditableValue(value);
  return cn(
    'w-full rounded-sm border px-1.5 py-0.5 outline-none transition-colors placeholder:text-muted-foreground/70 focus:border-primary/40 focus:bg-background',
    options.mono ? 'font-mono text-[11.5px]' : 'text-xs',
    empty
      ? 'border-border/60 bg-transparent text-muted-foreground hover:border-border'
      : 'border-transparent bg-transparent text-foreground hover:border-border/50 hover:bg-background/60',
    options.link && !empty && externalHref(value ?? '') && 'text-primary',
  );
}

export const sidebarPopoverSelectTriggerClassName =
  'inline-flex h-6 w-full items-center justify-between gap-1.5 rounded-sm border border-border/60 bg-transparent px-1.5 py-0.5 text-[12px] transition-colors hover:border-border hover:bg-background/60 focus-visible:border-primary/40 focus-visible:outline-none';

export const copyableRailValueClassName =
  'group/copy grid min-w-0 grid-cols-[minmax(0,1fr)_20px] items-start gap-1';

function CopyableRailValue({
  value,
  label,
  children,
}: {
  value?: string | null;
  label: string;
  children: React.ReactNode;
}) {
  const { copy } = useCopyToClipboard();
  const copyValue = nonEmptyString(value);

  return (
    <div className={copyableRailValueClassName}>
      <div className="min-w-0">{children}</div>
      <QuickTooltip label={copyValue ? `Copy ${label}` : `No ${label} to copy`}>
        <button
          type="button"
          className={cn(
            'flex h-6 w-5 items-center justify-center rounded-sm text-muted-foreground transition-opacity hover:bg-muted hover:text-foreground',
            copyValue ? 'opacity-0 group-hover/copy:opacity-100 group-focus-within/copy:opacity-100' : 'pointer-events-none opacity-0',
          )}
          disabled={!copyValue}
          aria-label={`Copy ${label}`}
          onClick={() => {
            if (!copyValue) return;
            copy(copyValue);
            toast.success(`${label} copied`);
          }}
        >
          <Copy01Icon className="h-3.5 w-3.5" />
        </button>
      </QuickTooltip>
    </div>
  );
}

function formatRailEmptyState(timestamp?: string | null, emptyLabel = 'None yet'): string {
  return timestamp ? formatRailTimestamp(timestamp) : emptyLabel;
}

function humanizeEnrichmentKey(key: string): string {
  return key
    .replace(/_url$/i, '')
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function buildEnrichedDetailRows(contact: CRMContact): EnrichedDetailRow[] {
  const rows: EnrichedDetailRow[] = [];
  const avatarURL = nonEmptyString(contact.avatar_url);

  if (avatarURL) {
    const href = externalHref(avatarURL);
    rows.push({
      key: 'avatar_url',
      label: 'Avatar',
      value: shortUrlLabel(avatarURL),
      href,
      icon: LinkSquare01Icon,
    });
  }

  const agentEnrichment = asRecord(contact.custom_properties?.agent_enrichment);
  if (!agentEnrichment) return rows;

  for (const [key, rawValue] of Object.entries(agentEnrichment)) {
    if (key === 'notes') continue;

    const valueRecord = asRecord(rawValue);
    const fieldValue = nonEmptyString(valueRecord?.value ?? rawValue);
    if (!fieldValue) continue;

    const sourceURL = nonEmptyString(valueRecord?.source_url);
    const href = /url$/i.test(key)
      ? externalHref(fieldValue) ?? (sourceURL && isHttpUrl(sourceURL) ? sourceURL : undefined)
      : undefined;

    rows.push({
      key,
      label: humanizeEnrichmentKey(key),
      value: href ? shortUrlLabel(fieldValue) : fieldValue,
      href,
      icon: /url$/i.test(key) ? LinkSquare01Icon : GlobeIcon,
    });
  }

  return rows;
}

function TabBadge({ children, active }: { children: React.ReactNode; active: boolean }) {
  return (
    <span
      className={cn(
        'ml-0.5 text-xs tabular-nums',
        active ? 'text-muted-foreground' : 'text-muted-foreground/70',
      )}
    >
      {children}
    </span>
  );
}

function MetadataRow({ icon: Icon, label, children }: { icon: React.ElementType; label: string; children: React.ReactNode }) {
  return (
    <>
      <span className="flex h-6 w-4 items-center justify-center">
        <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      </span>
      <span className="flex h-6 items-center text-[12px] text-muted-foreground">{label}</span>
      <div className="min-w-0 text-[12px]">{children}</div>
    </>
  );
}

function SocialMetadataRow({
  platform,
  label,
  children,
}: {
  platform: HelpcenterSocialPlatform;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <>
      <span className="flex h-6 w-4 items-center justify-center">
        <SocialPlatformIcon platform={platform} className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      </span>
      <span className="flex h-6 items-center text-[12px] text-muted-foreground">{label}</span>
      <div className="min-w-0 text-[12px]">{children}</div>
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
          className={sidebarPopoverSelectTriggerClassName}
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

function supportStatusDotClass(status: string) {
  if (status === 'open') return 'bg-blue-500';
  if (status === 'waiting_on_customer') return 'bg-purple-500';
  if (status === 'resolved') return 'bg-green-500';
  if (status === 'spam') return 'bg-red-500';
  return 'bg-muted-foreground/40';
}

function supportStatusBadgeClass(status: string) {
  switch (status) {
    case 'open': return 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400';
    case 'waiting_on_customer': return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400';
    case 'resolved': return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-400';
    default: return 'bg-muted text-muted-foreground';
  }
}

function humanizeStatusLabel(value: string) {
  return value
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

// ── Empty-state previews ──

function RecentActivityEmptyState() {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border/60 px-6 py-10 text-center">
      <div className="rounded-full bg-muted p-2.5">
        <Message01Icon className="h-5 w-5 text-muted-foreground" />
      </div>
      <p className="mt-3 text-sm font-medium">No activity yet</p>
      <p className="mt-1 max-w-xs text-xs text-muted-foreground">
        Emails, meetings, and notes for this contact will appear here.
      </p>
    </div>
  );
}

function NotesAndCallsEmptyState({ onAddNote, onLogCall }: { onAddNote: () => void; onLogCall: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border/60 px-6 py-10 text-center">
      <div className="rounded-full bg-muted p-2.5">
        <StickyNote01Icon className="h-5 w-5 text-muted-foreground" />
      </div>
      <p className="mt-3 text-sm font-medium">No notes or calls yet</p>
      <p className="mt-1 max-w-xs text-xs text-muted-foreground">
        Log a call or drop a quick note to keep a shared history.
      </p>
      <div className="mt-4 flex items-center gap-2">
        <Button variant="outline" size="sm" className="h-7 gap-1.5 text-xs" onClick={onLogCall}>
          <TelephoneIcon className="h-3 w-3" />
          Log call
        </Button>
        <Button size="sm" className="h-7 gap-1.5 text-xs" onClick={onAddNote}>
          <StickyNote01Icon className="h-3 w-3" />
          Add note
        </Button>
      </div>
    </div>
  );
}

// ── Main component ──

export function ContactDetailPage({ contactId }: { contactId: string }) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const wsSlug = currentWorkspace?.slug ?? '';
  const navigate = useNavigate();
  const location = useLocation();

  // ── Data hooks ──
  const { data: contact, isLoading, error: contactError } = useContact(wsId, contactId);
  useRegisterPageContext(contact ? {
    entity_type: 'crm_contact',
    entity_id: contact.id,
    display_title: [contact.first_name, contact.last_name].filter(Boolean).join(' ') || contact.email || 'CRM contact',
  } : null, 20);
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

  // ── Inline composer state ──
  const [composerMode, setComposerMode] = useState<'note' | 'call' | 'meeting'>('note');
  const [composerFocusSeq, setComposerFocusSeq] = useState(0);

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
  const [detailsExpanded, setDetailsExpanded] = useState(false);

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
  const enrichedDetailRows = useMemo(
    () => contact ? buildEnrichedDetailRows(contact) : [],
    [contact],
  );
  const interactionSummary = useMemo(
    () => deriveCRMContactInteractionSummary({
      emails: emailsData?.data,
      meetings: meetingsData?.data,
      activities: activitiesData?.data,
      supportConversations: supportConversationsData?.data,
    }),
    [activitiesData, emailsData, meetingsData, supportConversationsData],
  );
  const contactLocation = useMemo(
    () => formatCRMContactLocation({
      primary_location: form?.primary_location,
      country_name: form?.country_name,
    }),
    [form?.country_name, form?.primary_location],
  );

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

  // Push contact name + inline actions into the global Header breadcrumb.
  const setHeaderTitle = usePageHeaderStore((s) => s.setTitleOverride);
  const setHeaderActions = usePageHeaderStore((s) => s.setActions);
  const resetHeader = usePageHeaderStore((s) => s.reset);
  useEffect(() => {
    setHeaderTitle(fullName);
    setHeaderActions(
      <>
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
      </>,
    );
    return () => resetHeader();
  }, [fullName, saving, saveError, setHeaderTitle, setHeaderActions, resetHeader]);

  // ── Effects ──
  useEffect(() => {
    if (contact && !form) {
      setForm({
        first_name: contact.first_name,
        last_name: contact.last_name ?? '',
        email: contact.email ?? '',
        phone: contact.phone ?? '',
        job_title: contact.job_title ?? '',
        description: contact.description ?? '',
        labels: normalizeCRMContactLabels(contact.labels).join(', '),
        primary_location: contact.primary_location ?? '',
        country_code: contact.country_code ?? '',
        country_name: contact.country_name ?? '',
        linkedin_url: contact.linkedin_url ?? '',
        facebook_url: contact.facebook_url ?? '',
        instagram_url: contact.instagram_url ?? '',
        angellist_url: contact.angellist_url ?? '',
        x_url: contact.x_url ?? '',
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
  const goToBilling = () => navigate({
    to: '/w/$slug/settings/billing',
    params: { slug: wsSlug },
    search: BILLING_CHOOSE_PLAN_SEARCH,
  });
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

  if (isContactLimitError(contactError)) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
        <div>
          <h2 className="text-sm font-medium">Upgrade to view CRM contacts</h2>
          <p className="mt-1 max-w-sm text-sm text-muted-foreground">
            The Starter plan includes up to 5,000 contacts. Support can keep capturing new contacts, but CRM contact viewing requires the Growth plan once you exceed that limit.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={goBack}>
            <ArrowLeft02Icon className="mr-1 h-3.5 w-3.5" />
            Back to contacts
          </Button>
          <Button size="sm" onClick={goToBilling}>Upgrade</Button>
        </div>
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
    <div
      className={cn(
        'grid h-full min-h-0',
        activeTab === 'overview' ? contactDetailOverviewGridClassName : 'grid-cols-1',
      )}
    >
      <div className="flex min-h-0 flex-col overflow-hidden">
      {/* ── Contact identity header ── */}
      <ContactHeader
        firstName={form.first_name}
        lastName={form.last_name}
        jobTitle={form.job_title}
        companyName={primaryCompanyAssociation?.linked_object_name ?? undefined}
        companyHref={
          primaryCompanyAssociation
            ? `/w/${wsSlug}/crm/companies/${primaryCompanyAssociation.linkedId}`
            : undefined
        }
        lifecycleStage={form.lifecycle_stage}
        lifecycleLabel={
          lifecycleOptions.find((o) => o.value === form.lifecycle_stage)?.label ?? form.lifecycle_stage
        }
        onNameChange={(first, last) => {
          setForm((current) =>
            current ? { ...current, first_name: first, last_name: last } : current,
          );
          queuePatch({ first_name: first, last_name: last });
        }}
      />

      {/* ── Tabs + content ── */}
      <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as ContactTab)} className="min-h-0 flex-1 flex flex-col gap-0">
        <div className="flex items-center gap-0.5 border-b border-border/60 px-6">
          {[
            { id: 'overview' as const, label: 'Overview', count: 0 },
            { id: 'emails' as const, label: 'Emails', count: emailCount },
            { id: 'meetings' as const, label: 'Meetings', count: meetingCount },
            { id: 'tasks' as const, label: 'Tasks', count: taskCount },
            { id: 'deals' as const, label: 'Deals', count: dealCount },
            { id: 'support' as const, label: 'Support', count: supportCount },
          ].map((t) => {
            const active = activeTab === t.id;
            return (
              <button
                key={t.id}
                type="button"
                onClick={() => setActiveTab(t.id)}
                className={cn(
                  'inline-flex items-center gap-1 whitespace-nowrap border-b-2 px-2.5 py-1.5 -mb-px text-[13px] font-medium transition-colors',
                  active
                    ? 'border-primary text-foreground'
                    : 'border-transparent text-muted-foreground hover:text-foreground',
                )}
              >
                {t.label}
                {t.count > 0 && <TabBadge active={active}>{t.count}</TabBadge>}
              </button>
            );
          })}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
            {/* ──────── OVERVIEW TAB ──────── */}
            <TabsContent value="overview" className={contactDetailOverviewContentClassName}>
              {/* AI Summary (renders its own SUMMARY heading) */}
              <EntitySummaryCard workspaceId={wsId} contactId={contactId} />

              {/* Inline composer (replaces modal) */}
              <div className="mt-6">
                <ContactComposer
                  key={composerFocusSeq}
                  initialMode={composerMode}
                  isPending={createActivity.isPending}
                  onSubmit={async ({ activityType, subject, body }) => {
                    if (!wsId) return;
                    try {
                      await createActivity.mutateAsync({
                        workspace_id: wsId,
                        activity_type: activityType,
                        contact_id: contactId,
                        subject,
                        body,
                      });
                      toast.success('Activity logged');
                      refetchActivities();
                    } catch {
                      toast.error('Failed to log activity');
                      throw new Error('failed');
                    }
                  }}
                />
              </div>

              {/* Support callout banner */}
              {openSupportCount > 0 && mostRecentSupportDate && (
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
              )}

              <Separator className="my-6 bg-border/40" />

              {/* Notes & Calls */}
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Notes & calls</h3>
                <div className="mt-3">
                  {notesAndCalls.length === 0 ? (
                    <NotesAndCallsEmptyState
                      onAddNote={() => {
                        setComposerMode('note');
                        setComposerFocusSeq((n) => n + 1);
                      }}
                      onLogCall={() => {
                        setComposerMode('call');
                        setComposerFocusSeq((n) => n + 1);
                      }}
                    />
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

              <Separator className="my-6 bg-border/40" />

              {/* Buyer signals */}
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Buyer signals</h3>
                <div className="mt-3">
                  <BuyerSignals workspaceId={wsId} contactId={contactId} />
                </div>
              </div>

              <Separator className="my-6 bg-border/40" />

              {/* Recent activity (3 items across all types) */}
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Recent activity</h3>
                <div className="mt-3">
                  {unifiedItems.length === 0 ? (
                    <RecentActivityEmptyState />
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
                          <span className={cn('rounded-full px-2 py-0.5 text-[10px] font-medium', supportStatusBadgeClass(conversation.status))}>
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
        </Tabs>
      </div>

      {/* ── Right sidebar (Overview tab only) ── */}
      {activeTab === 'overview' && (
        <aside className="hidden min-h-0 overflow-y-auto border-l border-border/60 bg-muted/30 pb-10 lg:block">
              {/* DETAILS (always visible, non-collapsible) */}
              <div className="border-b border-border/50 px-4 py-3">
                <div className="grid grid-cols-[16px_84px_1fr] gap-x-2 gap-y-2.5">
                  <MetadataRow icon={Mail01Icon} label="Email">
                    <CopyableRailValue value={form.email} label="email">
                      <input
                        className={editableRailFieldClassName(form.email, { mono: true })}
                        value={form.email}
                        onChange={(event) => updateField('email', event.target.value, { email: event.target.value })}
                        placeholder="Add email"
                      />
                    </CopyableRailValue>
                  </MetadataRow>

                  <MetadataRow icon={TelephoneIcon} label="Phone">
                    <CopyableRailValue value={form.phone} label="phone">
                      <input
                        className={editableRailFieldClassName(form.phone, { mono: true })}
                        value={form.phone}
                        onChange={(event) => updateField('phone', event.target.value, { phone: event.target.value })}
                        placeholder="Add phone"
                      />
                    </CopyableRailValue>
                  </MetadataRow>

                  <MetadataRow icon={UserIcon} label="Job title">
                    <CopyableRailValue value={form.job_title} label="job title">
                      <input
                        className={editableRailFieldClassName(form.job_title)}
                        value={form.job_title}
                        onChange={(event) => updateField('job_title', event.target.value, { job_title: event.target.value })}
                        placeholder="Add job title"
                      />
                    </CopyableRailValue>
                  </MetadataRow>

                  <SocialMetadataRow platform="linkedin" label="LinkedIn">
                    <CopyableRailValue value={form.linkedin_url} label="LinkedIn URL">
                      <input
                        className={cn(editableRailFieldClassName(form.linkedin_url, { link: true }), 'underline-offset-2')}
                        value={form.linkedin_url}
                        onChange={(event) => updateField('linkedin_url', event.target.value, { linkedin_url: event.target.value })}
                        placeholder="Add LinkedIn URL"
                      />
                    </CopyableRailValue>
                  </SocialMetadataRow>

                  <MetadataRow icon={MapPinIcon} label="Location">
                    <CopyableRailValue value={contactLocation || form.primary_location} label="location">
                      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-1.5">
                        <input
                          className={cn(editableRailFieldClassName(form.primary_location), 'min-w-0 flex-1')}
                          value={form.primary_location}
                          onChange={(event) => updateField('primary_location', event.target.value, { primary_location: event.target.value })}
                          placeholder="Add location"
                          title={contactLocation || undefined}
                        />
                        <ContactCountryFlag countryCode={form.country_code} countryName={form.country_name} />
                      </div>
                    </CopyableRailValue>
                  </MetadataRow>

                  <MetadataRow icon={Tag01Icon} label="Labels">
                    <CopyableRailValue value={normalizeCRMContactLabels(form.labels).join(', ')} label="labels">
                      <div className="space-y-1.5">
                        <input
                          className={editableRailFieldClassName(form.labels)}
                          value={form.labels}
                          onChange={(event) => {
                            const nextValue = event.target.value;
                            updateField('labels', nextValue, { labels: normalizeCRMContactLabels(nextValue) });
                          }}
                          placeholder="Add labels"
                        />
                        {normalizeCRMContactLabels(form.labels).length > 0 && (
                          <div className="flex flex-wrap gap-1">
                            {normalizeCRMContactLabels(form.labels).map((label) => (
                              <SupportTagBadge
                                key={label}
                                name={label}
                                className="h-4 max-w-[120px] px-1.5 text-[10px]"
                              />
                            ))}
                          </div>
                        )}
                      </div>
                    </CopyableRailValue>
                  </MetadataRow>

                  <MetadataRow icon={InformationCircleIcon} label="Description">
                    <CopyableRailValue value={form.description} label="description">
                      <input
                        className={editableRailFieldClassName(form.description)}
                        value={form.description}
                        onChange={(event) => updateField('description', event.target.value, { description: event.target.value })}
                        placeholder="Add description"
                      />
                    </CopyableRailValue>
                  </MetadataRow>

                  <MetadataRow icon={Clock01Icon} label="Last touch">
                    <span
                      className="block truncate text-xs text-foreground"
                      title={formatRailTimestampTitle(interactionSummary.lastInteraction?.timestamp)}
                    >
                      {formatRailEmptyState(interactionSummary.lastInteraction?.timestamp, 'No activity yet')}
                    </span>
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

                  {detailsExpanded && (
                    <>
                      <MetadataRow icon={GlobeIcon} label="Source">
                        <CopyableRailValue value={form.source} label="source">
                          <input
                            className={editableRailFieldClassName(form.source)}
                            value={form.source}
                            onChange={(event) => updateField('source', event.target.value, { source: event.target.value })}
                            placeholder="Add source"
                          />
                        </CopyableRailValue>
                      </MetadataRow>

                      <SocialMetadataRow platform="facebook" label="Facebook">
                        <CopyableRailValue value={form.facebook_url} label="Facebook URL">
                          <input
                            className={editableRailFieldClassName(form.facebook_url, { link: true })}
                            value={form.facebook_url}
                            onChange={(event) => updateField('facebook_url', event.target.value, { facebook_url: event.target.value })}
                            placeholder="Add Facebook URL"
                          />
                        </CopyableRailValue>
                      </SocialMetadataRow>

                      <SocialMetadataRow platform="instagram" label="Instagram">
                        <CopyableRailValue value={form.instagram_url} label="Instagram URL">
                          <input
                            className={editableRailFieldClassName(form.instagram_url, { link: true })}
                            value={form.instagram_url}
                            onChange={(event) => updateField('instagram_url', event.target.value, { instagram_url: event.target.value })}
                            placeholder="Add Instagram URL"
                          />
                        </CopyableRailValue>
                      </SocialMetadataRow>

                      <SocialMetadataRow platform="x" label="X">
                        <CopyableRailValue value={form.x_url} label="X URL">
                          <input
                            className={editableRailFieldClassName(form.x_url, { link: true })}
                            value={form.x_url}
                            onChange={(event) => updateField('x_url', event.target.value, { x_url: event.target.value })}
                            placeholder="Add X URL"
                          />
                        </CopyableRailValue>
                      </SocialMetadataRow>

                      <MetadataRow icon={LinkSquare01Icon} label="AngelList">
                        <CopyableRailValue value={form.angellist_url} label="AngelList URL">
                          <input
                            className={editableRailFieldClassName(form.angellist_url, { link: true })}
                            value={form.angellist_url}
                            onChange={(event) => updateField('angellist_url', event.target.value, { angellist_url: event.target.value })}
                            placeholder="Add AngelList URL"
                          />
                        </CopyableRailValue>
                      </MetadataRow>

                      <MetadataRow icon={MapPinIcon} label="Country">
                        <CopyableRailValue value={[form.country_code, form.country_name].filter(Boolean).join(' ')} label="country">
                          <div className="grid grid-cols-[52px_1fr] gap-1.5">
                            <input
                              className={cn(editableRailFieldClassName(form.country_code), 'min-w-0 uppercase')}
                              value={form.country_code}
                              onChange={(event) => updateField('country_code', event.target.value.toUpperCase(), { country_code: event.target.value.toUpperCase() })}
                              placeholder="Code"
                            />
                            <input
                              className={cn(editableRailFieldClassName(form.country_name), 'min-w-0')}
                              value={form.country_name}
                              onChange={(event) => updateField('country_name', event.target.value, { country_name: event.target.value })}
                              placeholder="Country"
                            />
                          </div>
                        </CopyableRailValue>
                      </MetadataRow>

                      <MetadataRow icon={Mail01Icon} label="First email">
                        <span className="block truncate text-xs" title={formatRailTimestampTitle(interactionSummary.firstEmail?.timestamp)}>
                          {formatRailEmptyState(interactionSummary.firstEmail?.timestamp, 'No email yet')}
                        </span>
                      </MetadataRow>

                      <MetadataRow icon={Mail01Icon} label="Last email">
                        <span className="block truncate text-xs" title={formatRailTimestampTitle(interactionSummary.lastEmail?.timestamp)}>
                          {formatRailEmptyState(interactionSummary.lastEmail?.timestamp, 'No email yet')}
                        </span>
                      </MetadataRow>

                      <MetadataRow icon={Calendar01Icon} label="First meeting">
                        <span className="block truncate text-xs" title={formatRailTimestampTitle(interactionSummary.firstCalendar?.timestamp)}>
                          {formatRailEmptyState(interactionSummary.firstCalendar?.timestamp, 'No meeting yet')}
                        </span>
                      </MetadataRow>

                      <MetadataRow icon={Calendar01Icon} label="Last meeting">
                        <span className="block truncate text-xs" title={formatRailTimestampTitle(interactionSummary.lastCalendar?.timestamp)}>
                          {formatRailEmptyState(interactionSummary.lastCalendar?.timestamp, 'No meeting yet')}
                        </span>
                      </MetadataRow>

                      <MetadataRow icon={Calendar01Icon} label="Next meeting">
                        <span className="block truncate text-xs" title={formatRailTimestampTitle(interactionSummary.nextCalendar?.timestamp)}>
                          {formatRailEmptyState(interactionSummary.nextCalendar?.timestamp, 'None scheduled')}
                        </span>
                      </MetadataRow>

                      <MetadataRow icon={Clock01Icon} label="First touch">
                        <span className="block truncate text-xs" title={formatRailTimestampTitle(interactionSummary.firstInteraction?.timestamp)}>
                          {formatRailEmptyState(interactionSummary.firstInteraction?.timestamp, 'No activity yet')}
                        </span>
                      </MetadataRow>

                      <MetadataRow icon={Clock01Icon} label="Created">
                        <span className="block truncate text-xs" title={formatRailTimestampTitle(contact.created_at)}>
                          {formatRailTimestamp(contact.created_at)}
                        </span>
                      </MetadataRow>

                      <MetadataRow icon={Clock01Icon} label="Updated">
                        <span className="block truncate text-xs" title={formatRailTimestampTitle(contact.updated_at)}>
                          {formatRailTimestamp(contact.updated_at)}
                        </span>
                      </MetadataRow>

                      {enrichedDetailRows.map((row) => (
                        <MetadataRow key={row.key} icon={row.icon} label={row.label}>
                          {row.href ? (
                            <a
                              href={row.href}
                              target="_blank"
                              rel="noreferrer"
                              className="block truncate text-xs text-primary underline-offset-2 hover:underline"
                              title={row.href}
                            >
                              {row.value}
                            </a>
                          ) : (
                            <span className="block truncate text-xs text-foreground" title={row.value}>
                              {row.value}
                            </span>
                          )}
                        </MetadataRow>
                      ))}
                    </>
                  )}

                  <div className="col-span-3 pt-1">
                    <button
                      type="button"
                      className="text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
                      onClick={() => setDetailsExpanded((value) => !value)}
                    >
                      {detailsExpanded ? 'Show less' : 'Show all fields'}
                    </button>
                  </div>
                </div>
              </div>

              {/* ENRICHMENT (always visible, non-collapsible) */}
              <div className="border-b border-border/50 px-4 py-3">
                <h3 className="mb-2 text-[11px] font-medium uppercase tracking-tight text-foreground">
                  Enrichment
                </h3>
                <EnrichmentRailCard workspaceId={wsId} objectType="contact" objectId={contactId} />
              </div>

              {/* COMPANY */}
              <RailSection
                title="Company"
                action={
                  !primaryCompanyAssociation ? (
                    <button
                      type="button"
                      onClick={() => {
                        setCompanyPickerMode('primary');
                        setCompanyPickerOpen(true);
                      }}
                      className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                      aria-label="Link company"
                    >
                      <PlusSignIcon className="h-3.5 w-3.5" />
                    </button>
                  ) : null
                }
              >
                {primaryCompanyAssociation ? (
                  <CompanyRailCard
                    name={primaryCompanyAssociation.linked_object_name}
                    displayId={primaryCompanyAssociation.linked_object_display_id}
                    onOpen={() =>
                      navigate({
                        to: '/w/$slug/crm/companies/$companyId',
                        params: { slug: wsSlug, companyId: primaryCompanyAssociation.linkedId },
                      } as never)
                    }
                    onRemove={() => setRemoveAssocId(primaryCompanyAssociation.id)}
                  />
                ) : (
                  <button
                    type="button"
                    onClick={() => {
                      setCompanyPickerMode('primary');
                      setCompanyPickerOpen(true);
                    }}
                    className="flex w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-border/60 px-3 py-2 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-muted/40 hover:text-foreground"
                  >
                    <PlusSignIcon className="h-3.5 w-3.5" />
                    Link a company
                  </button>
                )}

                {otherCompanyCount > 0 && (
                  <div className="mt-2 space-y-1.5">
                    {visibleOtherCompanyAssociations.map((assoc) => (
                      <CompanyRailCard
                        key={assoc.id}
                        name={assoc.linked_object_name}
                        displayId={assoc.linked_object_display_id}
                        onOpen={() =>
                          navigate({
                            to: '/w/$slug/crm/companies/$companyId',
                            params: { slug: wsSlug, companyId: assoc.linkedId },
                          } as never)
                        }
                        onRemove={() => setRemoveAssocId(assoc.id)}
                        onChangePrimary={() => void handleMakePrimaryCompany(assoc.linkedId)}
                      />
                    ))}
                    {otherCompanyAssociations.length > SIDEBAR_PREVIEW_LIMIT && (
                      <button
                        type="button"
                        onClick={() => toggleExpandedSection('other-companies')}
                        className="px-1 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
                      >
                        {expandedSections['other-companies']
                          ? 'Show less'
                          : `Show ${otherCompanyAssociations.length - SIDEBAR_PREVIEW_LIMIT} more`}
                      </button>
                    )}
                  </div>
                )}
                {primaryCompanyAssociation && otherCompanyCount === 0 && (
                  <button
                    type="button"
                    onClick={() => {
                      setCompanyPickerMode('secondary');
                      setCompanyPickerOpen(true);
                    }}
                    className="mt-2 inline-flex items-center gap-1 rounded-md px-1 text-[11px] text-muted-foreground transition-colors hover:text-foreground"
                  >
                    <PlusSignIcon className="h-3 w-3" />
                    Link another company
                  </button>
                )}
              </RailSection>

              {/* DEALS */}
              <RailSection
                title="Deals"
                count={dealCount}
                action={
                  <button
                    type="button"
                    onClick={() => setDealPickerOpen(true)}
                    className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                    aria-label="Link deal"
                  >
                    <PlusSignIcon className="h-3.5 w-3.5" />
                  </button>
                }
              >
                {dealCount === 0 ? (
                  <button
                    type="button"
                    onClick={() => setDealPickerOpen(true)}
                    className="flex w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-border/60 px-3 py-2 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-muted/40 hover:text-foreground"
                  >
                    <PlusSignIcon className="h-3.5 w-3.5" />
                    Link a deal
                  </button>
                ) : (
                  <div className="space-y-0.5">
                    {visibleDealAssociations.map((assoc) => (
                      <div
                        key={assoc.id}
                        className="group flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40"
                      >
                        <button
                          type="button"
                          className="flex min-w-0 flex-1 items-center gap-2 text-left"
                          onClick={() =>
                            navigate({
                              to: '/w/$slug/crm/deals/$dealId',
                              params: { slug: wsSlug, dealId: assoc.linkedId },
                            } as never)
                          }
                        >
                          <DollarCircleIcon className="h-3 w-3 shrink-0 text-muted-foreground" />
                          <span className="truncate font-medium">{assoc.linked_object_name || 'Untitled'}</span>
                          {assoc.linked_object_display_id && (
                            <span className="ml-auto shrink-0 font-mono text-[10.5px] text-muted-foreground">
                              {assoc.linked_object_display_id}
                            </span>
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
                    {dealAssociations.length > SIDEBAR_PREVIEW_LIMIT && (
                      <button
                        type="button"
                        onClick={() => toggleExpandedSection('deals')}
                        className="px-1 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
                      >
                        {expandedSections.deals
                          ? 'Show less'
                          : `Show ${dealAssociations.length - SIDEBAR_PREVIEW_LIMIT} more`}
                      </button>
                    )}
                  </div>
                )}
              </RailSection>

              {/* OPEN TASKS */}
              <RailSection
                title="Open tasks"
                count={taskCount}
                action={
                  <button
                    type="button"
                    onClick={() => setActiveTab('tasks')}
                    className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                    aria-label="Manage tasks"
                  >
                    <PlusSignIcon className="h-3.5 w-3.5" />
                  </button>
                }
              >
                {taskCount === 0 ? (
                  <button
                    type="button"
                    onClick={() => setActiveTab('tasks')}
                    className="flex w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-border/60 px-3 py-2 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-muted/40 hover:text-foreground"
                  >
                    <PlusSignIcon className="h-3.5 w-3.5" />
                    Link a task
                  </button>
                ) : (
                  <div className="space-y-0.5">
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
                            {task.state_name ? (
                              <QuickTooltip label={task.state_name}>
                                <span
                                  className="h-2.5 w-2.5 shrink-0 rounded-full bg-muted-foreground/30"
                                  style={task.state_color ? { backgroundColor: task.state_color } : undefined}
                                  aria-label={task.state_name}
                                />
                              </QuickTooltip>
                            ) : (
                              <span
                                className="h-2.5 w-2.5 shrink-0 rounded-full bg-muted-foreground/30"
                                style={task.state_color ? { backgroundColor: task.state_color } : undefined}
                                aria-hidden="true"
                              />
                            )}
                            <span className="truncate font-medium">{task.name}</span>
                            <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">{task.task_key}</span>
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
                    {tasks.length > SIDEBAR_PREVIEW_LIMIT && (
                      <button
                        type="button"
                        onClick={() => toggleExpandedSection('tasks')}
                        className="px-1 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
                      >
                        {expandedSections.tasks
                          ? 'Show less'
                          : `Show ${tasks.length - SIDEBAR_PREVIEW_LIMIT} more`}
                      </button>
                    )}
                  </div>
                )}
              </RailSection>

              {/* SUPPORT */}
              <RailSection title="Support" count={supportCount}>
                {supportCount === 0 ? (
                  <p className="px-1 text-xs italic text-muted-foreground">No support conversations.</p>
                ) : (
                  <div className="space-y-0.5">
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
                          <QuickTooltip label={humanizeStatusLabel(c.status)}>
                            <span
                              className={cn('h-2.5 w-2.5 shrink-0 rounded-full bg-muted-foreground/30', supportStatusDotClass(c.status))}
                              aria-label={humanizeStatusLabel(c.status)}
                            />
                          </QuickTooltip>
                          <span className="truncate font-medium">{c.subject}</span>
                          <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">
                            C-{c.display_id}
                          </span>
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
                    {supportConvos.length > SIDEBAR_PREVIEW_LIMIT && (
                      <button
                        type="button"
                        onClick={() => toggleExpandedSection('support')}
                        className="px-1 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
                      >
                        {expandedSections.support
                          ? 'Show less'
                          : `Show ${supportConvos.length - SIDEBAR_PREVIEW_LIMIT} more`}
                      </button>
                    )}
                  </div>
                )}
              </RailSection>
        </aside>
      )}

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
