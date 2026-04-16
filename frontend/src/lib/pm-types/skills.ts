// ── Skills ──────────────────────────────────────────────────────────

/** A reference from an agent to a skill (built-in or workspace). */
export interface AgentSkillRef {
  skill_id?: string;
  key: string;
  version_key?: string;
  config?: Record<string, unknown>;
}

/** A skill entry in the merged catalog (built-in + workspace). */
export interface SkillCatalogEntry {
  id?: string;
  key: string;
  version_key?: string;
  title: string;
  description: string;
  source_kind: 'built_in' | 'workspace' | 'imported';
  source_runtime?: string;
  required_tools?: string[];
  supported_runtimes?: string[];
  presets?: string[];
}

/** Response from GET /api/automation/library/skills. */
export interface SkillCatalogResponse {
  skills: SkillCatalogEntry[];
}

/** Full workspace skill record returned by CRUD endpoints. */
export interface WorkspaceSkillResponse {
  id: string;
  workspace_id: string;
  source_kind: string;
  source_runtime?: string;
  key: string;
  version_key: string;
  title: string;
  description: string;
  instructions: string;
  required_tools?: string[];
  supported_runtimes?: string[];
  package_file_name: string;
  package_size: number;
  is_archived: boolean;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

/** Payload for creating a workspace skill. */
export interface CreateWorkspaceSkillRequest {
  key: string;
  title?: string;
  description: string;
  instructions: string;
  required_tools?: string[];
  supported_runtimes?: string[];
  source_runtime?: string;
}

/** Payload for updating a workspace skill. */
export interface UpdateWorkspaceSkillRequest {
  key?: string;
  title?: string;
  description?: string;
  instructions?: string;
  required_tools?: string[];
  supported_runtimes?: string[];
  source_runtime?: string;
}
