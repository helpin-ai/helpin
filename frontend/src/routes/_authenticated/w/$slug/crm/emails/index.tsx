import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router';
export const Route = createFileRoute('/_authenticated/w/$slug/crm/emails/')({ component: lazyRouteComponent(() => import('@/pages/crm/Emails'), 'EmailsPage') });
