import type { CRMSuggestion } from "./crmTypes";
import type { CRMPlaybookMilestoneProgress } from "./crmPlaybookTypes";

export type CRMSituationCategory = "sales" | "onboarding_adoption" | "expansion" | "retention";
export type CRMSituationMotion = "prospecting" | "conversion" | "onboarding" | "adoption" | "expansion" | "renewal" | "retention";
export type CRMSituationLifecycle = "open" | "paused" | "closed";
export type CRMSituationAttention = "needs_context" | "needs_approval" | "follow_up_due" | "waiting_customer" | "waiting_work" | "automation_failed";
export type CRMSituationOutcomeKind = "achieved" | "not_pursued" | "invalid" | "duplicate";

export interface CRMSituationWorkState {
  owner_member_id: string | null;
  next_action_owner_member_id: string | null;
  lifecycle: CRMSituationLifecycle;
  attention: CRMSituationAttention;
  next_step: string;
  next_checkpoint_at: string | null;

  playbook_id?: string;
  playbook_version_id?: string;
  playbook_applied_by_member_id?: string;
  playbook_applied_at?: string;
  playbook_milestones?: CRMPlaybookMilestoneProgress[];
  outcome_kind: CRMSituationOutcomeKind | null;
  outcome_summary: string | null;
  outcome_basis: "human_assessment" | null;
  duplicate_of_situation_id: string | null;
  closed_at: string | null;
  closed_by_member_id: string | null;
}

export interface CRMSituation extends CRMSituationWorkState {
  id: string;
  workspace_id: string;
  origin_kind: "manual" | "signal" | "suggestion" | "deal_won";
  title: string;
  objective: string;
  commercial_motion: CRMSituationMotion | "needs_context";
  company_id: string | null;
  contact_id: string | null;
  deal_id: string | null;
  revision: number;
  priority: number;
  created_by_member_id: string | null;
  created_at: string;
  updated_at: string;
}

export interface CRMSituationReference {
  kind: "signal" | "suggestion";
  source_id: string;
  created_at: string;
}

export interface CRMSituationItem {
  situation: CRMSituation;
  category: CRMSituationCategory | "";
  company_name: string;
  contact_name: string;
  deal_name: string;
  owner_name: string;
  next_action_owner_name: string;
  owner_available: boolean;
  next_action_owner_available: boolean;
  references?: CRMSituationReference[];
  actions?: CRMSuggestion[];
  evidence?: CRMSituationEvidence[];
  pending_action_count: number;
  failed_action_count: number;
  uncertain_action_count: number;
  manual_action_count: number;
  executing_action_count: number;
  effective_attention: CRMSituationAttention;
  checkpoint_status?: "scheduled" | "processing" | "delivered" | "cancelled" | "failed";
  checkpoint_result?: string;
  checkpoint_attempts?: number;
  checkpoint_completed_at?: string;
}

export interface CRMSituationEvidence {
  id: string;
  summary: string;
  evidence_excerpt?: string;
  source_type: string;
  source_id?: string;
  source_thread_id?: string;
  contact_id?: string;
  company_id?: string;
  evidence_identity_trust: string;
  detected_at: string;
  reviewed_at?: string;
  dismissed_at?: string;
  superseded_at?: string;
}

export interface CRMSituationList {
  data: CRMSituationItem[];
  total: number;
  page: number;
  page_size: number;
  category_counts: Record<CRMSituationCategory | "all", number>;
  uncategorized_count: number;
  pending_action_total: number;
}

export interface CreateCRMSituationRequest {
  creation_key: string;
  title: string;
  objective: string;
  commercial_motion: CRMSituationMotion;
  company_id?: string | null;
  contact_id?: string | null;
  deal_id?: string | null;
  owner_mode?: "routing" | "member" | "unassigned";
  owner_member_id?: string | null;
  next_action_owner_member_id?: string | null;
  next_step?: string;
  priority?: number;
  references?: Pick<CRMSituationReference, "kind" | "source_id">[];
}

export interface CRMSituationChanges {
  owner?: { member_id: string | null };
  next_action_owner?: { member_id: string | null };
  next_step?: string;
  attention?: Exclude<CRMSituationAttention, "needs_approval" | "automation_failed">;
  checkpoint?: { at: string | null };
}

export type CRMSituationOutcome =
  | { kind: Exclude<CRMSituationOutcomeKind, "duplicate">; summary: string; duplicate_of_situation_id?: never }
  | { kind: "duplicate"; summary: string; duplicate_of_situation_id: string };

export type CRMSituationCommandRequest = {
  command_key: string;
  expected_revision: number;
  reason?: string;
} & (
  | { operation: "update"; changes: CRMSituationChanges; outcome?: never }
  | { operation: "pause"; reason: string; changes?: never; outcome?: never }
  | { operation: "resume"; changes?: never; outcome?: never }
  | { operation: "close"; outcome: CRMSituationOutcome; changes?: never }
);

export interface CRMSituationChange {
  id: string;
  situation_id: string;
  revision: number;
  operation: "created" | "update" | "pause" | "resume" | "close" | "apply_playbook" | "update_milestone";
  actor_kind: "member" | "signal" | "suggestion" | "deal_won";
  actor_member_id: string | null;
  reason: string;
  before: CRMSituationWorkState | null;
  after: CRMSituationWorkState;
  in_flight_action_count: number;
  created_at: string;
}

// A replay returns the original receipt. Refetch detail for the current work state.
export interface CRMSituationCommandResult {
  change: CRMSituationChange;
  replayed: boolean;
}

export interface CRMSituationHistory {
  data: CRMSituationChange[];
  next_before_revision: number | null;
}
