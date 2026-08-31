import { memo, useEffect, useState, type JSX, type ReactNode, type SVGProps } from 'react';
import { formatDistance } from 'date-fns';
import * as Flags from 'country-flag-icons/react/3x2';
import { Link } from '@tanstack/react-router';
import { toast } from 'sonner';
import { ArrowDown01Icon, ArrowLeft01Icon, ArrowRight01Icon, Cancel01Icon, CheckmarkCircle02Icon, Copy01Icon, Mail01Icon, Message01Icon, PencilEdit01Icon, PlusSignIcon, Tag01Icon, UserIcon } from '@/lib/icons';
import { EmptyState } from './EmptyState';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { findAssignableMember, formatAssignableMemberName } from '@/lib/assignableMembers';
import { SidebarAssociations } from './SidebarAssociations';
import { SidebarOtherConversations, SidebarVisitorContext } from './SidebarVisitorContext';
import { SidebarCompanyDetails } from './SidebarCompanyDetails';
import { SupportTagPicker } from './SupportTagPicker';
import { CustomerProfileDrawer } from './CustomerProfileDrawer';
import { useConversation, useConversationAssignees, useVisitorContext, useAssignConversationUser, useUpdateConversationCustomerName, useUpdateConversationEmailRecipients } from '@/hooks/queries/useSupport';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { SupportConversation, SupportMessage } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { getInitial, getAvatarColor } from './helpers';
import { SupportInboxPanelHeader } from './SupportInboxPanelHeader';

type LastActiveSource = 'anonymous_id' | 'crm_contact' | string | null | undefined;

export const customerNameEditButtonClassName =
  'col-start-3 h-6 w-6 justify-self-center p-0 text-muted-foreground opacity-0 transition-opacity group-hover/name:opacity-100 group-focus-within/name:opacity-100';
export const customerNameDisplayRowClassName =
  'group/name grid w-fit max-w-full grid-cols-[1.5rem_minmax(0,1fr)_1.5rem] items-center';
export const customerEmailDisplayRowClassName =
  'group/email grid w-fit max-w-full grid-cols-[1.5rem_minmax(0,1fr)_1.5rem] items-center text-xs text-muted-foreground';
export const customerEmailCopyButtonClassName =
  'col-start-3 h-6 w-6 justify-self-center p-0 opacity-0 transition-opacity group-hover/email:opacity-100 group-focus-within/email:opacity-100';

interface ConversationDetailSidebarProps {
  workspaceId: string;
  conversationId: string | null;
}

export const conversationDetailSidebarRootClassName = 'flex h-full min-h-0 w-[300px] flex-col bg-muted/30';

export function shouldShowConversationDetailLoading(
  conversationId: string | null,
  conversation: SupportConversation | undefined,
  isLoading: boolean
): boolean {
  return Boolean(conversationId && !conversation && isLoading);
}

function ConversationDetailLoadingSkeleton() {
  return (
    <div
      className="h-full overflow-hidden px-3 py-4"
      role="status"
      aria-label="Loading conversation details"
    >
      <span className="sr-only">Loading conversation details</span>
      <div className="flex flex-col items-center gap-2 border-b border-border/50 pb-4">
        <Skeleton className="h-12 w-12 rounded-full" />
        <Skeleton className="h-3.5 w-28 rounded" />
        <Skeleton className="h-3 w-40 rounded" />
      </div>
      <div className="space-y-5 py-4">
        {[0, 1, 2, 3].map((section) => (
          <div key={section} className="space-y-3">
            <div className="flex items-center gap-2">
              <Skeleton className="h-4 w-4 rounded" />
              <Skeleton className="h-3 w-24 rounded" />
            </div>
            <Skeleton className="h-8 w-full rounded-md" />
          </div>
        ))}
      </div>
    </div>
  );
}

function normalizeCountryCode(code?: string | null): keyof typeof Flags | null {
  const normalized = code?.trim().toUpperCase().replace(/-/g, '_');
  if (!normalized || !/^[A-Z]{2,3}(?:_[A-Z]{2,3})?$/.test(normalized)) {
    return null;
  }
  return normalized as keyof typeof Flags;
}

