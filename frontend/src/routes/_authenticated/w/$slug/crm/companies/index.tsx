import { createFileRoute } from '@tanstack/react-router';
import { CompaniesPage } from '@/pages/crm/Companies';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/companies/')({
  component: CompaniesRoute,
});

function CompaniesRoute() {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <CompaniesPage />
    </div>
  );
}
