export interface User {
  id: string;
  email: string;
  full_name: string;
  avatar_url?: string;
  avatar_style?: string;
  avatar_seed?: string;
  avatar_background_mode?: string;
  avatar_background_color?: string;
  default_workspace_id?: string;
  two_fa_enabled?: boolean;
  mfa_satisfied_in_token?: boolean;
  email_verified?: boolean;
  email_verified_at?: string;
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  user: User;
  access_token: string;
  refresh_token: string;
}

export interface SigninResponse {
  user?: User;
  access_token?: string;
  refresh_token?: string;
  requires_2fa?: boolean;
  two_fa_token?: string;
}

export interface Passkey {
  id: string;
  name: string;
  verified: boolean;
  created_at: string;
  updated_at: string;
}

export interface PasskeyOptionsResponse {
  challenge: string;
  options: Record<string, unknown>;
}

export interface PasskeyListResponse {
  passkeys: Passkey[];
}

export type PasskeyAuthenticationResponse = SigninResponse;

export interface TwoFAStatusResponse {
  enabled: boolean;
}

export interface TwoFASetupResponse {
  provisioning_uri: string;
  recovery_codes: string[];
}

export interface RecoveryCodesResponse {
  recovery_codes: string[];
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
  role: 'owner' | 'admin' | 'member' | 'viewer';
}

export interface Workspace {
  id: string;
  name: string;
  slug: string;
  workspace_key: string;
  owner_id: string;
  organization_id?: string;
  role?: string;
  description?: string;
  company_product_context?: string;
  website_url?: string;
  logo_url?: string;
  timezone: string;
  created_at: string;
  updated_at: string;
  billing?: WorkspaceBillingSummary;
}

export type BillingPlan = 'starter' | 'growth' | 'founder';
export type BillingStatus = 'trialing' | 'active' | 'trial_expired' | 'past_due' | 'unpaid' | 'canceled';
export type BillingInterval = 'monthly' | 'annual';

export interface WorkspaceBillingSummary {
  workspace_id: string;
  plan: BillingPlan;
  status: BillingStatus | string;
  locked: boolean;
  billing_interval: BillingInterval | string;
  trialing: boolean;
  trial_ends_at?: string;
  current_period_start: string;
  current_period_end: string;
  included_credits: number;
  credits_used: number;
  credits_remaining: number;
  next_charge_cents: number;
  on_demand_enabled: boolean;
  on_demand_available: boolean;
  stripe_customer_id?: string;
  stripe_subscription_id?: string;
  pending_plan?: BillingPlan;
  pending_billing_interval?: BillingInterval;
  pending_change_at?: string;
  cancel_at_period_end: boolean;
  canceled_at?: string;
  billing_notice_type?: 'payment_failed' | 'trial_will_end' | string;
  billing_notice_message?: string;
  billing_notice_at?: string;
  payment_failed_at?: string;
  trial_will_end_at?: string;
  manage_billing_enabled: boolean;
  warning?: string;
  seat_limit?: number;
  seat_usage?: number;
  seat_over_limit?: boolean;
  entitlement_warning?: string;
  on_demand_blocks_invoiced: number;
  ai_usage_allowance_microusd?: number;
  ai_usage_used_microusd?: number;
  ai_usage_remaining_microusd?: number;
  ai_usage_reserved_microusd?: number;
  ai_usage_overage_microusd?: number;
  ai_usage_period_start?: string;
  ai_usage_period_end?: string;
  ai_usage_unlimited?: boolean;
  extra_ai_usage_enabled?: boolean;
  extra_ai_usage_available?: boolean;
  pricing_version?: string;
  billing_managers?: BillingManagerRef[];
}

