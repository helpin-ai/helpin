import { API_BASE } from '../api';

export interface ShortcutImportPreviewSummary {
  total_tasks: number;
  tasks_by_type: Record<string, number>;
  epics_count: number;
  objectives_count: number;
  sprints_count: number;
  labels_count: number;
  docs_count: number;
  teams_count: number;
  workflows_count: number;
  workflow_states_count: number;
  checklist_items_count: number;
  duplicate_tasks: number;
}

export interface ShortcutWorkflowStatePreview {
  name: string;
  suggested_type: string;
  task_count: number;
}

export interface ShortcutWorkflowPreview {
  id?: string;
  name: string;
  task_count: number;
  states: ShortcutWorkflowStatePreview[];
}

export interface ShortcutTeamPreview {
  name: string;
  task_count: number;
}

export interface ShortcutUserMatch {
  email: string;
  shortcut_member_id?: string | null;
  matched_user_id: string | null;
  matched_member_id?: string | null;
  matched_member_status?: string | null;
  matched_name: string | null;
  shortcut_name?: string | null;
  story_count: number;
  owner_count: number;
  requester_count: number;
}

export interface ShortcutImportPreviewResponse {
  summary: ShortcutImportPreviewSummary;
  users: ShortcutUserMatch[];
  teams: ShortcutTeamPreview[];
  workflows: ShortcutWorkflowPreview[];
  warnings: string[];
}

export interface ShortcutImportResult {
  teams_created: number;
  workflows_created: number;
  workflow_states_created: number;
  labels_created: number;
  objectives_created: number;
  epics_created: number;
  sprints_created: number;
  tasks_created: number;
  tasks_skipped: number;
  docs_created: number;
  docs_skipped: number;
  checklist_items_created: number;
  owner_links_created: number;
  label_links_created: number;
  external_links_created: number;
  task_links_created: number;
  attachments_created: number;
  comments_created: number;
  warnings: string[];
}

export interface ShortcutImportStatusProgress {
  current_step: string;
  steps_completed: number;
  steps_total: number;
  entities_processed: number;
  entities_total: number;
}

export interface ShortcutImportStatusResponse {
  import_id: string;
  status: 'pending' | 'scanning' | 'ready' | 'processing' | 'completed' | 'failed' | 'canceled';
  file_name?: string;
  total_rows?: number;
  progress: ShortcutImportStatusProgress;
  result?: ShortcutImportResult;
  error?: string;
  created_at?: string;
  updated_at?: string;
  completed_at?: string | null;
}

export interface ShortcutImportCount {
  entity: string;
  count: number;
}

export interface ShortcutImportWarningGroup {
  type: string;
  count: number;
  warnings: string[];
}

export interface ShortcutImportDiagnosticItem {
  type: string;
  key?: string;
  message: string;
  count?: number;
  retryable: boolean;
}

export interface ShortcutImportDiagnostics {
  counts: ShortcutImportCount[];
  warning_groups: ShortcutImportWarningGroup[];
  failed_media: ShortcutImportDiagnosticItem[];
  unmapped_members: ShortcutImportDiagnosticItem[];
  unmapped_states: ShortcutImportDiagnosticItem[];
  unmapped_teams: ShortcutImportDiagnosticItem[];
  retryable_failures: ShortcutImportDiagnosticItem[];
  non_retryable_failures: ShortcutImportDiagnosticItem[];
}

export interface ShortcutImportDetailResponse extends ShortcutImportStatusResponse {
  options?: ShortcutImportOptionsPayload;
  diagnostics: ShortcutImportDiagnostics;
  retryable: boolean;
  retry_blocked_reason?: string;
  cancelable: boolean;
}

export interface ShortcutImportExecuteResponse {
  import_id: string;
  status: string;
}

export interface WorkflowStateMappingPayload {
  shortcut_workflow_id?: string;
  shortcut_workflow_name: string;
  mode: 'create_new' | 'use_existing';
  new_workflow_name?: string;
  existing_workflow_id?: string;
  states: {
    shortcut_state: string;
    new_state_name?: string;
    state_type?: string;
    position?: number;
    existing_state_id?: string;
  }[];
}

