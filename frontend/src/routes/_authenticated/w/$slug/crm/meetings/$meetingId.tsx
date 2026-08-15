import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/meetings/$meetingId')({
  component: MeetingDetailRoute,
});

const MeetingDetailPage = lazyRouteComponent(() => import('@/pages/crm/MeetingDetail'), 'MeetingDetailPage');

function MeetingDetailRoute() {
  const { meetingId } = Route.useParams();
  return <div className="h-full overflow-hidden"><MeetingDetailPage meetingId={meetingId} /></div>;
}
