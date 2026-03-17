export interface User {
  id: string;
  email: string;
  full_name: string;
  avatar_url?: string;
  default_workspace_id?: string;
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  user: User;
  access_token: string;
  refresh_token: string;
}

export interface Organization {
  id: string;
  name: string;
  slug: string;
  owner_id: string;
  logo_url?: string;
  created_at: string;
  updated_at: string;
}

export interface OrganizationWithRole extends Organization {
  role: 'owner' | 'admin' | 'member';
}

export interface Workspace {
  id: string;
  name: string;
  slug: string;
  owner_id: string;
  organization_id?: string;
  description?: string;
  logo_url?: string;
  timezone: string;
  created_at: string;
  updated_at: string;
}

export interface WorkspaceMember {
  id: string;
  workspace_id: string;
  user_id?: string;
  email?: string;
  display_name?: string;
  role: 'owner' | 'admin' | 'member' | 'viewer';
  status?: 'pending' | 'active' | 'revoked' | 'inactive';
  invited_by?: string;
  invited_at?: string;
  accepted_at?: string;
  created_at: string;
  updated_at: string;
}

// Permission strings matching the backend authorization catalog.
export type Permission =
  | 'workspace.read'
  | 'workspace.update'
  | 'workspace.delete'
  | 'workspace.members.read'
  | 'workspace.members.manage'
  | 'workspace.invites.manage'
  | 'workspace.roles.manage'
  | 'settings.read'
  | 'settings.manage'
  | 'team.read'
  | 'team.manage'
  | 'team.members.read'
  | 'team.members.manage'
  | 'pm.read'
  | 'pm.edit'
  | 'pm.admin.workflows'
  | 'pm.admin.labels'
  | 'pm.admin.automations'
  | 'pm.import'
  | 'docs.read'
  | 'docs.edit'
  | 'docs.publish'
  | 'docs.admin'
  | 'crm.read'
  | 'crm.edit'
  | 'crm.admin'
  | 'support.read'
  | 'support.edit'
  | 'support.admin'
  | 'notifications.read'
  | 'notifications.manage'
  | 'search.read'
  | 'ws.connect';

// Response from GET /api/workspaces/{id}/me
export interface WorkspaceAccess {
  workspace_id: string;
  membership: {
    id: string;
    user_id: string;
    role: 'owner' | 'admin' | 'member' | 'viewer';
    status: string;
  };
  permissions: Permission[];
  team_memberships: {
    team_id: string;
    role: string;
  }[];
}

export interface MemberWithUser {
  id: string;
  user_id: string;
  role: string;
  email: string;
  full_name: string;
  avatar_url?: string;
}

export interface AssignableMember {
  id: string;
  user_id?: string;
  role: string;
  email: string;
  display_name: string;
  avatar_url?: string;
  status: 'pending' | 'active' | 'revoked' | 'inactive';
  invited_by?: string;
  invited_at?: string;
  accepted_at?: string;
}

export interface WorkspaceSettings {
  settings: WorkspaceConfig;
  teams: WorkspaceTeam[];
  people: WorkspacePerson[];
  memberships: TeamMembership[];
  user_memberships: TeamUserMembership[];
  managers: WorkspaceManager[];
  job_role_criteria: JobRoleCriteria[];
  invitation_team_preassignments: InvitationTeamPreassignment[];
  team_estimate_settings: TeamEstimateSettings[];
  team_field_visibility: TeamFieldVisibility[];
  team_repo_defaults: TeamRepoDefault[];
}

export interface TeamFieldVisibility {
  id: string;
  team_id: string;
  priority: boolean;
  story_type: boolean;
  severity: boolean;
  labels: boolean;
  epic: boolean;
  sprint: boolean;
  estimate: boolean;
  due_date: boolean;
  blocked: boolean;
  delivery: boolean;
  dev_history: boolean;
  created_at: string;
  updated_at: string;
}

export interface InvitationTeamPreassignment {
  id: string;
  invitation_id: string;
  team_id: string;
  created_at: string;
}

export type EstimateScale = 'exponential' | 'fibonacci' | 'linear' | 'tshirt' | 'hours';

