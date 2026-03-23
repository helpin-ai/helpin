import { api } from '../api';

const qs = (wsId: string) => `?workspace_id=${encodeURIComponent(wsId)}`;

export interface DocsRedirect {
  id: string;
  workspace_id: string;
  source_path: string;
  target_collection_slug: string;
  target_article_slug?: string;
  type: 'imported' | 'slug_change' | 'manual';
  source_system?: string;
  source_object_type?: string;
  source_object_id?: string;
  created_at: string;
}

export interface RedirectListResponse {
  items: DocsRedirect[];
  total: number;
}

export const docsRedirectService = {
  list: (wsId: string, params?: { search?: string; type?: string; page?: number; per_page?: number }) => {
    const searchParams = new URLSearchParams({ workspace_id: wsId });
    if (params?.search) searchParams.set('search', params.search);
    if (params?.type) searchParams.set('type', params.type);
    if (params?.page) searchParams.set('page', String(params.page));
    if (params?.per_page) searchParams.set('per_page', String(params.per_page));
    return api.get<RedirectListResponse>(`/docs/redirects?${searchParams}`);
  },
  create: (wsId: string, data: { source_path: string; target_collection_slug: string; target_article_slug?: string }) =>
    api.post<DocsRedirect>(`/docs/redirects${qs(wsId)}`, data),
  delete: (wsId: string, id: string) =>
    api.del(`/docs/redirects/${id}${qs(wsId)}`),
};
