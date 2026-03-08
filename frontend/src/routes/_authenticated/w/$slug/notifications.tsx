import { createFileRoute } from '@tanstack/react-router';
import { NotificationsPage } from '@/pages/notifications/NotificationsPage';

export const Route = createFileRoute('/_authenticated/w/$slug/notifications')({
  component: NotificationsRoute,
});

function NotificationsRoute() {
  return (
    <div className="h-full overflow-hidden">
      <NotificationsPage />
    </div>
  );
}
