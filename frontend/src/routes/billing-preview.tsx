import { createFileRoute, notFound } from '@tanstack/react-router';
import { BillingPreviewRoute } from '@edition';
import { billingEnabled } from '@edition/config';

export const Route = createFileRoute('/billing-preview')({
 beforeLoad: () => { if (!billingEnabled) throw notFound(); },
 component: BillingPreviewRoute,
});
