import { useDeferredValue, useMemo, useState } from 'react';
import { format } from 'date-fns';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';

import {
  Calendar03Icon,
  DollarCircleIcon,
  Link01Icon,
  Loading01Icon,
  Message01Icon,
  PlusSignIcon,
  Search01Icon,
  UserGroupIcon,
} from '@/lib/icons';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { getOptionalSectionActionClass } from '@/components/pm/optionalSectionActionPill';
import { CreateContactDialog } from '@/components/crm/CreateContactDialog';
import { CreateDealDialog } from '@/components/crm/CreateDealDialog';
import { CreateMeetingDialog } from '@/components/crm/CreateMeetingDialog';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { MeetingPlatformLabel } from '@/components/crm/MeetingPlatform';
import { MeetingStatusText } from '@/components/crm/MeetingStatusText';
import { useCompanyContacts, useCompanyDeals, useCompanySupportConversations, useContactSupportConversations, useDeals } from '@/hooks/queries';
import { useCRMMeetings } from '@/hooks/queries/useCRMMeetings';
import { useAssignableWorkspaceMembers } from '@/hooks/useAssignableWorkspaceMembers';
import { associationsService } from '@/lib/services/associationsService';
import { crmDealService, crmSearchService } from '@/lib/services/crmService';
import { formatMeetingDate } from '@/lib/meetingPresentation';
import { supportService } from '@/lib/services/supportService';
import type { CRMContact, CRMDeal, CRMObjectType, CRMSearchResult } from '@/lib/crmTypes';
import type { SupportConversation } from '@/lib/pmTypes';
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation';

function CollectionHeader({
  title,
  search,
  onSearchChange,
  actions,
  children,
}: {
  title: string;
  search: string;
  onSearchChange: (value: string) => void;
  actions?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <div className="flex min-h-12 flex-wrap items-center gap-2 border-b border-border/60 px-4 py-2 sm:px-6 lg:px-8">
      <h2 className="mr-2 text-sm font-semibold">{title}</h2>
      <div className="relative min-w-44 flex-1 sm:max-w-64">
        <Search01Icon className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
          placeholder={`Search ${title.toLowerCase()}`}
          className="h-7 pl-8 text-xs"
        />
      </div>
      {children}
      <div className="ml-auto flex items-center gap-[18px]">{actions}</div>
    </div>
  );
}

function CollectionState({
  icon: Icon,
  title,
  detail,
  loading,
}: {
  icon: React.ElementType;
  title: string;
  detail: string;
  loading?: boolean;
}) {
  return (
    <div className="flex min-h-64 flex-col items-center justify-center px-6 text-center">
      {loading ? (
        <Loading01Icon className="h-5 w-5 animate-spin text-muted-foreground" />
      ) : (
        <Icon className="h-5 w-5 text-muted-foreground/50" />
      )}
      <p className="mt-3 text-sm font-medium">{loading ? 'Loading…' : title}</p>
      {!loading && <p className="mt-1 max-w-sm text-xs leading-5 text-muted-foreground">{detail}</p>}
    </div>
  );
}

