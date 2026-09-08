import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

const PlaybookDetail = lazyRouteComponent(() => import('@/pages/crm/PlaybookDetail'), 'PlaybookDetailPage');
export const Route = createFileRoute('/_authenticated/w/$slug/crm/playbooks/$playbookId')({ component: Detail });
function Detail() {
  const { playbookId } = Route.useParams();
  return <PlaybookDetail key={playbookId} playbookId={playbookId} />;
}
