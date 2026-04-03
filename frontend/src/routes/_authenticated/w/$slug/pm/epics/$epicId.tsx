import { createFileRoute } from '@tanstack/react-router';
import { EpicDetailPage } from '@/pages/pm/EpicDetail';

type EpicDetailSearch = { task?: string };

export const Route = createFileRoute('/_authenticated/w/$slug/pm/epics/$epicId')({
  validateSearch: (search: Record<string, unknown>): EpicDetailSearch => ({
    task: search.task != null ? String(search.task) : undefined,
  }),
  component: () => (
    <div className="h-full overflow-hidden">
      <EpicDetailPage />
    </div>
  ),
});
