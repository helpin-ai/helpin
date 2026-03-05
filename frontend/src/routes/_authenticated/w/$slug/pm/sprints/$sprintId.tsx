import { createFileRoute } from '@tanstack/react-router';
import { SprintDetailPage } from '@/pages/pm/SprintDetail';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/sprints/$sprintId')({
  component: () => (
    <div className="h-full overflow-hidden">
      <SprintDetailPage />
    </div>
  ),
});
