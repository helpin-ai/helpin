import type { Capability } from '@/lib/capabilityTypes';
import { SetupAdminHint, SetupSettingsLink } from './CapabilityActions';

export type SetupWidgetStepProps = {
  capability: Capability;
  slug: string;
  canManage: boolean;
};

/** Points to the widget install snippet; the widget is verified when it first loads on a site. */
export function SetupWidgetStep({ capability, slug, canManage }: SetupWidgetStepProps) {
  if (capability.status === 'ready' || capability.status === 'unavailable') return null;
  if (!canManage) return <SetupAdminHint />;
  const action = capability.action;
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
      <SetupSettingsLink slug={slug} path={action?.path || 'settings/chat-general'}>
        {action?.label || 'Install the chat widget'}
      </SetupSettingsLink>
      <span className="text-[12px] text-quiet-text-tertiary">Add the snippet to your site. It’s verified the first time it loads for a visitor.</span>
    </div>
  );
}
