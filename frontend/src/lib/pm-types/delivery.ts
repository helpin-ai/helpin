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
  default_commit_author_name?: string;
  default_commit_author_email?: string;
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

export type TaskDeliveryTargetSource = 'manual' | 'team_default' | 'epic';

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
  target_source?: TaskDeliveryTargetSource;
  source_epic_id?: string | null;
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

export interface EpicDeliveryTarget {
  id: string;
  workspace_id: string;
  epic_id: string;
  repository_id?: string;
  repo_full_name?: string;
  integration_id?: string;
  base_branch?: string;
  epic_branch?: string;
  delivery_state: string;
  final_pr_number?: number;
  final_pr_title?: string;
  final_pr_url?: string;
  final_pr_status?: string;
  last_commit_sha?: string;
  last_run_id?: string;
  last_synced_at?: string;
  created_at: string;
  updated_at: string;
}

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
  default_commit_author_name?: string;
  default_commit_author_email?: string;
}

export interface UpdateGitIntegrationRequest {
  default_commit_author_name?: string;
  default_commit_author_email?: string;
}

export interface GitHubInstallURLResponse {
  install_url: string;
  action: 'install' | 'pick_repos';
  integration_id?: string;
}

/** Instance GitHub App status (no secrets). */
export interface GitHubAppStatus {
  configured: boolean;
  source: 'env' | 'database' | 'none';
  slug: string;
  install_url: string;
  webhook_configured: boolean;
  manifest_available: boolean;
}

/** GitHub App manifest to POST to `post_url` in a form field named `manifest`. */
export interface GitHubAppManifestResponse {
  manifest: Record<string, unknown>;
  post_url: string;
  state: string;
}

export type GitLabTokenAuthType = 'personal_token' | 'group_token' | 'project_token';

export interface GitLabConnectTokenRequest {
  base_url: string;
  token: string;
  auth_type: GitLabTokenAuthType;
  label?: string;
  default_commit_author_name?: string;
  default_commit_author_email?: string;
}

export interface GitLabConnectResponse {
  integration_id: string;
  account_login: string;
  base_url: string;
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

export interface UpdateEpicDeliveryTargetRequest {
  repository_id?: string;
  base_branch?: string;
  epic_branch?: string;
}
