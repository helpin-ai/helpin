import { createFileRoute } from '@tanstack/react-router';
import { SequenceDetailPage } from '@/pages/crm/SequenceDetail';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/sequences/$sequenceId')({
  component: SequenceDetailRoute,
});

function SequenceDetailRoute() {
  const { sequenceId } = Route.useParams();
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <SequenceDetailPage sequenceId={sequenceId} />
    </div>
  );
}
