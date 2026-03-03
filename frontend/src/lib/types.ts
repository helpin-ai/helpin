export interface User {
  id: string;
  email: string;
  full_name: string;
  avatar_url?: string;
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  user: User;
  access_token: string;
  refresh_token: string;
}

export interface Workspace {
  id: string;
  name: string;
  slug: string;
  owner_id: string;
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface WorkspaceMember {
  id: string;
  workspace_id: string;
  user_id: string;
  role: 'owner' | 'admin' | 'manager' | 'member' | 'viewer';
  created_at: string;
  updated_at: string;
}

export interface Quarter {
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

export interface Sprint {
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

export interface CompanyGoal {
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
  team_contributions?: GoalTeamContribution[];
}

export interface GoalTeamContribution {
  id: string;
  goal_id: string;
  team_id: string;
  team_name?: string;
  contribution_pct: number;
  target_value?: number;
  current_value?: number;
  rationale?: string;
}

export interface SprintGoal {
  id: string;
  sprint_id: string;
  team_id: string;
  title: string;
  description?: string;
  weight: number;
  done: boolean;
  kr_id?: string;
}

export interface GoalDraft {
  id: string;
  workspace_id: string;
  quarter_id: string;
  created_by: string;
  draft_data: Record<string, unknown>;
  status: 'draft' | 'submitted' | 'approved';
  created_at: string;
  updated_at: string;
}

export interface IndividualCheck {
  id: string;
  sprint_id: string;
  workspace_id: string;
  employee_id: string;
  scored_by: string;
  criteria_id: string;
  answer: boolean;
  notes?: string;
}

export interface BonusCalculation {
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

export interface FinanceSettings {
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
  managers: WorkspaceManager[];
  job_role_criteria: JobRoleCriteria[];
  bonus_tiers: BonusTierConfig[];
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
  description?: string;
  manager_id?: string;
}

export interface WorkspacePerson {
  id: string;
  workspace_id: string;
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

export interface AuditEntry {
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
