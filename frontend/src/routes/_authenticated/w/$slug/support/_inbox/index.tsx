import { createFileRoute } from '@tanstack/react-router';
import { normalizeSupportInboxRouteSearch, type SupportInboxRouteSearch } from '@/lib/supportInboxRouting';

export const Route = createFileRoute('/_authenticated/w/$slug/support/_inbox/')({
  validateSearch: (search: Record<string, unknown>): SupportInboxRouteSearch =>
    normalizeSupportInboxRouteSearch(search),
});