function EntityLinkDialog({
  open,
  onOpenChange,
  workspaceId,
  targetType,
  targetId,
  type,
  onLinked,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  targetType: Extract<CRMObjectType, 'company' | 'contact'>;
  targetId: string;
  type: Extract<CRMObjectType, 'contact' | 'deal' | 'support_conversation'>;
  onLinked: () => void;
}) {
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(false);
  const [crmResults, setCRMResults] = useState<CRMSearchResult[]>([]);
  const [supportResults, setSupportResults] = useState<SupportConversation[]>([]);
	const [pendingDealCustomerId, setPendingDealCustomerId] = useState<string | null>(null);

  const runSearch = async (value: string) => {
    setQuery(value);
    if (value.trim().length < 2 && type !== 'support_conversation') {
      setCRMResults([]);
      return;
    }
    setLoading(true);
    try {
      if (type === 'support_conversation') {
        const response = await supportService.listConversations(workspaceId, {
          search: value.trim() || undefined,
          per_page: 25,
        });
        setSupportResults(response.data?.data ?? []);
      } else {
        const response = await crmSearchService.search(workspaceId, value.trim());
        setCRMResults((response.data ?? []).filter((item) => item.type === type));
      }
    } finally {
      setLoading(false);
    }
  };

  const link = async (id: string) => {
	try {
		if (type === 'deal' && targetType === 'company') {
			const response = await crmDealService.setCustomer(workspaceId, id, { workspace_id: workspaceId, company_id: targetId });
			if (response.error) throw new Error(response.error);
		} else if (type === 'support_conversation' && targetType === 'contact') {
        const response = await supportService.updateConversationCRMContact(workspaceId, id, {
          crm_contact_id: targetId,
        });
        if (response.error) throw new Error(response.error);
      } else {
      const response = await associationsService.createAssociation({
        workspace_id: workspaceId,
        from_object_type: type,
        from_object_id: id,
          to_object_type: targetType,
          to_object_id: targetId,
      });
      if (response.error) throw new Error(response.error);
      }
		toast.success(type === 'deal' && targetType === 'company' ? 'Deal customer updated' : `Linked to ${targetType}`);
      onOpenChange(false);
      setQuery('');
      onLinked();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not link record');
    }
  };

  const results =
    type === 'support_conversation'
      ? supportResults.map((item) => ({
          id: item.id,
          name: item.subject,
          detail: `C-${item.display_id} · ${item.status.replaceAll('_', ' ')}`,
        }))
      : crmResults.map((item) => ({
          id: item.id,
          name: item.name,
          detail: item.detail,
        }));

	const requestLink = (id: string) => {
		if (type === 'deal' && targetType === 'company') {
			setPendingDealCustomerId(id);
			return;
		}
		void link(id);
	};

  return (<>
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
			<DialogTitle>{type === 'deal' && targetType === 'company' ? 'Choose an existing deal' : `Link existing ${type === 'support_conversation' ? 'conversation' : type}`}</DialogTitle>
        </DialogHeader>
        <div className="relative">
          <Search01Icon className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            autoFocus
            value={query}
            onChange={(event) => void runSearch(event.target.value)}
            placeholder="Search existing records"
            className="pl-9"
          />
        </div>
        <div className="max-h-72 overflow-y-auto border-t border-border/60">
          {loading ? (
            <CollectionState icon={Loading01Icon} title="" detail="" loading />
          ) : results.length ? (
            results.map((item) => (
              <button
                key={item.id}
                type="button"
				onClick={() => requestLink(item.id)}
                className="flex w-full items-center justify-between gap-3 border-b border-border/50 px-2 py-3 text-left hover:bg-muted/30"
              >
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">{item.name}</span>
                  <span className="block truncate text-xs text-muted-foreground">{item.detail}</span>
                </span>
                <Link01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              </button>
            ))
          ) : (
            <p className="px-2 py-8 text-center text-xs text-muted-foreground">Enter a search to find records.</p>
          )}
        </div>
      </DialogContent>
    </Dialog>
		<ConfirmDialog
			open={!!pendingDealCustomerId}
			onOpenChange={(nextOpen) => { if (!nextOpen) setPendingDealCustomerId(null); }}
			title="Change deal customer"
			description="This company will become the deal customer. The existing customer relationship will be replaced, while linked people remain participants."
			confirmLabel="Change customer"
			onConfirm={async () => {
				if (!pendingDealCustomerId) return;
				const dealId = pendingDealCustomerId;
				setPendingDealCustomerId(null);
				await link(dealId);
			}}
		/>
	</>);
}

