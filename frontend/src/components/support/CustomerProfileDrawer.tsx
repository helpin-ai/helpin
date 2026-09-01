import { useEffect, useMemo, useState, type ElementType, type ReactNode } from 'react';
import { Link } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  ArrowLeft01Icon,
  Building03Icon,
  DollarCircleIcon,
  File01Icon,
  GlobeIcon,
  Loading01Icon,
  Mail01Icon,
  Message01Icon,
  PlusSignIcon,
  Tag01Icon,
  TelephoneIcon,
  UserIcon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { CreateContactDialog } from '@/components/crm/CreateContactDialog';
import { useContact, useContactAssociations, useContactSupportConversations, useUpdateContact } from '@/hooks/queries/useCRM';
import { crmSearchService } from '@/lib/services/crmService';
import { supportService } from '@/lib/services/supportService';
import { queryKeys } from '@/lib/queryKeys';
import { cn } from '@/lib/utils';
import type { CRMContact, CRMAssociationEnriched, CRMSearchResult, CreateCRMContactRequest, LeadStatus, LifecycleStage, UpdateCRMContactRequest } from '@/lib/crmTypes';
import type { SupportConversation } from '@/lib/pmTypes';

interface CustomerProfileDrawerProps {
  workspaceId: string;
  workspaceSlug?: string | null;
  conversation: SupportConversation | null | undefined;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const lifecycleOptions: Array<{ value: LifecycleStage; label: string }> = [
  { value: 'subscriber', label: 'Subscriber' },
  { value: 'lead', label: 'Lead' },
  { value: 'marketing_qualified', label: 'Marketing Qualified' },
  { value: 'sales_qualified', label: 'Sales Qualified' },
  { value: 'opportunity', label: 'Opportunity' },
  { value: 'customer', label: 'Customer' },
  { value: 'evangelist', label: 'Evangelist' },
];

const leadStatusOptions: Array<{ value: LeadStatus; label: string }> = [
  { value: 'new', label: 'New' },
  { value: 'open', label: 'Open' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'unqualified', label: 'Unqualified' },
];

const previewLimit = 4;
const profilePanelAnimationMs = 180;

export const customerProfilePanelClassName =
  'translate-x-0 opacity-100 transition-all duration-200 ease-out';
export const customerProfilePanelClosedClassName =
  'translate-x-3 opacity-0 transition-all duration-150 ease-in';

export function customerProfileDisplayName(contact: CRMContact | null | undefined, conversation: Pick<SupportConversation, 'customer_name' | 'customer_email'> | null | undefined): string {
  const contactName = [contact?.first_name, contact?.last_name].map((part) => part?.trim()).filter(Boolean).join(' ');
  return contactName || conversation?.customer_name?.trim() || contact?.email?.trim() || conversation?.customer_email?.trim() || 'Customer';
}

export function splitCustomerDisplayName(value: string): { firstName: string; lastName?: string } {
  const parts = value.trim().replace(/\s+/g, ' ').split(' ').filter(Boolean);
  if (parts.length === 0) return { firstName: '' };
  if (parts.length === 1) return { firstName: parts[0] };
  return { firstName: parts[0], lastName: parts.slice(1).join(' ') };
}

export function customerProfileInitialContactValues(conversation: Pick<SupportConversation, 'customer_name' | 'customer_email'> | null | undefined): CreateCRMContactRequest {
  const { firstName, lastName } = splitCustomerDisplayName(conversation?.customer_name?.trim() || conversation?.customer_email?.split('@')[0] || 'Customer');
  return {
    workspace_id: '',
    first_name: firstName || 'Customer',
    last_name: lastName,
    email: conversation?.customer_email?.trim() || undefined,
    lifecycle_stage: 'subscriber',
    lead_status: 'new',
    source: 'support',
  };
}

export function CustomerProfileDrawer({
  workspaceId,
  workspaceSlug,
  conversation,
  open,
  onOpenChange,
}: CustomerProfileDrawerProps) {
  const queryClient = useQueryClient();
  const [rendered, setRendered] = useState(open);
  const [visible, setVisible] = useState(open);
  const [contactQuery, setContactQuery] = useState('');
  const [contactResults, setContactResults] = useState<CRMSearchResult[]>([]);
  const [searchingContacts, setSearchingContacts] = useState(false);
  const [createContactOpen, setCreateContactOpen] = useState(false);
  const [linkingContactId, setLinkingContactId] = useState<string | null>(null);
  const contactId = conversation?.crm_contact_id?.trim() ?? '';
  const { data: contact, isLoading: contactLoading } = useContact(workspaceId, contactId, open);
  const { data: associations = [] } = useContactAssociations(workspaceId, contactId, open);
  const { data: supportPage } = useContactSupportConversations(workspaceId, contactId, open);
  const updateContact = useUpdateContact(workspaceId);

  const linkedAssociations = useMemo(() => normalizeContactAssociations(associations, contactId), [associations, contactId]);
  const companies = linkedAssociations.filter((assoc) => assoc.linkedType === 'company');
  const deals = linkedAssociations.filter((assoc) => assoc.linkedType === 'deal');
  const tasks = linkedAssociations.filter((assoc) => assoc.linkedType === 'task');
  const supportConversations = supportPage?.data ?? [];
  const displayName = customerProfileDisplayName(contact, conversation);
  const hasLinkedContact = Boolean(contactId);
  const initialContactValues = useMemo(
    () => ({ ...customerProfileInitialContactValues(conversation), workspace_id: workspaceId }),
    [conversation, workspaceId],
  );

  useEffect(() => {
    if (open) {
      setRendered(true);
      window.requestAnimationFrame(() => setVisible(true));
      return;
    }
    setVisible(false);
    const timeout = window.setTimeout(() => setRendered(false), profilePanelAnimationMs);
    return () => window.clearTimeout(timeout);
  }, [open]);

  useEffect(() => {
    if (!open || hasLinkedContact) {
      setContactQuery('');
      setContactResults([]);
      setSearchingContacts(false);
      return;
    }
    if (contactQuery.trim().length < 2) {
      setContactResults([]);
      setSearchingContacts(false);
      return;
    }
    const handle = window.setTimeout(async () => {
      setSearchingContacts(true);
      try {
        const response = await crmSearchService.search(workspaceId, contactQuery.trim());
        setContactResults((response.data ?? []).filter((result) => result.type === 'contact'));
      } catch {
        setContactResults([]);
      } finally {
        setSearchingContacts(false);
      }
    }, 250);
    return () => window.clearTimeout(handle);
  }, [contactQuery, hasLinkedContact, open, workspaceId]);

  if (!rendered || !conversation) return null;

  const patchContact = async (patch: UpdateCRMContactRequest) => {
    if (!contact) return;
    await updateContact.mutateAsync({ id: contact.id, ...patch });
  };

  const handleNameCommit = async (value: string) => {
    const { firstName, lastName } = splitCustomerDisplayName(value);
    if (!firstName || !contact) return;
    await patchContact({ first_name: firstName, last_name: lastName });
  };

  const invalidateLinkedContactViews = async (nextContactId: string) => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversation.id) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversationAssociations(workspaceId, conversation.id) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.crm.contacts(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.crm.contact(workspaceId, nextContactId) }),
    ]);
  };

  const linkContact = async (nextContactId: string, options: { showErrorToast?: boolean } = {}) => {
    const showErrorToast = options.showErrorToast ?? true;
    setLinkingContactId(nextContactId);
    try {
      const response = await supportService.updateConversationCRMContact(workspaceId, conversation.id, {
        crm_contact_id: nextContactId,
      });
      if (response.error) throw new Error(response.error);
      await invalidateLinkedContactViews(nextContactId);
      setContactQuery('');
      setContactResults([]);
      toast.success('CRM contact linked');
    } catch (error) {
      if (showErrorToast) {
        toast.error('Failed to link CRM contact');
      }
      throw error;
    } finally {
      setLinkingContactId(null);
    }
  };

  return (
    <div className={cn('absolute inset-0 z-10 flex flex-col bg-muted/30', visible ? customerProfilePanelClassName : customerProfilePanelClosedClassName)}>
      <div className="flex items-center gap-2 border-b bg-muted/30 px-3 py-2">
        <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={() => onOpenChange(false)} aria-label="Back to details">
          <ArrowLeft01Icon className="h-4 w-4" />
        </Button>
        <div className="min-w-0">
          <h3 className="truncate text-sm font-semibold">Customer profile</h3>
          <p className="truncate text-[11px] text-muted-foreground">
            {hasLinkedContact ? 'CRM contact details' : 'Link or create a CRM contact'}
          </p>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto pb-8">
          <div className="border-b border-border/50 px-5 py-5">
            <div className="flex items-center gap-3">
              <UserAvatar name={displayName} className="h-11 w-11 shrink-0" fallbackClassName="text-sm font-semibold" />
              <div className="min-w-0 flex-1">
                {contact ? (
                  <EditableText
                    value={displayName}
                    className="text-base font-semibold"
                    placeholder="Customer name"
                    disabled={updateContact.isPending}
                    onCommit={handleNameCommit}
                  />
                ) : (
                  <p className="truncate text-base font-semibold">{displayName}</p>
                )}
                <p className="mt-0.5 truncate text-xs text-muted-foreground">
                  {contact?.display_id || conversation?.customer_email || 'Support customer'}
                </p>
              </div>
            </div>
            {contact && workspaceSlug && (
              <Button asChild variant="outline" size="sm" className="mt-4 w-full">
                <Link to="/w/$slug/crm/contacts/$contactId" params={{ slug: workspaceSlug, contactId: contact.id }}>
                  Open full CRM contact
                </Link>
              </Button>
            )}
          </div>

          {!hasLinkedContact ? (
            <UnlinkedCustomerState
              conversation={conversation}
              query={contactQuery}
              results={contactResults}
              searching={searchingContacts}
              linkingContactId={linkingContactId}
              onQueryChange={setContactQuery}
              onLink={(id) => { linkContact(id).catch(() => undefined); }}
              onCreate={() => setCreateContactOpen(true)}
            />
          ) : contactLoading && !contact ? (
            <div className="px-5 py-6 text-sm text-muted-foreground">Loading customer profile...</div>
          ) : contact ? (
            <>
              <div className="border-b border-border/50 px-5 py-4">
                <h3 className="mb-3 text-[11px] font-medium uppercase tracking-tight text-foreground">Details</h3>
                <div className="space-y-2">
                  <ProfileRow icon={Mail01Icon} label="Email">
                    <EditableText
                      value={contact.email ?? ''}
                      placeholder="Add email"
                      className="font-mono text-[11.5px]"
                      disabled={updateContact.isPending}
                      onCommit={(value) => patchContact({ email: value || undefined })}
                    />
                  </ProfileRow>
                  <ProfileRow icon={TelephoneIcon} label="Phone">
                    <EditableText
                      value={contact.phone ?? ''}
                      placeholder="Add phone"
                      className="font-mono text-[11.5px]"
                      disabled={updateContact.isPending}
                      onCommit={(value) => patchContact({ phone: value || undefined })}
                    />
                  </ProfileRow>
                  <ProfileRow icon={UserIcon} label="Title">
                    <EditableText
                      value={contact.job_title ?? ''}
                      placeholder="Add title"
                      disabled={updateContact.isPending}
                      onCommit={(value) => patchContact({ job_title: value || undefined })}
                    />
                  </ProfileRow>
                  <ProfileRow icon={GlobeIcon} label="Source">
                    <EditableText
                      value={contact.source ?? ''}
                      placeholder="Add source"
                      disabled={updateContact.isPending}
                      onCommit={(value) => patchContact({ source: value || undefined })}
                    />
                  </ProfileRow>
                  <ProfileRow icon={Tag01Icon} label="Stage">
                    <ProfileSelect
                      value={contact.lifecycle_stage}
                      options={lifecycleOptions}
                      disabled={updateContact.isPending}
                      onChange={(value) => patchContact({ lifecycle_stage: value as LifecycleStage })}
                    />
                  </ProfileRow>
                  <ProfileRow icon={Tag01Icon} label="Status">
                    <ProfileSelect
                      value={contact.lead_status}
                      options={leadStatusOptions}
                      disabled={updateContact.isPending}
                      onChange={(value) => patchContact({ lead_status: value as LeadStatus })}
                    />
                  </ProfileRow>
                </div>
              </div>

              <AssociationSection title="Company" count={companies.length} icon={Building03Icon} items={companies} workspaceSlug={workspaceSlug} />
              <AssociationSection title="Deals" count={deals.length} icon={DollarCircleIcon} items={deals} workspaceSlug={workspaceSlug} />
              <AssociationSection title="Open tasks" count={tasks.length} icon={File01Icon} items={tasks} workspaceSlug={workspaceSlug} />
              <SupportSection conversations={supportConversations} workspaceSlug={workspaceSlug} />
            </>
          ) : (
            <div className="px-5 py-6 text-sm text-muted-foreground">CRM contact not found.</div>
          )}
      </div>
      <CreateContactDialog
        open={createContactOpen}
        onOpenChange={setCreateContactOpen}
        initialValues={initialContactValues}
        showCreatedToast={false}
        onCreated={async (nextContact) => {
          await linkContact(nextContact.id, { showErrorToast: false });
        }}
      />
    </div>
  );
}

