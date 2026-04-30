import { createFileRoute, Navigate } from '@tanstack/react-router';

type AutomationFlowsSearch = {
  workflow?: string;
  team?: string;
  template?: string;
  template_title?: string;
  template_description?: string;
  show_trigger?: string;
  show_trigger_title?: string;
  show_rule?: string;
  show_rule_title?: string;
  create_event_rule?: boolean;
  trigger_type?: string;
  agent_id?: string;
  repo_full_name?: string;
  branch?: string;
  base_branch?: string;
  tag_name?: string;
  conclusion?: string;
  target_mode?: 'event' | 'task' | 'epic' | 'repository';
  target_id?: string;
};

const TARGET_MODES = new Set(['event', 'task', 'epic', 'repository']);

export const Route = createFileRoute('/_authenticated/w/$slug/pm/flows')({
  component: LegacyAutomationFlowsRoute,
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
    create_event_rule:
      search.create_event_rule === true
      || search.create_event_rule === 'true'
      || search.create_event_rule === '1',
    trigger_type: typeof search.trigger_type === 'string' ? search.trigger_type : undefined,
    agent_id: typeof search.agent_id === 'string' ? search.agent_id : undefined,
    repo_full_name: typeof search.repo_full_name === 'string' ? search.repo_full_name : undefined,
    branch: typeof search.branch === 'string' ? search.branch : undefined,
    base_branch: typeof search.base_branch === 'string' ? search.base_branch : undefined,
    tag_name: typeof search.tag_name === 'string' ? search.tag_name : undefined,
    conclusion: typeof search.conclusion === 'string' ? search.conclusion : undefined,
    target_mode:
      typeof search.target_mode === 'string' && TARGET_MODES.has(search.target_mode)
        ? search.target_mode as AutomationFlowsSearch['target_mode']
        : undefined,
    target_id: typeof search.target_id === 'string' ? search.target_id : undefined,
  }),
});

function LegacyAutomationFlowsRoute() {
  const { slug } = Route.useParams();
  const search = Route.useSearch();
  return <Navigate to="/w/$slug/automation/flows" params={{ slug }} search={search} replace />;
}
