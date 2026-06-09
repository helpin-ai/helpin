import type { AutomationTriggerExecutionSearchPreset, WorkflowRuleSearchPreset } from './types';

export type AutomationActivitySearch = Partial<AutomationTriggerExecutionSearchPreset> & {
  page?: number;
  execution_id?: string;
  run_id?: string;
  fired_after?: string;
  fired_before?: string;
};

export type AutomationFlowsSearch = {
  workflow?: string;
  team?: string;
  template?: string;
  template_title?: string;
  template_description?: string;
  show_trigger?: string;
  show_trigger_title?: string;
  show_rule?: string;
  show_rule_title?: string;
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

function buildPathWithSearch(base: string, search?: Record<string, string | number | boolean | undefined>, hash?: string) {
  const params = new URLSearchParams();
  if (search) {
    for (const [key, value] of Object.entries(search)) {
      if (value === undefined || value === '') continue;
      if (key === 'page' && Number(value) <= 1) continue;
      if (typeof value === 'boolean') {
        if (value) params.set(key, '1');
        continue;
      }
      params.set(key, String(value));
    }
  }
  const query = params.toString();
  return `${base}${query ? `?${query}` : ''}${hash ? `#${hash}` : ''}`;
}

export function buildAutomationFlowsPath(slug: string | undefined, search?: AutomationFlowsSearch | WorkflowRuleSearchPreset) {
  const base = slug ? `/w/${slug}/automation/flows` : '/automation/flows';
  return buildPathWithSearch(base, search as Record<string, string | number | boolean | undefined> | undefined);
}

export function buildAutomationActivityPath(slug: string | undefined, search?: AutomationActivitySearch, hash?: string) {
  const base = slug ? `/w/${slug}/automation/activity` : '/automation/activity';
  return buildPathWithSearch(base, search as Record<string, string | number | boolean | undefined> | undefined, hash);
}

export function buildAutomationLibraryPath(slug: string | undefined, hash?: string) {
  const base = slug ? `/w/${slug}/automation/library` : '/automation/library';
  return buildPathWithSearch(base, undefined, hash);
}

export function buildAutomationToolsPath(slug: string | undefined, hash?: string) {
  const base = slug ? `/w/${slug}/automation/tools` : '/automation/tools';
  return buildPathWithSearch(base, undefined, hash);
}
