import { createFileRoute } from '@tanstack/react-router';
import { ExternalMCPCallbackPage } from '@/pages/oauth/ExternalMCPCallbackPage';

export const Route = createFileRoute('/_authenticated/oauth/external-mcp/callback')({
  validateSearch: (search: Record<string, unknown>) => ({
    state: typeof search.state === 'string' ? search.state : '',
    code: typeof search.code === 'string' ? search.code : '',
    error: typeof search.error === 'string' ? search.error : '',
  }),
  component: CallbackRoute,
});

function CallbackRoute() {
  const query = Route.useSearch();
  return <ExternalMCPCallbackPage key={`${query.state}:${query.code}:${query.error}`} query={query} />;
}
