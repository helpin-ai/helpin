import type { CustomerPortalCompanyAccess, CustomerPortalCompanyContact, CustomerPortalCompanyRole, CustomerPortalRole } from '@/lib/pmTypes';

/** A contact's portal role choice; `default` follows its companies. */
export type PortalRoleChoice = CustomerPortalRole | 'default';

export const portalRoleChoices: { value: PortalRoleChoice; label: string }[] = [
  { value: 'default', label: 'Company default' },
  { value: 'requester', label: 'Requester' },
  { value: 'member', label: 'Member' },
];

export const portalRoleLabels: Record<CustomerPortalCompanyRole, string> = {
  requester: 'Requester',
  member: 'Member',
  admin: 'Admin',
};

/** emailOnDomain mirrors the server: the address is on the domain or a subdomain; the domain may be a URL. */
export function emailOnDomain(email: string, domain: string) {
  const host = email.trim().toLowerCase().split('@').at(-1) ?? '';
  const bare = domain.trim().toLowerCase().replace(/^https?:\/\//, '').replace(/^www\./, '').split(/[/:]/)[0];
  return Boolean(bare) && email.includes('@') && (host === bare || host.endsWith(`.${bare}`));
}

/**
 * domainAddableContacts are the contacts the bulk action would add, matching
 * the server: not yet members, not blocked, with an email of their own on the
 * company's domain. CRM links alone are not trusted, because the Messenger
 * creates them from unverified claims.
 */
export function domainAddableContacts(contacts: CustomerPortalCompanyContact[], domain?: string | null) {
  if (!domain?.trim()) return [];
  return contacts.filter((contact) =>
    !contact.member && contact.portal_access !== 'blocked' && contact.shared_email_contacts === 0 &&
    !!contact.email?.trim() && emailOnDomain(contact.email, domain));
}

/** companyScopeSummary states what the company setting does for its contacts. */
export function companyScopeSummary(access: CustomerPortalCompanyAccess) {
  const count = access.request_count === 1 ? '1 request so far' : `${access.request_count} requests so far`;
  switch (access.default_role) {
    case 'member':
      return `Allowed contacts see and reply to every request from this company (${count}). Give a contact the Requester role to keep them to their own.`;
    case 'requester':
      return `Allowed contacts see only their own requests. Give a contact the Member role to share every request from this company (${count}) with them.`;
    default:
      return `Contacts use a personal portal and see only their own requests. Sharing all company requests would show members every request from this company (${count}).`;
  }
}
