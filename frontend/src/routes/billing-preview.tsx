import { createFileRoute } from '@tanstack/react-router';
import { BillingPreviewRoute } from '@edition';

export const Route = createFileRoute('/billing-preview')({
 component: BillingPreviewRoute,
});
