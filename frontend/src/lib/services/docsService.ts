import { api, API_BASE, fetchWithSessionAuth } from '../api';
import type {
  DocsSpace,
  DocsCollection,
  DocsDocument,
  DocsContent,
  DocsBlock,
  AISectionCandidateResponse,
  DocsChangeProposal,
  DocsChangeProposalApplyResponse,
  DocsReferencesResponse,
  DocsVersion,
  DocsLink,
  DocsHelpcenterConfig,
  DocsHelpcenterArticle,
  DocsHelpcenterSpaceTranslation,
  DocsHelpcenterCollectionTranslation,
  DocsHelpcenterArticleTranslation,
  DocsHelpcenterLocalesConfig,
  DocsSearchResult,
  CreateDocsSpaceRequest,
  UpdateDocsSpaceRequest,
  CreateDocsCollectionRequest,
  DocsCollectionDeleteImpact,
  UpdateDocsCollectionRequest,
  CreateDocsDocumentRequest,
  UpdateDocsDocumentRequest,
  MoveDocsDocumentRequest,
  PublishDocsDocumentRequest,
  PublishDocsHelpcenterArticleTranslationRequest,
  SaveDocsContentRequest,
  CreateDocsVersionRequest,
  UpdateDocsVersionRequest,
  CreateDocsLinkRequest,
  UpdateDocsHelpcenterConfigRequest,
  UpdateDocsHelpcenterArticleMetadataRequest,
  UpdateDocsHelpcenterLocalesRequest,
  AutoTranslateMissingResponse,
  UpsertDocsHelpcenterSpaceTranslationRequest,
  UpsertDocsHelpcenterCollectionTranslationRequest,
  UpsertDocsHelpcenterArticleTranslationRequest,
  DocsArticleFeedbackRequest,
  PublicDocResponse,
  DocsResolvedEmbed,
  DocsEntityRefRequest,
  ResolveDocsEntityRefsResponse,
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
  listAllCollections: (wsId: string) =>
    api.get<DocsCollection[]>(`/docs/collections${qs(wsId)}`),
  listCollections: (wsId: string, spaceId: string) =>
    api.get<DocsCollection[]>(`/docs/spaces/${spaceId}/collections${qs(wsId)}`),
  createCollection: (wsId: string, spaceId: string, payload: CreateDocsCollectionRequest) =>
    api.post<DocsCollection>(`/docs/spaces/${spaceId}/collections${qs(wsId)}`, payload),
  updateCollection: (wsId: string, collectionId: string, payload: UpdateDocsCollectionRequest) =>
    api.patch<DocsCollection>(`/docs/collections/${collectionId}${qs(wsId)}`, payload),
  getCollectionDeleteImpact: (wsId: string, collectionId: string) =>
    api.get<DocsCollectionDeleteImpact>(`/docs/collections/${collectionId}/delete-impact${qs(wsId)}`),
  getSpaceDeleteImpact: (wsId: string, spaceId: string) =>
    api.get<import('../docsTypes').DocsSpaceDeleteImpact>(`/docs/spaces/${spaceId}/delete-impact${qs(wsId)}`),
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
  publishDocument: (wsId: string, docId: string, payload?: PublishDocsDocumentRequest) =>
    api.post<DocsDocument>(`/docs/documents/${docId}/publish${qs(wsId)}`, payload ?? {}),
  unpublishDocument: (wsId: string, docId: string) =>
    api.post<DocsDocument>(`/docs/documents/${docId}/unpublish${qs(wsId)}`),

  // ── Content ─────────────────────────────────────────────────────────────
  getContent: (wsId: string, docId: string) =>
    api.get<DocsContent>(`/docs/documents/${docId}/content${qs(wsId)}`),
  saveContent: (wsId: string, docId: string, payload: SaveDocsContentRequest) =>
    api.put<DocsContent>(`/docs/documents/${docId}/content${qs(wsId)}`, payload),
  saveMarkdownContent: (wsId: string, docId: string, markdown: string) =>
    api.put<DocsContent>(`/docs/documents/${docId}/content/markdown${qs(wsId)}`, { markdown }),
  listBlocks: (wsId: string, docId: string) =>
    api.get<DocsBlock[]>(`/docs/documents/${docId}/blocks${qs(wsId)}`),
  createBlock: (wsId: string, docId: string, payload: { after_block_id?: string; content: unknown }) =>
    api.post<DocsContent>(`/docs/documents/${docId}/blocks${qs(wsId)}`, payload),
  patchBlock: (wsId: string, docId: string, blockId: string, payload: { revision: number; content: unknown }) =>
    api.patch<DocsContent>(`/docs/documents/${docId}/blocks/${blockId}${qs(wsId)}`, payload),
  reorderBlocks: (wsId: string, docId: string, payload: { block_ids: string[] }) =>
    api.post<DocsContent>(`/docs/documents/${docId}/blocks/reorder${qs(wsId)}`, payload),
  deleteBlock: (wsId: string, docId: string, blockId: string) =>
    api.del<DocsContent>(`/docs/documents/${docId}/blocks/${blockId}${qs(wsId)}`),
  getAISectionCandidate: (wsId: string, docId: string, blockId: string) =>
    api.get<AISectionCandidateResponse>(`/docs/documents/${docId}/blocks/${blockId}/ai-section/candidate${qs(wsId)}`),
  regenerateAISection: (wsId: string, docId: string, blockId: string, payload: { agent_id: string; instructions?: string }) =>
    api.post<AISectionCandidateResponse>(`/docs/documents/${docId}/blocks/${blockId}/ai-section/regenerate${qs(wsId)}`, payload),
  approveAISection: (wsId: string, docId: string, blockId: string) =>
    api.post<AISectionCandidateResponse>(`/docs/documents/${docId}/blocks/${blockId}/ai-section/approve${qs(wsId)}`, {}),
  rejectAISection: (wsId: string, docId: string, blockId: string) =>
    api.post<AISectionCandidateResponse>(`/docs/documents/${docId}/blocks/${blockId}/ai-section/reject${qs(wsId)}`, {}),
  listChangeProposals: (wsId: string, docId: string) =>
    api.get<DocsChangeProposal[]>(`/docs/documents/${docId}/change-proposals${qs(wsId)}`),
  getChangeProposal: (wsId: string, docId: string, proposalId: string) =>
    api.get<DocsChangeProposal>(`/docs/documents/${docId}/change-proposals/${proposalId}${qs(wsId)}`),
  applyChangeProposal: (wsId: string, docId: string, proposalId: string) =>
    api.post<DocsChangeProposalApplyResponse>(`/docs/documents/${docId}/change-proposals/${proposalId}/apply${qs(wsId)}`, {}),
  discardChangeProposal: (wsId: string, docId: string, proposalId: string) =>
    api.post<DocsChangeProposal>(`/docs/documents/${docId}/change-proposals/${proposalId}/discard${qs(wsId)}`, {}),
  listReferences: (wsId: string, docId: string) =>
    api.get<DocsReferencesResponse>(`/docs/documents/${docId}/references${qs(wsId)}`),

  // ── Versions ────────────────────────────────────────────────────────────
  listVersions: (wsId: string, docId: string) =>
    api.get<DocsVersion[]>(`/docs/documents/${docId}/versions${qs(wsId)}`),
  getVersion: (wsId: string, docId: string, versionId: string) =>
    api.get<DocsVersion>(`/docs/documents/${docId}/versions/${versionId}${qs(wsId)}`),
  createVersion: (wsId: string, docId: string, payload: CreateDocsVersionRequest) =>
    api.post<DocsVersion>(`/docs/documents/${docId}/versions${qs(wsId)}`, payload),
  updateVersionLabel: (wsId: string, docId: string, versionId: string, payload: UpdateDocsVersionRequest) =>
    api.patch<DocsVersion>(`/docs/documents/${docId}/versions/${versionId}${qs(wsId)}`, payload),
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
  publishExternally: (wsId: string, docId: string, payload?: PublishDocsDocumentRequest) =>
    api.post(`/docs/documents/${docId}/publish-external${qs(wsId)}`, payload ?? {}),
  unpublishExternally: (wsId: string, docId: string) =>
    api.post(`/docs/documents/${docId}/unpublish-external${qs(wsId)}`),
  updateArticleSlug: (wsId: string, docId: string, slug: string) =>
    api.post(`/docs/documents/${docId}/update-slug${qs(wsId)}`, { slug }),

  // ── Search ──────────────────────────────────────────────────────────────
  search: (wsId: string, query: string, filters?: { status?: string; limit?: number }) => {
    const search = new URLSearchParams();
    search.set('workspace_id', wsId);
    search.set('q', query);
    if (filters?.status) search.set('status', filters.status);
    if (filters?.limit) search.set('limit', String(filters.limit));
    return api.get<DocsSearchResult[]>(`/docs/search?${search.toString()}`);
  },
  resolveEmbed: (wsId: string, url: string) =>
    api.get<DocsResolvedEmbed>(`/docs/embeds/resolve${qs(wsId)}&url=${encodeURIComponent(url)}`),
  resolveEntityRefs: (wsId: string, refs: DocsEntityRefRequest[]) =>
    api.post<ResolveDocsEntityRefsResponse>(`/docs/entity-refs/resolve${qs(wsId)}`, { refs }),

  // ── Help Center Config ──────────────────────────────────────────────────
  getHelpcenterConfig: (wsId: string) =>
    api.get<DocsHelpcenterConfig>(`/docs/helpcenter/config${qs(wsId)}`),
  updateHelpcenterConfig: (wsId: string, payload: UpdateDocsHelpcenterConfigRequest) =>
    api.put<DocsHelpcenterConfig>(`/docs/helpcenter/config${qs(wsId)}`, payload),
  getHelpcenterLocales: (wsId: string) =>
    api.get<DocsHelpcenterLocalesConfig>(`/docs/helpcenter/locales${qs(wsId)}`),
  updateHelpcenterLocales: (wsId: string, payload: UpdateDocsHelpcenterLocalesRequest) =>
    api.put<DocsHelpcenterLocalesConfig>(`/docs/helpcenter/locales${qs(wsId)}`, payload),
  listSpaceTranslations: (wsId: string, spaceId: string) =>
    api.get<DocsHelpcenterSpaceTranslation[]>(`/docs/spaces/${spaceId}/helpcenter/translations${qs(wsId)}`),
  upsertSpaceTranslation: (wsId: string, spaceId: string, payload: UpsertDocsHelpcenterSpaceTranslationRequest) =>
    api.put<DocsHelpcenterSpaceTranslation>(`/docs/spaces/${spaceId}/helpcenter/translations${qs(wsId)}`, payload),
  publishSpaceTranslation: (wsId: string, spaceId: string, locale: string, slug?: string) =>
    api.post<DocsHelpcenterSpaceTranslation>(
      `/docs/spaces/${spaceId}/helpcenter/translations/${encodeURIComponent(locale)}/publish${qs(wsId)}`,
      slug ? { slug } : undefined,
    ),
  unpublishSpaceTranslation: (wsId: string, spaceId: string, locale: string) =>
    api.post<DocsHelpcenterSpaceTranslation>(`/docs/spaces/${spaceId}/helpcenter/translations/${encodeURIComponent(locale)}/unpublish${qs(wsId)}`),
  markSpaceTranslationReviewed: (wsId: string, spaceId: string, locale: string) =>
    api.post<DocsHelpcenterSpaceTranslation>(`/docs/spaces/${spaceId}/helpcenter/translations/${encodeURIComponent(locale)}/mark-reviewed${qs(wsId)}`),
  generateSpaceTranslation: (wsId: string, spaceId: string, locale: string) =>
    api.post<DocsHelpcenterSpaceTranslation>(`/docs/spaces/${spaceId}/helpcenter/translations/${encodeURIComponent(locale)}/generate${qs(wsId)}`),
  listCollectionTranslations: (wsId: string, collectionId: string) =>
    api.get<DocsHelpcenterCollectionTranslation[]>(`/docs/collections/${collectionId}/helpcenter/translations${qs(wsId)}`),
  upsertCollectionTranslation: (wsId: string, collectionId: string, payload: UpsertDocsHelpcenterCollectionTranslationRequest) =>
    api.put<DocsHelpcenterCollectionTranslation>(`/docs/collections/${collectionId}/helpcenter/translations${qs(wsId)}`, payload),
  publishCollectionTranslation: (wsId: string, collectionId: string, locale: string, slug?: string) =>
    api.post<DocsHelpcenterCollectionTranslation>(
      `/docs/collections/${collectionId}/helpcenter/translations/${encodeURIComponent(locale)}/publish${qs(wsId)}`,
      slug ? { slug } : undefined,
    ),
  unpublishCollectionTranslation: (wsId: string, collectionId: string, locale: string) =>
    api.post<DocsHelpcenterCollectionTranslation>(`/docs/collections/${collectionId}/helpcenter/translations/${encodeURIComponent(locale)}/unpublish${qs(wsId)}`),
  markCollectionTranslationReviewed: (wsId: string, collectionId: string, locale: string) =>
    api.post<DocsHelpcenterCollectionTranslation>(`/docs/collections/${collectionId}/helpcenter/translations/${encodeURIComponent(locale)}/mark-reviewed${qs(wsId)}`),
  generateCollectionTranslation: (wsId: string, collectionId: string, locale: string) =>
    api.post<DocsHelpcenterCollectionTranslation>(`/docs/collections/${collectionId}/helpcenter/translations/${encodeURIComponent(locale)}/generate${qs(wsId)}`),
  autoTranslateMissing: (wsId: string, locale: string) =>
    api.post<AutoTranslateMissingResponse>(`/docs/helpcenter/translations/auto-translate-missing${qs(wsId)}`, { locale }),
  listArticleTranslations: (wsId: string, docId: string) =>
    api.get<DocsHelpcenterArticleTranslation[]>(`/docs/documents/${docId}/helpcenter/translations${qs(wsId)}`),
  upsertArticleTranslation: (wsId: string, docId: string, payload: UpsertDocsHelpcenterArticleTranslationRequest) =>
    api.put<DocsHelpcenterArticleTranslation>(`/docs/documents/${docId}/helpcenter/translations${qs(wsId)}`, payload),
  generateArticleTranslation: (wsId: string, docId: string, locale: string) =>
    api.post<DocsHelpcenterArticleTranslation>(`/docs/documents/${docId}/helpcenter/translations/${encodeURIComponent(locale)}/generate${qs(wsId)}`),
  publishArticleTranslation: (wsId: string, docId: string, payload: PublishDocsHelpcenterArticleTranslationRequest) =>
    api.post<DocsHelpcenterArticleTranslation>(
      `/docs/documents/${docId}/helpcenter/translations/${encodeURIComponent(payload.locale)}/publish${qs(wsId)}`,
      {
        ...(payload.slug ? { slug: payload.slug } : {}),
        ...(payload.published_content ? { published_content: payload.published_content } : {}),
      },
    ),
  updateArticleTranslationSlug: (wsId: string, docId: string, locale: string, slug: string) =>
    api.post(
      `/docs/documents/${docId}/helpcenter/translations/${encodeURIComponent(locale)}/update-slug${qs(wsId)}`,
      { slug },
    ),
  updateHelpcenterArticleMetadata: (wsId: string, docId: string, payload: UpdateDocsHelpcenterArticleMetadataRequest) =>
    api.put<DocsHelpcenterArticle>(`/docs/documents/${docId}/helpcenter/metadata${qs(wsId)}`, payload),
  unpublishArticleTranslation: (wsId: string, docId: string, locale: string) =>
    api.post<DocsHelpcenterArticleTranslation>(`/docs/documents/${docId}/helpcenter/translations/${encodeURIComponent(locale)}/unpublish${qs(wsId)}`),
  markArticleTranslationReviewed: (wsId: string, docId: string, locale: string) =>
    api.post<DocsHelpcenterArticleTranslation>(`/docs/documents/${docId}/helpcenter/translations/${encodeURIComponent(locale)}/mark-reviewed${qs(wsId)}`),
  uploadHelpcenterAsset: async (wsId: string, assetType: 'logo' | 'logo_dark' | 'favicon' | 'og_image', file: File): Promise<{ data: { url: string } | null; error: string | null }> => {
    const formData = new FormData();
    formData.append('file', file);
    try {
      const res = await fetchWithSessionAuth(API_BASE, `/docs/helpcenter/upload?workspace_id=${encodeURIComponent(wsId)}&type=${assetType}`, {
        method: 'POST',
        body: formData,
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        return { data: null, error: err.error || res.statusText };
      }
      const data = await res.json();
      return { data, error: null };
    } catch (e) {
      return { data: null, error: e instanceof Error ? e.message : 'Upload failed' };
    }
  },

  // ── Share Toggle ────────────────────────────────────────────────────────
  toggleDocShare: (wsId: string, docId: string, isPubliclyShared: boolean) =>
    api.post<DocsDocument>(`/docs/documents/${docId}/toggle-share${qs(wsId)}`, { is_publicly_shared: isPubliclyShared }),

  // ── Lock Toggle ────────────────────────────────────────────────────────
  toggleDocLock: (wsId: string, docId: string, isLocked: boolean) =>
    api.post<DocsDocument>(`/docs/documents/${docId}/toggle-lock${qs(wsId)}`, { is_locked: isLocked }),

  // ── Public Shared Document (no auth) ──────────────────────────────────
  getSharedDoc: (shareToken: string) =>
    api.get<PublicDocResponse>(`/docs/shared/${shareToken}`),

  // ── Reorder ────────────────────────────────────────────────────────────
  reorderSpaces: (wsId: string, data: import('../docsTypes').ReorderDocsSpacesRequest) =>
    api.put(`/docs/spaces/reorder${qs(wsId)}`, data),
  reorderCollections: (wsId: string, spaceId: string, data: import('../docsTypes').ReorderDocsCollectionsRequest) =>
    api.put(`/docs/spaces/${spaceId}/collections/reorder${qs(wsId)}`, data),
  reorderDocuments: (wsId: string, spaceId: string, data: import('../docsTypes').ReorderDocsDocumentsRequest) =>
    api.put(`/docs/spaces/${spaceId}/documents/reorder${qs(wsId)}`, data),
  reorderChildren: (wsId: string, spaceId: string, data: import('../docsTypes').ReorderDocsChildrenRequest) =>
    api.put(`/docs/spaces/${spaceId}/children/reorder${qs(wsId)}`, data),

  // Move a single item (doc or collection) to a specific position using sort keys.
  moveItem: (wsId: string, data: import('../docsTypes').MoveDocsItemRequest) =>
    api.post<{ message: string }>(`/docs/items/move${qs(wsId)}`, data),

  // ── Preview ────────────────────────────────────────────────────────────
  getPreviewToken: (wsId: string, docId: string) =>
    api.post<{
      token: string
      subdomain: string
      custom_domain?: string | null
    }>(`/docs/documents/${docId}/preview-token${qs(wsId)}`),
  importExternalImage: (wsId: string, imageUrl: string) =>
    api.post<{ url: string }>(`/docs/images/import${qs(wsId)}`, { image_url: imageUrl }),

  editImage: (wsId: string, docId: string, payload: { source_attachment_id: string; prompt: string; annotation_data_url?: string }) =>
    api.post<{ attachment_id: string }>(`/docs/documents/${docId}/images/edit${qs(wsId)}`, payload),

  // ── Help Center Article ─────────────────────────────────────────────────
  submitArticleFeedback: (wsId: string, docId: string, payload: DocsArticleFeedbackRequest) =>
    api.post(`/docs/articles/${docId}/feedback${qs(wsId)}`, payload),
};
