import { createFileRoute } from '@tanstack/react-router';
import { KnowledgeSettingsPage } from '@/pages/settings/KnowledgeSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/knowledge')({
  component: KnowledgeSettingsRoute,
});

function KnowledgeSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <KnowledgeSettingsPage />
    </SettingsRouteViewport>
  );
}
