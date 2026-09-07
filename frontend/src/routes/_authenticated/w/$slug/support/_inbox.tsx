import { createFileRoute } from '@tanstack/react-router';
import { SupportPage } from '@/pages/pm/Support';

export const Route = createFileRoute('/_authenticated/w/$slug/support/_inbox')({
  component: SupportInboxRoute,
});

function SupportInboxRoute() {
  // Keep the inbox mounted when auto-selection or navigation changes the
  // conversation URL. The child routes only validate their search parameters.
  return (
    <div className="h-full overflow-hidden">
      <SupportPage />
    </div>
  );
}
