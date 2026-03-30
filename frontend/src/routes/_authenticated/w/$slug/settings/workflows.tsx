import { createFileRoute, Navigate } from '@tanstack/react-router';

type WorkflowsSettingsSearch = {
  workflow?: string;
  team?: string;
};

export const Route = createFileRoute('/_authenticated/w/$slug/settings/workflows')({
  component: WorkflowsSettingsRoute,
  validateSearch: (search: Record<string, unknown>): WorkflowsSettingsSearch => ({
    workflow: typeof search.workflow === 'string' ? search.workflow : undefined,
    team: typeof search.team === 'string' ? search.team : undefined,
  }),
});

function WorkflowsSettingsRoute() {
  const { slug } = Route.useParams();
  const { team } = Route.useSearch();

  return <Navigate to="/w/$slug/settings/teams" params={{ slug }} search={{ team }} replace />;
}
