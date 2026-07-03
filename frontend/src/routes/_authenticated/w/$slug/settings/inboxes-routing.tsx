import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { InboxesRoutingSettingsPage, normalizeInboxesRoutingTab, type InboxesRoutingTab } from '@/pages/settings/InboxesRoutingSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

type InboxesRoutingSearch = {
  tab: InboxesRoutingTab;
};

export const Route = createFileRoute('/_authenticated/w/$slug/settings/inboxes-routing')({
  component: InboxesRoutingRoute,
  validateSearch: (search: Record<string, unknown>): InboxesRoutingSearch => ({
    tab: normalizeInboxesRoutingTab(search.tab),
  }),
});

function InboxesRoutingRoute() {
  const { tab } = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });

  const handleTabChange = (value: string) => {
    void navigate({
      to: Route.fullPath,
      search: { tab: normalizeInboxesRoutingTab(value) },
      replace: true,
    });
  };

  return (
    <SettingsRouteViewport>
      <InboxesRoutingSettingsPage tab={tab} onTabChange={handleTabChange} />
    </SettingsRouteViewport>
  );
}
