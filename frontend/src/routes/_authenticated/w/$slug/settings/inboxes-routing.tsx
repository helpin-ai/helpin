import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { InboxesRoutingSettingsPage, normalizeInboxesRoutingTab, type InboxesRoutingTab } from '@/pages/settings/InboxesRoutingSettingsPage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

type InboxesRoutingSearch = {
  tab: InboxesRoutingTab;
  create_inbox: boolean;
};

export const Route = createFileRoute('/_authenticated/w/$slug/settings/inboxes-routing')({
  component: InboxesRoutingRoute,
  validateSearch: (search: Record<string, unknown>): InboxesRoutingSearch => ({
    tab: normalizeInboxesRoutingTab(search.tab),
    create_inbox: search.create_inbox === true || search.create_inbox === 'true',
  }),
});

function InboxesRoutingRoute() {
  const { tab, create_inbox: createInbox } = Route.useSearch();
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
      <InboxesRoutingSettingsPage tab={tab} createInbox={createInbox} onTabChange={handleTabChange} />
    </SettingsRouteViewport>
  );
}
