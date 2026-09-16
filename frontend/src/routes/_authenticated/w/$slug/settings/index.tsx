import { createFileRoute } from '@tanstack/react-router';
import { SettingsHomePage } from '@/pages/settings/SettingsHomePage';
import { SettingsRouteViewport } from '@/pages/settings/SettingsRouteViewport';

export const Route = createFileRoute('/_authenticated/w/$slug/settings/')({
  validateSearch: (search: Record<string, unknown>) => ({ search: search.search === '1' || search.search === 1 ? '1' : undefined }),
  component: SettingsIndex,
});
function SettingsIndex() {
  const { search } = Route.useSearch();
  return <SettingsRouteViewport><SettingsHomePage autoFocus={search === '1'} /></SettingsRouteViewport>;
}
