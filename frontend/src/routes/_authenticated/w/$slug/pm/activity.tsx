import { createFileRoute, Navigate } from '@tanstack/react-router';

type AutomationActivitySearch = {
  page: number;
  agent_id?: string;
  binding_id?: string;
  trigger_type?: string;
  status?: string;
  source?: string;
  reference_id?: string;
  fired_after?: string;
  fired_before?: string;
};

export const Route = createFileRoute('/_authenticated/w/$slug/pm/activity')({
  component: LegacyAutomationActivityRoute,
  validateSearch: (search: Record<string, unknown>): AutomationActivitySearch => ({
    page: typeof search.page === 'number'
      ? Math.max(1, Math.floor(search.page))
      : typeof search.page === 'string' && Number.parseInt(search.page, 10) > 0
        ? Number.parseInt(search.page, 10)
        : 1,
    agent_id: typeof search.agent_id === 'string' ? search.agent_id : undefined,
    binding_id: typeof search.binding_id === 'string' ? search.binding_id : undefined,
    trigger_type: typeof search.trigger_type === 'string' ? search.trigger_type : undefined,
    status: typeof search.status === 'string' ? search.status : undefined,
    source: typeof search.source === 'string' ? search.source : undefined,
    reference_id: typeof search.reference_id === 'string' ? search.reference_id : undefined,
    fired_after: typeof search.fired_after === 'string' ? search.fired_after : undefined,
    fired_before: typeof search.fired_before === 'string' ? search.fired_before : undefined,
  }),
});

function LegacyAutomationActivityRoute() {
  const { slug } = Route.useParams();
  const search = Route.useSearch();
  return <Navigate to="/w/$slug/automation/activity" params={{ slug }} search={search} replace />;
}
