import { createFileRoute } from '@tanstack/react-router';
import { SequencesPage } from '@/pages/crm/Sequences';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/sequences/')({
  component: SequencesRoute,
});

function SequencesRoute() {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <SequencesPage />
    </div>
  );
}
