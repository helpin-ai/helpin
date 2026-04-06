import { createFileRoute } from '@tanstack/react-router';
import { ObjectiveDetailPage } from '@/pages/pm/ObjectiveDetail';

type ObjectiveDetailSearch = { task?: string };

export const Route = createFileRoute('/_authenticated/w/$slug/pm/objectives/$objectiveId')({
  validateSearch: (search: Record<string, unknown>): ObjectiveDetailSearch => ({
    task: search.task != null ? String(search.task) : undefined,
  }),
  component: () => (
    <div className="h-full overflow-hidden">
      <ObjectiveDetailPage />
    </div>
  ),
});
