import { createFileRoute } from '@tanstack/react-router';
import { ObjectivesPage } from '@/pages/pm/Objectives';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/objectives/')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <ObjectivesPage />
    </div>
  ),
});