export interface TeamEstimateSettings {
  id: string;
  team_id: string;
  enabled: boolean;
  scale: EstimateScale;
  extended: boolean;
  allow_zero: boolean;
  count_unestimated_as_one: boolean;
  created_at: string;
  updated_at: string;
}

export interface TeamRepoDefault {
  id: string;
  team_id: string;
  repository_id: string;
  base_branch: string;
  branch_template: string;
  auto_sync_states: boolean;
  review_state_id?: string;
  done_state_id?: string;
  created_at: string;
  updated_at: string;
}

export interface WorkspaceConfig {
  id: string;
  workspace_id: string;
  quarter_start_date: string;
  sprint_duration_weeks: number;
  notifications_enabled: boolean;
  team_weight: number;
  planning_methodology: 'structured_v1' | 'basic_v1';
  planning_web_search_enabled: boolean;
  planning_web_search_provider: 'brave';
}

export interface WorkspaceTeam {
  id: string;
  workspace_id: string;
  name: string;
  handle?: string;
  description?: string;
  manager_id?: string;
  team_type?: 'engineering' | 'product' | 'design' | 'support' | 'marketing' | 'sales' | 'hr' | 'operations' | 'custom';
  default_story_type?: 'feature' | 'bug' | 'chore';
}

export interface WorkspacePerson {
  id: string;
  workspace_id: string;
  user_id?: string;
  name: string;
  email: string;
  role: 'executive' | 'manager' | 'employee';
  job_role: string;
  manager_id?: string;
  hire_date: string;
  status: 'active' | 'inactive';
  base_salary: number;
  active_for_bonus: boolean;
  active_for_evaluation: boolean;
  is_account_owner: boolean;
}

export interface TeamMembership {
  id: string;
  team_id: string;
  person_id: string;
}

export interface TeamUserMembership {
  id: string;
  team_id: string;
  user_id: string;
  role: 'owner' | 'member';
  created_at: string;
  updated_at: string;
}

export interface WorkspaceManager {
  id: string;
  workspace_id: string;
  person_id: string;
  can_create_goals: boolean;
  can_score_performance: boolean;
  reporting_to?: string;
}

export interface JobRoleCriteria {
  id: string;
  workspace_id: string;
  job_role: string;
  criteria_id: string;
  name: string;
  description?: string;
  question: string;
  enabled: boolean;
  weight: number;
}

export interface Invitation {
  id: string;
  workspace_id: string;
  email: string;
  role: string;
  status: 'pending' | 'accepted' | 'revoked';
  invited_by: string;
  expires_at: string;
  accepted_at?: string;
  created_at: string;
  join_url?: string;
}

export interface InviteInfo {
  workspace_name: string;
  workspace_slug: string;
  email: string;
  role: string;
  invited_by_name: string;
  status: string;
  expired: boolean;
}

export type AutomationKind = 'built_in_automation' | 'automation_rule' | 'contextual_agent' | 'custom_automation';
export type AutomationHealthStatus = 'healthy' | 'warning' | 'error' | 'inactive' | 'unknown';

export interface AutomationHealthSummary {
  status: AutomationHealthStatus;
  last_seen_at?: string;
  last_success_at?: string;
  last_error_at?: string;
  last_error_message?: string;
  freshness: string;
  metrics: Record<string, unknown>;
}

export interface AutomationInventoryItem {
  inventory_id: string;
  catalog_id: string;
  kind: AutomationKind;
  module: string;
  group: string;
  title: string;
  description: string;
  scope_type: string;
  scope_id: string;
  scope_label: string;
  target_types: string[];
  trigger_modes: string[];
  config_scope: string;
  execution_style: string;
  user_governed: boolean;
  enabled: boolean;
  current_write_surface: string;
  current_write_path?: string;
  current_run_surface: string;
  current_run_path?: string;
  output_surface: string;
  diagnostics_surface: string;
  health: AutomationHealthSummary;
}

export interface AutomationInventoryGroup {
  id: string;
  title: string;
  description: string;
}

export interface AutomationInventoryResponse {
  groups: AutomationInventoryGroup[];
  items: AutomationInventoryItem[];
  generated_at: string;
}
