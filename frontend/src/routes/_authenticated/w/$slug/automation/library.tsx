import { createFileRoute } from '@tanstack/react-router';
import { AutomationLibraryPage } from '@/pages/automation/AutomationLibrary';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/library')({
  component: AutomationLibraryRoute,
});

function AutomationLibraryRoute() {
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <AutomationLibraryPage />
    </div>
  );
}
