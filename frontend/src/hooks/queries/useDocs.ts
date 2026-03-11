import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { docsService } from '@/lib/services/docsService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type {
  CreateDocsSpaceRequest,
  UpdateDocsSpaceRequest,
  CreateDocsCollectionRequest,
  UpdateDocsCollectionRequest,
  CreateDocsDocumentRequest,
  UpdateDocsDocumentRequest,
  MoveDocsDocumentRequest,
  SaveDocsContentRequest,
  CreateDocsVersionRequest,
  UpdateDocsVersionRequest,
  CreateDocsLinkRequest,
  UpdateDocsHelpcenterConfigRequest,
  DocsArticleFeedbackRequest,
  DocsDocument,
} from '@/lib/docsTypes'

// ── Spaces ──────────────────────────────────────────────────────────────────

export function useDocsSpaces(wsId: string) {
  return useQuery({
    queryKey: queryKeys.docs.spaces(wsId),
    queryFn: async () => unwrap(await docsService.listSpaces(wsId)),
    enabled: !!wsId,
  })
}

export function useDocsSpace(wsId: string, spaceId: string) {
  return useQuery({
    queryKey: queryKeys.docs.space(wsId, spaceId),
    queryFn: async () => unwrap(await docsService.getSpace(wsId, spaceId)),
    enabled: !!wsId && !!spaceId,
  })
}

export function useCreateDocsSpace(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateDocsSpaceRequest) =>
      unwrap(await docsService.createSpace(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.spaces(wsId) })
    },
  })
}

export function useUpdateDocsSpace(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateDocsSpaceRequest & { id: string }) =>
      unwrap(await docsService.updateSpace(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.spaces(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.docs.space(wsId, id) })
    },
  })
}

export function useDeleteDocsSpace(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await docsService.deleteSpace(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.spaces(wsId) })
    },
  })
}

export function useRestoreDocsSpace(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await docsService.restoreSpace(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.spaces(wsId) })
    },
  })
}

// ── Collections ─────────────────────────────────────────────────────────────

export function useDocsCollections(wsId: string, spaceId: string) {
  return useQuery({
    queryKey: queryKeys.docs.collections(wsId, spaceId),
    queryFn: async () => unwrap(await docsService.listCollections(wsId, spaceId)),
    enabled: !!wsId && !!spaceId,
  })
}

export function useCreateDocsCollection(wsId: string, spaceId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateDocsCollectionRequest) =>
      unwrap(await docsService.createCollection(wsId, spaceId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.collections(wsId, spaceId) })
    },
  })
}

export function useUpdateDocsCollection(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, spaceId, ...data }: UpdateDocsCollectionRequest & { id: string; spaceId: string }) =>
      unwrap(await docsService.updateCollection(wsId, id, data)),
    onSuccess: (_, { spaceId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.collections(wsId, spaceId) })
    },
  })
}

export function useDeleteDocsCollection(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id }: { id: string; spaceId: string }) =>
      unwrap(await docsService.deleteCollection(wsId, id)),
    onSuccess: (_, { spaceId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.collections(wsId, spaceId) })
    },
  })
}

export function useRestoreDocsCollection(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id }: { id: string; spaceId: string }) =>
      unwrap(await docsService.restoreCollection(wsId, id)),
    onSuccess: (_, { spaceId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.collections(wsId, spaceId) })
    },
  })
}

// ── Documents ───────────────────────────────────────────────────────────────

interface DocFilters {
  space_id?: string
  collection_id?: string
  status?: string
  owner_id?: string
  team_id?: string
  include_archived?: string
}

export function useDocsDocuments(wsId: string, filters?: DocFilters) {
  return useQuery({
    queryKey: [...queryKeys.docs.documents(wsId), filters],
    queryFn: async () => {
      const filterRecord: Record<string, string | undefined> = {}
      if (filters) {
        Object.entries(filters).forEach(([k, v]) => {
          filterRecord[k] = v
        })
      }
      console.log('[DEBUG] useDocsDocuments filterRecord:', filterRecord)
      const result = await docsService.listDocuments(wsId, filterRecord)
      console.log('[DEBUG] useDocsDocuments response:', result)
      return unwrap(result)
    },
    enabled: !!wsId,
  })
}

export function useDocsDocument(wsId: string, docId: string) {
  return useQuery({
    queryKey: queryKeys.docs.document(wsId, docId),
    queryFn: async () => unwrap(await docsService.getDocument(wsId, docId)),
    enabled: !!wsId && !!docId,
  })
}

