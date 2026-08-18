import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/meetings/')({
  component: MeetingsRoute,
});

const MeetingsPage = lazyRouteComponent(() => import('@/pages/crm/Meetings'), 'MeetingsPage');

function MeetingsRoute() {
  return <div className="h-full overflow-hidden"><MeetingsPage /></div>;
}
