import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/sequences/$sequenceId')({
  component: SequenceDetailRoute,
});

const SequenceDetailPage = lazyRouteComponent(() => import('@/pages/crm/SequenceDetail'), 'SequenceDetailPage');

function SequenceDetailRoute() {
  const { sequenceId } = Route.useParams();
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <SequenceDetailPage sequenceId={sequenceId} />
    </div>
  );
}