export function useCreateDocsDocument(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateDocsDocumentRequest) =>
      unwrap(await docsService.createDocument(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.documents(wsId) })
    },
  })
}

export function useDuplicateDocsDocument(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (sourceDoc: DocsDocument) => {
      const duplicatedDoc = unwrap(await docsService.createDocument(wsId, {
        title: `${sourceDoc.title} (Copy)`,
        space_id: sourceDoc.space_id,
        collection_id: sourceDoc.collection_id,
        owner_id: sourceDoc.owner_id,
        template_key: sourceDoc.template_key,
        icon: sourceDoc.icon,
        tags: sourceDoc.tags,
      }))

      if (sourceDoc.excerpt) {
        await unwrap(await docsService.updateDocument(wsId, duplicatedDoc.id, {
          excerpt: sourceDoc.excerpt,
        }))
      }

      const sourceContent = await unwrap(await docsService.getContent(wsId, sourceDoc.id))
      if (sourceContent?.content) {
        const savedContent = await unwrap(await docsService.saveContent(wsId, duplicatedDoc.id, {
          content: sourceContent.content,
        }))
        qc.setQueryData(queryKeys.docs.content(wsId, duplicatedDoc.id), savedContent)
      }

      return unwrap(await docsService.getDocument(wsId, duplicatedDoc.id))
    },
    onSuccess: (duplicatedDoc) => {
      qc.setQueryData(queryKeys.docs.document(wsId, duplicatedDoc.id), duplicatedDoc)
      qc.invalidateQueries({
        predicate: (query) => {
          const k = query.queryKey
          return k[0] === 'docs' && k[1] === wsId && k[2] === 'documents'
            && (k.length === 3 || (k.length === 4 && typeof k[3] !== 'string'))
        },
      })
    },
  })
}

export function useUpdateDocsDocument(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateDocsDocumentRequest & { id: string }) =>
      unwrap(await docsService.updateDocument(wsId, id, data)),
    onSuccess: (updatedDoc, { id }) => {
      // Update document cache directly to avoid refetch cascade that disrupts the editor.
      // Prefix-matching on ['docs', wsId, 'documents'] would also invalidate the
      // content/versions/links queries, causing the TipTap editor to reset.
      qc.setQueryData(queryKeys.docs.document(wsId, id), updatedDoc)
      // Invalidate only document list queries (for nav/space views), not individual
      // document or nested content/versions/links queries.
      qc.invalidateQueries({
        predicate: (query) => {
          const k = query.queryKey
          return k[0] === 'docs' && k[1] === wsId && k[2] === 'documents'
            && (k.length === 3 || (k.length === 4 && typeof k[3] !== 'string'))
        },
      })
    },
  })
}

export function useDeleteDocsDocument(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await docsService.deleteDocument(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.documents(wsId) })
    },
  })
}

export function useRestoreDocsDocument(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await docsService.restoreDocument(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.documents(wsId) })
    },
  })
}

export function useArchiveDocsDocument(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await docsService.archiveDocument(wsId, id)),
    onSuccess: (updatedDoc, id) => {
      qc.setQueryData(queryKeys.docs.document(wsId, id), updatedDoc)
      qc.invalidateQueries({
        predicate: (query) => {
          const k = query.queryKey
          return k[0] === 'docs' && k[1] === wsId && k[2] === 'documents'
            && (k.length === 3 || (k.length === 4 && typeof k[3] !== 'string'))
        },
      })
    },
  })
}

export function useUnarchiveDocsDocument(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await docsService.unarchiveDocument(wsId, id)),
    onSuccess: (updatedDoc, id) => {
      qc.setQueryData(queryKeys.docs.document(wsId, id), updatedDoc)
      qc.invalidateQueries({
        predicate: (query) => {
          const k = query.queryKey
          return k[0] === 'docs' && k[1] === wsId && k[2] === 'documents'
            && (k.length === 3 || (k.length === 4 && typeof k[3] !== 'string'))
        },
      })
    },
  })
}

export function useMoveDocsDocument(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: MoveDocsDocumentRequest & { id: string }) =>
      unwrap(await docsService.moveDocument(wsId, id, data)),
    onSuccess: (updatedDoc, { id }) => {
      qc.setQueryData(queryKeys.docs.document(wsId, id), updatedDoc)
      qc.invalidateQueries({
        predicate: (query) => {
          const k = query.queryKey
          return k[0] === 'docs' && k[1] === wsId && k[2] === 'documents'
            && (k.length === 3 || (k.length === 4 && typeof k[3] !== 'string'))
        },
      })
    },
  })
}

