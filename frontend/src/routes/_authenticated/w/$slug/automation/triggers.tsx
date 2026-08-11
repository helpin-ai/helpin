import { createFileRoute } from '@tanstack/react-router';
import { AutomationRouteViewport } from '@/components/automation/AutomationRouteViewport';
import { AutomationLibraryPage } from '@/pages/automation/AutomationLibrary';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/triggers')({
  component: AutomationTriggersRoute,
});

function AutomationTriggersRoute() {
  return (
    <AutomationRouteViewport>
      <AutomationLibraryPage />
    </AutomationRouteViewport>
  );
}