const DetailCountryFlag = memo(function DetailCountryFlag({
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

  const label = countryName?.trim() || countryCode?.trim()?.toUpperCase() || 'Visitor country';

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="absolute -bottom-0.5 -right-0.5 flex h-3 w-[18px] items-center justify-center overflow-hidden rounded-[3px] border border-background/80 shadow-sm">
          <Flag aria-label={label} className="h-full w-full object-cover" />
        </span>
      </TooltipTrigger>
      <TooltipContent side="left">
        <span className="text-xs">{label}</span>
      </TooltipContent>
    </Tooltip>
  );
});

export function shouldShowLastActiveIndicator(isVisitorOnline: boolean, lastActiveAt?: string | null): boolean {
  return !isVisitorOnline && Boolean(lastActiveAt);
}

export function getLastActiveTooltipLabel(lastActiveAt: string, _source?: LastActiveSource, now = new Date()): string {
  const date = new Date(lastActiveAt);
  if (Number.isNaN(date.getTime())) {
    return 'Last active time unavailable';
  }

  const relative = formatDistance(date, now, { addSuffix: true });
  return `Last active ${relative}`;
}

function VisitorLastActiveDot({
  lastActiveAt,
  source,
}: {
  lastActiveAt: string;
  source?: LastActiveSource;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          aria-label={getLastActiveTooltipLabel(lastActiveAt, source)}
          className="absolute -left-0.5 -top-0.5 h-2.5 w-2.5 rounded-full bg-amber-400 ring-2 ring-background shadow-sm"
        />
      </TooltipTrigger>
      <TooltipContent side="left">
        <span className="text-xs">{getLastActiveTooltipLabel(lastActiveAt, source)}</span>
      </TooltipContent>
    </Tooltip>
  );
}

export interface EmailRecipientsSummary {
  to?: string;
  cc: string[];
  bcc: string[];
}

export interface ConversationEmailRecipientsSummary {
  primary: string[];
  cc: string[];
  alsoOnThread: string[];
}

export interface AddConversationCCResult {
  cc: string[];
  error?: string;
}

function normalizeRecipients(values?: string[] | null): string[] {
  return Array.from(new Set((values ?? []).map((value) => value.trim()).filter(Boolean)));
}

function normalizeEmailInput(value: string): string {
  return value.trim().toLowerCase();
}

function isLikelyEmailAddress(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
}

export function getConversationEmailRecipients(conversation?: SupportConversation | null): ConversationEmailRecipientsSummary {
  const primary = normalizeRecipients(conversation?.customer_email ? [conversation.customer_email] : []);
  const cc = normalizeRecipients(conversation?.email_cc);
  const excluded = new Set([...primary, ...cc].map((value) => value.toLowerCase()));
  const alsoOnThread = normalizeRecipients(conversation?.email_thread_participants)
    .filter((value) => !excluded.has(value.toLowerCase()));

  return { primary, cc, alsoOnThread };
}

export function getLatestEmailRecipients(messages: SupportMessage[]): EmailRecipientsSummary | null {
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const message = messages[i];
    if (message.sender_type === 'customer' || message.is_internal) {
      continue;
    }

    const to = message.email_to?.trim();
    const cc = normalizeRecipients(message.email_cc);
    const bcc = normalizeRecipients(message.email_bcc);
    if (to || cc.length > 0 || bcc.length > 0) {
      return { to, cc, bcc };
    }
  }

  return null;
}

export function getAddConversationCCResult(
  recipients: ConversationEmailRecipientsSummary,
  rawEmail: string
): AddConversationCCResult {
  const cc = normalizeRecipients(recipients.cc);
  const email = normalizeEmailInput(rawEmail);
  if (!isLikelyEmailAddress(email)) {
    return { cc, error: 'Enter a valid email address.' };
  }
  if (recipients.primary.some((value) => value.toLowerCase() === email)) {
    return { cc, error: 'This is already the To recipient.' };
  }
  if (cc.some((value) => value.toLowerCase() === email)) {
    return { cc, error: 'Already added to Cc.' };
  }

  return { cc: [...cc, email] };
}

type ClipboardWriteText = (text: string) => Promise<void> | void;

