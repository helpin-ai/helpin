import { createFileRoute } from '@tanstack/react-router';
import { AutomationLibraryPage } from '@/pages/automation/AutomationLibrary';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/triggers')({
  component: AutomationTriggersRoute,
});

function AutomationTriggersRoute() {
  return (
    <div className="h-full overflow-auto p-4 pb-20 md:p-6 md:pb-24">
      <AutomationLibraryPage />
    </div>
  );
}
