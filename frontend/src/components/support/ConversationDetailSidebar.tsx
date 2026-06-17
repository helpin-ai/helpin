import { memo, type JSX, type SVGProps } from 'react';
import { format, formatDistance } from 'date-fns';
import * as Flags from 'country-flag-icons/react/3x2';
import { Link } from '@tanstack/react-router';
import { ArrowDown01Icon, ArrowLeft01Icon, ArrowRight01Icon, Mail01Icon, Message01Icon, Tag01Icon, UserIcon } from '@/lib/icons';
import { EmptyState } from './EmptyState';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { CollapsibleSection } from '@/components/ui/collapsible-section';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { findAssignableMember, formatAssignableMemberName } from '@/lib/assignableMembers';
import { SidebarAssociations } from './SidebarAssociations';
import { SidebarVisitorContext } from './SidebarVisitorContext';
import { SupportTagPicker } from './SupportTagPicker';
import { useConversation, useConversationAssignees, useVisitorContext, useAssignConversationUser, useConversationMessages } from '@/hooks/queries/useSupport';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { SupportMessage } from '@/lib/pmTypes';
import { getInitial, getAvatarColor } from './helpers';

type LastActiveSource = 'anonymous_id' | 'crm_contact' | string | null | undefined;

interface ConversationDetailSidebarProps {
  workspaceId: string;
  conversationId: string | null;
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

export function getLastActiveTooltipLabel(lastActiveAt: string, source?: LastActiveSource, now = new Date()): string {
  const date = new Date(lastActiveAt);
  if (Number.isNaN(date.getTime())) {
    return 'Last active time unavailable';
  }

  const relative = formatDistance(date, now, { addSuffix: true });
  const scope = source === 'crm_contact' ? ' across this contact' : '';
  return `Last active ${relative}${scope} · ${format(date, 'PPp')}`;
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

function normalizeRecipients(values?: string[] | null): string[] {
  return Array.from(new Set((values ?? []).map((value) => value.trim()).filter(Boolean)));
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

function EmailRecipientRow({ label, values }: { label: string; values: string[] }) {
  if (values.length === 0) return null;

  return (
    <div className="space-y-1">
      <div className="text-[10px] font-medium uppercase tracking-tight text-muted-foreground">{label}</div>
      <div className="space-y-1">
        {values.map((value) => (
          <div
            key={`${label}-${value}`}
            className="truncate rounded border bg-background px-2 py-1 text-xs text-foreground"
            title={value}
          >
            {value}
          </div>
        ))}
      </div>
    </div>
  );
}

export function ConversationDetailSidebar({ workspaceId, conversationId }: ConversationDetailSidebarProps) {
  const { detailSidebarCollapsed, toggleDetailSidebar } = useSupportInboxStore();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const { data: conversation } = useConversation(workspaceId, conversationId);
  const { data: messages = [] } = useConversationMessages(workspaceId, conversationId);
  const { data: visitorContext } = useVisitorContext(workspaceId, conversationId);
  const { data: assignableMembers = [], isLoading: assigneesLoading } = useConversationAssignees(workspaceId, conversationId);
  const assignConversationUser = useAssignConversationUser(workspaceId);
  const isVisitorOnline = useSupportPresenceStore((s) =>
    conversation?.anonymous_id ? !!s.onlineVisitors[conversation.anonymous_id] : false
  );

  if (detailSidebarCollapsed) {
    return (
      <div className="flex w-10 flex-col items-center border-l bg-muted/30 pt-2">
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
  const latestEmailRecipients = getLatestEmailRecipients(messages);
  const emailRecipientCount = latestEmailRecipients
    ? [latestEmailRecipients.to, ...latestEmailRecipients.cc, ...latestEmailRecipients.bcc].filter(Boolean).length
    : 0;

  return (
    <div className="flex w-[300px] flex-col border-l bg-muted/30">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-3 py-2">
        <h3 className="text-sm font-semibold">Details</h3>
        <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={toggleDetailSidebar}>
          <ArrowRight01Icon className="h-4 w-4" />
        </Button>
      </div>

      {!conversation ? (
        <EmptyState
          icon={Message01Icon}
          title="No conversation selected"
          subtitle="Select a conversation to see contact and context details here."
        />
      ) : (
        <div className="flex-1 overflow-y-auto pb-16">
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
            <span className="text-sm font-semibold truncate max-w-full">{displayName}</span>
            {conversation.customer_email && conversation.customer_name && (
              <span className="flex items-center gap-1 text-xs text-muted-foreground truncate max-w-full">
                <Mail01Icon className="h-3 w-3 shrink-0" />
                {conversation.customer_email}
              </span>
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

          {latestEmailRecipients && (
            <CollapsibleSection title="Email recipients" icon={Mail01Icon} count={emailRecipientCount}>
              <EmailRecipientRow label="To" values={latestEmailRecipients.to ? [latestEmailRecipients.to] : []} />
              <EmailRecipientRow label="Cc" values={latestEmailRecipients.cc} />
              <EmailRecipientRow label="Bcc" values={latestEmailRecipients.bcc} />
            </CollapsibleSection>
          )}

          {/* ── Visitor Intelligence ─────────────────────── */}
          <SidebarVisitorContext
            workspaceId={workspaceId}
            conversationId={conversation.id}
          />

          {/* ── Links / Associations ─────────────────────── */}
          <SidebarAssociations
            workspaceId={workspaceId}
            conversationId={conversation.id}
          />
        </div>
      )}
    </div>
  );
}
