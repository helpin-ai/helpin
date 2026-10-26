import { Link } from '@tanstack/react-router';
import { QuietEmptyState, QuietTextAction } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import {
  useAllowCompanyPortalContacts,
  useCompanyPortalAccess,
  useSetCompanyPortalRole,
  useSetContactPortalAccess,
  useSetContactPortalRole,
} from '@/hooks/queries/useSupport';
import { Loading01Icon } from '@/lib/icons';
import type { CustomerPortalCompanyContact, CustomerPortalContactAccessValue, CustomerPortalRole } from '@/lib/pmTypes';
import { formatDistanceToNow } from 'date-fns';
import { allowableContacts, companyScopeSummary, portalRoleChoices, portalRoleLabels, type PortalRoleChoice } from './companyPortal';

type CompanyAccessChoice = CustomerPortalRole | 'off';

const companyChoices: { value: CompanyAccessChoice; label: string }[] = [
  { value: 'off', label: 'Off' },
  { value: 'requester', label: 'Own requests only' },
  { value: 'member', label: 'All company requests' },
];

type AccessChoice = CustomerPortalContactAccessValue | 'unset';

const accessChoices: { value: AccessChoice; label: string }[] = [
  { value: 'unset', label: 'Not set' },
  { value: 'allowed', label: 'Allowed' },
  { value: 'blocked', label: 'Blocked' },
];

const rowGrid = 'sm:grid sm:grid-cols-[minmax(0,1fr)_128px_160px_112px] sm:items-center sm:gap-4';

function contactName(contact: CustomerPortalCompanyContact) {
  return [contact.first_name, contact.last_name].filter(Boolean).join(' ').trim() || contact.email || 'Unnamed contact';
}

/**
 * CompanyPortalPanel lets support admins decide how a company's contacts use
 * the customer portal: who may sign in, and who sees every request from the
 * company. Signing in always needs an explicit decision per contact.
 */
