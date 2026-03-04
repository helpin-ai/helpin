import { createFileRoute } from '@tanstack/react-router';
import { EpicDetailPage } from '@/pages/pm/EpicDetail';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/epics/$epicId')({
  component: () => (
    <div className="h-full overflow-hidden">
      <EpicDetailPage />
    </div>
  ),
});
