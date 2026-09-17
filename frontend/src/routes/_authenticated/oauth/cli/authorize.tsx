import { createFileRoute } from '@tanstack/react-router';
import { CLIAuthorizePage } from '@/pages/oauth/CLIAuthorizePage';
import { parseCLIAuthorizationQuery } from '@/lib/cliTypes';

export const Route = createFileRoute('/_authenticated/oauth/cli/authorize')({
  validateSearch: parseCLIAuthorizationQuery,
  component: AuthorizeRoute,
});
function AuthorizeRoute() {
  return <CLIAuthorizePage query={Route.useSearch()} />;
}
