import { toast } from 'sonner';
import { ArrowDown01Icon } from '@/lib/icons';
import { QuietDropdown } from '@/components/design-system/quiet';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries';
import { useUpdateContact } from '@/hooks/queries/useCRM';
import {
  LIFECYCLE_BADGE_CLASSNAMES,
  LIFECYCLE_DOT_CLASSNAMES,
  LIFECYCLE_STAGE_OPTIONS,
  lifecycleStageLabel,
} from '@/lib/crmLifecycle';
import type { LifecycleStage } from '@/lib/crmTypes';
import { cn } from '@/lib/utils';

interface SidebarLifecyclePickerProps {
  workspaceId: string;
  contactId: string;
  stage: string;
}

const badgeClassName = 'inline-flex h-5 items-center gap-1 rounded-full px-2 text-[11px] font-medium';

/**
 * SidebarLifecyclePicker shows the CRM contact's lifecycle stage in the
 * conversation identity card. People who can edit CRM change it in place.
 */
export function SidebarLifecyclePicker({ workspaceId, contactId, stage }: SidebarLifecyclePickerProps) {
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { has } = usePermissions(access);
  const updateContact = useUpdateContact(workspaceId);
  const label = lifecycleStageLabel(stage);
  const colorClassName = LIFECYCLE_BADGE_CLASSNAMES[stage as LifecycleStage] ?? LIFECYCLE_BADGE_CLASSNAMES.subscriber;

  if (!has('crm.edit')) {
    return <span className={cn(badgeClassName, colorClassName)}>{label}</span>;
  }

  return (
    <QuietDropdown
      label="Lifecycle stage"
      searchMode="off"
      selected={[stage]}
      disabled={updateContact.isPending}
      contentProps={{ align: 'center' }}
      contentClassName="w-52"
      options={LIFECYCLE_STAGE_OPTIONS.map((option) => ({
        value: option.value,
        label: option.label,
        leading: <span className={cn('h-2 w-2 rounded-full', LIFECYCLE_DOT_CLASSNAMES[option.value])} />,
      }))}
      onSelect={(value) => {
        if (value === stage) return;
        updateContact.mutate(
          { id: contactId, lifecycle_stage: value as LifecycleStage },
          { onError: () => toast.error('Could not update the lifecycle stage') },
        );
      }}
      trigger={(
        <button
          type="button"
          aria-label={`Lifecycle stage: ${label}`}
          className={cn(badgeClassName, 'pr-1.5 transition-[filter] hover:brightness-95 focus-visible:outline-2 focus-visible:outline-ring', colorClassName)}
        >
          {label}
          <ArrowDown01Icon className="h-2.5 w-2.5 opacity-70" />
        </button>
      )}
    />
  );
}
