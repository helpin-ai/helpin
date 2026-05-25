// ── Git Integration ─────────────────────────────────────────────────

export interface GitIntegration {
  id: string;
  workspace_id?: string;
  organization_id?: string;
  provider: string;
  display_name: string;
  credential_mode?: string;
  credential_id?: string;
  account_login?: string;
  base_url?: string;
  installation_id?: string;
  app_id?: string;
  last_synced_at?: string;
  last_sync_error?: string;
  active: boolean;
  deleted_at?: string;
  created_at: string;
  updated_at: string;
}

export interface GitRepository {
  id: string;
  workspace_id: string;
  integration_id: string;
  provider: string;
  base_url?: string;
  external_id: string;
  full_name: string;
  default_branch: string;
  permissions: Record<string, unknown>;
  private: boolean;
  archived: boolean;
  selected: boolean;
  active: boolean;
  deleted_at?: string;
  created_at: string;
  updated_at: string;
}

export interface GitAvailableRepoClaim {
  workspace_id: string;
  workspace_name: string;
  repo_id: string;
}

export interface GitAvailableRepo {
  external_id: string;
  full_name: string;
  default_branch: string;
  permissions: Record<string, unknown>;
  private: boolean;
  archived: boolean;
  claimed_by?: GitAvailableRepoClaim | null;
}

export interface GitIntegrationWorkspaceUsage {
  workspace_id: string;
  workspace_name: string;
  repo_count: number;
}

export interface GitIntegrationDetail {
  integration: GitIntegration;
  affected_workspaces: GitIntegrationWorkspaceUsage[];
}

export interface WireGitRepositoriesRequest {
  workspace_id?: string;
  repo_ids: string[];
}

export interface WireGitRepositoriesConflict {
  external_id: string;
  claimed_by_workspace_id: string;
  claimed_by_workspace_name?: string;
}

export interface WireGitRepositoriesConflictResponse {
  error?: string;
  conflicts: WireGitRepositoriesConflict[];
}

export interface WireGitRepositoriesResponse {
  repositories: GitRepository[];
}

export interface GitBranch {
  name: string;
  is_default: boolean;
}

export interface TaskDeliveryTarget {
  id: string;
  workspace_id: string;
  task_id: string;
  repository_id?: string;
  repo_full_name?: string;
  integration_id?: string;
  base_branch?: string;
  working_branch?: string;
  delivery_state: string;
  active_pr_number?: number;
  active_pr_title?: string;
  active_pr_url?: string;
  active_pr_status?: string;
  last_commit_sha?: string;
  last_run_id?: string;
  last_synced_at?: string;
  created_at: string;
  updated_at: string;
}

/** @deprecated Use TaskDeliveryTarget instead */
export type StoryDeliveryTarget = TaskDeliveryTarget;

export interface TaskGitLink {
  id: string;
  workspace_id: string;
  task_id: string;
  integration_id: string;
  repository_id?: string;
  run_id?: string;
  provider: string;
  base_url?: string;
  repo: string;
  branch?: string;
  pr_number?: number;
  pr_title?: string;
  pr_url?: string;
  pr_status?: string;
  commit_sha?: string;
  created_at: string;
  updated_at: string;
}

/** @deprecated Use TaskGitLink instead */
export type StoryGitLink = TaskGitLink;

export interface CreateGitIntegrationRequest {
  provider: string;
  display_name: string;
  credential_mode?: string;
  account_login?: string;
  base_url?: string;
  installation_id?: string;
  app_id?: string;
  webhook_secret?: string;
  access_token?: string;
}

export interface GitHubInstallURLResponse {
  install_url: string;
  action: 'install' | 'pick_repos';
  integration_id?: string;
}

export interface GitLabConnectURLResponse {
  connect_url: string;
}

export interface UpdateGitRepositoryRequest {
  selected: boolean;
}

export interface CreateBranchRequest {
  integration_id: string;
  repo: string;
  branch_name: string;
}

export interface UpdateTaskDeliveryTargetRequest {
  repository_id?: string;
  base_branch?: string;
  working_branch?: string;
}

/** @deprecated Use UpdateTaskDeliveryTargetRequest instead */
export type UpdateStoryDeliveryTargetRequest = UpdateTaskDeliveryTargetRequest;
