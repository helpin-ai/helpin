import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/sequences/')({
  component: SequencesRoute,
});

const SequencesPage = lazyRouteComponent(() => import('@/pages/crm/Sequences'), 'SequencesPage');

function SequencesRoute() {
  return (
    <div className="h-full overflow-hidden">
      <SequencesPage />
    </div>
  );
}
