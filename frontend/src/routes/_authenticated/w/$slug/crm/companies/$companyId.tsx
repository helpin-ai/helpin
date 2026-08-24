import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';
import { normalizeCompanyDetailTab, type CompanyDetailTab } from '@/lib/companyDetailTabs';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/companies/$companyId')({
  component: CompanyDetailRoute,
  validateSearch: (search: Record<string, unknown>): { tab?: CompanyDetailTab; thread?: string } => ({
    tab: typeof search.tab === 'string' ? normalizeCompanyDetailTab(search.tab) : undefined,
    thread: typeof search.thread === 'string' && search.thread.trim() ? search.thread : undefined,
  }),
});

const CompanyDetailPage = lazyRouteComponent(() => import('@/pages/crm/CompanyDetail'), 'CompanyDetailPage');

function CompanyDetailRoute() {
  const { companyId } = Route.useParams();
  const { tab, thread } = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <div className="h-full overflow-hidden">
      <CompanyDetailPage
        companyId={companyId}
        activeTab={normalizeCompanyDetailTab(tab)}
        emailThreadId={thread}
        onTabChange={(nextTab) => void navigate({ search: { tab: normalizeCompanyDetailTab(nextTab), thread: nextTab === 'emails' ? thread : undefined } })}
        onEmailThreadChange={(nextThread) => void navigate({ search: { tab: 'emails', thread: nextThread } })}
      />
    </div>
  );
}
