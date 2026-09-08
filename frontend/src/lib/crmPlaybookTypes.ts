import type { QueryFilterGroup } from "./queryBuilder";
import type { CRMSituationList, CRMSituationMotion } from "./crmSituationTypes";

export interface CRMPlaybookMilestone {
  key: string;
  name: string;
  success_criteria: string;
}

export interface CRMPlaybookMilestoneProgress {
  key: string;
  status: "pending" | "achieved" | "not_applicable";
  summary: string;
  assessed_by_member_id: string | null;
  assessed_at: string | null;
  basis: "human_assessment" | null;
}

// CRM business policy only. These settings do not enable a Flow or Agent.
export interface CRMPlaybookDefinition {
  name: string;
  description: string;
  journey: "buying_intent" | "sales_handoff" | "renewal_recovery" | "custom";
  objective: string;
  eligibility: {
    commercial_motions: CRMSituationMotion[];
    filter?: QueryFilterGroup;
  };
  responsibilities: {
    owner_role: "signal_owner" | "account_owner" | "deal_owner" | "customer_success_owner";
    approver_role: "signal_owner" | "next_action_owner";
    escalation_member_id: string | null;
  };
  milestones: CRMPlaybookMilestone[] | null;
  policy: {
    outbound_messages: "approval_required" | "not_allowed";
    crm_changes: "approval_required" | "not_allowed";
    pm_tasks: "approval_required" | "not_allowed";
    check_after_hours: number;
    escalate_after_hours: number;
    stop_conditions: Array<"customer_declined" | "objective_achieved" | "no_longer_eligible" | "contact_restricted">;
  };
}

export interface CRMPlaybook {
  id: string;
  workspace_id: string;
  revision: number;
  draft: CRMPlaybookDefinition;
  published_version_id: string | null;
  accepting_customers: boolean;
  created_by_member_id: string;
  updated_by_member_id: string;
  created_at: string;
  updated_at: string;
}

export interface CRMPlaybookVersion {
  id: string;
  playbook_id: string;
  version: number;
  definition: CRMPlaybookDefinition;
  published_by_member_id: string;
  published_at: string;
}

export interface CRMPlaybookItem {
  execution_enabled: false;
  playbook: CRMPlaybook;
  open_count: number;
  paused_count: number;
  closed_count: number;
  published_version?: CRMPlaybookVersion;
}

export interface CRMPlaybookList {
  data: CRMPlaybookItem[];
  total: number;
  page: number;
  page_size: number;
}

export interface CreateCRMPlaybookRequest {
  creation_key: string;
  definition: CRMPlaybookDefinition;
}

export type CRMPlaybookCommandRequest = {
  command_key: string;
  expected_revision: number;
  reason?: string;
} & (
  | { operation: "update_draft"; definition: CRMPlaybookDefinition; accepting_customers?: never }
  | { operation: "publish"; definition?: never; accepting_customers?: never }
  | { operation: "set_enrollment"; accepting_customers: boolean; definition?: never }
);

export interface CRMPlaybookChange {
  id: string;
  playbook_id: string;
  revision: number;
  operation: "created" | "update_draft" | "publish" | "set_enrollment";
  actor_member_id: string;
  reason: string;
  after: CRMPlaybook;
  created_at: string;
}

export interface CRMPlaybookCommandResult {
  change: CRMPlaybookChange;
  replayed: boolean;
}

export interface CRMPlaybookHistory {
  data: CRMPlaybookChange[];
  next_before_revision: number | null;
}

export interface CRMPlaybookVersions {
  data: CRMPlaybookVersion[];
  next_before_version: number | null;
}

export interface CRMPlaybookPreview {
  version_id: string | null;
  playbook_revision: number;
  definition: CRMPlaybookDefinition;
  signals: CRMSituationList;
  scope: "existing_signals";
  execution_enabled: false;
}

// Product-owned skill selection only; this is not a published or runnable connection.
export interface CRMPlaybookAutomationPreview {
  playbook_id: string;
  playbook_revision: number;
  playbook_version_id: string | null;
  scope: "draft" | "published";
  journey: CRMPlaybookDefinition["journey"];
  preset_key: string;
  specialization_version: string;
  skills: Array<{
    key: string;
    title: string;
    role: "core" | "job";
    version: string;
  }>;
  status: "not_connected" | "unsupported_journey";
  explanation: string;
  execution_enabled: false;
}

