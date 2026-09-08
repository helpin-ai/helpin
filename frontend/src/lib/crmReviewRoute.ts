import { redirect } from '@tanstack/react-router';
import { reviewSignalsSearch, type SignalsSearch } from './crmSignalInboxQueryBuilder';

// Shared by the real route and browser harness so the compatibility redirect is tested.
export const crmReviewRouteOptions = {
  validateSearch: reviewSignalsSearch,
  beforeLoad: ({ params, search }: { params: { slug: string }; search: SignalsSearch }) => {
    throw redirect({ to: '/w/$slug/crm/insights', params: { slug: params.slug }, search, replace: true });
  },
};
