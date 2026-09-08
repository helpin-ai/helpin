import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';
import { parseSignalsSearch } from '@/lib/crmSignalInboxQueryBuilder';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/insights')({
  validateSearch: parseSignalsSearch,
  component: InsightsRoute,
});

const InsightsPage = lazyRouteComponent(() => import('@/pages/crm/Insights'), 'InsightsPage');
const SignalsPage = lazyRouteComponent(() => import('@/pages/crm/Signals'), 'SignalsPage');

function InsightsRoute() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <div className="h-full overflow-hidden">
      {search.view === 'evidence' ? <InsightsPage /> : <SignalsPage search={search} onChange={(next, replace) => void navigate({ search: next, replace, resetScroll: false })} />}
    </div>
  );
}
