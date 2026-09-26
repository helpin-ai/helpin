import { api } from '@/lib/api';

export type TriageSource = 'task' | 'task_draft' | 'support_conversation';
export interface TriageOption { id: string; name: string; description?: string }
export interface TriageSuggestion { id: string; probability: number }
export interface TriageMatch { task_id: string; relationship: 'duplicates' | 'relates_to'; probability: number }
export interface TriageView {
  id?: string;
  status: 'disabled' | 'input_limit' | 'daily_limit' | 'pending' | 'failed' | 'shadow' | 'ready';
  source_kind: TriageSource;
  source_id: string;
  source_hash?: string;
  assessment?: {
    actionable: boolean;
    task_type?: TriageSuggestion;
    team?: TriageSuggestion;
    labels: TriageSuggestion[];
    matches: TriageMatch[];
    candidates_checked: number;
  };
  teams: TriageOption[];
  labels: TriageOption[];
  candidates: { id: string; name: string; display_id: number }[];
  reviewed?: Record<string, 'accepted' | 'dismissed'>;
}
export interface TriageReview {
  assessment_id: string;
  action: 'task_type' | 'team' | 'match';
  value: string;
  dismiss: boolean;
}
const path = (kind: TriageSource, id: string) => kind === 'task'
  ? `/pm/tasks/${encodeURIComponent(id)}/triage`
  : `/support/inbox/conversations/${encodeURIComponent(id)}/task-triage`;
const query = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;
export interface SupportTaskDraft {
  name: string;
  description: string;
  task_type: 'bug' | 'feature' | 'chore';
  priority: 'none' | 'low' | 'medium' | 'high' | 'urgent';
  source_hash: string;
}
export const pmTriageService = {
  analyzeDraft: (workspaceId: string, draft: { draft_id: string; name: string; description: string; team_id?: string }) => api.post<TriageView>(`/pm/task-drafts/triage${query(workspaceId)}`, draft),
  draft: (workspaceId: string, id: string) => api.post<SupportTaskDraft>(`/support/inbox/conversations/${encodeURIComponent(id)}/task-draft${query(workspaceId)}`, {}),
  analyze: (workspaceId: string, kind: TriageSource, id: string) =>
    api.post<TriageView>(`${path(kind, id)}${query(workspaceId)}`, {}),
  review: (workspaceId: string, kind: TriageSource, id: string, request: TriageReview) =>
    api.post<{ key: string; status: 'accepted' | 'dismissed' }>(`${path(kind, id)}/review${query(workspaceId)}`, request),
};
