export type SetupGoalKey = 'product_delivery' | 'customer_support' | 'automation_mastery' | 'team_project_management' | 'help_center_docs' | 'internal_docs' | 'sales_crm';
export type SetupTaskStatus = 'available' | 'completed' | 'blocked' | 'unavailable' | 'unable_to_verify' | 'needs_attention';
export type SetupMaturity = 'preparing' | 'ready' | 'activated' | 'established' | 'advanced';

export interface SetupAction {
  key: string;
  label: string;
}

export interface SetupTask {
  key: string;
  title: string;
  description: string;
  stage: string;
  status: SetupTaskStatus;
  shared: boolean;
	core: boolean;
	blocked_reason?: string;
  action?: SetupAction;
}

export interface SetupJourney {
  key: string;
  title: string;
  description: string;
  accent: 'indigo' | 'blue' | 'emerald' | 'violet';
	scope: 'foundation' | 'active' | 'featured';
  maturity: SetupMaturity;
  completed_count: number;
  total_count: number;
  tasks: SetupTask[];
}

export interface MemberSetupPreference {
  id: string;
  workspace_id: string;
  user_id: string;
  sidebar_dismissed: boolean;
}

export interface SetupView {
  goals: SetupGoalKey[];
  journeys: SetupJourney[];
  recommended?: {
    journey_key: string;
    task_key: string;
    title: string;
    reason: string;
    action: SetupAction;
  };
  preference: MemberSetupPreference;
  completed_count: number;
  total_count: number;
	placeholder_goals: Array<{ key: SetupGoalKey; title: string; description: string }>;
}

export interface SupportSetupGuide {
  workspace_id: string;
  instructions: string;
  steps: Array<{
    key: string;
    title: string;
    status: SetupTaskStatus;
    path?: string;
    blocked_reason?: string;
    verification: string;
  }>;
}