export interface ShortcutImportOptionsPayload {
  import_archived: boolean;
  import_completed: boolean;
  import_docs?: boolean;
  docs_space_id?: string;
  docs_collection_id?: string;
  docs_lookback_months?: number;
  story_date_field?: 'updated_at' | 'created_at';
  story_lookback_months?: number;
  epic_lookback_months?: number;
  objective_lookback_months?: number;
  max_stories?: number;
}

async function jsonRequest<T>(path: string, body: unknown): Promise<{ data: T | null; error: string | null }> {
  const token = localStorage.getItem('access_token');
  try {
    const res = await fetch(`${API_BASE}${path}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      return { data: null, error: err.error || res.statusText };
    }
    const data = await res.json();
    return { data, error: null };
  } catch (e) {
    return { data: null, error: e instanceof Error ? e.message : 'Network error' };
  }
}

function statusRequest<T>(path: string): Promise<{ data: T | null; error: string | null }> {
  const token = localStorage.getItem('access_token');
  return fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  })
    .then(async (res) => {
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        return { data: null as T | null, error: (err.error || res.statusText) as string | null };
      }
      const data = await res.json();
      return { data: data as T, error: null };
    })
    .catch((e) => ({ data: null as T | null, error: e instanceof Error ? e.message : 'Network error' }));
}

function postStatusRequest<T>(path: string): Promise<{ data: T | null; error: string | null }> {
  const token = localStorage.getItem('access_token');
  return fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  })
    .then(async (res) => {
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        return { data: null as T | null, error: (err.error || res.statusText) as string | null };
      }
      const data = await res.json();
      return { data: data as T, error: null };
    })
    .catch((e) => ({ data: null as T | null, error: e instanceof Error ? e.message : 'Network error' }));
}

export const pmImportService = {
  previewShortcutAPI: (
    workspaceId: string,
    apiToken: string,
    options: ShortcutImportOptionsPayload = {
      import_archived: true,
      import_completed: true,
    },
    scanId?: string,
  ) =>
    jsonRequest<ShortcutImportPreviewResponse>(
      `/workspaces/${encodeURIComponent(workspaceId)}/import/shortcut/api/preview`,
      { api_token: apiToken, options, ...(scanId ? { scan_id: scanId } : {}) },
    ),

  executeShortcutAPI: (
    workspaceId: string,
    apiToken: string,
    userMappings: Record<string, string>,
    memberMappings: Record<string, string>,
    teamMappings: Record<string, string>,
    workflowStateMappings: WorkflowStateMappingPayload[],
    options: ShortcutImportOptionsPayload,
  ) =>
    jsonRequest<ShortcutImportExecuteResponse>(
      `/workspaces/${encodeURIComponent(workspaceId)}/import/shortcut/api/execute`,
      {
        api_token: apiToken,
        user_mappings: userMappings,
        member_mappings: memberMappings,
        team_mappings: teamMappings,
        workflow_state_mappings: workflowStateMappings,
        options,
      },
    ),

  getShortcutStatus: (workspaceId: string, importId: string) =>
    statusRequest<ShortcutImportStatusResponse>(
      `/workspaces/${encodeURIComponent(workspaceId)}/import/shortcut/status/${encodeURIComponent(importId)}`,
    ),

  getShortcutStatusDetail: (workspaceId: string, importId: string) =>
    statusRequest<ShortcutImportDetailResponse>(
      `/workspaces/${encodeURIComponent(workspaceId)}/import/shortcut/status/${encodeURIComponent(importId)}/detail`,
    ),

  cancelShortcutImport: (workspaceId: string, importId: string) =>
    postStatusRequest<ShortcutImportStatusResponse>(
      `/workspaces/${encodeURIComponent(workspaceId)}/import/shortcut/status/${encodeURIComponent(importId)}/cancel`,
    ),

  retryShortcutImport: (workspaceId: string, importId: string) =>
    postStatusRequest<ShortcutImportExecuteResponse>(
      `/workspaces/${encodeURIComponent(workspaceId)}/import/shortcut/status/${encodeURIComponent(importId)}/retry`,
    ),

  listShortcutStatuses: (workspaceId: string) =>
    statusRequest<ShortcutImportStatusResponse[]>(
      `/workspaces/${encodeURIComponent(workspaceId)}/import/shortcut/status`,
    ),
};
