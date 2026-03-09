import { createFileRoute } from '@tanstack/react-router';
import { CompanyDetailPage } from '@/pages/crm/CompanyDetail';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/companies/$companyId')({
  component: CompanyDetailRoute,
});

function CompanyDetailRoute() {
  const { companyId } = Route.useParams();
  return (
    <div className="h-full overflow-auto pt-4 md:pt-6">
      <CompanyDetailPage companyId={companyId} />
    </div>
  );
}
