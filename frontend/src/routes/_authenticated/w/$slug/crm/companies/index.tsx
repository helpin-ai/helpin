import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/companies/')({
  component: CompaniesRoute,
});

const CompaniesPage = lazyRouteComponent(() => import('@/pages/crm/Companies'), 'CompaniesPage');

function CompaniesRoute() {
  return (
    <div className="h-full overflow-hidden">
      <CompaniesPage />
    </div>
  );
}
