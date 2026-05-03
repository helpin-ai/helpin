import { api, API_BASE } from '../api';

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

export interface ImportSummary {
  collections_created: number;
  articles_published: number;
  articles_drafted: number;
  redirects_created: number;
  articles_uncategorized: number;
  articles_with_conversion_warnings: number;
  html_block_fallbacks: number;
  image_rewrite_failures: number;
  normalized_note_blocks: number;
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
  summary: ImportSummary | null;
  started_by: string;
  started_at: string;
  completed_at: string | null;
  created_at: string;
}

// --- Nextra Import Types ---

export interface NextraImportSpacePreview {
  source_id: string;
  name: string;
  collection_count: number;
  article_count: number;
}

export interface NextraImportWarning {
  type: string;
  source_path?: string;
  message: string;
}

export interface NextraImportBrokenLink {
  source_path: string;
  target: string;
  message: string;
}

export interface NextraImportUnsupportedComponent {
  name: string;
  count: number;
}

export interface NextraImportPreviewResponse {
  job_id: string;
  archive_name: string;
  source_commit?: string;
  detected_root: string;
  spaces: NextraImportSpacePreview[];
  collections: number;
  articles: number;
  assets: number;
  redirects: number;
  warnings: NextraImportWarning[];
  broken_links: NextraImportBrokenLink[];
  unsupported_components: NextraImportUnsupportedComponent[];
}

export interface NextraImportStartRequest {
  job_id: string;
  target_space_id?: string;
  new_space_name?: string;
  import_status: 'draft' | 'published';
  create_redirects: boolean;
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
    api.post<{
      total: number;
      converted: number;
      failed: number;
      articles_with_warnings: number;
      html_block_fallbacks: number;
      normalized_note_blocks: number;
    }>(`/docs/import/${jobId}/reconvert${qs(workspaceId)}`, {}),

  // --- Nextra Import ---

  previewNextra: async (workspaceId: string, payload: { archive: File; source_commit?: string }) => {
    const formData = new FormData();
    formData.append('archive', payload.archive);
    if (payload.source_commit) {
      formData.append('source_commit', payload.source_commit);
    }
    try {
      const res = await fetch(
        `${API_BASE}/docs/import/nextra/preview${qs(workspaceId)}`,
        {
          method: 'POST',
          credentials: 'include',
          body: formData,
        },
      );
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        return { data: null, error: err.error || res.statusText };
      }
      const data: NextraImportPreviewResponse = await res.json();
      return { data, error: null };
    } catch (e) {
      return { data: null, error: e instanceof Error ? e.message : 'Network error' };
    }
  },

  startNextra: (workspaceId: string, payload: NextraImportStartRequest) =>
    api.post<{ job_id: string }>(`/docs/import/nextra/start${qs(workspaceId)}`, payload),
};