export function usePublishDocsDocument(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, slug }: { id: string; slug?: string }) =>
      unwrap(await docsService.publishDocument(wsId, id, slug)),
    onSuccess: (updatedDoc, { id }) => {
      qc.setQueryData(queryKeys.docs.document(wsId, id), updatedDoc)
      qc.invalidateQueries({
        predicate: (query) => {
          const k = query.queryKey
          return k[0] === 'docs' && k[1] === wsId && k[2] === 'documents'
            && (k.length === 3 || (k.length === 4 && typeof k[3] !== 'string'))
        },
      })
    },
  })
}

export function useUnpublishDocsDocument(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await docsService.unpublishDocument(wsId, id)),
    onSuccess: (updatedDoc, id) => {
      qc.setQueryData(queryKeys.docs.document(wsId, id), updatedDoc)
      qc.invalidateQueries({
        predicate: (query) => {
          const k = query.queryKey
          return k[0] === 'docs' && k[1] === wsId && k[2] === 'documents'
            && (k.length === 3 || (k.length === 4 && typeof k[3] !== 'string'))
        },
      })
    },
  })
}

// ── Content ─────────────────────────────────────────────────────────────────

export function useDocsContent(wsId: string, docId: string) {
  return useQuery({
    queryKey: queryKeys.docs.content(wsId, docId),
    queryFn: async () => unwrap(await docsService.getContent(wsId, docId)),
    enabled: !!wsId && !!docId,
  })
}

export function useSaveDocsContent(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ docId, ...data }: SaveDocsContentRequest & { docId: string }) =>
      unwrap(await docsService.saveContent(wsId, docId, data)),
    onSuccess: (savedContent, { docId }) => {
      // Update content cache directly from the response — avoids an extra refetch
      qc.setQueryData(queryKeys.docs.content(wsId, docId), savedContent)
      // Refetch the document to pick up the server-updated updated_at.
      // exact: true prevents cascading to content/versions/links queries.
      qc.invalidateQueries({ queryKey: queryKeys.docs.document(wsId, docId), exact: true })
      // Invalidate document list queries so the docs list view shows updated times
      qc.invalidateQueries({
        predicate: (query) => {
          const k = query.queryKey
          return k[0] === 'docs' && k[1] === wsId && k[2] === 'documents'
            && (k.length === 3 || (k.length === 4 && typeof k[3] !== 'string'))
        },
      })
    },
  })
}

export function useSaveDocsMarkdown(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ docId, markdown }: { docId: string; markdown: string }) =>
      unwrap(await docsService.saveMarkdownContent(wsId, docId, markdown)),
    onSuccess: (savedContent, { docId }) => {
      qc.setQueryData(queryKeys.docs.content(wsId, docId), savedContent)
      qc.invalidateQueries({ queryKey: queryKeys.docs.document(wsId, docId), exact: true })
      qc.invalidateQueries({
        predicate: (query) => {
          const k = query.queryKey
          return k[0] === 'docs' && k[1] === wsId && k[2] === 'documents'
            && (k.length === 3 || (k.length === 4 && typeof k[3] !== 'string'))
        },
      })
    },
  })
}

// ── Versions ────────────────────────────────────────────────────────────────

export function useDocsVersions(wsId: string, docId: string) {
  return useQuery({
    queryKey: queryKeys.docs.versions(wsId, docId),
    queryFn: async () => unwrap(await docsService.listVersions(wsId, docId)),
    enabled: !!wsId && !!docId,
  })
}

export function useCreateDocsVersion(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ docId, ...data }: CreateDocsVersionRequest & { docId: string }) =>
      unwrap(await docsService.createVersion(wsId, docId, data)),
    onSuccess: (_, { docId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.versions(wsId, docId) })
    },
  })
}

export function useDocsVersion(wsId: string, docId: string, versionId: string) {
  return useQuery({
    queryKey: queryKeys.docs.version(wsId, docId, versionId),
    queryFn: async () => unwrap(await docsService.getVersion(wsId, docId, versionId)),
    enabled: !!wsId && !!docId && !!versionId,
  })
}

export function useUpdateDocsVersionLabel(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ docId, versionId, ...data }: UpdateDocsVersionRequest & { docId: string; versionId: string }) =>
      unwrap(await docsService.updateVersionLabel(wsId, docId, versionId, data)),
    onSuccess: (_, { docId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.versions(wsId, docId) })
    },
  })
}