export function CompanyContactsView({
  workspaceId,
  workspaceSlug,
  companyId,
}: {
  workspaceId: string;
  workspaceSlug: string;
  companyId: string;
}) {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search.trim());
  const [createOpen, setCreateOpen] = useState(false);
  const [linkOpen, setLinkOpen] = useState(false);
  const query = useCompanyContacts(workspaceId, companyId, {
    search: deferredSearch || undefined,
    per_page: 50,
  });
  const contacts = query.data?.data ?? [];
  const linkCreatedContact = async (contact: CRMContact) => {
    const response = await associationsService.createAssociation({
      workspace_id: workspaceId,
      from_object_type: 'contact',
      from_object_id: contact.id,
      to_object_type: 'company',
      to_object_id: companyId,
    });
    if (response.error) throw new Error(response.error);
    await query.refetch();
  };
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <CollectionHeader
        title="Contacts"
        search={search}
        onSearchChange={setSearch}
        actions={
          <>
            <button
              type="button"
              className={getOptionalSectionActionClass(linkOpen ? 'open' : 'available', 'borderless')}
              onClick={() => setLinkOpen(true)}
            >
              <Link01Icon className="h-[15px] w-[15px]" />
			  Link existing
            </button>
            <button
              type="button"
              className={getOptionalSectionActionClass(createOpen ? 'open' : 'available', 'borderless')}
              onClick={() => setCreateOpen(true)}
            >
              <PlusSignIcon className="h-[15px] w-[15px]" />
              Contact
            </button>
          </>
        }
      />
      <div className="min-h-0 flex-1 overflow-y-auto">
        {query.isLoading ? (
          <CollectionState icon={UserGroupIcon} title="" detail="" loading />
        ) : contacts.length ? (
          contacts.map((contact) => {
            const name = `${contact.first_name} ${contact.last_name ?? ''}`.trim();
            return (
              <button
                key={contact.id}
                type="button"
                onClick={() =>
                  navigate({
                    to: '/w/$slug/crm/contacts/$contactId',
                    params: { slug: workspaceSlug, contactId: contact.id },
                  })
                }
                className="grid w-full grid-cols-[minmax(0,1fr)_180px_120px_100px] items-center gap-4 border-b border-border/50 px-4 py-3 text-left hover:bg-muted/25 sm:px-6 lg:px-8"
              >
                <span className="flex min-w-0 items-center gap-2.5">
                  <UserAvatar name={name} className="h-7 w-7 shrink-0" fallbackClassName="text-[10px]" />
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-medium">{name}</span>
                    <span className="block truncate text-xs text-muted-foreground">{contact.email || 'No email'}</span>
                  </span>
                </span>
                <span className="truncate text-xs text-muted-foreground">{contact.job_title || 'No job title'}</span>
                <span className="truncate text-xs capitalize text-muted-foreground">
                  {contact.lifecycle_stage.replaceAll('_', ' ')}
                </span>
                <span className="truncate text-xs text-muted-foreground">{contact.display_id}</span>
              </button>
            );
          })
        ) : (
          <CollectionState
            icon={UserGroupIcon}
            title="No company contacts"
            detail="Create a contact or link an existing CRM contact to this company."
          />
        )}
      </div>
      <CreateContactDialog open={createOpen} onOpenChange={setCreateOpen} onCreated={linkCreatedContact} />
      <EntityLinkDialog
        open={linkOpen}
        onOpenChange={setLinkOpen}
        workspaceId={workspaceId}
        targetType="company"
        targetId={companyId}
        type="contact"
        onLinked={() => void query.refetch()}
      />
    </div>
  );
}

