import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { useContactPortalAccess, useSetContactPortalAccess, useSetContactPortalRole } from '@/hooks/queries/useSupport';
import type { CustomerPortalContactAccessValue, CustomerPortalRole } from '@/lib/pmTypes';
import { portalRoleChoices, type PortalRoleChoice } from './companyPortal';

type PortalAccessChoice = CustomerPortalContactAccessValue | 'unset';

const choices: { value: PortalAccessChoice; label: string }[] = [
  { value: 'unset', label: 'Not set' },
  { value: 'allowed', label: 'Allowed' },
  { value: 'blocked', label: 'Blocked' },
];

function choiceLabel(value: PortalAccessChoice) {
  return choices.find((choice) => choice.value === value)?.label ?? 'Not set';
}

/**
 * ContactPortalAccessControl shows a contact's customer portal decision.
 * Support admins can change it and the contact's company role; everyone else
 * sees the decision read-only, because portal access is managed outside
 * generic contact editing.
 */
export function ContactPortalAccessControl({ workspaceId, contactId, currentAccess, canManage }: {
  workspaceId: string;
  contactId: string;
  currentAccess?: CustomerPortalContactAccessValue | null;
  canManage: boolean;
}) {
  const access = useContactPortalAccess(workspaceId, contactId, canManage);
  const update = useSetContactPortalAccess(workspaceId, contactId);
  const updateRole = useSetContactPortalRole(workspaceId, contactId);

  if (!canManage) {
    return <span className="block truncate text-xs text-foreground">{choiceLabel(currentAccess ?? 'unset')}</span>;
  }

  const value: PortalAccessChoice = access.data?.portal_access ?? currentAccess ?? 'unset';
  const role: PortalRoleChoice = access.data?.portal_role ?? 'default';
  const shared = access.data?.shared_email_contacts ?? 0;

  return (
    <div className="flex min-w-0 flex-col gap-1">
      <Select
        size="sm"
        value={value}
        disabled={access.isLoading || update.isPending}
        onValueChange={(next) => update.mutate(next === 'unset' ? null : (next as CustomerPortalContactAccessValue))}
      >
        <SelectTrigger aria-label="Customer portal access" className="h-6 w-full max-w-40 text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {choices.map((choice) => (
            <SelectItem key={choice.value} value={choice.value}>{choice.label}</SelectItem>
          ))}
        </SelectContent>
      </Select>
      {value === 'allowed' && (
        <Select
          size="sm"
          value={role}
          disabled={access.isLoading || updateRole.isPending}
          onValueChange={(next) => updateRole.mutate(next === 'default' ? null : (next as CustomerPortalRole))}
        >
          <SelectTrigger aria-label="Customer portal role" className="h-6 w-full max-w-40 text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {portalRoleChoices.map((choice) => (
              <SelectItem key={choice.value} value={choice.value}>{choice.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
      {shared > 0 && (
        <p role="note" className="text-[11px] leading-4 text-muted-foreground">
          {shared === 1 ? '1 other contact shares' : `${shared} other contacts share`} this email. The portal only works when exactly one of them is allowed and none is blocked.
        </p>
      )}
    </div>
  );
}