export function useRevertDocsVersion(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ docId, versionId }: { docId: string; versionId: string }) =>
      unwrap(await docsService.revertVersion(wsId, docId, versionId)),
    onSuccess: (_, { docId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.content(wsId, docId) })
      qc.invalidateQueries({ queryKey: queryKeys.docs.versions(wsId, docId) })
    },
  })
}

// ── Links ───────────────────────────────────────────────────────────────────

export function useDocsLinks(wsId: string, docId: string) {
  return useQuery({
    queryKey: queryKeys.docs.links(wsId, docId),
    queryFn: async () => unwrap(await docsService.listLinks(wsId, docId)),
    enabled: !!wsId && !!docId,
  })
}

export function useCreateDocsLink(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ docId, ...data }: CreateDocsLinkRequest & { docId: string }) =>
      unwrap(await docsService.createLink(wsId, docId, data)),
    onSuccess: (_, { docId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.links(wsId, docId) })
    },
  })
}

export function useDeleteDocsLink(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ linkId }: { linkId: string; docId: string }) =>
      unwrap(await docsService.deleteLink(wsId, linkId)),
    onSuccess: (_, { docId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.links(wsId, docId) })
    },
  })
}

export function useDocsLinkedDocs(wsId: string, objectType: string, objectId: string) {
  return useQuery({
    queryKey: queryKeys.docs.linkedDocs(wsId, objectType, objectId),
    queryFn: async () => unwrap(await docsService.listLinkedDocs(wsId, objectType, objectId)),
    enabled: !!wsId && !!objectType && !!objectId,
  })
}

// ── External Publish ────────────────────────────────────────────────────────

export function usePublishDocsExternally(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ docId, slug }: { docId: string; slug?: string }) =>
      unwrap(await docsService.publishExternally(wsId, docId, slug)),
    onSuccess: (_, { docId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.document(wsId, docId) })
    },
  })
}

export function useUnpublishDocsExternally(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (docId: string) =>
      unwrap(await docsService.unpublishExternally(wsId, docId)),
    onSuccess: (_, docId) => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.document(wsId, docId) })
    },
  })
}

// ── Lock Toggle ──────────────────────────────────────────────────────────────

export function useToggleDocLock(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ docId, isLocked }: { docId: string; isLocked: boolean }) =>
      unwrap(await docsService.toggleDocLock(wsId, docId, isLocked)),
    onSuccess: (updatedDoc, { docId }) => {
      qc.setQueryData(queryKeys.docs.document(wsId, docId), updatedDoc)
    },
  })
}

// ── Share Toggle ─────────────────────────────────────────────────────────────

export function useToggleDocShare(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ docId, isPubliclyShared }: { docId: string; isPubliclyShared: boolean }) =>
      unwrap(await docsService.toggleDocShare(wsId, docId, isPubliclyShared)),
    onSuccess: (updatedDoc, { docId }) => {
      qc.setQueryData(queryKeys.docs.document(wsId, docId), updatedDoc)
    },
  })
}

// ── Search ──────────────────────────────────────────────────────────────────

export function useDocsSearch(wsId: string, query: string, filters?: { status?: string; limit?: number }) {
  return useQuery({
    queryKey: [...queryKeys.docs.search(wsId, query), filters],
    queryFn: async () => unwrap(await docsService.search(wsId, query, filters)),
    enabled: !!wsId && !!query,
  })
}

// ── Help Center Config ──────────────────────────────────────────────────────

export function useDocsHelpcenterConfig(wsId: string) {
  return useQuery({
    queryKey: queryKeys.docs.helpcenterConfig(wsId),
    queryFn: async () => unwrap(await docsService.getHelpcenterConfig(wsId)),
    enabled: !!wsId,
  })
}

export function useUpdateDocsHelpcenterConfig(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: UpdateDocsHelpcenterConfigRequest) =>
      unwrap(await docsService.updateHelpcenterConfig(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.docs.helpcenterConfig(wsId) })
    },
  })
}

// ── Article Feedback ────────────────────────────────────────────────────────

export function useSubmitDocsArticleFeedback(wsId: string) {
  return useMutation({
    mutationFn: async ({ docId, ...data }: DocsArticleFeedbackRequest & { docId: string }) =>
      unwrap(await docsService.submitArticleFeedback(wsId, docId, data)),
  })
}
