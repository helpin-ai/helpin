import { createFileRoute } from '@tanstack/react-router';
import { SupportTranslationSettingsPage } from '@/pages/settings/SupportTranslationSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/support-translation')({
  component: SupportTranslationSettingsRoute,
});

function SupportTranslationSettingsRoute() {
  return (
    <SettingsRouteViewport>
      <SupportTranslationSettingsPage />
    </SettingsRouteViewport>
  );
}
