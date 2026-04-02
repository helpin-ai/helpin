import type { EpicWithStats, Label } from './project';

// ── Objectives ──────────────────────────────────────────────────────

export type ObjectiveType = 'tactical' | 'strategic';
export type ObjectiveState = 'not_started' | 'active' | 'closed';
export type ObjectiveHealth = 'on_track' | 'at_risk' | 'off_track';
export type KeyResultType = 'boolean' | 'percent' | 'numeric';

export interface Objective {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  objective_type: ObjectiveType;
  state: ObjectiveState;
  planned_start_date?: string;
  deadline?: string;
  health: ObjectiveHealth;
  health_comment?: string;
  position: number;
  archived: boolean;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface KeyResult {
  id: string;
  objective_id: string;
  name: string;
  result_type: KeyResultType;
  initial_value: number;
  current_value: number;
  target_value: number;
  progress: number;
  note?: string;
  note_updated_by?: string;
  note_updated_at?: string;
  position: number;
  updated_by?: string;
  created_at: string;
  updated_at: string;
}

export interface ObjectiveStats {
  key_result_count: number;
  key_result_avg_pct: number;
  epic_count: number;
  epic_done_count: number;
  epic_task_count: number;
  epic_done_tasks: number;
  epic_progress_pct: number;
}

export interface ObjectiveWithDetails {
  objective: Objective;
  teams: string[];
  owners: string[];
  owner_member_ids?: string[];
  labels: Label[];
  key_results: KeyResult[];
  epics: EpicWithStats[];
  stats: ObjectiveStats;
  suggested_health: ObjectiveHealth;
}

export interface CreateObjectiveRequest {
  workspace_id: string;
  name: string;
  description?: string;
  attachment_ids?: string[];
  objective_type: ObjectiveType;
  state?: ObjectiveState;
  planned_start_date?: string;
  deadline?: string;
  health?: ObjectiveHealth;
  health_comment?: string;
  position?: number;
  team_ids?: string[];
  owner_ids?: string[];
  owner_member_ids?: string[];
  label_ids?: string[];
  epic_ids?: string[];
}

export interface UpdateObjectiveRequest {
  name?: string;
  description?: string;
  objective_type?: ObjectiveType;
  state?: ObjectiveState;
  planned_start_date?: string;
  deadline?: string;
  health?: ObjectiveHealth;
  health_comment?: string;
  position?: number;
  archived?: boolean;
  team_ids?: string[];
  owner_ids?: string[];
  owner_member_ids?: string[];
  label_ids?: string[];
  epic_ids?: string[];
}

export interface CreateKeyResultRequest {
  name: string;
  result_type?: KeyResultType;
  initial_value?: number;
  current_value?: number;
  target_value?: number;
  note?: string;
  position?: number;
}

export interface UpdateKeyResultRequest {
  name?: string;
  result_type?: KeyResultType;
  initial_value?: number;
  current_value?: number;
  target_value?: number;
  note?: string;
  position?: number;
}
