import { API_BASE } from '../api';

export interface ShortcutImportPreviewSummary {
  total_stories: number;
  stories_by_type: Record<string, number>;
  epics_count: number;
  objectives_count: number;
  sprints_count: number;
  labels_count: number;
  teams_count: number;
  workflows_count: number;
  workflow_states_count: number;
  checklist_items_count: number;
  duplicate_stories: number;
}

export interface ShortcutWorkflowStatePreview {
  name: string;
  suggested_type: string;
  story_count: number;
}

export interface ShortcutWorkflowPreview {
  name: string;
  story_count: number;
  states: ShortcutWorkflowStatePreview[];
}

export interface ShortcutTeamPreview {
  name: string;
  story_count: number;
}

export interface ShortcutUserMatch {
  email: string;
  matched_user_id: string | null;
  matched_name: string | null;
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
  stories_created: number;
  stories_skipped: number;
  checklist_items_created: number;
  owner_links_created: number;
  label_links_created: number;
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
  status: 'pending' | 'processing' | 'completed' | 'failed';
  progress: ShortcutImportStatusProgress;
  result?: ShortcutImportResult;
  error?: string;
}

export interface ShortcutImportExecuteResponse {
  import_id: string;
  status: string;
}

export interface WorkflowStateMappingPayload {
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

// Multipart upload needs raw fetch (api.ts adds Content-Type: application/json)
async function multipartRequest<T>(path: string, form: FormData): Promise<{ data: T | null; error: string | null }> {
  const token = localStorage.getItem('access_token');
  try {
    const res = await fetch(`${API_BASE}${path}`, {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: form,
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

export const pmImportService = {
  previewShortcut: (workspaceId: string, file: File) => {
    const form = new FormData();
    form.append('file', file);
    return multipartRequest<ShortcutImportPreviewResponse>(
      `/workspaces/${encodeURIComponent(workspaceId)}/import/shortcut/preview`,
      form,
    );
  },

  executeShortcut: (
    workspaceId: string,
    file: File,
    userMappings: Record<string, string>,
    workflowStateMappings: WorkflowStateMappingPayload[],
    options: { import_archived: boolean; import_completed: boolean },
  ) => {
    const form = new FormData();
    form.append('file', file);
    form.append('user_mappings', JSON.stringify(userMappings));
    form.append('workflow_state_mappings', JSON.stringify(workflowStateMappings));
    form.append('options', JSON.stringify(options));
    return multipartRequest<ShortcutImportExecuteResponse>(
      `/workspaces/${encodeURIComponent(workspaceId)}/import/shortcut/execute`,
      form,
    );
  },

  getShortcutStatus: (workspaceId: string, importId: string) =>
    statusRequest<ShortcutImportStatusResponse>(
      `/workspaces/${encodeURIComponent(workspaceId)}/import/shortcut/status/${encodeURIComponent(importId)}`,
    ),
};
