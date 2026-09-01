import { createFileRoute } from '@tanstack/react-router';
import { SprintsPage as Sprints } from '@/pages/pm/Sprints';

export const Route = createFileRoute('/_authenticated/w/$slug/sprints/')({
  component: () => (
    <div className="h-full overflow-hidden">
      <Sprints />
    </div>
  ),
});