export function CompanyDealsView({
  workspaceId,
  workspaceSlug,
  companyId,
  companyName,
}: {
  workspaceId: string;
  workspaceSlug: string;
  companyId: string;
  companyName: string;
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search.trim());
  const [createOpen, setCreateOpen] = useState(false);
  const [linkOpen, setLinkOpen] = useState(false);
  const query = useCompanyDeals(workspaceId, companyId, {
    search: deferredSearch || undefined,
    per_page: 50,
  });
  const { members } = useAssignableWorkspaceMembers(workspaceId);
  const owners = useMemo(
    () => new Map(members.map((member) => [member.id, member.display_name || member.email])),
    [members],
  );
  const deals = query.data?.data ?? [];
  const openDeal = (deal: CRMDeal) => openDealRoute(navigate as never, location, workspaceSlug, deal.id);
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <CollectionHeader
        title="Deals"
        search={search}
        onSearchChange={setSearch}
        actions={
          <>
            <button
              type="button"
              className={getOptionalSectionActionClass(linkOpen ? 'open' : 'available', 'borderless')}
              onClick={() => setLinkOpen(true)}
            >
			  <Link01Icon className="h-[15px] w-[15px]" />
			  Assign existing
            </button>
            <button
              type="button"
              className={getOptionalSectionActionClass(createOpen ? 'open' : 'available', 'borderless')}
              onClick={() => setCreateOpen(true)}
            >
              <PlusSignIcon className="h-[15px] w-[15px]" />
              Deal
            </button>
          </>
        }
      />
      <div className="min-h-0 flex-1 overflow-auto">
        {query.isLoading ? (
          <CollectionState icon={DollarCircleIcon} title="" detail="" loading />
        ) : deals.length ? (
          <div className="min-w-[760px]">
            <div className="grid grid-cols-[minmax(220px,1fr)_180px_130px_160px_140px] gap-3 border-b border-border/60 bg-muted/20 px-4 py-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground sm:px-6 lg:px-8">
              <span>Deal</span>
              <span>Stage</span>
              <span>Amount</span>
              <span>Owner</span>
              <span>Close date</span>
            </div>
            {deals.map((deal) => (
              <button
                key={deal.id}
                type="button"
                onClick={() => openDeal(deal)}
                className="grid w-full grid-cols-[minmax(220px,1fr)_180px_130px_160px_140px] items-center gap-3 border-b border-border/50 px-4 py-3 text-left hover:bg-muted/25 sm:px-6 lg:px-8"
              >
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">{deal.name}</span>
                  <span className="block text-xs text-muted-foreground">{deal.display_id}</span>
                </span>
                <span className="truncate text-xs">{deal.stage?.name ?? 'No stage'}</span>
                <span className="text-xs">
                  {deal.amount != null
                    ? new Intl.NumberFormat(undefined, {
                        style: 'currency',
                        currency: deal.currency || 'USD',
                        maximumFractionDigits: 0,
                      }).format(deal.amount)
                    : '—'}
                </span>
                <span className="truncate text-xs text-muted-foreground">
                  {deal.owner_member_id ? (owners.get(deal.owner_member_id) ?? 'Unknown') : 'Unassigned'}
                </span>
                <span className="text-xs text-muted-foreground">
                  {deal.close_date ? format(new Date(deal.close_date), 'MMM d, yyyy') : '—'}
                </span>
              </button>
            ))}
          </div>
        ) : (
          <CollectionState
            icon={DollarCircleIcon}
            title="No company deals"
            detail="Create a deal or link an existing opportunity to this company."
          />
        )}
      </div>
      <CreateDealDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        companyContext={{ id: companyId, name: companyName }}
        onDealCreated={() => void query.refetch()}
      />
      <EntityLinkDialog
        open={linkOpen}
        onOpenChange={setLinkOpen}
        workspaceId={workspaceId}
        targetType="company"
        targetId={companyId}
        type="deal"
        onLinked={() => void query.refetch()}
      />
    </div>
  );
}

