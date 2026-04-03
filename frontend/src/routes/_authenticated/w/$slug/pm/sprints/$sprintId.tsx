import { createFileRoute } from '@tanstack/react-router';
import { SprintDetailPage } from '@/pages/pm/SprintDetail';

type SprintDetailSearch = { task?: string };

export const Route = createFileRoute('/_authenticated/w/$slug/pm/sprints/$sprintId')({
  validateSearch: (search: Record<string, unknown>): SprintDetailSearch => ({
    task: search.task != null ? String(search.task) : undefined,
  }),
  component: () => (
    <div className="h-full overflow-hidden">
      <SprintDetailPage />
    </div>
  ),
});
