import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/lists/')({
  component: ListsRoute,
});

const ListsPage = lazyRouteComponent(() => import('@/pages/crm/Lists'), 'ListsPage');

function ListsRoute() {
  return (
    <div className="h-full overflow-hidden">
      <ListsPage />
    </div>
  );
}