export function CompanyMeetingsView({
  workspaceId,
  workspaceSlug,
  companyId,
}: {
  workspaceId: string;
  workspaceSlug: string;
  companyId: string;
}) {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const deferredSearch = useDeferredValue(search.trim());
  const query = useCRMMeetings(workspaceId, {
    company_rollup_id: companyId,
    search: deferredSearch || undefined,
    per_page: 50,
  });
  const meetings = query.data?.data ?? [];
  return (
    <div className="flex min-h-0 flex-1 flex-col bg-transparent">
      <CollectionHeader
        title="Meetings"
        search={search}
        onSearchChange={setSearch}
        actions={
          <button
            type="button"
            className={getOptionalSectionActionClass(createOpen ? 'open' : 'available', 'borderless')}
            onClick={() => setCreateOpen(true)}
          >
            <PlusSignIcon className="h-[15px] w-[15px]" />
            Meeting
          </button>
        }
      />
      <div className="min-h-0 flex-1 overflow-y-auto">
        {query.isLoading ? (
          <CollectionState icon={Calendar03Icon} title="" detail="" loading />
        ) : meetings.length ? (
          meetings.map((meeting) => {
            const date = meeting.scheduled_start_at ?? meeting.actual_start_at ?? meeting.created_at;
            return (
              <button
                key={meeting.id}
                type="button"
                onClick={() =>
                  navigate({
                    to: '/w/$slug/crm/meetings/$meetingId',
                    params: { slug: workspaceSlug, meetingId: meeting.id },
                  })
                }
                className="grid w-full grid-cols-[minmax(0,1fr)_150px_180px_120px] items-center gap-4 border-b border-border/50 px-4 py-3 text-left hover:bg-muted/25 sm:px-6 lg:px-8"
              >
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">{meeting.title}</span>
                  <span className="block truncate text-xs text-muted-foreground">
                    {meeting.participants?.length
                      ? `${meeting.participants.length} participants`
                      : 'Participants pending'}
                  </span>
                </span>
                <MeetingPlatformLabel platform={meeting.platform} compact presentation="quiet" />
                <span className="text-xs text-muted-foreground">{formatMeetingDate(date, 'MMM d, yyyy · p')}</span>
                <MeetingStatusText status={meeting.status} className="justify-self-end" />
              </button>
            );
          })
        ) : (
          <CollectionState
            icon={Calendar03Icon}
            title="No company meetings"
            detail="Add a meeting and keep its notes, transcript, and follow-up connected to this company."
          />
        )}
      </div>
      <CreateMeetingDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        workspaceId={workspaceId}
        workspaceSlug={workspaceSlug}
        associationContext={{ type: 'company', id: companyId }}
      />
    </div>
  );
}

