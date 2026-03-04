import { createFileRoute } from '@tanstack/react-router';
import { EpicsPage } from '@/pages/pm/Epics';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/epics/')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <EpicsPage />
    </div>
  ),
});
