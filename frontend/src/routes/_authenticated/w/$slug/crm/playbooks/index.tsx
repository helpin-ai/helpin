import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/playbooks/')({
  component: lazyRouteComponent(() => import('@/pages/crm/Playbooks'), 'PlaybooksPage'),
});