export function CompanyPortalPanel({ workspaceId, workspaceSlug, companyId }: {
  workspaceId: string;
  workspaceSlug: string;
  companyId: string;
}) {
  const access = useCompanyPortalAccess(workspaceId, companyId);
  const setRole = useSetCompanyPortalRole(workspaceId, companyId);
  const allowAll = useAllowCompanyPortalContacts(workspaceId, companyId);

  if (access.isLoading) {
    return (
      <div className="flex min-h-64 items-center justify-center" role="status" aria-label="Loading customer portal access">
        <Loading01Icon className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    );
  }
  if (access.isError || !access.data) {
    return (
      <QuietEmptyState
        className="border-t-0"
        title="Customer portal access is unavailable"
        description="It could not be loaded. Check your connection and try again."
        action={<QuietTextAction onClick={() => void access.refetch()}>Try again</QuietTextAction>}
      />
    );
  }

  const data = access.data;
  const allowable = allowableContacts(data.contacts).length;
  const companyChoice: CompanyAccessChoice = data.default_role ?? 'off';

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex min-h-12 flex-wrap items-center gap-2 border-b border-border/60 px-4 py-2 sm:px-6 lg:px-8">
        <h2 className="mr-2 text-sm font-semibold">Customer portal</h2>
        <div className="ml-auto">
          {allowable > 0 && (
            <QuietTextAction disabled={allowAll.isPending} onClick={() => allowAll.mutate()}>
              {allowAll.isPending ? 'Allowing…' : allowable === 1 ? 'Allow 1 contact' : `Allow ${allowable} contacts`}
            </QuietTextAction>
          )}
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <section aria-labelledby="company-portal-requests" className="border-b border-border/60 px-4 py-4 sm:px-6 lg:px-8">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
            <h3 id="company-portal-requests" className="text-sm font-medium text-quiet-text-primary">Company requests</h3>
            <Select
              size="sm"
              value={companyChoice}
              disabled={setRole.isPending}
              onValueChange={(next) => setRole.mutate(next === 'off' ? null : (next as CustomerPortalRole))}
            >
              <SelectTrigger aria-label="Company requests in the portal" className="h-7 w-48 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {companyChoices.map((choice) => (
                  <SelectItem key={choice.value} value={choice.value}>{choice.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <p className="mt-2 max-w-[680px] text-xs leading-5 text-muted-foreground">{companyScopeSummary(data)}</p>
          {!data.portal_enabled && (
            <p role="note" className="mt-2 max-w-[680px] text-xs leading-5 text-quiet-accent">
              The customer portal is off for this workspace, so nobody can sign in yet.{' '}
              <Link to="/w/$slug/settings/customer-portal" params={{ slug: workspaceSlug }} className="underline underline-offset-2 hover:text-quiet-accent-hover">
                Open portal settings
              </Link>
            </p>
          )}
        </section>
        {data.contacts.length === 0 ? (
          <QuietEmptyState
            className="border-t-0"
            title="No contacts at this company"
            description="Associate contacts with this company to let them sign in to the customer portal."
          />
        ) : (
          <div role="table" aria-label="Company contacts in the customer portal">
            <div role="row" className={`hidden border-b border-border/60 px-4 py-2 text-[11.5px] font-medium text-muted-foreground sm:px-6 lg:px-8 ${rowGrid}`}>
              <span role="columnheader">Contact</span>
              <span role="columnheader">Sign-in</span>
              <span role="columnheader">Role</span>
              <span role="columnheader">Last signed in</span>
            </div>
            {data.contacts.map((contact) => (
              <CompanyPortalContactRow key={contact.id} workspaceId={workspaceId} contact={contact} companyRole={data.default_role} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function CompanyPortalContactRow({ workspaceId, contact, companyRole }: {
  workspaceId: string;
  contact: CustomerPortalCompanyContact;
  companyRole: CustomerPortalRole | null;
}) {
  const setAccess = useSetContactPortalAccess(workspaceId, contact.id);
  const setRole = useSetContactPortalRole(workspaceId, contact.id);
  const name = contactName(contact);
  const access: AccessChoice = contact.portal_access ?? 'unset';
  const role: PortalRoleChoice = contact.portal_role ?? 'default';
  const roleLabel = (choice: PortalRoleChoice) =>
    choice === 'default' && companyRole ? `Company default (${portalRoleLabels[companyRole]})` : portalRoleChoices.find((item) => item.value === choice)?.label;

  return (
    <div role="row" className={`flex flex-col gap-2 border-b border-border/50 px-4 py-3 sm:px-6 lg:px-8 ${rowGrid}`}>
      <span role="cell" className="min-w-0">
        <span className="block truncate text-sm font-medium">{name}</span>
        <span className="block truncate text-xs text-muted-foreground">{contact.email || 'No email, so this contact cannot sign in'}</span>
        {contact.shared_email_contacts > 0 && (
          <span role="note" className="mt-0.5 block text-[11px] leading-4 text-quiet-accent">
            {contact.shared_email_contacts === 1 ? '1 other contact shares' : `${contact.shared_email_contacts} other contacts share`} this email. Only one of them can be allowed.
          </span>
        )}
      </span>
      <span role="cell">
        <Select
          size="sm"
          value={access}
          disabled={setAccess.isPending}
          onValueChange={(next) => setAccess.mutate(next === 'unset' ? null : (next as CustomerPortalContactAccessValue))}
        >
          <SelectTrigger aria-label={`Portal sign-in for ${name}`} className="h-7 w-32 text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {accessChoices.map((choice) => (
              <SelectItem key={choice.value} value={choice.value}>{choice.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </span>
      <span role="cell">
        <Select
          size="sm"
          value={role}
          disabled={setRole.isPending || access !== 'allowed'}
          onValueChange={(next) => setRole.mutate(next === 'default' ? null : (next as CustomerPortalRole))}
        >
          <SelectTrigger aria-label={`Portal role for ${name}`} className="h-7 w-40 text-xs">
            <SelectValue>{roleLabel(role)}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            {portalRoleChoices.map((choice) => (
              <SelectItem key={choice.value} value={choice.value}>{roleLabel(choice.value)}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </span>
      <span role="cell" className="text-xs text-muted-foreground">
        {contact.last_signed_in_at ? formatDistanceToNow(new Date(contact.last_signed_in_at), { addSuffix: true }) : 'Never'}
      </span>
    </div>
  );
}
