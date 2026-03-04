import { createFileRoute } from '@tanstack/react-router';
import { ObjectiveDetailPage } from '@/pages/pm/ObjectiveDetail';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/objectives/$objectiveId')({
  component: () => (
    <div className="h-full overflow-hidden">
      <ObjectiveDetailPage />
    </div>
  ),
});
