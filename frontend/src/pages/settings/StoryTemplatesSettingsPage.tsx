import { StoryTemplatesSettings } from '@/components/pm/StoryTemplatesSettings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function StoryTemplatesSettingsPage({ initialTeamId }: { initialTeamId?: string }) {
  return (
    <SettingsPageFrame section="story-templates">
      {({ workspaceId }) => (
        <StoryTemplatesSettings workspaceId={workspaceId} initialTeamId={initialTeamId} />
      )}
    </SettingsPageFrame>
  );
}
