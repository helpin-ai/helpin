import { createFileRoute } from '@tanstack/react-router';
import { SupportPage } from '@/pages/pm/Support';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/support')({
  component: SupportRoute,
});

function SupportRoute() {
  return (
    <div className="h-full overflow-hidden">
      <SupportPage />
    </div>
  );
}
