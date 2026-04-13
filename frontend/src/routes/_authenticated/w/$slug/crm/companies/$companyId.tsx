import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/companies/$companyId')({
  component: CompanyDetailRoute,
});

const CompanyDetailPage = lazyRouteComponent(() => import('@/pages/crm/CompanyDetail'), 'CompanyDetailPage');

function CompanyDetailRoute() {
  const { companyId } = Route.useParams();
  return (
    <div className="h-full overflow-hidden">
      <CompanyDetailPage companyId={companyId} />
    </div>
  );
}