export function CompanySupportView({
  workspaceId,
  workspaceSlug,
  companyId,
}: {
  workspaceId: string;
  workspaceSlug: string;
  companyId: string;
}) {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('all');
  const [linkOpen, setLinkOpen] = useState(false);
  const deferredSearch = useDeferredValue(search.trim());
  const query = useCompanySupportConversations(workspaceId, companyId, {
    search: deferredSearch || undefined,
    status: status === 'all' ? undefined : status,
    per_page: 50,
  });
  const conversations = query.data?.data ?? [];
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <CollectionHeader
        title="Support"
        search={search}
        onSearchChange={setSearch}
        actions={
          <button
            type="button"
            className={getOptionalSectionActionClass(linkOpen ? 'open' : 'available', 'borderless')}
            onClick={() => setLinkOpen(true)}
          >
            <Link01Icon className="h-[15px] w-[15px]" />
            Link conversation
          </button>
        }
      >
        <Select value={status} onValueChange={setStatus}>
          <SelectTrigger className="h-7 w-36 text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            <SelectItem value="open">Open</SelectItem>
            <SelectItem value="waiting_on_customer">Waiting</SelectItem>
            <SelectItem value="resolved">Resolved</SelectItem>
            <SelectItem value="spam">Spam</SelectItem>
          </SelectContent>
        </Select>
      </CollectionHeader>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {query.isLoading ? (
          <CollectionState icon={Message01Icon} title="" detail="" loading />
        ) : conversations.length ? (
          conversations.map((conversation) => (
            <button
              key={conversation.id}
              type="button"
              onClick={() =>
                navigate({
                  to: '/w/$slug/support/$conversationId',
                  params: {
                    slug: workspaceSlug,
                    conversationId: conversation.id,
                  },
                })
              }
              className="grid w-full grid-cols-[minmax(0,1fr)_150px_160px_130px] items-center gap-4 border-b border-border/50 px-4 py-3 text-left hover:bg-muted/25 sm:px-6 lg:px-8"
            >
              <span className="min-w-0">
                <span className="block truncate text-sm font-medium">{conversation.subject}</span>
                <span className="block truncate text-xs text-muted-foreground">
                  C-{conversation.display_id} ·{' '}
                  {conversation.customer_name || conversation.customer_email || 'Unknown contact'}
                </span>
              </span>
              <span className="text-xs capitalize text-muted-foreground">
                {conversation.status.replaceAll('_', ' ')}
              </span>
              <span className="truncate text-xs text-muted-foreground">{conversation.mailbox_name || 'No inbox'}</span>
              <span className="text-xs text-muted-foreground">
                {format(new Date(conversation.updated_at), 'MMM d, yyyy')}
              </span>
            </button>
          ))
        ) : (
          <CollectionState
            icon={Message01Icon}
            title="No support conversations"
            detail="Link an existing support conversation to see the company’s service history here."
          />
        )}
      </div>
      <EntityLinkDialog
        open={linkOpen}
        onOpenChange={setLinkOpen}
        workspaceId={workspaceId}
        targetType="company"
        targetId={companyId}
        type="support_conversation"
        onLinked={() => void query.refetch()}
      />
    </div>
  );
}

export function ContactDealsView({
  workspaceId,
  workspaceSlug,
  contactId,
  contactName,
  onChanged,
}: {
  workspaceId: string;
  workspaceSlug: string;
  contactId: string;
  contactName: string;
  onChanged?: () => void;
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search.trim());
  const [createOpen, setCreateOpen] = useState(false);
  const [linkOpen, setLinkOpen] = useState(false);
  const query = useDeals(workspaceId, {
    contact_id: contactId,
    search: deferredSearch || undefined,
    per_page: 50,
  });
  const { members } = useAssignableWorkspaceMembers(workspaceId);
  const owners = useMemo(
    () => new Map(members.map((member) => [member.id, member.display_name || member.email])),
    [members],
  );
  const deals = query.data?.data ?? [];
  const refresh = () => {
    void query.refetch();
    onChanged?.();
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <CollectionHeader
        title="Deals"
        search={search}
        onSearchChange={setSearch}
        actions={
          <>
            <button
              type="button"
              className={getOptionalSectionActionClass(linkOpen ? 'open' : 'available', 'borderless')}
              onClick={() => setLinkOpen(true)}
            >
              <Link01Icon className="h-[15px] w-[15px]" />
              Link existing
            </button>
            <button
              type="button"
              className={getOptionalSectionActionClass(createOpen ? 'open' : 'available', 'borderless')}
              onClick={() => setCreateOpen(true)}
            >
              <PlusSignIcon className="h-[15px] w-[15px]" />
              Deal
            </button>
          </>
        }
      />
      <div className="min-h-0 flex-1 overflow-auto">
        {query.isLoading ? (
          <CollectionState icon={DollarCircleIcon} title="" detail="" loading />
        ) : deals.length ? (
          <div className="min-w-[760px]">
            <div className="grid grid-cols-[minmax(220px,1fr)_180px_130px_160px_140px] gap-3 border-b border-border/60 bg-muted/20 px-4 py-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground sm:px-6 lg:px-8">
              <span>Deal</span>
              <span>Stage</span>
              <span>Amount</span>
              <span>Owner</span>
              <span>Close date</span>
            </div>
            {deals.map((deal) => (
              <button
                key={deal.id}
                type="button"
                onClick={() => openDealRoute(navigate as never, location, workspaceSlug, deal.id)}
                className="grid w-full grid-cols-[minmax(220px,1fr)_180px_130px_160px_140px] items-center gap-3 border-b border-border/50 px-4 py-3 text-left hover:bg-muted/25 sm:px-6 lg:px-8"
              >
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">{deal.name}</span>
                  <span className="block text-xs text-muted-foreground">{deal.display_id}</span>
                </span>
                <span className="truncate text-xs">{deal.stage?.name ?? 'No stage'}</span>
                <span className="text-xs">
                  {deal.amount != null
                    ? new Intl.NumberFormat(undefined, {
                        style: 'currency',
                        currency: deal.currency || 'USD',
                        maximumFractionDigits: 0,
                      }).format(deal.amount)
                    : '—'}
                </span>
                <span className="truncate text-xs text-muted-foreground">
                  {deal.owner_member_id ? (owners.get(deal.owner_member_id) ?? 'Unknown') : 'Unassigned'}
                </span>
                <span className="text-xs text-muted-foreground">
                  {deal.close_date ? format(new Date(deal.close_date), 'MMM d, yyyy') : '—'}
                </span>
              </button>
            ))}
          </div>
        ) : (
          <CollectionState
            icon={DollarCircleIcon}
            title="No contact deals"
            detail="Create a deal or link an existing opportunity to this contact."
          />
        )}
      </div>
      <CreateDealDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        contactContext={{ id: contactId, name: contactName }}
        onDealCreated={refresh}
      />
      <EntityLinkDialog
        open={linkOpen}
        onOpenChange={setLinkOpen}
        workspaceId={workspaceId}
        targetType="contact"
        targetId={contactId}
        type="deal"
        onLinked={refresh}
      />
    </div>
  );
}

