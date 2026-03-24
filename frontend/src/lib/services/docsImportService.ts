import { api } from '../api';

const qs = (wsId: string) => `?workspace_id=${encodeURIComponent(wsId)}`;

export interface HelpscoutCollectionPreview {
  id: string;
  name: string;
  slug: string;
  category_count: number;
  article_count: number;
}

export interface ImportPreviewResponse {
  collections: HelpscoutCollectionPreview[];
}

export interface ImportFailure {
  article_id: string;
  title: string;
  error: string;
}

export interface ImportStatusResponse {
  id: string;
  status: 'pending' | 'running' | 'done' | 'failed' | 'interrupted';
  source: string;
  total: number;
  completed: number;
  failed: number;
  failures: ImportFailure[];
  redirect_map: unknown;
  started_by: string;
  started_at: string;
  completed_at: string | null;
  created_at: string;
}

export const docsImportService = {
  listJobs: (workspaceId: string) =>
    api.get<ImportStatusResponse[]>(`/docs/import/jobs${qs(workspaceId)}`),

  previewHelpscout: (workspaceId: string, apiKey: string) =>
    api.post<ImportPreviewResponse>(`/docs/import/helpscout/preview${qs(workspaceId)}`, { api_key: apiKey }),

  startHelpscout: (workspaceId: string, data: {
    api_key: string;
    helpscout_collection_id: string;
    target_space_id?: string;
    new_space_name?: string;
    import_status: 'draft' | 'published' | 'match_source';
  }) => api.post<{ job_id: string }>(`/docs/import/helpscout/start${qs(workspaceId)}`, data),

  getStatus: (workspaceId: string, jobId: string) =>
    api.get<ImportStatusResponse>(`/docs/import/${jobId}/status${qs(workspaceId)}`),

  retry: (workspaceId: string, jobId: string, apiKey: string) =>
    api.post(`/docs/import/${jobId}/retry${qs(workspaceId)}`, { api_key: apiKey }),

  getRedirectMap: (workspaceId: string, jobId: string) =>
    api.get<unknown>(`/docs/import/${jobId}/redirect-map${qs(workspaceId)}`),

  reconvert: (workspaceId: string, jobId: string) =>
    api.post<{ total: number; converted: number; failed: number }>(`/docs/import/${jobId}/reconvert${qs(workspaceId)}`, {}),
};