export async function copyCustomerEmailToClipboard(email: string, writeText?: ClipboardWriteText): Promise<boolean> {
  const trimmed = email.trim();
  if (!trimmed) return false;

  const writer = writeText ?? (typeof navigator !== 'undefined' ? navigator.clipboard?.writeText?.bind(navigator.clipboard) : undefined);
  if (!writer) return false;

  await writer(trimmed);
  return true;
}

function EmailRecipientRow({
  label,
  tooltip,
  values,
  onRemove,
  children,
}: {
  label: string;
  tooltip: string;
  values: string[];
  onRemove?: (value: string) => void;
  children?: ReactNode;
}) {
  if (values.length === 0 && !children) return null;

  return (
    <div className="grid grid-cols-[2rem_minmax(0,1fr)] gap-2">
      <Tooltip>
        <TooltipTrigger asChild>
          <div className="h-6 cursor-help pt-1 text-[10px] font-medium uppercase tracking-tight text-muted-foreground">{label}</div>
        </TooltipTrigger>
        <TooltipContent side="left">
          <span className="text-xs">{tooltip}</span>
        </TooltipContent>
      </Tooltip>
      <div className="min-w-0 space-y-1">
        {values.map((value) => (
          <div key={`${label}-${value}`} className="flex h-6 items-center gap-1 rounded border bg-background px-2 text-xs text-foreground" title={value}>
            <span className="min-w-0 flex-1 truncate">{value}</span>
            {onRemove ? (
              <button
                type="button"
                className="inline-flex h-4 w-4 shrink-0 items-center justify-center rounded text-muted-foreground hover:bg-muted hover:text-foreground"
                onClick={() => onRemove(value)}
                aria-label={`Remove ${value} from Cc replies`}
              >
                <Cancel01Icon className="h-3 w-3" />
              </button>
            ) : null}
          </div>
        ))}
        {children}
      </div>
    </div>
  );
}

