import type { AgentPresetKey } from './agents';

export interface AgentHandoff {
  id: string;
  workspace_id: string;
  from_agent_id?: string;
  to_agent_id?: string;
  to_user_id?: string;
  story_id?: string;
  task_id?: string;
  epic_id?: string;
  run_id?: string;
  handoff_type: string;
  reason: string;
  context: Record<string, unknown>;
  created_at: string;
}

export interface StructuredQuestionOption {
  value: string;
  label: string;
  freetext?: boolean;
}

export interface StructuredQuestion {
  id: string;
  type?: 'single_select';
  text: string;
  options: StructuredQuestionOption[];
}

export interface ToolInputSchemaProperty {
  type: string;
  description?: string;
  items?: Record<string, unknown>;
}

export interface ToolInputSchema {
  type: string;
  properties: Record<string, ToolInputSchemaProperty>;
  required?: string[];
}

export interface ToolCatalogEntry {
  name: string;
  description: string;
  category: string;
  input_schema: ToolInputSchema;
  presets: AgentPresetKey[];
}

export interface ToolCatalogResponse {
  tools: ToolCatalogEntry[];
  categories: string[];
}
