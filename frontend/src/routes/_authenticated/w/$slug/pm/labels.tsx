import { createFileRoute } from '@tanstack/react-router';
import { LabelsPage } from '@/pages/pm/Labels';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/labels')({
  component: LabelsRoute,
});

function LabelsRoute() {
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <LabelsPage />
    </div>
  );
}
