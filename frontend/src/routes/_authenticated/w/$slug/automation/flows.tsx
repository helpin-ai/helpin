import { useEffect } from 'react';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { AutomationRouteViewport } from '@/components/automation/AutomationRouteViewport';
import { AutomationFlowsPage, type AutomationFlowsSearch } from '@/pages/automation/AutomationFlows';
import { FLOW_TARGET_MODES } from '@/lib/automationUi';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/flows')({
  component: AutomationFlowsRoute,
  validateSearch: (search: Record<string, unknown>): AutomationFlowsSearch => ({
    workflow: typeof search.workflow === 'string' ? search.workflow : undefined,
    team: typeof search.team === 'string' ? search.team : undefined,
    template: typeof search.template === 'string' ? search.template : undefined,
    template_title: typeof search.template_title === 'string' ? search.template_title : undefined,
    template_description: typeof search.template_description === 'string' ? search.template_description : undefined,
    show_trigger: typeof search.show_trigger === 'string' ? search.show_trigger : undefined,
    show_trigger_title: typeof search.show_trigger_title === 'string' ? search.show_trigger_title : undefined,
    show_rule: typeof search.show_rule === 'string' ? search.show_rule : undefined,
    show_rule_title: typeof search.show_rule_title === 'string' ? search.show_rule_title : undefined,
    trigger_type: typeof search.trigger_type === 'string' ? search.trigger_type : undefined,
    agent_id: typeof search.agent_id === 'string' ? search.agent_id : undefined,
    repo_full_name: typeof search.repo_full_name === 'string' ? search.repo_full_name : undefined,
    branch: typeof search.branch === 'string' ? search.branch : undefined,
    base_branch: typeof search.base_branch === 'string' ? search.base_branch : undefined,
    tag_name: typeof search.tag_name === 'string' ? search.tag_name : undefined,
    conclusion: typeof search.conclusion === 'string' ? search.conclusion : undefined,
    target_mode:
      typeof search.target_mode === 'string' && FLOW_TARGET_MODES.has(search.target_mode)
        ? search.target_mode as AutomationFlowsSearch['target_mode']
        : undefined,
    target_id: typeof search.target_id === 'string' ? search.target_id : undefined,
  }),
});

function AutomationFlowsRoute() {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });

  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (!new URLSearchParams(window.location.search).has('create_event_rule')) return;
    navigate({ search, replace: true });
  }, [navigate, search]);

  const handleSearchChange = (updates: Partial<AutomationFlowsSearch>) => {
    const nextSearch: AutomationFlowsSearch = {
      ...search,
      ...updates,
    };
    navigate({
      search: nextSearch,
      replace: true,
    });
  };

  return (
    <AutomationRouteViewport>
      <AutomationFlowsPage search={search} onSearchChange={handleSearchChange} />
    </AutomationRouteViewport>
  );
}
