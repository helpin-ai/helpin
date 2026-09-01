import { createFileRoute } from '@tanstack/react-router';
import { RoadmapPage } from '@/pages/pm/Roadmap';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/roadmap')({
  component: () => (
    <div className="h-full overflow-hidden">
      <RoadmapPage />
    </div>
  ),
});
