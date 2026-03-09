import { createFileRoute } from '@tanstack/react-router';
import { ListsPage } from '@/pages/crm/Lists';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/lists/')({
  component: ListsRoute,
});

function ListsRoute() {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <ListsPage />
    </div>
  );
}
