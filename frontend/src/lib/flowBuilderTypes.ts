import type {
  Agent,
  AutomationRule,
  CreateAgentFromTemplateOverrides,
} from "@/lib/pmTypes";

export interface FlowBuilderDraft {
  summary?: string;
  schedule_timezone?: string;
  name: string;
  description?: string;
  team_id?: string;
  workflow_id?: string;
  trigger_type: string;
  trigger_config: Record<string, unknown>;
  action_type: string;
  action_config: Record<string, unknown>;
  template_inputs?: Record<string, unknown>;
  agent_name?: string;
  agent_overrides?: CreateAgentFromTemplateOverrides;
  semantic_condition?: string;
  paused?: boolean;
}

export interface FlowBuilderState {
  timezone?: string;
  next_runs?: string[];
  creates_agent?: boolean;
  labels?: Record<string, string>;
  template_key?: string;
  source_rule?: AutomationRule;
  draft?: FlowBuilderDraft;
  revision?: string;
  created_flow_id?: string;
  agent?: Agent;
}