export interface CRMPlaybookConnectionSelection {
  playbook_version_id: string;
  expected_revision: number;
  flow_id: string;
  agent_id: string;
}

export interface CRMPlaybookConnectionReview extends CRMPlaybookConnectionSelection {
  connection_version: number;
  review_fingerprint: string;
  flow_name: string;
  agent_name: string;
  runtime_kind: string;
  skills: CRMPlaybookAutomationPreview['skills'];
  execution_enabled: false;
  explanation: string;
}

export interface PublishCRMPlaybookConnectionRequest extends CRMPlaybookConnectionSelection {
  command_key: string;
  expected_connection_version: number;
  review_fingerprint: string;
}

export interface CRMPlaybookConnection {
  id: string;
  playbook_id: string;
  playbook_version_id: string;
  version: number;
  fingerprint: string;
  execution_enabled: false;
  published_by_member_id: string;
  published_at: string;
}

export interface CRMPlaybookConnectionResult {
  connection: CRMPlaybookConnection;
  replayed: boolean;
}

export interface CRMPlaybookConnections {
  data: CRMPlaybookConnection[];
  next_before_version: number | null;
}

export interface CRMPlaybookAutomationSettings {
  playbook_id: string;
  connection_id: string;
  revision: number;
  enabled: boolean;
  entry_mode: 'manual' | 'automatic';
  max_runs_per_day: number;
  max_no_progress_runs: number;
  authorized_by_member_id: string;
  updated_at: string;
}
export interface CRMPlaybookAutomationBinding {
  situation_id: string;
  playbook_id: string;
  connection_id: string;
  enabled: boolean;
  generation: number;
  last_run_id?: string;
  last_checked_at?: string;
  no_progress_runs: number;
  escalated_at?: string;
  blocker: string;
}
export interface CRMPlaybookAutomationOverview {
  settings: CRMPlaybookAutomationSettings | null;
  connection: CRMPlaybookConnection | null;
  flow_id?: string;
  flow_name?: string;
  agent_id?: string;
  agent_name?: string;
  runtime_available: boolean;
}
export interface CRMPlaybookAgentUsage { playbook_id: string; name: string }
export interface CRMPlaybookAutomationActivityPage {
  data: { id: string; kind: 'connection' | 'settings' | 'signal_automation' | 'check'; status: string; situation_id?: string; situation_title?: string; occurred_at: string }[];
  total: number;
  page: number;
}
export interface CRMPlaybookAutomationCommand {
  command_key: string;
  expected_revision: number;
  connection_id: string;
  enabled: boolean;
  entry_mode: 'manual' | 'automatic';
  max_runs_per_day: number;
  max_no_progress_runs: number;
  confirmed: true;
}
export interface CRMPlaybookAutomationAdoption {
  command_key: string;
  expected_revision: number;
  expected_generation: number;
  connection_id: string;
  enabled: boolean;
  confirmed: true;
}
export interface CRMPlaybookAutomationReceipt {
  settings?: CRMPlaybookAutomationSettings;
  binding?: CRMPlaybookAutomationBinding;
  created_at: string;
}
export interface CRMPlaybookAction {
  version: 1;
  kind: 'email' | 'task' | 'handoff' | 'milestone' | 'deal_stage';
  title: string;
  reason: string;
  email?: { account_id: string; contact_id: string; to: string; subject: string; body_html: string };
  task?: { team_id: string; owner_member_id: string; name: string; description: string; deadline?: string };
  handoff?: { receiving_member_id: string; summary: string };
  milestone?: { key: string; evidence: string };
  deal_stage?: { deal_id: string; stage_id: string };
}
export interface CRMPlaybookActionIntent {
  suggestion_id: string;
  situation_id: string;
  run_id: string;
  connection_id: string;
  approver_member_id: string;
  expires_at: string;
  approved_by_member_id?: string;
  approved_at?: string;
  result_type?: string;
  result_id?: string;
}
export interface CRMPlaybookActionInspection {
  revision: string;
  outcome: 'completed' | 'not_completed';
  evidence: string;
  confirmed: true;
}

export interface ApplyCRMPlaybookRequest {
  command_key: string;
  situation_id: string;
  version_id: string;
  expected_playbook_revision: number;
  expected_situation_revision: number;
  confirmed: true;
}

export interface CRMPlaybookMilestoneRequest {
  command_key: string;
  expected_revision: number;
  milestone_key: string;
  status: CRMPlaybookMilestoneProgress["status"];
  summary: string;
}
