import { TeamInboxesTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function TeamInboxesSettingsPage() {
  return (
    <SettingsPageFrame section="team-inboxes">
      {({ workspaceId }) => <TeamInboxesTab workspaceId={workspaceId} />}
    </SettingsPageFrame>
  );
}
