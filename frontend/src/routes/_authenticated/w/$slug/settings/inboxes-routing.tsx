import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { InboxesRoutingSettingsPage, type InboxesRoutingTab } from '@/pages/settings/InboxesRoutingSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

const VALID_TABS: InboxesRoutingTab[] = ['inboxes', 'email', 'senders'];

type InboxesRoutingSearch = {
  tab: InboxesRoutingTab;
};

export const Route = createFileRoute('/_authenticated/w/$slug/settings/inboxes-routing')({
  component: InboxesRoutingRoute,
  validateSearch: (search: Record<string, unknown>): InboxesRoutingSearch => ({
    tab: VALID_TABS.includes(search.tab as InboxesRoutingTab)
      ? (search.tab as InboxesRoutingTab)
      : 'inboxes',
  }),
});

function InboxesRoutingRoute() {
  const { tab } = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });

  const handleTabChange = (value: string) => {
    navigate({
      search: { tab: value as InboxesRoutingTab },
      replace: true,
    });
  };

  return (
    <SettingsRouteViewport>
      <InboxesRoutingSettingsPage tab={tab} onTabChange={handleTabChange} />
    </SettingsRouteViewport>
  );
}
