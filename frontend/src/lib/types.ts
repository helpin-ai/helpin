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
  role: 'owner' | 'admin' | 'manager' | 'member' | 'viewer';
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
  | 'rewards.read'
  | 'rewards.manage'
  | 'docs.read'
  | 'docs.edit'
  | 'docs.publish'
  | 'docs.admin'
  | 'search.read'
  | 'ws.connect';

// Response from GET /api/workspaces/{id}/me
export interface WorkspaceAccess {
  workspace_id: string;
  membership: {
    id: string;
    role: 'owner' | 'admin' | 'manager' | 'member' | 'viewer';
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

export interface RewardQuarter {
  id: string;
  workspace_id: string;
  name: string;
  start_date: string;
  end_date: string;
  status: 'draft' | 'active' | 'completed' | 'archived';
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface RewardSprint {
  id: string;
  quarter_id: string;
  workspace_id: string;
  sprint_number: number;
  start_date: string;
  end_date: string;
  status: 'planning' | 'active' | 'completed' | 'locked';
  locked_at?: string;
  locked_by?: string;
  created_at: string;
  updated_at: string;
}

export interface RewardCompanyGoal {
  id: string;
  workspace_id: string;
  quarter_id: string;
  title: string;
  description?: string;
  goal_type: 'metric' | 'milestone';
  baseline?: number;
  target?: number;
  current_value?: number;
  unit?: string;
  status: 'draft' | 'active' | 'completed' | 'cancelled';
  created_by: string;
  created_at: string;
  updated_at: string;
  team_contributions?: RewardGoalTeamContribution[];
}

export interface RewardGoalTeamContribution {
  id: string;
  goal_id: string;
  team_id: string;
  team_name?: string;
  contribution_pct: number;
  target_value?: number;
  current_value?: number;
  rationale?: string;
}

export interface RewardSprintGoal {
  id: string;
  sprint_id: string;
  team_id: string;
  title: string;
  description?: string;
  weight: number;
  done: boolean;
  kr_id?: string;
}

export interface RewardGoalDraft {
  id: string;
  workspace_id: string;
  quarter_id: string;
  created_by: string;
  draft_data: Record<string, unknown>;
  status: 'draft' | 'submitted' | 'approved';
  created_at: string;
  updated_at: string;
}

export interface RewardIndividualCheck {
  id: string;
  sprint_id: string;
  workspace_id: string;
  employee_id: string;
  scored_by: string;
  criteria_id: string;
  answer: boolean;
  notes?: string;
}

export interface RewardBonusCalculation {
  id: string;
  workspace_id: string;
  quarter_id: string;
  employee_id: string;
  final_amount: number;
  bonus_tier: 'A' | 'B' | 'C';
  team_tqi: number;
  individual_iqi: number;
  final_score: number;
  base_salary: number;
  is_override: boolean;
  override_reason?: string;
  calculation_details?: Record<string, unknown>;
  locked_at?: string;
}

export interface RewardFinanceSettings {
  id: string;
  workspace_id: string;
  quarter_id: string;
  mrr_start: number;
  mrr_end: number;
  bonus_pool_percentage: number;
  max_bonus_pool?: number;
  team_weight: number;
  bonus_tiers: unknown;
  total_pool: number;
  total_paid: number;
  pool_utilization: number;
  budget_factor: number;
  total_basic_salary: number;
  locked_at?: string;
  locked_by?: string;
}

export interface WorkspaceSettings {
  settings: WorkspaceConfig;
  teams: WorkspaceTeam[];
  people: WorkspacePerson[];
  memberships: TeamMembership[];
  user_memberships: TeamUserMembership[];
  managers: WorkspaceManager[];
  job_role_criteria: JobRoleCriteria[];
  bonus_tiers: BonusTierConfig[];
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
  auto_calculate_bonuses: boolean;
  team_weight: number;
}

export interface WorkspaceTeam {
  id: string;
  workspace_id: string;
  name: string;
  handle?: string;
  description?: string;
  manager_id?: string;
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

export interface BonusTierConfig {
  id: string;
  workspace_id: string;
  tier: 'A' | 'B' | 'C';
  min_score: number;
  max_score: number;
  salary_multiplier: number;
  description?: string;
  editable: boolean;
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

export interface RewardAuditEntry {
  id: string;
  workspace_id: string;
  quarter_id: string;
  action: string;
  employee_id?: string;
  performed_by: string;
  performed_by_name: string;
  performed_by_role: string;
  old_value?: unknown;
  new_value?: unknown;
  justification?: string;
  affected_count: number;
  performed_at: string;
}