export function ContactMeetingsView({
  workspaceId,
  workspaceSlug,
  contactId,
}: {
  workspaceId: string;
  workspaceSlug: string;
  contactId: string;
}) {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const deferredSearch = useDeferredValue(search.trim());
  const query = useCRMMeetings(workspaceId, {
    contact_id: contactId,
    search: deferredSearch || undefined,
    per_page: 50,
  });
  const meetings = query.data?.data ?? [];

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-transparent">
      <CollectionHeader
        title="Meetings"
        search={search}
        onSearchChange={setSearch}
        actions={
          <button
            type="button"
            className={getOptionalSectionActionClass(createOpen ? 'open' : 'available', 'borderless')}
            onClick={() => setCreateOpen(true)}
          >
            <PlusSignIcon className="h-[15px] w-[15px]" />
            Meeting
          </button>
        }
      />
      <div className="min-h-0 flex-1 overflow-y-auto">
        {query.isLoading ? (
          <CollectionState icon={Calendar03Icon} title="" detail="" loading />
        ) : meetings.length ? (
          meetings.map((meeting) => {
            const date = meeting.scheduled_start_at ?? meeting.actual_start_at ?? meeting.created_at;
            return (
              <button
                key={meeting.id}
                type="button"
                onClick={() =>
                  navigate({
                    to: '/w/$slug/crm/meetings/$meetingId',
                    params: { slug: workspaceSlug, meetingId: meeting.id },
                  })
                }
                className="grid w-full grid-cols-[minmax(0,1fr)_150px_180px_120px] items-center gap-4 border-b border-border/50 px-4 py-3 text-left hover:bg-muted/25 sm:px-6 lg:px-8"
              >
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">{meeting.title}</span>
                  <span className="block truncate text-xs text-muted-foreground">
                    {meeting.participants?.length
                      ? `${meeting.participants.length} participants`
                      : 'Participants pending'}
                  </span>
                </span>
                <MeetingPlatformLabel platform={meeting.platform} compact presentation="quiet" />
                <span className="text-xs text-muted-foreground">{formatMeetingDate(date, 'MMM d, yyyy · p')}</span>
                <MeetingStatusText status={meeting.status} className="justify-self-end" />
              </button>
            );
          })
        ) : (
          <CollectionState
            icon={Calendar03Icon}
            title="No contact meetings"
            detail="Add a meeting and keep its notes, transcript, and follow-up connected to this contact."
          />
        )}
      </div>
      <CreateMeetingDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        workspaceId={workspaceId}
        workspaceSlug={workspaceSlug}
        associationContext={{ type: 'contact', id: contactId }}
      />
    </div>
  );
}

