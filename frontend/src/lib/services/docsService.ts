import { api } from '../api';
import type {
  DocsSpace,
  DocsCollection,
  DocsDocument,
  DocsContent,
  DocsVersion,
  DocsLink,
  DocsHelpcenterConfig,
  DocsHelpcenterArticle,
  DocsSearchResult,
  CreateDocsSpaceRequest,
  UpdateDocsSpaceRequest,
  CreateDocsCollectionRequest,
  UpdateDocsCollectionRequest,
  CreateDocsDocumentRequest,
  UpdateDocsDocumentRequest,
  MoveDocsDocumentRequest,
  SaveDocsContentRequest,
  CreateDocsVersionRequest,
  CreateDocsLinkRequest,
  UpdateDocsHelpcenterConfigRequest,
  DocsArticleFeedbackRequest,
} from '../docsTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const docsService = {
  // ── Spaces ──────────────────────────────────────────────────────────────
  listSpaces: (wsId: string) =>
    api.get<DocsSpace[]>(`/docs/spaces${qs(wsId)}`),
  createSpace: (wsId: string, payload: CreateDocsSpaceRequest) =>
    api.post<DocsSpace>(`/docs/spaces${qs(wsId)}`, payload),
  getSpace: (wsId: string, spaceId: string) =>
    api.get<DocsSpace>(`/docs/spaces/${spaceId}${qs(wsId)}`),
  updateSpace: (wsId: string, spaceId: string, payload: UpdateDocsSpaceRequest) =>
    api.patch<DocsSpace>(`/docs/spaces/${spaceId}${qs(wsId)}`, payload),
  deleteSpace: (wsId: string, spaceId: string) =>
    api.del(`/docs/spaces/${spaceId}${qs(wsId)}`),
  restoreSpace: (wsId: string, spaceId: string) =>
    api.post<DocsSpace>(`/docs/spaces/${spaceId}/restore${qs(wsId)}`),

  // ── Collections ─────────────────────────────────────────────────────────
  listCollections: (wsId: string, spaceId: string) =>
    api.get<DocsCollection[]>(`/docs/spaces/${spaceId}/collections${qs(wsId)}`),
  createCollection: (wsId: string, spaceId: string, payload: CreateDocsCollectionRequest) =>
    api.post<DocsCollection>(`/docs/spaces/${spaceId}/collections${qs(wsId)}`, payload),
  updateCollection: (wsId: string, collectionId: string, payload: UpdateDocsCollectionRequest) =>
    api.patch<DocsCollection>(`/docs/collections/${collectionId}${qs(wsId)}`, payload),
  deleteCollection: (wsId: string, collectionId: string) =>
    api.del(`/docs/collections/${collectionId}${qs(wsId)}`),
  restoreCollection: (wsId: string, collectionId: string) =>
    api.post<DocsCollection>(`/docs/collections/${collectionId}/restore${qs(wsId)}`),

  // ── Documents ───────────────────────────────────────────────────────────
  listDocuments: (wsId: string, filters?: Record<string, string | undefined>) => {
    const search = new URLSearchParams();
    search.set('workspace_id', wsId);
    if (filters) {
      Object.entries(filters).forEach(([key, value]) => {
        if (value) search.set(key, value);
      });
    }
    return api.get<DocsDocument[]>(`/docs/documents?${search.toString()}`);
  },
  createDocument: (wsId: string, payload: CreateDocsDocumentRequest) =>
    api.post<DocsDocument>(`/docs/documents${qs(wsId)}`, payload),
  getDocument: (wsId: string, docId: string) =>
    api.get<DocsDocument>(`/docs/documents/${docId}${qs(wsId)}`),
  updateDocument: (wsId: string, docId: string, payload: UpdateDocsDocumentRequest) =>
    api.patch<DocsDocument>(`/docs/documents/${docId}${qs(wsId)}`, payload),
  deleteDocument: (wsId: string, docId: string) =>
    api.del(`/docs/documents/${docId}${qs(wsId)}`),
  restoreDocument: (wsId: string, docId: string) =>
    api.post<DocsDocument>(`/docs/documents/${docId}/restore${qs(wsId)}`),
  archiveDocument: (wsId: string, docId: string) =>
    api.post<DocsDocument>(`/docs/documents/${docId}/archive${qs(wsId)}`),
  unarchiveDocument: (wsId: string, docId: string) =>
    api.post<DocsDocument>(`/docs/documents/${docId}/unarchive${qs(wsId)}`),
  moveDocument: (wsId: string, docId: string, payload: MoveDocsDocumentRequest) =>
    api.post<DocsDocument>(`/docs/documents/${docId}/move${qs(wsId)}`, payload),
  publishDocument: (wsId: string, docId: string) =>
    api.post<DocsDocument>(`/docs/documents/${docId}/publish${qs(wsId)}`),

  // ── Content ─────────────────────────────────────────────────────────────
  getContent: (wsId: string, docId: string) =>
    api.get<DocsContent>(`/docs/documents/${docId}/content${qs(wsId)}`),
  saveContent: (wsId: string, docId: string, payload: SaveDocsContentRequest) =>
    api.put<DocsContent>(`/docs/documents/${docId}/content${qs(wsId)}`, payload),

  // ── Versions ────────────────────────────────────────────────────────────
  listVersions: (wsId: string, docId: string) =>
    api.get<DocsVersion[]>(`/docs/documents/${docId}/versions${qs(wsId)}`),
  createVersion: (wsId: string, docId: string, payload: CreateDocsVersionRequest) =>
    api.post<DocsVersion>(`/docs/documents/${docId}/versions${qs(wsId)}`, payload),
  revertVersion: (wsId: string, docId: string, versionId: string) =>
    api.post<DocsContent>(`/docs/documents/${docId}/revert/${versionId}${qs(wsId)}`),

  // ── Links ───────────────────────────────────────────────────────────────
  listLinks: (wsId: string, docId: string) =>
    api.get<DocsLink[]>(`/docs/documents/${docId}/links${qs(wsId)}`),
  createLink: (wsId: string, docId: string, payload: CreateDocsLinkRequest) =>
    api.post<DocsLink>(`/docs/documents/${docId}/links${qs(wsId)}`, payload),
  deleteLink: (wsId: string, linkId: string) =>
    api.del(`/docs/links/${linkId}${qs(wsId)}`),
  listLinkedDocs: (wsId: string, objectType: string, objectId: string) =>
    api.get<DocsLink[]>(`/docs/linked-docs/${objectType}/${objectId}${qs(wsId)}`),

  // ── External Publish ────────────────────────────────────────────────────
  publishExternally: (wsId: string, docId: string, slug?: string) =>
    api.post(`/docs/documents/${docId}/publish-external${qs(wsId)}`, slug ? { slug } : {}),
  unpublishExternally: (wsId: string, docId: string) =>
    api.post(`/docs/documents/${docId}/unpublish-external${qs(wsId)}`),

  // ── Search ──────────────────────────────────────────────────────────────
  search: (wsId: string, query: string, filters?: { doc_type?: string; status?: string; limit?: number }) => {
    const search = new URLSearchParams();
    search.set('workspace_id', wsId);
    search.set('q', query);
    if (filters?.doc_type) search.set('doc_type', filters.doc_type);
    if (filters?.status) search.set('status', filters.status);
    if (filters?.limit) search.set('limit', String(filters.limit));
    return api.get<DocsSearchResult[]>(`/docs/search?${search.toString()}`);
  },

  // ── Help Center Config ──────────────────────────────────────────────────
  getHelpcenterConfig: (wsId: string) =>
    api.get<DocsHelpcenterConfig>(`/docs/helpcenter/config${qs(wsId)}`),
  updateHelpcenterConfig: (wsId: string, payload: UpdateDocsHelpcenterConfigRequest) =>
    api.put<DocsHelpcenterConfig>(`/docs/helpcenter/config${qs(wsId)}`, payload),

  // ── Help Center Article ─────────────────────────────────────────────────
  submitArticleFeedback: (wsId: string, docId: string, payload: DocsArticleFeedbackRequest) =>
    api.post(`/docs/articles/${docId}/feedback${qs(wsId)}`, payload),
};
