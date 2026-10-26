import type { CustomerPortalCompanyAccess, CustomerPortalCompanyContact, CustomerPortalRole } from '@/lib/pmTypes';

/** A contact's portal role choice; `default` follows its companies. */
export type PortalRoleChoice = CustomerPortalRole | 'default';

export const portalRoleChoices: { value: PortalRoleChoice; label: string }[] = [
  { value: 'default', label: 'Company default' },
  { value: 'requester', label: 'Requester' },
  { value: 'member', label: 'Member' },
];

export const portalRoleLabels: Record<CustomerPortalRole, string> = {
  requester: 'Requester',
  member: 'Member',
};

/** Contacts the bulk action would allow, matching the server rule. */
export function allowableContacts(contacts: CustomerPortalCompanyContact[]) {
  return contacts.filter((contact) => !contact.portal_access && contact.email?.trim() && contact.shared_email_contacts === 0);
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