export interface BillingManagerRef {
  user_id: string;
  name: string;
  email: string;
  role: string;
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
  | 'module_access.manage'
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
  | 'docs.import'
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

export type WorkspaceModule = 'pm' | 'docs' | 'crm' | 'support' | 'automation';
export type ManagedWorkspaceModule = Extract<WorkspaceModule, 'crm' | 'support' | 'automation'>;
export type ModuleGrantSubjectType = 'team' | 'workspace_member';

export interface WorkspaceModuleGrant {
  id: string;
  workspace_id: string;
  module: WorkspaceModule;
  subject_type: ModuleGrantSubjectType;
  subject_id: string;
  access_level: 'member';
  created_by_id?: string;
  created_at: string;
  updated_at: string;
}

export interface WorkspaceModuleAccessSettings {
  modules: WorkspaceModule[];
  grants: WorkspaceModuleGrant[];
}

// Response from GET /api/workspaces/{id}/me
export interface WorkspaceAccess {
  workspace_id: string;
  membership: {
    id: string;
    user_id: string;
    role: 'owner' | 'admin' | 'member' | 'viewer';
    status: string;
    support_default_team_id?: string;
    support_task_dialog_dismissed: boolean;
  };
  permissions: Permission[];
  team_memberships: {
    team_id: string;
    role: string;
  }[];
  modules: WorkspaceModule[];
  security_policy?: WorkspaceMFAPolicy;
}

export interface WorkspaceMFAPolicy {
  enforce_two_factor: boolean;
  mfa_required: boolean;
  mfa_enabled: boolean;
  mfa_satisfied: boolean;
}

export interface MemberWithUser {
  id: string;
  user_id: string;
  role: string;
  email: string;
  full_name: string;
  two_fa_enabled?: boolean;
  avatar_url?: string;
  avatar_style?: string;
  avatar_seed?: string;
  avatar_background_mode?: string;
  avatar_background_color?: string;
}

export interface AssignableMember {
  id: string;
  user_id?: string;
  role: string;
  email: string;
  display_name: string;
  avatar_url?: string;
  avatar_style?: string;
  avatar_seed?: string;
  avatar_background_mode?: string;
  avatar_background_color?: string;
  status: 'pending' | 'active' | 'revoked' | 'inactive';
  invited_by?: string;
  invited_at?: string;
  accepted_at?: string;
}

export interface WorkspaceMemberPresenceStatus {
  user_id: string;
  status: 'online' | 'away' | 'offline';
  source: 'auto' | 'manual';
  manual_status?: 'online' | 'away' | 'offline';
  last_seen_at?: string;
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
  task_type: boolean;
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
  closed_state_id?: string;
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
  enforce_two_factor?: boolean;
}

export interface WorkspaceTeam {
  id: string;
  workspace_id: string;
  name: string;
  handle?: string;
  description?: string;
  manager_id?: string;
  team_type?: 'engineering' | 'product' | 'design' | 'support' | 'marketing' | 'sales' | 'hr' | 'operations' | 'custom';
  default_task_type?: 'feature' | 'bug' | 'chore';
  docs_publisher_enabled?: boolean;
  sprints_enabled?: boolean;
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
  workspace_member_id?: string;
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
  account_exists: boolean;
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

export interface AutomationTriggerCatalogEntry {
  id: string;
  binding_kind: string;
  category: string;
  trigger_type: string;
  title: string;
  description: string;
  source_surface: string;
  config_surface?: string;
  supports_agent_runs: boolean;
  binding_count: number;
  execution_search?: AutomationTriggerExecutionSearchPreset;
  show_rules_search?: WorkflowRuleSearchPreset;
  create_rule_search?: WorkflowRuleSearchPreset;
}

export interface AutomationInventoryResponse {
  groups: AutomationInventoryGroup[];
  items: AutomationInventoryItem[];
  trigger_catalog: AutomationTriggerCatalogEntry[];
  generated_at: string;
}

export interface AutomationTriggerExecutionFilters {
  execution_id?: string;
  agent_id?: string;
  binding_id?: string;
  trigger_type?: string;
  status?: string;
  source?: string;
  reference_id?: string;
  run_id?: string;
  fired_after?: string;
  fired_before?: string;
  page?: number;
  per_page?: number;
}

export interface AutomationTriggerExecutionSearchPreset {
  agent_id?: string;
  binding_id?: string;
  trigger_type?: string;
  source?: string;
  reference_id?: string;
  status?: string;
}

export interface WorkflowRuleSearchPreset {
  show_trigger?: string;
  show_trigger_title?: string;
  show_rule?: string;
  show_rule_title?: string;
  template?: string;
  template_title?: string;
  template_description?: string;
  create_event_rule?: boolean;
  trigger_type?: string;
  agent_id?: string;
  repo_full_name?: string;
  branch?: string;
  base_branch?: string;
  tag_name?: string;
  conclusion?: string;
  target_mode?: 'event' | 'task' | 'epic' | 'repository';
  target_id?: string;
}

export interface AutomationTriggerExecutionListItem {
  execution_id: string;
  agent_id: string;
  agent_name: string;
  actor_id?: string;
  actor_name?: string;
  binding_id: string;
  binding_kind: string;
  binding_title: string;
  trigger_type?: string;
  trigger_title?: string;
  reference_id?: string;
  reference_type?: string;
  reference_title?: string;
  manage_path?: string;
  target_type?: string;
  target_id?: string;
  target_title?: string;
  target_key?: string;
  run_id?: string;
  status: string;
  error_message?: string;
  fired_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface AutomationTriggerExecutionListResponse {
  data: AutomationTriggerExecutionListItem[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}