export function ConversationDetailSidebar({ workspaceId, conversationId }: ConversationDetailSidebarProps) {
  const { detailSidebarCollapsed, toggleDetailSidebar } = useSupportInboxStore();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const [nameEditing, setNameEditing] = useState(false);
  const [customerNameDraft, setCustomerNameDraft] = useState('');
  const [customerProfileOpen, setCustomerProfileOpen] = useState(false);
  const [ccAdding, setCcAdding] = useState(false);
  const [ccDraft, setCcDraft] = useState('');
  const [ccError, setCcError] = useState('');
  const { data: conversation, isLoading: conversationLoading } = useConversation(workspaceId, conversationId);
  const { data: visitorContext } = useVisitorContext(workspaceId, conversationId);
  const { data: assignableMembers = [], isLoading: assigneesLoading } = useConversationAssignees(workspaceId, conversationId);
  const assignConversationUser = useAssignConversationUser(workspaceId);
  const updateCustomerName = useUpdateConversationCustomerName(workspaceId);
  const updateEmailRecipients = useUpdateConversationEmailRecipients(workspaceId);
  const isVisitorOnline = useSupportPresenceStore((s) =>
    conversation?.anonymous_id ? !!s.onlineVisitors[conversation.anonymous_id] : false
  );

  useEffect(() => {
    setCustomerProfileOpen(false);
    setCcAdding(false);
    setCcDraft('');
    setCcError('');
  }, [conversationId]);

  if (detailSidebarCollapsed) {
    return (
      <div className="flex h-full min-h-0 w-10 flex-col items-center bg-muted/30 pt-2">
        <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={toggleDetailSidebar}>
          <ArrowLeft01Icon className="h-4 w-4" />
        </Button>
      </div>
    );
  }

  const displayName = conversation?.customer_name || conversation?.customer_email || (conversation?.anonymous_id ? `Visitor #${conversation.anonymous_id.slice(0, 6)}` : 'Anonymous');
  const location = visitorContext?.location;
  const countryCode = location?.country_code ?? conversation?.country_code;
  const countryName = location?.country_name ?? conversation?.country_name;
  const assignableUsers = assignableMembers.filter((member) => !!member.user_id);
  const assignedMember = conversation
    ? findAssignableMember(assignableUsers, conversation.assigned_user_id, (member) => member.user_id ?? member.id)
    : undefined;
  const conversationEmailRecipients = getConversationEmailRecipients(conversation);
  const emailRecipientCount = [
    ...conversationEmailRecipients.primary,
    ...conversationEmailRecipients.cc,
  ].length;
  const beginCustomerNameEdit = () => {
    setCustomerNameDraft(conversation?.customer_name?.trim() ?? '');
    setNameEditing(true);
  };
  const cancelCustomerNameEdit = () => {
    setCustomerNameDraft('');
    setNameEditing(false);
  };
  const saveCustomerName = () => {
    if (!conversation) return;
    const trimmed = customerNameDraft.trim();
    if (!trimmed) return;
    updateCustomerName.mutate({ conversationId: conversation.id, customerName: trimmed }, {
      onSuccess: () => {
        setNameEditing(false);
        setCustomerNameDraft('');
      },
    });
  };
  const copyCustomerEmail = async () => {
    if (!conversation?.customer_email) return;
    try {
      const copied = await copyCustomerEmailToClipboard(conversation.customer_email);
      if (copied) {
        toast.success('Email copied');
      } else {
        toast.error('Could not copy email');
      }
    } catch {
      toast.error('Could not copy email');
    }
  };
  const removeCCRecipient = (email: string) => {
    if (!conversation) return;
    updateEmailRecipients.mutate({
      conversationId: conversation.id,
      payload: {
        cc_emails: conversationEmailRecipients.cc.filter((value) => value.toLowerCase() !== email.toLowerCase()),
      },
    });
  };
  const beginAddCCRecipient = () => {
    setCcAdding(true);
    setCcDraft('');
    setCcError('');
  };
  const cancelAddCCRecipient = () => {
    setCcAdding(false);
    setCcDraft('');
    setCcError('');
  };
  const saveCCRecipient = () => {
    if (!conversation) return;
    const result = getAddConversationCCResult(conversationEmailRecipients, ccDraft);
    if (result.error) {
      setCcError(result.error);
      return;
    }

    updateEmailRecipients.mutate({
      conversationId: conversation.id,
      payload: {
        cc_emails: result.cc,
      },
    }, {
      onSuccess: () => {
        setCcAdding(false);
        setCcDraft('');
        setCcError('');
      },
    });
  };

  return (
    <div className={conversationDetailSidebarRootClassName}>
      {/* Header */}
      <SupportInboxPanelHeader className="justify-between bg-muted/30 px-3">
        <h3 className="text-sm font-semibold">Details</h3>
        <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={toggleDetailSidebar}>
          <ArrowRight01Icon className="h-4 w-4" />
        </Button>
      </SupportInboxPanelHeader>

      <div className="relative min-h-0 flex-1 overflow-hidden">
        {shouldShowConversationDetailLoading(conversationId, conversation, conversationLoading) ? (
          <ConversationDetailLoadingSkeleton />
        ) : !conversation ? (
          <EmptyState
            icon={Message01Icon}
            title="No conversation selected"
            subtitle="Select a conversation to see contact and context details here."
          />
        ) : (
        <div className={cn('h-full overflow-y-auto pb-16 transition-all duration-200 ease-out', customerProfileOpen ? 'pointer-events-none -translate-x-2 opacity-0' : 'translate-x-0 opacity-100')}>
          {/* ── Contact Card ─────────────────────────────── */}
          <div className="flex flex-col items-center gap-1.5 px-3 py-4 border-b border-border/50">
            <div className="relative">
              <div className={`flex h-12 w-12 items-center justify-center rounded-full text-base font-semibold ${getAvatarColor(conversation.customer_email || conversation.customer_name || conversation.id)}`}>
                {getInitial(conversation.customer_name || conversation.customer_email)}
              </div>
              {isVisitorOnline && (
                <span className="absolute -left-0.5 -top-0.5 h-2.5 w-2.5 rounded-full bg-green-400 ring-2 ring-background shadow-sm" />
              )}
              {shouldShowLastActiveIndicator(isVisitorOnline, visitorContext?.last_active_at) && visitorContext?.last_active_at && (
                <VisitorLastActiveDot
                  lastActiveAt={visitorContext.last_active_at}
                  source={visitorContext.last_active_source}
                />
              )}
              <DetailCountryFlag countryCode={countryCode} countryName={countryName} />
            </div>
            {nameEditing ? (
              <div className="flex w-full items-center justify-center gap-1">
                <Input
                  value={customerNameDraft}
                  onChange={(event) => setCustomerNameDraft(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      event.preventDefault();
                      saveCustomerName();
                    }
                    if (event.key === 'Escape') {
                      event.preventDefault();
                      cancelCustomerNameEdit();
                    }
                  }}
                  placeholder="Customer name"
                  autoFocus
                  className="h-8 max-w-[190px] text-sm"
                  aria-label="Customer name"
                />
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 w-8 p-0"
                  onClick={saveCustomerName}
                  disabled={updateCustomerName.isPending || customerNameDraft.trim().length === 0}
                  aria-label="Save customer name"
                >
                  <CheckmarkCircle02Icon className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 w-8 p-0"
                  onClick={cancelCustomerNameEdit}
                  disabled={updateCustomerName.isPending}
                  aria-label="Cancel customer name edit"
                >
                  <Cancel01Icon className="h-4 w-4" />
                </Button>
              </div>
            ) : (
              <div className={customerNameDisplayRowClassName}>
                <span aria-hidden="true" className="col-start-1 h-6 w-6" />
                <button
                  type="button"
                  onClick={() => setCustomerProfileOpen(true)}
                  className="col-start-2 min-w-0 truncate rounded-sm text-sm font-semibold underline-offset-2 transition-colors hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/30"
                  title="Open customer profile"
                >
                  {displayName}
                </button>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className={customerNameEditButtonClassName}
                      onClick={beginCustomerNameEdit}
                      aria-label="Edit customer name"
                    >
                      <PencilEdit01Icon className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    <span className="text-xs">Edit customer name</span>
                  </TooltipContent>
                </Tooltip>
              </div>
            )}
            {conversation.customer_email && conversation.customer_name && (
              <div className={customerEmailDisplayRowClassName}>
                <Mail01Icon className="col-start-1 h-3 w-3 justify-self-center" />
                <span className="col-start-2 min-w-0 truncate">{conversation.customer_email}</span>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      className={customerEmailCopyButtonClassName}
                      onClick={() => { void copyCustomerEmail(); }}
                      aria-label="Copy customer email"
                    >
                      <Copy01Icon className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    <span className="text-xs">Copy email</span>
                  </TooltipContent>
                </Tooltip>
              </div>
            )}
            {conversation.crm_contact_id && workspace?.slug && (
              <Link
                to="/w/$slug/crm/contacts/$contactId"
                params={{ slug: workspace.slug, contactId: conversation.crm_contact_id }}
                className="inline-flex items-center gap-1 rounded-md bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:bg-blue-950/30 dark:text-blue-400 mt-0.5"
              >
                <UserIcon className="h-3 w-3" />
                View CRM Contact
              </Link>
            )}
          </div>

          <CollapsibleSection title="Conversation Routing" icon={UserIcon} count={0} defaultOpen>
            <MemberPickerPopover
              value={conversation.assigned_user_id ?? ''}
              members={assignableUsers}
              getMemberValue={(member) => member.user_id ?? member.id}
              noneLabel="Unassigned"
              disabled={assignConversationUser.isPending || assigneesLoading}
              onChange={(value) => {
                assignConversationUser.mutate({
                  conversationId: conversation.id,
                  userId: value === '__none__' ? null : value,
                });
              }}
              triggerClassName="w-full rounded-md border bg-background px-2 py-1 hover:bg-accent/50 [&>span]:w-full"
              contentClassName="w-[260px]"
              renderTrigger={() => (
                <>
                  {assignedMember ? (
                    <UserAvatar
                      name={assignedMember.display_name || assignedMember.email}
                      avatarUrl={assignedMember.avatar_url}
                      avatarStyle={assignedMember.avatar_style}
                      avatarSeed={assignedMember.avatar_seed}
                      avatarBackgroundMode={assignedMember.avatar_background_mode}
                      avatarBackgroundColor={assignedMember.avatar_background_color}
                      className="h-5 w-5"
                      fallbackClassName="text-[9px]"
                    />
                  ) : (
                    <span className="flex h-5 w-5 items-center justify-center rounded-full border border-dashed border-border bg-muted text-muted-foreground">
                      <UserIcon className="h-3 w-3" />
                    </span>
                  )}
                  <span className="min-w-0 flex-1 truncate text-left text-xs font-medium text-foreground">
                    {assignedMember ? formatAssignableMemberName(assignedMember) : 'Unassigned'}
                  </span>
                  <ArrowDown01Icon className="h-3 w-3 shrink-0 text-muted-foreground" />
                </>
              )}
            />
            {!assigneesLoading && assignableUsers.length === 0 ? (
              <p className="text-[11px] text-muted-foreground">
                No eligible teammates can be assigned to this conversation yet.
              </p>
            ) : null}
          </CollapsibleSection>

          <CollapsibleSection title="Tags" icon={Tag01Icon} count={conversation.tags?.length ?? 0} defaultOpen>
            <SupportTagPicker
              workspaceId={workspaceId}
              conversationId={conversation.id}
              selectedTags={conversation.tags ?? []}
            />
          </CollapsibleSection>

          {emailRecipientCount > 0 && (
            <CollapsibleSection title="Email recipients" icon={Mail01Icon} count={emailRecipientCount}>
              <div className="space-y-2">
                <EmailRecipientRow
                  label="To"
                  tooltip="Main recipient for future replies."
                  values={conversationEmailRecipients.primary}
                />
                <EmailRecipientRow
                  label="Cc"
                  tooltip="Also included on future email replies."
                  values={conversationEmailRecipients.cc}
                  onRemove={removeCCRecipient}
                >
                  {ccAdding ? (
                    <div className="space-y-1">
                      <div className="flex items-center gap-1">
                        <Input
                          value={ccDraft}
                          onChange={(event) => {
                            setCcDraft(event.target.value);
                            setCcError('');
                          }}
                          onKeyDown={(event) => {
                            if (event.key === 'Enter') {
                              event.preventDefault();
                              saveCCRecipient();
                            }
                            if (event.key === 'Escape') {
                              event.preventDefault();
                              cancelAddCCRecipient();
                            }
                          }}
                          placeholder="email@example.com"
                          autoFocus
                          className="h-7 min-w-0 text-xs"
                          aria-label="Cc email"
                        />
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-7 w-7 shrink-0 p-0"
                              onClick={saveCCRecipient}
                              disabled={updateEmailRecipients.isPending}
                              aria-label="Add Cc recipient"
                            >
                              <CheckmarkCircle02Icon className="h-3.5 w-3.5" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side="bottom">
                            <span className="text-xs">Add</span>
                          </TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-7 w-7 shrink-0 p-0"
                              onClick={cancelAddCCRecipient}
                              disabled={updateEmailRecipients.isPending}
                              aria-label="Cancel adding Cc recipient"
                            >
                              <Cancel01Icon className="h-3.5 w-3.5" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side="bottom">
                            <span className="text-xs">Cancel</span>
                          </TooltipContent>
                        </Tooltip>
                      </div>
                      {ccError ? <p className="text-[11px] text-destructive">{ccError}</p> : null}
                    </div>
                  ) : (
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-6 px-1.5 text-xs text-muted-foreground hover:text-foreground"
                      onClick={beginAddCCRecipient}
                      disabled={updateEmailRecipients.isPending}
                    >
                      <PlusSignIcon className="h-3 w-3" />
                      Add
                    </Button>
                  )}
                </EmailRecipientRow>
              </div>
            </CollapsibleSection>
          )}

          {/* ── Visitor Intelligence ─────────────────────── */}
          <SidebarVisitorContext
            workspaceId={workspaceId}
            conversationId={conversation.id}
          />

          <SidebarCompanyDetails
            workspaceId={workspaceId}
            conversationId={conversation.id}
          />

          <SidebarOtherConversations
            workspaceId={workspaceId}
            conversationId={conversation.id}
          />

          {/* ── Links / Associations ─────────────────────── */}
          <SidebarAssociations
            workspaceId={workspaceId}
            conversationId={conversation.id}
            excludeCRMCompanyId={visitorContext?.company?.id}
          />
        </div>
        )}
        <CustomerProfileDrawer
          workspaceId={workspaceId}
          workspaceSlug={workspace?.slug}
          conversation={conversation}
          open={customerProfileOpen}
          onOpenChange={setCustomerProfileOpen}
        />
      </div>
    </div>
  );
}
