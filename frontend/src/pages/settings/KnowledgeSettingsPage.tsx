import { KnowledgeTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function KnowledgeSettingsPage() {
  return (
    <SettingsPageFrame section="knowledge">
      {({ workspaceId }) => <KnowledgeTab workspaceId={workspaceId} />}
    </SettingsPageFrame>
  );
}