function UnlinkedCustomerState({
  conversation,
  query,
  results,
  searching,
  linkingContactId,
  onQueryChange,
  onLink,
  onCreate,
}: {
  conversation: SupportConversation | null | undefined;
  query: string;
  results: CRMSearchResult[];
  searching: boolean;
  linkingContactId: string | null;
  onQueryChange: (value: string) => void;
  onLink: (contactId: string) => void;
  onCreate: () => void;
}) {
  return (
    <div className="space-y-4 px-5 py-5">
      <div className="rounded-md border border-dashed border-border/70 px-4 py-4">
        <p className="text-sm font-medium">No CRM contact linked</p>
        <p className="mt-1 text-sm text-muted-foreground">
          Link an existing CRM contact or create one from this support customer to edit full profile fields here.
        </p>
        {conversation?.customer_email && (
          <div className="mt-4 rounded-md bg-muted/40 px-3 py-2">
            <div className="text-[10px] font-medium uppercase tracking-tight text-muted-foreground">Support email</div>
            <div className="mt-1 truncate font-mono text-xs">{conversation.customer_email}</div>
          </div>
        )}
      </div>
      <Button size="sm" className="w-full gap-1.5" onClick={onCreate}>
        <PlusSignIcon className="h-3.5 w-3.5" />
        Create and link contact
      </Button>
      <div className="space-y-2">
        <QuietSearchInput
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder="Search existing contacts"
        />
        <div className="max-h-64 space-y-1 overflow-y-auto">
          {searching && (
            <div className="flex items-center justify-center gap-2 py-4 text-sm text-muted-foreground">
              <Loading01Icon className="h-4 w-4 animate-spin" />
              Searching...
            </div>
          )}
          {!searching && query.trim().length >= 2 && results.length === 0 && (
            <p className="py-4 text-center text-sm text-muted-foreground">No matching contacts</p>
          )}
          {!searching && results.map((result) => (
            <button
              key={result.id}
              type="button"
              disabled={Boolean(linkingContactId)}
              className="w-full rounded-md border bg-background px-3 py-2 text-left text-sm transition-colors hover:bg-accent"
              onClick={() => onLink(result.id)}
            >
              <div className="flex min-w-0 items-center gap-2">
                {linkingContactId === result.id ? (
                  <Loading01Icon className="h-3.5 w-3.5 shrink-0 animate-spin text-muted-foreground" />
                ) : (
                  <UserIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                )}
                <span className="min-w-0 flex-1 truncate font-medium">{result.name}</span>
                {'display_id' in result.object && result.object.display_id && (
                  <Badge variant="outline" className="h-5 shrink-0 px-1.5 text-[10px]">
                    #{result.object.display_id}
                  </Badge>
                )}
              </div>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

function ProfileRow({ icon: Icon, label, children }: { icon: ElementType; label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[18px_72px_1fr] items-center gap-2 rounded-md px-1.5 py-1.5 text-xs hover:bg-muted/40">
      <Icon className="h-3.5 w-3.5 text-muted-foreground" />
      <span className="text-muted-foreground">{label}</span>
      <div className="min-w-0">{children}</div>
    </div>
  );
}

function EditableText({
  value,
  placeholder,
  onCommit,
  disabled,
  className,
}: {
  value: string;
  placeholder: string;
  onCommit: (value: string) => void | Promise<void>;
  disabled?: boolean;
  className?: string;
}) {
  const [draft, setDraft] = useState(value);

  useEffect(() => {
    setDraft(value);
  }, [value]);

  const commit = async () => {
    const next = draft.trim();
    if (next === value.trim()) return;
    try {
      await onCommit(next);
    } catch (error) {
      setDraft(value);
      toast.error('Failed to update profile');
    }
  };

  return (
    <input
      value={draft}
      disabled={disabled}
      onChange={(event) => setDraft(event.target.value)}
      onBlur={() => { void commit(); }}
      onKeyDown={(event) => {
        if (event.key === 'Enter') {
          event.preventDefault();
          event.currentTarget.blur();
        }
        if (event.key === 'Escape') {
          event.preventDefault();
          setDraft(value);
          event.currentTarget.blur();
        }
      }}
      placeholder={placeholder}
      className={cn('w-full truncate rounded-sm bg-transparent px-1 py-0.5 text-xs outline-none transition-colors focus:bg-background focus:ring-1 focus:ring-border', className)}
    />
  );
}

function ProfileSelect<T extends string>({
  value,
  options,
  onChange,
  disabled,
}: {
  value: T;
  options: Array<{ value: T; label: string }>;
  onChange: (value: T) => void | Promise<void>;
  disabled?: boolean;
}) {
  return (
    <select
      value={value}
      disabled={disabled}
      onChange={(event) => { void onChange(event.target.value as T); }}
      className="w-full rounded-sm bg-transparent px-1 py-0.5 text-xs outline-none transition-colors focus:bg-background focus:ring-1 focus:ring-border"
    >
      {options.map((option) => (
        <option key={option.value} value={option.value}>{option.label}</option>
      ))}
    </select>
  );
}

type LinkedAssociation = CRMAssociationEnriched & {
  linkedType: string;
  linkedId: string;
};

function normalizeContactAssociations(associations: CRMAssociationEnriched[], contactId: string): LinkedAssociation[] {
  return associations.map((assoc) => {
    const isFrom = assoc.from_object_type === 'contact' && assoc.from_object_id === contactId;
    return {
      ...assoc,
      linkedType: isFrom ? assoc.to_object_type : assoc.from_object_type,
      linkedId: isFrom ? assoc.to_object_id : assoc.from_object_id,
    };
  });
}

function AssociationSection({
  title,
  count,
  icon: Icon,
  items,
  workspaceSlug,
}: {
  title: string;
  count: number;
  icon: ElementType;
  items: LinkedAssociation[];
  workspaceSlug?: string | null;
}) {
  return (
    <div className="border-b border-border/50 px-5 py-4">
      <div className="mb-2 flex items-center justify-between">
        <h3 className="text-[11px] font-medium uppercase tracking-tight text-foreground">{title}</h3>
        {count > 0 && <Badge variant="secondary" className="h-4 rounded-full px-1.5 text-[10px]">{count}</Badge>}
      </div>
      {items.length === 0 ? (
        <p className="text-xs italic text-muted-foreground">None linked.</p>
      ) : (
        <div className="space-y-1">
          {items.slice(0, previewLimit).map((item) => (
            <AssociationLink key={item.id} item={item} icon={Icon} workspaceSlug={workspaceSlug} />
          ))}
          {items.length > previewLimit && (
            <p className="px-1 text-[11px] text-muted-foreground">+{items.length - previewLimit} more in CRM</p>
          )}
        </div>
      )}
    </div>
  );
}

function AssociationLink({ item, icon: Icon, workspaceSlug }: { item: LinkedAssociation; icon: ElementType; workspaceSlug?: string | null }) {
  const href = workspaceSlug ? associationHref(workspaceSlug, item.linkedType, item.linkedId) : null;
  const content = (
    <>
      <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      <span className="min-w-0 flex-1 truncate font-medium">{item.linked_object_name || 'Untitled'}</span>
      {item.linked_object_display_id && <span className="shrink-0 font-mono text-[10px] text-muted-foreground">{item.linked_object_display_id}</span>}
    </>
  );

  if (!href) {
    return <div className="flex items-center gap-2 rounded-md px-1 py-1.5 text-xs">{content}</div>;
  }

  return (
    <Link to={href.to as never} params={href.params as never} className="flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40">
      {content}
    </Link>
  );
}

function associationHref(workspaceSlug: string, type: string, id: string) {
  if (type === 'company') return { to: '/w/$slug/crm/companies/$companyId', params: { slug: workspaceSlug, companyId: id } };
  if (type === 'deal') return { to: '/w/$slug/crm/deals/$dealId', params: { slug: workspaceSlug, dealId: id } };
  if (type === 'task') return { to: '/w/$slug/pm/tasks/$taskId', params: { slug: workspaceSlug, taskId: id } };
  return null;
}

function SupportSection({ conversations, workspaceSlug }: { conversations: SupportConversation[]; workspaceSlug?: string | null }) {
  return (
    <div className="border-b border-border/50 px-5 py-4">
      <div className="mb-2 flex items-center justify-between">
        <h3 className="text-[11px] font-medium uppercase tracking-tight text-foreground">Support</h3>
        {conversations.length > 0 && <Badge variant="secondary" className="h-4 rounded-full px-1.5 text-[10px]">{conversations.length}</Badge>}
      </div>
      {conversations.length === 0 ? (
        <p className="text-xs italic text-muted-foreground">No support conversations.</p>
      ) : (
        <div className="space-y-1">
          {conversations.slice(0, previewLimit).map((conversation) => {
            const content = (
              <>
                <Message01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate font-medium">{conversation.subject}</span>
                <span className="shrink-0 text-[10px] text-muted-foreground">C-{conversation.display_id}</span>
              </>
            );
            if (!workspaceSlug) {
              return <div key={conversation.id} className="flex items-center gap-2 rounded-md px-1 py-1.5 text-xs">{content}</div>;
            }
            return (
              <Link
                key={conversation.id}
                to="/w/$slug/support/$conversationId"
                params={{ slug: workspaceSlug, conversationId: conversation.id }}
                className="flex items-center gap-2 rounded-md px-1 py-1.5 text-xs transition-colors hover:bg-muted/40"
              >
                {content}
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}
