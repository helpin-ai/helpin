import { createFileRoute } from '@tanstack/react-router';
import { MCPAuthorizePage } from '@/pages/oauth/MCPAuthorizePage';
import type { MCPAuthorizationQuery } from '@/lib/mcpTypes';

export const Route = createFileRoute('/_authenticated/oauth/authorize')({
  validateSearch: (search: Record<string, unknown>): MCPAuthorizationQuery => ({
    client_id: typeof search.client_id === 'string' ? search.client_id : '',
    redirect_uri: typeof search.redirect_uri === 'string' ? search.redirect_uri : '',
    response_type: typeof search.response_type === 'string' ? search.response_type : '',
    scope: typeof search.scope === 'string' ? search.scope : '',
    state: typeof search.state === 'string' ? search.state : '',
    code_challenge: typeof search.code_challenge === 'string' ? search.code_challenge : '',
    code_challenge_method: typeof search.code_challenge_method === 'string' ? search.code_challenge_method : '',
  }),
  component: AuthorizeRoute,
});

function AuthorizeRoute() {
  return <MCPAuthorizePage query={Route.useSearch()} />;
}
