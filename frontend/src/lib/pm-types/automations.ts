
export type AutomationType = 'epic_auto_start' | 'epic_auto_complete' | 'sprint_auto_create' | 'sprint_move_unfinished';

export interface PMAutomation {
  id: string;
  workspace_id: string;
  automation_type: AutomationType;
  enabled: boolean;
  team_id?: string;
  config_state_id?: string;
  config_int?: number;
  config_int2?: number;
  config_int3?: number;
  created_at: string;
  updated_at: string;
}

export interface UpsertAutomationRequest {
  workspace_id: string;
  automation_type: AutomationType;
  enabled: boolean;
  team_id?: string;
  config_state_id?: string;
  config_int?: number;
  config_int2?: number;
  config_int3?: number;
}

export interface DeleteAutomationRequest {
  workspace_id: string;
  automation_type: AutomationType;
  team_id?: string;
}

// ── Automation Rules ────────────────────────────────────────────────

export interface AutomationRule {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  enabled: boolean;
  team_id?: string;
  workflow_id?: string;
  trigger_type: string;
  trigger_config: Record<string, string>;
  action_type: string;
  action_config: Record<string, unknown>;
  template_key?: string;
  template_instance_id?: string;
  template_version?: number;
  position: number;
  stop_on_match: boolean;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface FlowTemplateInput {
  key: string;
  type: string;
  required?: boolean;
  label: string;
  section?: string;
  help_text?: string;
  placeholder?: string;
  default?: unknown;
  depends_on?: string;
  show_if?: string;
  space_type?: string;
  min?: number;
  max?: number;
  options?: Array<{ value: string; label: string }>;
}

export interface FlowTemplateManifest {
  key: string;
  version: number;
  name: string;
  icon: string;
  short_description: string;
  description_ref?: string;
  categories: string[];
  agent: {
    create?: {
      preset: string;
      runtime_kind?: string;
      name_template?: string;
      system_prompt_ref?: string;
      system_prompt?: string;
      skills?: string[];
      allowed_tools?: string[];
      allowed_targets?: string[];
      approval_mode?: string;
    };
    reuse_system?: string;
    pick_existing?: {
      required?: boolean;
      constraints?: {
        presets?: string[];
        targets?: string[];
      };
    };
    none?: boolean;
  };
  trigger: {
    type: string;
    event?: string;
  };
  inputs: FlowTemplateInput[];
  flow: {
    action: string;
    target?: Record<string, string>;
    conditions?: Record<string, string>[];
    parameters?: Record<string, unknown>;
    additional_context?: string;
  };
}

export interface InstallFlowTemplateRequest {
  name?: string;
  agent_name?: string;
  inputs: Record<string, unknown>;
  agent_overrides?: import('./agents').CreateAgentFromTemplateOverrides;
}

export interface InstallFlowTemplateResponse {
  template: FlowTemplateManifest;
  agent?: import('./agents').Agent;
  rule: AutomationRule;
}

export type TemplateAgentUninstallAction = 'none' | 'kept' | 'deleted' | 'still_referenced';

export interface UninstallFlowTemplateRequest {
  delete_created_agent?: boolean;
}

export interface UninstallFlowTemplateResponse {
  template_key: string;
  template_instance_id: string;
  rule_id: string;
  agent_id?: string;
  agent_action: TemplateAgentUninstallAction;
}

export interface CreateAutomationRuleRequest {
  workspace_id: string;
  name: string;
  description?: string;
  team_id?: string;
  workflow_id?: string;
  trigger_type: string;
  trigger_config: Record<string, string>;
  action_type: string;
  action_config: Record<string, unknown>;
  position?: number;
  stop_on_match?: boolean;
}

export interface UpdateAutomationRuleRequest {
  name?: string;
  description?: string;
  enabled?: boolean;
  trigger_type?: string;
  trigger_config?: Record<string, string>;
  action_type?: string;
  action_config?: Record<string, unknown>;
  position?: number;
  stop_on_match?: boolean;
}

// ── Views (Spaces) ──────────────────────────────────────────────────

export interface PMView {
  id: string;
  workspace_id: string;
  name: string;
  filters: Record<string, string>;
  is_shared: boolean;
  is_pinned: boolean;
  position: number;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CreateViewRequest {
  name: string;
  filters: Record<string, string>;
  is_shared: boolean;
  is_pinned: boolean;
}

export interface UpdateViewRequest {
  name?: string;
  filters?: Record<string, string>;
  is_shared?: boolean;
  is_pinned?: boolean;
  position?: number;
}