export function ContactSupportView({
  workspaceId,
  workspaceSlug,
  contactId,
  onChanged,
}: {
  workspaceId: string;
  workspaceSlug: string;
  contactId: string;
  onChanged?: () => void;
}) {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('all');
  const [linkOpen, setLinkOpen] = useState(false);
  const deferredSearch = useDeferredValue(search.trim());
  const query = useContactSupportConversations(workspaceId, contactId, {
    search: deferredSearch || undefined,
    status: status === 'all' ? undefined : status,
    per_page: 50,
  });
  const conversations = query.data?.data ?? [];
  const refresh = () => {
    void query.refetch();
    onChanged?.();
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <CollectionHeader
        title="Support"
        search={search}
        onSearchChange={setSearch}
        actions={
          <button
            type="button"
            className={getOptionalSectionActionClass(linkOpen ? 'open' : 'available', 'borderless')}
            onClick={() => setLinkOpen(true)}
          >
            <Link01Icon className="h-[15px] w-[15px]" />
            Link conversation
          </button>
        }
      >
        <Select value={status} onValueChange={setStatus}>
          <SelectTrigger className="h-7 w-36 text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            <SelectItem value="open">Open</SelectItem>
            <SelectItem value="waiting_on_customer">Waiting</SelectItem>
            <SelectItem value="resolved">Resolved</SelectItem>
            <SelectItem value="spam">Spam</SelectItem>
          </SelectContent>
        </Select>
      </CollectionHeader>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {query.isLoading ? (
          <CollectionState icon={Message01Icon} title="" detail="" loading />
        ) : conversations.length ? (
          conversations.map((conversation) => (
            <button
              key={conversation.id}
              type="button"
              onClick={() =>
                navigate({
                  to: '/w/$slug/support/$conversationId',
                  params: { slug: workspaceSlug, conversationId: conversation.id },
                })
              }
              className="grid w-full grid-cols-[minmax(0,1fr)_150px_160px_130px] items-center gap-4 border-b border-border/50 px-4 py-3 text-left hover:bg-muted/25 sm:px-6 lg:px-8"
            >
              <span className="min-w-0">
                <span className="block truncate text-sm font-medium">{conversation.subject}</span>
                <span className="block truncate text-xs text-muted-foreground">
                  C-{conversation.display_id} · {conversation.customer_name || conversation.customer_email || 'Unknown contact'}
                </span>
              </span>
              <span className="text-xs capitalize text-muted-foreground">
                {conversation.status.replaceAll('_', ' ')}
              </span>
              <span className="truncate text-xs text-muted-foreground">{conversation.mailbox_name || 'No inbox'}</span>
              <span className="text-xs text-muted-foreground">
                {format(new Date(conversation.updated_at), 'MMM d, yyyy')}
              </span>
            </button>
          ))
        ) : (
          <CollectionState
            icon={Message01Icon}
            title="No support conversations"
            detail="Link an existing support conversation to see this contact’s service history here."
          />
        )}
      </div>
      <EntityLinkDialog
        open={linkOpen}
        onOpenChange={setLinkOpen}
        workspaceId={workspaceId}
        targetType="contact"
        targetId={contactId}
        type="support_conversation"
        onLinked={refresh}
      />
    </div>
  );
}
