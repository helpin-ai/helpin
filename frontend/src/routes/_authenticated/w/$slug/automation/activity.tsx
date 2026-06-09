import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { AutomationActivityPage } from '@/pages/automation/AutomationActivity';

type AutomationActivitySearch = {
  page: number;
  execution_id?: string;
  agent_id?: string;
  binding_id?: string;
  trigger_type?: string;
  status?: string;
  source?: string;
  reference_id?: string;
  run_id?: string;
  fired_after?: string;
  fired_before?: string;
};

export const Route = createFileRoute('/_authenticated/w/$slug/automation/activity')({
  component: AutomationActivityRoute,
  validateSearch: (search: Record<string, unknown>): AutomationActivitySearch => ({
    page: typeof search.page === 'number'
      ? Math.max(1, Math.floor(search.page))
      : typeof search.page === 'string' && Number.parseInt(search.page, 10) > 0
        ? Number.parseInt(search.page, 10)
        : 1,
    execution_id: typeof search.execution_id === 'string' ? search.execution_id : undefined,
    agent_id: typeof search.agent_id === 'string' ? search.agent_id : undefined,
    binding_id: typeof search.binding_id === 'string' ? search.binding_id : undefined,
    trigger_type: typeof search.trigger_type === 'string' ? search.trigger_type : undefined,
    status: typeof search.status === 'string' ? search.status : undefined,
    source: typeof search.source === 'string' ? search.source : undefined,
    reference_id: typeof search.reference_id === 'string' ? search.reference_id : undefined,
    run_id: typeof search.run_id === 'string' ? search.run_id : undefined,
    fired_after: typeof search.fired_after === 'string' ? search.fired_after : undefined,
    fired_before: typeof search.fired_before === 'string' ? search.fired_before : undefined,
  }),
});

function AutomationActivityRoute() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });

  const handleSearchChange = (updates: Partial<AutomationActivitySearch>) => {
    navigate({
      search: {
        ...search,
        ...updates,
      },
      replace: true,
    });
  };

  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <AutomationActivityPage search={search} onSearchChange={handleSearchChange} />
    </div>
  );
}
