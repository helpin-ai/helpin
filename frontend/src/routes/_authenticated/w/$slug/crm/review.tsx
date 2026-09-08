import { createFileRoute } from '@tanstack/react-router';
import { crmReviewRouteOptions } from '@/lib/crmReviewRoute';

export const Route = createFileRoute('/_authenticated/w/$slug/crm/review')(crmReviewRouteOptions);
