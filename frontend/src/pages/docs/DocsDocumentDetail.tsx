import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate, useParams, useRouter } from '@tanstack/react-router'
import { format, parseISO } from 'date-fns'
import type { JSONContent } from '@tiptap/react'
import {
  ArrowLeft02Icon,
  ArchiveIcon,
  Calendar03Icon,
  Tick01Icon,
  ArrowRight01Icon,
  Clock01Icon,
  Copy01Icon,
  LeftToRightListBulletIcon,
  ViewIcon,
  LinkSquare01Icon,
  File01Icon,
  FolderOpenIcon,
  GlobeIcon,
  Link01Icon,
  MoreHorizontalIcon,
  RotateLeft01Icon,
  SentIcon,
  Delete01Icon,
  UserIcon,
  Cancel01Icon,
  Loading01Icon,
  MagicWand01Icon,
  Message01Icon,
  ArchiveRestoreIcon,
  FolderInputIcon,
  LockIcon,
  SquareUnlock01Icon,
  UserCheck01Icon,
} from '@/lib/icons'
import { ICON_MAP, StoredIcon } from '@/components/ui/icon-picker'
import { toast } from 'sonner'
import { useTitle } from '@/hooks/useTitle'
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard'
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams'
import { useAuthStore } from '@/stores/authStore'
import { useDocsPresenceStore } from '@/stores/docsPresenceStore'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useWSStore } from '@/hooks/useWebSocket'
import {
  useDocsDocument,
  useDocsContent,
  useDocsSpace,
  useDocsCollections,
  useDocsHelpcenterArticleTranslations,
  useDocsHelpcenterConfig,
  useDocsHelpcenterCollectionTranslations,
  useDocsHelpcenterLocales,
  useDocsHelpcenterSpaceTranslations,
  useCreateDocsCollection,
  useGenerateDocsHelpcenterArticleTranslation,
  usePublishDocsHelpcenterArticleTranslation,
  useUnpublishDocsHelpcenterArticleTranslation,
  useMarkDocsHelpcenterArticleTranslationReviewed,
  useSaveDocsContent,
  useUpdateDocsDocument,
  useUpsertDocsHelpcenterArticleTranslation,
  usePublishDocsDocument,
  useUnpublishDocsDocument,
  useArchiveDocsDocument,
  useUnarchiveDocsDocument,
  useDeleteDocsDocument,
  useAssignableMembers,
  useAgents,
  useWorkspaceAccess,
  usePermissions,
  useToggleDocShare,
  useToggleDocLock,
  useRevertDocsVersion,
} from '@/hooks/queries'
import { timeAgo } from '@/lib/utils'
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover'
import { formatAssignableMemberName } from '@/lib/assignableMembers'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { DocsEditor, type DocsCommentAnchor, type DocsCommentAnchorDecoration, type DocsEditingPresenceSignal } from '@/components/docs/DocsEditor'
import {
  buildCollectionTree,
  collectionAncestorChain,
  findCollectionNode,
  type CollectionTreeNode,
} from '@/components/docs/docsCollectionTree'
import { VersionHistoryPanel, VersionTypeBadge, AuthorDisplay } from '@/components/docs/VersionHistoryPanel'
import { DocumentLinksPanel } from '@/components/docs/DocumentLinksPanel'
import { MoveDocumentDialog } from '@/components/docs/MoveDocumentDialog'
import { EditArticleTranslationDialog } from '@/components/docs/helpcenter/EditArticleTranslationDialog'
import type { TranslationRow } from '@/components/docs/helpcenter/TranslationsPanel'
import { ArticleLocalePillRail } from '@/components/docs/helpcenter/ArticleLocalePillRail'
import { MissingArticleTranslationDialog } from '@/components/docs/helpcenter/MissingArticleTranslationDialog'
import { PublishSlugDialog } from '@/components/docs/helpcenter/PublishSlugDialog'
import { ConfirmDialog } from '@/components/pm/ConfirmDialog'
import { CommentThread } from '@/components/pm/CommentThread'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DOC_STATUS_LABELS, getHelpcenterLocaleLabel } from '@/lib/docsTypes'
import { suggestDocsSlug } from '@/lib/docsSlugs'
import { buildHelpcenterPreviewUrlFromEnv } from '@/lib/helpcenterPreview'
import { docsService } from '@/lib/services/docsService'
import { docsCommentService } from '@/lib/services/docsCommentService'
import { supportCoverageService } from '@/lib/services/supportCoverageService'
import { queryKeys } from '@/lib/queryKeys'
import type { DocsReferenceItem, DocsVersion, DocsHelpcenterTranslationState } from '@/lib/docsTypes'
import type { CommentWithAuthor } from '@/lib/pmTypes'
import { prepareDocsContentForPublish } from '@/lib/docsPublishTransforms'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import { AvatarGroupCount } from '@/components/ui/avatar'
import { UserAvatar } from '@/components/pm/UserAvatar'
import { loadCoverageHandoffContent } from '@/components/support/coverage/coverageHandoff'
import { useRegisterPageContext } from '@/components/command-bar/pageContext'

function docStatusColor(status: string): string {
  switch (status) {
    case 'published':
      return 'text-emerald-600 dark:text-emerald-400'
    case 'archived':
      return 'text-muted-foreground/60'
    default:
      return 'text-amber-600 dark:text-amber-400'
  }
}

function DocCollectionIcon({ name }: { name?: string | null }) {
  if (name) {
    const Icon = ICON_MAP[name];
    if (Icon) return <Icon className="h-3 w-3 shrink-0" />;
  }
  return <FolderOpenIcon className="h-3 w-3 shrink-0" />;
}

interface DocumentOutlineItem {
  index: number
  level: number
  text: string
}

function collectDocumentOutline(content: JSONContent | null | undefined): DocumentOutlineItem[] {
  const items: DocumentOutlineItem[] = []
  const walk = (node: JSONContent | undefined) => {
    if (!node) return
    if (node.type === 'heading') {
      const text = collectJSONText(node).trim()
      if (text) {
        items.push({
          index: items.length,
          level: typeof node.attrs?.level === 'number' ? node.attrs.level : 2,
          text,
        })
      }
    }
    node.content?.forEach(walk)
  }
  walk(content ?? undefined)
  return items
}

function collectJSONText(node: JSONContent): string {
  if (typeof node.text === 'string') return node.text
  return node.content?.map(collectJSONText).join('') ?? ''
}

function DocsOutlineSidebar({
  items,
  onSelect,
}: {
  items: DocumentOutlineItem[]
  onSelect: (index: number) => void
}) {
  if (items.length === 0) return null
  return (
    <aside className="hidden w-56 shrink-0 overflow-y-auto border-l border-border/60 px-3 py-5 xl:block">
      <div className="mb-2 flex items-center gap-2 text-[11px] font-semibold uppercase text-muted-foreground">
        <LeftToRightListBulletIcon className="h-3.5 w-3.5" />
        <span>Outline</span>
      </div>
      <div className="space-y-0.5">
        {items.map((item) => (
          <button
            key={`${item.index}:${item.text}`}
            type="button"
            className="block w-full truncate rounded px-2 py-1 text-left text-xs text-muted-foreground hover:bg-muted/60 hover:text-foreground"
            style={{ paddingLeft: `${8 + Math.max(0, item.level - 2) * 10}px` }}
            onClick={() => onSelect(item.index)}
          >
            {item.text}
          </button>
        ))}
      </div>
    </aside>
  )
}

interface ArticleTranslationDraftState {
  locale: string
  title: string
  slug: string
  excerpt: string
  content: JSONContent | null
}

interface SourceSocialDraftState {
  og_title: string
  og_description: string
  og_image_url: string
  og_image_alt: string
}

function emptyTranslationDraft(locale: string): ArticleTranslationDraftState {
  return {
    locale,
    title: '',
    slug: '',
    excerpt: '',
    content: null,
  }
}

function translationDraftFromTranslation(locale: string, translation?: {
  title?: string
  slug?: string
  excerpt?: string | null
  seo_title?: string | null
  seo_description?: string | null
  content?: unknown
} | null): ArticleTranslationDraftState {
  return {
    locale,
    title: translation?.title ?? '',
    slug: translation?.slug ?? '',
    excerpt: translation?.excerpt ?? '',
    content: (translation?.content as JSONContent | null | undefined) ?? null,
  }
}

function translationDraftKey(docId: string, locale: string): string {
  return `${docId}:${locale}`
}

const EMPTY_DOC_VIEWERS: Record<string, never> = {}
const EMPTY_DOC_EDITORS: Record<string, never> = {}

function formatEditorPresenceLabel(area: string, section?: string): string {
  if (area === 'title') return 'editing the title'
  if (section?.trim()) return `editing ${section.trim()}`
  return 'editing the body'
}

function EditingIndicator() {
  return (
    <span className="absolute -bottom-1 left-1/2 inline-flex -translate-x-1/2 items-center gap-0.5 rounded-full border border-amber-500/30 bg-background/95 px-1 py-0.5 shadow-sm">
      {[0, 1, 2].map((index) => (
        <span
          key={index}
          className="h-1 w-1 animate-bounce rounded-full bg-amber-600 dark:bg-amber-300"
          style={{
            animationDelay: `${index * 0.14}s`,
          }}
        />
      ))}
    </span>
  )
}

export function DocsDocumentDetail({
  fromGapId,
  fromSuggestionId,
}: {
  fromGapId?: string
  fromSuggestionId?: string
} = {}) {
  const navigate = useNavigate()
  const router = useRouter()
  const queryClient = useQueryClient()
  const { docId } = useParams({ strict: false }) as { docId: string }
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''
  const coverageGapClosedRef = useRef(false)
  const editorShellRef = useRef<HTMLDivElement | null>(null)
  const [coverageInitialContent] = useState(() =>
    loadCoverageHandoffContent(fromGapId, fromSuggestionId),
  )

  const { data: access } = useWorkspaceAccess(wsId)
  const { canEditDocs, canPublishDocs, canAdminDocs, isAdmin } = usePermissions(access)
  const currentUserId = useAuthStore((s) => s.user?.id)

  const { data: doc, isLoading: docLoading } = useDocsDocument(wsId, docId)
  useRegisterPageContext(doc ? {
    entity_type: 'document',
    entity_id: doc.id,
    display_title: doc.title,
  } : null, 20)
  const { data: content, isLoading: contentLoading } = useDocsContent(wsId, docId)
  const { data: localesConfig } = useDocsHelpcenterLocales(wsId)
  const { data: articleTranslations = [] } = useDocsHelpcenterArticleTranslations(wsId, docId)

  const saveContent = useSaveDocsContent(wsId)
  const updateDoc = useUpdateDocsDocument(wsId)
  const publishDoc = usePublishDocsDocument(wsId)
  const unpublishDoc = useUnpublishDocsDocument(wsId)
  const archiveDoc = useArchiveDocsDocument(wsId)
  const unarchiveDoc = useUnarchiveDocsDocument(wsId)
  const deleteDoc = useDeleteDocsDocument(wsId)

  const { data: space } = useDocsSpace(wsId, doc?.space_id ?? '')
  const { data: members = [] } = useAssignableMembers(wsId)
  const { teams = [] } = useWorkspaceTeams(wsId)
  const { data: workspaceAgents = [] } = useAgents(wsId)
  const documentAgents = useMemo(
    () => workspaceAgents.filter((agent) => agent.allowed_targets?.includes('document')),
    [workspaceAgents],
  )
  const remoteViewers = useDocsPresenceStore((s) => s.viewingUsers[docId] ?? EMPTY_DOC_VIEWERS)
  const remoteEditors = useDocsPresenceStore((s) => s.editingUsers[docId] ?? EMPTY_DOC_EDITORS)
  const wsSendRaw = useWSStore((s) => s.send)
  const { data: collections = [] } = useDocsCollections(wsId, doc?.space_id ?? '')
  const { data: spaceTranslations = [] } = useDocsHelpcenterSpaceTranslations(wsId, doc?.space_id ?? '')
  const { data: collectionTranslations = [] } = useDocsHelpcenterCollectionTranslations(wsId, doc?.collection_id ?? '')
  const createCollection = useCreateDocsCollection(wsId, doc?.space_id ?? '')
  const upsertArticleTranslation = useUpsertDocsHelpcenterArticleTranslation(wsId, docId)
  const generateArticleTranslation = useGenerateDocsHelpcenterArticleTranslation(wsId, docId)
  const publishArticleTranslation = usePublishDocsHelpcenterArticleTranslation(wsId, docId)
  const unpublishArticleTranslation = useUnpublishDocsHelpcenterArticleTranslation(wsId, docId)
  const markArticleTranslationReviewed = useMarkDocsHelpcenterArticleTranslationReviewed(wsId, docId)

  const toggleShare = useToggleDocShare(wsId)
  const toggleLock = useToggleDocLock(wsId)

  const collectionBreadcrumbNodes = useMemo<CollectionTreeNode[]>(() => {
    if (!doc?.space_id || !doc.collection_id || collections.length === 0) return []
    const tree = buildCollectionTree(doc.space_id, collections, [])
    const current = findCollectionNode(tree.topLevel, doc.collection_id)
    if (!current) return []
    return [...collectionAncestorChain(tree.topLevel, doc.collection_id), current]
  }, [collections, doc?.collection_id, doc?.space_id])

  const activeDocViewers = useMemo(() => {
    return Object.entries(remoteViewers).map(([userId, viewer]) => {
      const member = members.find((candidate) => candidate.user_id === userId)
      return {
        userId,
        name: viewer.name ?? (member ? formatAssignableMemberName(member) : 'Teammate'),
        avatarUrl: viewer.avatarUrl ?? member?.avatar_url ?? undefined,
      }
    })
  }, [members, remoteViewers])

  const activeDocEditors = useMemo(() => {
    return Object.entries(remoteEditors).map(([userId, editor]) => {
      const member = members.find((candidate) => candidate.user_id === userId)
      const name = editor.name ?? (member ? formatAssignableMemberName(member) : 'Teammate')
      return {
        userId,
        name,
        avatarUrl: editor.avatarUrl ?? member?.avatar_url ?? undefined,
        area: editor.area,
        section: editor.section,
        label: formatEditorPresenceLabel(editor.area, editor.section),
      }
    })
  }, [members, remoteEditors])

  const headerPresencePeople = useMemo(() => {
    const editorsById = new Map(activeDocEditors.map((editor) => [editor.userId, editor]))
    const viewers = activeDocViewers
      .filter((viewer) => !editorsById.has(viewer.userId))
      .map((viewer) => ({
        ...viewer,
        isEditing: false,
        tooltip: `${viewer.name} is reading this doc`,
      }))

    const editors = activeDocEditors.map((editor) => ({
      ...editor,
      isEditing: true,
      tooltip: `${editor.name} is ${editor.label}`,
    }))

    return [...editors, ...viewers]
  }, [activeDocEditors, activeDocViewers])

  useEffect(() => {
    let cancelled = false
    if (!wsId || !docId || !doc) {
      setComments([])
      return
    }
    setCommentsLoading(true)
    docsCommentService.list(wsId, 'doc', docId).then(({ data }) => {
      if (!cancelled) setComments(data ?? [])
    }).finally(() => {
      if (!cancelled) setCommentsLoading(false)
    })
    return () => {
      cancelled = true
    }
  }, [doc?.id, docId, wsId])

  // Lock-aware editing: locked docs are read-only for everyone — unlock to edit
  const effectiveReadOnly = !canEditDocs || doc?.status === 'archived' || !!doc?.is_locked
  const canUnlock = doc?.is_locked && (doc.locked_by === currentUserId || isAdmin)

  const [metaOpen, setMetaOpen] = useState(false)
  const [versionsOpen, setVersionsOpen] = useState(false)
  const [moveDialogOpen, setMoveDialogOpen] = useState(false)
  const [linksOpen, setLinksOpen] = useState(false)
  const [commentsOpen, setCommentsOpen] = useState(false)
  const [comments, setComments] = useState<CommentWithAuthor[]>([])
  const [commentsLoading, setCommentsLoading] = useState(false)
  const [commentAnchor, setCommentAnchor] = useState<DocsCommentAnchor | null>(null)
  const [activeCommentId, setActiveCommentId] = useState<string | null>(null)
  const [referencesOpen, setReferencesOpen] = useState(false)
  const [references, setReferences] = useState<DocsReferenceItem[]>([])
  const [referencesLoading, setReferencesLoading] = useState(false)
  const [editingTranslationLocale, setEditingTranslationLocale] = useState<string | null>(null)
  const [pendingTranslationLocale, setPendingTranslationLocale] = useState<string | null>(null)
  const [regenerateConfirmOpen, setRegenerateConfirmOpen] = useState(false)
  const [regenerating, setRegenerating] = useState(false)
  const [generatingParents, setGeneratingParents] = useState(false)
  const [parentPublishConfirmOpen, setParentPublishConfirmOpen] = useState(false)
  const [revertConfirmOpen, setRevertConfirmOpen] = useState(false)
  const [slugDialogOpen, setSlugDialogOpen] = useState(false)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [pendingSlug, setPendingSlug] = useState('')
  const [pendingPublishLocale, setPendingPublishLocale] = useState<string | null>(null)
  const { copied: linkCopied, copy: copyLink } = useCopyToClipboard()

  // Version preview state — when set, the editor shows version content read-only
  const [previewVersion, setPreviewVersion] = useState<DocsVersion | null>(null)
  const revertVersion = useRevertDocsVersion(wsId)

  useEffect(() => {
    let cancelled = false
    if (!referencesOpen || !wsId || !docId || !doc) return
    setReferencesLoading(true)
    docsService.listReferences(wsId, docId).then(({ data }) => {
      if (!cancelled) setReferences(data?.items ?? [])
    }).finally(() => {
      if (!cancelled) setReferencesLoading(false)
    })
    return () => {
      cancelled = true
    }
  }, [doc?.id, docId, referencesOpen, wsId])

  const handlePreview = useCallback((version: DocsVersion) => {
    setPreviewVersion((prev) => prev?.id === version.id ? null : version)
  }, [])

  const handleRestoreFromPreview = useCallback(async () => {
    if (!previewVersion) return
    try {
      await revertVersion.mutateAsync({ docId, versionId: previewVersion.id })
      toast.success('Reverted to selected version')
      setPreviewVersion(null)
      setVersionsOpen(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to revert')
    }
  }, [previewVersion, revertVersion, docId])

  // Title state — keep a local draft only for the active document.
  const [titleDraftState, setTitleDraftState] = useState<{ docId: string; value: string } | null>(null)
  const titleDraft = titleDraftState?.docId === docId ? titleDraftState.value : (doc?.title ?? '')
  const titleTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)
  const [selectedLocaleState, setSelectedLocaleState] = useState<{ docId: string; locale: string } | null>(null)
  const [translationDrafts, setTranslationDrafts] = useState<Record<string, ArticleTranslationDraftState>>({})
  const [sourceSocialDraft, setSourceSocialDraft] = useState<SourceSocialDraftState>({
    og_title: '',
    og_description: '',
    og_image_url: '',
    og_image_alt: '',
  })
  const [savingSourceSocial, setSavingSourceSocial] = useState(false)
  const [uploadingSourceOGImage, setUploadingSourceOGImage] = useState(false)
  const sourceOGImageInputRef = useRef<HTMLInputElement>(null)
  const translationSaveTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  const patchDoc = useCallback(
    async (patch: Record<string, unknown>) => {
      try {
        await updateDoc.mutateAsync({ id: docId, ...patch })
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Failed to update')
      }
    },
    [updateDoc, docId],
  )

  const handleTitleChange = useCallback(
    (newTitle: string) => {
      setTitleDraftState({ docId, value: newTitle })
      if (titleTimerRef.current) clearTimeout(titleTimerRef.current)
      titleTimerRef.current = setTimeout(() => {
        const trimmed = newTitle.trim()
        if (trimmed && trimmed !== doc?.title) {
          patchDoc({ title: trimmed })
        }
      }, 800)
    },
    [patchDoc, doc?.title, docId],
  )

  // Cleanup title timer
  useEffect(() => () => {
    if (titleTimerRef.current) clearTimeout(titleTimerRef.current)
  }, [])

  useEffect(() => {
    setSourceSocialDraft({
      og_title: doc?.hc_og_title ?? '',
      og_description: doc?.hc_og_description ?? '',
      og_image_url: doc?.hc_og_image_url ?? '',
      og_image_alt: doc?.hc_og_image_alt ?? '',
    })
  }, [doc?.id, doc?.hc_og_title, doc?.hc_og_description, doc?.hc_og_image_url, doc?.hc_og_image_alt])

  const handleSaveSourceSocial = useCallback(async () => {
    setSavingSourceSocial(true)
    try {
      const res = await docsService.updateHelpcenterArticleMetadata(wsId, docId, sourceSocialDraft)
      if (res.error) throw new Error(res.error)
      toast.success('Social metadata saved')
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.document(wsId, docId) })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save social metadata')
    } finally {
      setSavingSourceSocial(false)
    }
  }, [docId, queryClient, sourceSocialDraft, wsId])

  const handleUploadSourceOGImage = useCallback(async (file?: File | null) => {
    if (!file) return
    if (!file.type.startsWith('image/')) {
      toast.error('Please select an image file')
      return
    }
    setUploadingSourceOGImage(true)
    try {
      const res = await docsService.uploadHelpcenterAsset(wsId, 'og_image', file)
      if (res.error || !res.data) throw new Error(res.error ?? 'Upload failed')
      setSourceSocialDraft((prev) => ({ ...prev, og_image_url: res.data!.url }))
      toast.success('Social image uploaded')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setUploadingSourceOGImage(false)
      if (sourceOGImageInputRef.current) sourceOGImageInputRef.current.value = ''
    }
  }, [wsId])

  useEffect(() => {
    if (!docId || !wsSendRaw) return

    wsSendRaw({ type: 'docs:presence:sync', data: { document_ids: [docId] } })
    wsSendRaw({ type: 'docs:viewing:start', data: { document_id: docId } })

    return () => {
      wsSendRaw({ type: 'docs:viewing:stop', data: { document_id: docId } })
    }
  }, [docId, wsSendRaw])

  const handleEditingPresenceChange = useCallback((presence: DocsEditingPresenceSignal | null) => {
    if (!docId || !wsSendRaw) return
    if (presence) {
      wsSendRaw({
        type: 'docs:editing:update',
        data: {
          document_id: docId,
          area: presence.area,
          section: presence.section,
        },
      })
      return
    }
    wsSendRaw({
      type: 'docs:editing:stop',
      data: {
        document_id: docId,
      },
    })
  }, [docId, wsSendRaw])

  const handleCreateCommentAnchor = useCallback((anchor: DocsCommentAnchor) => {
    setCommentAnchor(anchor)
    setActiveCommentId(null)
    setCommentsOpen(true)
  }, [])

  const handleOpenComment = useCallback((commentId: string) => {
    setActiveCommentId(commentId)
    setCommentsOpen(true)
    window.setTimeout(() => {
      document
        .querySelector(`[data-comment-thread-id="${CSS.escape(commentId)}"]`)
        ?.scrollIntoView({ block: 'center', behavior: 'smooth' })
    }, 0)
  }, [])

  const commentAnchors = useMemo<DocsCommentAnchorDecoration[]>(() => {
    return comments
      .flatMap((entry) => [entry, ...(entry.replies ?? [])])
      .map((entry) => entry.comment)
      .filter((comment) => comment.block_id || comment.range || comment.anchor_text)
      .map((comment) => ({
        id: comment.id,
        block_id: comment.block_id,
        range: comment.range,
        anchor_text: comment.anchor_text,
      }))
  }, [comments])

  const handleSave = useCallback(
    async (json: JSONContent) => {
      await saveContent.mutateAsync({ docId, content: json })
      if (fromGapId && fromSuggestionId && !coverageGapClosedRef.current) {
        coverageGapClosedRef.current = true
        const result = await supportCoverageService.addDocumentToGap(wsId, fromGapId, {
          route: 'update_article',
          target_document_id: docId,
        })
        if (result.error) throw new Error(result.error)
      }
      // Invalidate translation data so needs_review status updates promptly
      if (space?.type === 'external_capable' && (localesConfig?.enabled_locales?.length ?? 0) > 1) {
        queryClient.invalidateQueries({ queryKey: queryKeys.docs.documents(wsId) })
      }
    },
    [
      saveContent,
      docId,
      fromGapId,
      fromSuggestionId,
      wsId,
      space?.type,
      localesConfig?.enabled_locales?.length,
      queryClient,
    ],
  )

  const isExternalHelpCenter = space?.type === 'external_capable'
  const { data: helpcenterConfig } = useDocsHelpcenterConfig(wsId)
  const defaultLocale = localesConfig?.default_locale ?? 'en'
  const enabledLocales = localesConfig?.enabled_locales?.length
    ? localesConfig.enabled_locales
    : [defaultLocale]
  const articleTranslationsByLocale = useMemo(
    () => new Map(articleTranslations.map((translation) => [translation.locale, translation])),
    [articleTranslations],
  )
  const spaceTranslationsByLocale = useMemo(
    () => new Map(spaceTranslations.map((translation) => [translation.locale, translation])),
    [spaceTranslations],
  )
  const collectionTranslationsByLocale = useMemo(
    () => new Map(collectionTranslations.map((translation) => [translation.locale, translation])),
    [collectionTranslations],
  )
  const articleTranslationRows: TranslationRow[] = enabledLocales.map((locale) => {
    const translation = articleTranslationsByLocale.get(locale)
    const isDefaultLocale = locale === defaultLocale
    const spaceReady = spaceTranslationsByLocale.get(locale)?.status === 'published'
    const collectionReady = doc?.collection_id
      ? collectionTranslationsByLocale.get(locale)?.status === 'published'
      : true
    let publishBlockedReason: string | undefined
    if (!isDefaultLocale && translation) {
      if (!spaceReady) publishBlockedReason = 'Publish the parent translation first'
      else if (!collectionReady) publishBlockedReason = 'Publish the collection translation first'
    }
    return {
      locale,
      state: translation?.status ?? 'missing',
      updatedAtLabel: translation ? `Updated ${timeAgo(translation.updated_at)}` : undefined,
      helperText: isDefaultLocale
        ? 'Mirrored from the source article and refreshed automatically when the source document changes.'
        : translation?.source_synced === false
          ? 'Source content changed after this translation was last reviewed. Update and confirm the localized copy.'
          : 'Localized article content is served on locale-specific public routes and search results.',
      publishBlockedReason,
      isDefaultLocale,
      sourceMirrorLabel: 'Source mirror',
    }
  })
  const localeRowsByLocale = useMemo(
    () => new Map(articleTranslationRows.map((row) => [row.locale, row])),
    [articleTranslationRows],
  )
  const showLocalePills = isExternalHelpCenter && canAdminDocs && enabledLocales.length > 1
  const activeLocaleCandidate = selectedLocaleState?.docId === docId ? selectedLocaleState.locale : defaultLocale
  const activeLocale = enabledLocales.includes(activeLocaleCandidate) ? activeLocaleCandidate : defaultLocale
  const isSourceLocaleActive = activeLocale === defaultLocale
  const activeLocaleRow = localeRowsByLocale.get(activeLocale)
  const activeTranslation = !isSourceLocaleActive ? articleTranslationsByLocale.get(activeLocale) ?? null : null
  const activeTranslationStatus = activeLocaleRow?.state ?? 'missing'
  const activeTranslationDraft = isSourceLocaleActive
    ? emptyTranslationDraft(activeLocale)
    : translationDrafts[translationDraftKey(docId, activeLocale)] ?? translationDraftFromTranslation(activeLocale, activeTranslation)
  const displayedTitle = isSourceLocaleActive ? titleDraft : activeTranslationDraft.title
  const outlineContent = previewVersion
    ? previewVersion.content as JSONContent | null
    : isSourceLocaleActive
      ? coverageInitialContent ?? (content?.content as JSONContent | null)
      : activeTranslationDraft.content
  const outlineItems = useMemo(() => collectDocumentOutline(outlineContent), [outlineContent])
  const handleOutlineSelect = useCallback((index: number) => {
    const headings = editorShellRef.current?.querySelectorAll('.docs-editor-prose h1, .docs-editor-prose h2, .docs-editor-prose h3, .docs-editor-prose h4, .docs-editor-prose h5, .docs-editor-prose h6')
    const heading = headings?.[index] as HTMLElement | undefined
    heading?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }, [])

  const preparePublishedContent = useCallback(async (rawContent: JSONContent | null | undefined) => {
    if (!isExternalHelpCenter || !rawContent) return undefined
    return prepareDocsContentForPublish(rawContent, {
      uploadConfig: { workspaceId: wsId, entityType: 'editor_upload', entityId: docId },
      diagramTheme: { brandColor: helpcenterConfig?.brand_color },
    })
  }, [docId, helpcenterConfig?.brand_color, isExternalHelpCenter, wsId])

  useTitle(displayedTitle || 'Document')

  const handlePublish = async () => {
    // For external help center articles, show slug confirmation first
    if (isExternalHelpCenter && doc?.status === 'draft') {
      setPendingSlug(suggestDocsSlug(doc.title ?? 'untitled'))
      setPendingPublishLocale(null)
      setSlugDialogOpen(true)
      return
    }
    try {
      const publishedContent = await preparePublishedContent(content?.content as JSONContent | null | undefined)
      await publishDoc.mutateAsync({ id: docId, published_content: publishedContent })
      toast.success('Document published')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to publish')
    }
  }

  const handleConfirmPublish = async () => {
    try {
      if (pendingPublishLocale && pendingPublishLocale !== defaultLocale) {
        const draft = translationDrafts[translationDraftKey(docId, pendingPublishLocale)]
          ?? translationDraftFromTranslation(pendingPublishLocale, articleTranslationsByLocale.get(pendingPublishLocale) ?? null)
        const publishedContent = await preparePublishedContent(draft.content as JSONContent | null | undefined)
        await publishArticleTranslation.mutateAsync({
          locale: pendingPublishLocale,
          slug: pendingSlug,
          published_content: publishedContent,
        })
        toast.success(`${getHelpcenterLocaleLabel(pendingPublishLocale)} translation published`)
      } else {
        const publishedContent = await preparePublishedContent(content?.content as JSONContent | null | undefined)
        await publishDoc.mutateAsync({ id: docId, slug: pendingSlug, published_content: publishedContent })
        toast.success('Document published')
      }
      setSlugDialogOpen(false)
      setPendingPublishLocale(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to publish')
    }
  }

  const handleArchive = async () => {
    try {
      await archiveDoc.mutateAsync(docId)
      toast.success('Document archived')
      navigate({ to: '/w/$slug/docs', params: { slug: wsSlug } })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to archive')
    }
  }

  const handleUnarchive = async () => {
    try {
      await unarchiveDoc.mutateAsync(docId)
      toast.success('Document restored from archive')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to unarchive')
    }
  }

  const handleDelete = async () => {
    try {
      await deleteDoc.mutateAsync(docId)
      toast.success('Document deleted')
      navigate({ to: '/w/$slug/docs', params: { slug: wsSlug } })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete')
    }
  }

  const handleSaveTranslation = useCallback(async (data: Parameters<typeof upsertArticleTranslation.mutateAsync>[0]) => {
    try {
      await upsertArticleTranslation.mutateAsync(data)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save translation')
      throw err
    }
  }, [upsertArticleTranslation])

  const buildTranslationPayload = useCallback((draft: ArticleTranslationDraftState) => {
    const title = draft.title.trim()
    if (!title) return null

    return {
      locale: draft.locale,
      title,
      slug: draft.slug.trim() || undefined,
      excerpt: draft.excerpt.trim() || undefined,
      content: (draft.content as JSONContent | null) ?? { type: 'doc', content: [] },
      seo_title: title,
      seo_description: draft.excerpt.trim() || undefined,
      og_title: activeTranslation?.og_title ?? undefined,
      og_description: activeTranslation?.og_description ?? undefined,
      og_image_url: activeTranslation?.og_image_url ?? undefined,
      og_image_alt: activeTranslation?.og_image_alt ?? undefined,
    }
  }, [activeTranslation?.og_description, activeTranslation?.og_image_alt, activeTranslation?.og_image_url, activeTranslation?.og_title])

  const persistTranslationDraft = useCallback(async (draft: ArticleTranslationDraftState) => {
    const payload = buildTranslationPayload(draft)
    if (!payload) return
    await handleSaveTranslation(payload)
  }, [buildTranslationPayload, handleSaveTranslation])

  const scheduleTranslationDraftSave = useCallback((draft: ArticleTranslationDraftState) => {
    if (translationSaveTimerRef.current) clearTimeout(translationSaveTimerRef.current)
    translationSaveTimerRef.current = setTimeout(() => {
      void persistTranslationDraft(draft)
    }, 700)
  }, [persistTranslationDraft])

  useEffect(() => {
    return () => {
      if (translationSaveTimerRef.current) clearTimeout(translationSaveTimerRef.current)
    }
  }, [activeLocale])

  const handleTranslationTitleChange = useCallback((newTitle: string) => {
    setTranslationDrafts((current) => {
      const key = translationDraftKey(docId, activeLocale)
      const base = current[key] ?? translationDraftFromTranslation(activeLocale, articleTranslationsByLocale.get(activeLocale) ?? null)
      const next = {
        ...base,
        title: newTitle,
      }
      scheduleTranslationDraftSave(next)
      return { ...current, [key]: next }
    })
  }, [activeLocale, articleTranslationsByLocale, docId, scheduleTranslationDraftSave])

  const handleTranslationContentSave = useCallback(async (json: JSONContent) => {
    let next: ArticleTranslationDraftState = emptyTranslationDraft(activeLocale)
    setTranslationDrafts((current) => {
      const key = translationDraftKey(docId, activeLocale)
      const base = current[key] ?? translationDraftFromTranslation(activeLocale, articleTranslationsByLocale.get(activeLocale) ?? null)
      next = { ...base, content: json }
      return { ...current, [key]: next }
    })
    await persistTranslationDraft(next)
  }, [activeLocale, articleTranslationsByLocale, docId, persistTranslationDraft])

  const handleSelectLocale = useCallback((locale: string) => {
    setPreviewVersion(null)
    setPendingTranslationLocale(null)
    if (locale === defaultLocale || articleTranslationsByLocale.has(locale)) {
      setSelectedLocaleState({ docId, locale })
      return
    }
    setPendingTranslationLocale(locale)
  }, [articleTranslationsByLocale, defaultLocale, docId])

  const handleCreateTranslationManually = useCallback(() => {
    if (!pendingTranslationLocale) return
    setTranslationDrafts((current) => ({
      ...current,
      [translationDraftKey(docId, pendingTranslationLocale)]: emptyTranslationDraft(pendingTranslationLocale),
    }))
    setSelectedLocaleState({ docId, locale: pendingTranslationLocale })
    setPendingTranslationLocale(null)
  }, [docId, pendingTranslationLocale])

  const handleGenerateTranslationWithAI = useCallback(async () => {
    if (!pendingTranslationLocale) return
    try {
      const generated = await generateArticleTranslation.mutateAsync(pendingTranslationLocale)
      setTranslationDrafts((current) => ({
        ...current,
        [translationDraftKey(docId, pendingTranslationLocale)]: translationDraftFromTranslation(
          pendingTranslationLocale,
          generated,
        ),
      }))
      setSelectedLocaleState({ docId, locale: pendingTranslationLocale })
      setPendingTranslationLocale(null)
      toast.success(`${getHelpcenterLocaleLabel(pendingTranslationLocale)} translation draft created`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to generate translation')
    }
  }, [docId, generateArticleTranslation, pendingTranslationLocale])

  const handleRegenerateTranslation = useCallback(async () => {
    if (isSourceLocaleActive) return
    setRegenerating(true)
    try {
      const generated = await generateArticleTranslation.mutateAsync(activeLocale)
      setTranslationDrafts((current) => ({
        ...current,
        [translationDraftKey(docId, activeLocale)]: translationDraftFromTranslation(activeLocale, generated),
      }))
      toast.success(`${getHelpcenterLocaleLabel(activeLocale)} translation regenerated from source`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to regenerate translation')
    } finally {
      setRegenerating(false)
      setRegenerateConfirmOpen(false)
    }
  }, [activeLocale, docId, generateArticleTranslation, isSourceLocaleActive])

  const handleGenerateParentsAndPublish = useCallback(async () => {
    if (!doc || isSourceLocaleActive) return
    setGeneratingParents(true)
    try {
      const spaceRow = spaceTranslationsByLocale.get(activeLocale)
      if (!spaceRow || spaceRow.status !== 'published') {
        await docsService.generateSpaceTranslation(wsId, doc.space_id, activeLocale)
        await docsService.publishSpaceTranslation(wsId, doc.space_id, activeLocale)
      }
      if (doc.collection_id) {
        const collRow = collectionTranslationsByLocale.get(activeLocale)
        if (!collRow || collRow.status !== 'published') {
          await docsService.generateCollectionTranslation(wsId, doc.collection_id, activeLocale)
          await docsService.publishCollectionTranslation(wsId, doc.collection_id, activeLocale)
        }
      }
      const publishedContent = await preparePublishedContent(activeTranslationDraft.content as JSONContent | null | undefined)
      await publishArticleTranslation.mutateAsync({ locale: activeLocale, published_content: publishedContent })
      toast.success(`${getHelpcenterLocaleLabel(activeLocale)} translation published`)
      setParentPublishConfirmOpen(false)
      queryClient.invalidateQueries({ queryKey: queryKeys.docs.documents(wsId) })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to publish translation')
    } finally {
      setGeneratingParents(false)
    }
  }, [activeLocale, activeTranslationDraft.content, doc, isSourceLocaleActive, wsId, spaceTranslationsByLocale, collectionTranslationsByLocale, preparePublishedContent, publishArticleTranslation, queryClient])

  const activeLocaleShortLabel = activeLocale.toUpperCase()
  const sourceLivePublished = isExternalHelpCenter ? !!doc?.live_published_at : doc?.status === 'published'
  const sourceHasUnpublishedChanges = isExternalHelpCenter ? !!doc?.has_unpublished_changes : false
  const translationLivePublished = !!activeTranslation?.live_published_at
  const translationHasUnpublishedChanges = !!activeTranslation?.has_unpublished_changes
  const isPublished = isSourceLocaleActive ? sourceLivePublished : translationLivePublished
  const hasUnpublishedChanges = isSourceLocaleActive ? sourceHasUnpublishedChanges : translationHasUnpublishedChanges
  const showLocaleInPublish = enabledLocales.length > 1
  const localeSuffix = showLocaleInPublish ? ` (${activeLocaleShortLabel})` : ''
  const activePublishLabel = !isPublished
    ? `Publish${localeSuffix}`
    : hasUnpublishedChanges
      ? `Update${localeSuffix}`
      : `Published${localeSuffix}`
  const showContextualPublish = canPublishDocs && doc?.status !== 'archived'
  const parentTranslationsMissing = !isSourceLocaleActive && Boolean(activeLocaleRow?.publishBlockedReason)
  const publishDisabled = isSourceLocaleActive
    ? publishDoc.isPending || doc?.is_locked || (isPublished && !hasUnpublishedChanges)
    : !activeTranslation || publishArticleTranslation.isPending || doc?.is_locked || (isPublished && !hasUnpublishedChanges)
  const activeDialogTranslation = editingTranslationLocale === activeLocale && !isSourceLocaleActive
    ? {
        ...(activeTranslation ?? {
          id: '',
          document_id: docId,
          workspace_id: wsId,
          space_id: doc?.space_id ?? '',
          collection_id: doc?.collection_id,
          locale: activeLocale,
          title: '',
          slug: '',
          excerpt: undefined,
          content: null,
          content_text: '',
          seo_title: undefined,
          seo_description: undefined,
          og_title: undefined,
          og_description: undefined,
          og_image_url: undefined,
          og_image_alt: undefined,
          status: 'draft' as const,
          source_updated_at: undefined,
          source_synced: true,
          published_at: undefined,
          view_count: 0,
          helpful_count: 0,
          not_helpful_count: 0,
          created_at: '',
          updated_at: '',
        }),
        title: activeTranslationDraft.title,
        slug: activeTranslationDraft.slug,
        excerpt: activeTranslationDraft.excerpt || undefined,
        seo_title: activeTranslationDraft.title || undefined,
        seo_description: activeTranslationDraft.excerpt || undefined,
        og_title: activeTranslation?.og_title,
        og_description: activeTranslation?.og_description,
        og_image_url: activeTranslation?.og_image_url,
        og_image_alt: activeTranslation?.og_image_alt,
        content: activeTranslationDraft.content,
      }
    : articleTranslationsByLocale.get(editingTranslationLocale ?? '') ?? null

  if (docLoading || contentLoading) {
    return (
      <div className="flex h-full flex-col">
        <div className="flex items-center gap-3 border-b border-border/60 px-4 py-2">
          <div className="h-8 w-8 animate-pulse rounded bg-muted/60" />
          <div className="h-5 w-64 animate-pulse rounded bg-muted/60" />
        </div>
        <div className="flex-1 animate-pulse bg-muted/20" />
      </div>
    )
  }

  if (!doc) {
    return (
      <div className="flex h-full flex-col items-center justify-center p-4">
        <File01Icon className="h-12 w-12 text-muted-foreground/30 mb-3" />
        <p className="text-sm text-muted-foreground">Document not found.</p>
        <Button
          variant="ghost"
          size="sm"
          className="mt-3"
          onClick={() => navigate({ to: '/w/$slug/docs', params: { slug: wsSlug } })}
        >
          Back to Docs
        </Button>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col">
      {/* Top bar */}
      <div className="flex items-center gap-2 border-b border-border/60 px-3 py-1.5">
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 shrink-0"
          onClick={() => router.history.back()}
        >
          <ArrowLeft02Icon className="h-4 w-4" />
        </Button>

        {/* Breadcrumb — starts at the space, not "Docs". The sidebar
            already tells users they're in the Docs module; repeating
            it here wastes the first breadcrumb slot on something
            they already know. */}
        <nav className="flex min-w-0 flex-1 items-center gap-1 text-xs text-muted-foreground">
          {space && (
            <button
              type="button"
              onClick={() =>
                navigate({
                  to: '/w/$slug/docs/spaces/$spaceId',
                  params: { slug: wsSlug, spaceId: space.id },
                })
              }
              className="truncate hover:text-foreground transition-colors"
            >
              <span className="inline-flex items-center gap-1">
                <StoredIcon name={space.icon} className="h-3.5 w-3.5 shrink-0" textClassName="" />
                <span>{space.name}</span>
              </span>
            </button>
          )}
          {collectionBreadcrumbNodes.map((node) => (
            <div key={node.collection.id} className="flex min-w-0 items-center gap-1">
              <ArrowRight01Icon className="h-3 w-3 shrink-0" />
              <button
                type="button"
                onClick={() =>
                  navigate({
                    to: '/w/$slug/docs/spaces/$spaceId',
                    params: { slug: wsSlug, spaceId: node.collection.space_id },
                    search: { collection: node.collection.id },
                  })
                }
                className="inline-flex min-w-0 items-center gap-1 truncate hover:text-foreground transition-colors"
              >
                <DocCollectionIcon name={node.collection.icon} />
                <span className="truncate">{node.collection.name}</span>
              </button>
            </div>
          ))}
        </nav>

        {!showLocalePills && !(isPublished && hasUnpublishedChanges) && (
          <span className={`shrink-0 text-xs font-medium ${docStatusColor(doc.status)}`}>
            {DOC_STATUS_LABELS[doc.status] ?? doc.status}
          </span>
        )}

        {showContextualPublish && isPublished && hasUnpublishedChanges && (
          <span className="shrink-0 text-[11px] font-medium text-amber-600 dark:text-amber-400">
            Unpublished changes
          </span>
        )}

        {headerPresencePeople.length > 0 && (
          <div className={`hidden shrink-0 items-center gap-2 rounded-full border px-2.5 py-1 md:flex ${
            activeDocEditors.length > 0
              ? 'border-amber-500/30 bg-amber-500/10'
              : 'border-border/60 bg-muted/40'
          }`}>
            <span className={`text-[11px] font-medium ${
              activeDocEditors.length > 0
                ? 'text-amber-700 dark:text-amber-300'
                : 'text-muted-foreground'
            }`}>
              {activeDocEditors.length > 0 ? 'Editing now' : 'Viewing now'}
            </span>
            <div className="flex items-end gap-2">
              {headerPresencePeople.slice(0, 4).map((person) => (
                <QuickTooltip key={person.userId} label={person.tooltip}>
                  <div className="relative pb-1">
                    <UserAvatar
                      name={person.name}
                      avatarUrl={person.avatarUrl}
                      className={person.isEditing ? 'h-6 w-6 ring-1 ring-amber-200 dark:ring-amber-500/40' : 'h-6 w-6 opacity-75'}
                      fallbackClassName="text-[8px]"
                    />
                    {person.isEditing && <EditingIndicator />}
                  </div>
                </QuickTooltip>
              ))}
              {headerPresencePeople.length > 4 && (
                <AvatarGroupCount className="size-6 text-[10px]">
                  +{headerPresencePeople.length - 4}
                </AvatarGroupCount>
              )}
            </div>
          </div>
        )}

        {isExternalHelpCenter && (
          <Button
            variant="outline"
            size="sm"
            className="h-8 shrink-0 gap-1.5 text-xs"
            onClick={async () => {
              try {
                const res = await docsService.getPreviewToken(wsId, docId)
                if (res.error || !res.data) {
                  toast.error(res.error || 'Failed to generate preview')
                  return
                }
                const { token, subdomain, custom_domain } = res.data
                window.open(
                  buildHelpcenterPreviewUrlFromEnv(
                    { subdomain, customDomain: custom_domain },
                    docId,
                    token,
                  ),
                  '_blank',
                  'noopener',
                )
              } catch {
                toast.error('Failed to generate preview')
              }
            }}
          >
            <ViewIcon className="h-3.5 w-3.5" />
            Preview
          </Button>
        )}

        {showContextualPublish && (
          <>
            <Button
              size="sm"
              variant={isPublished && !hasUnpublishedChanges ? 'secondary' : 'default'}
              className="h-7 gap-1.5 text-xs"
              onClick={async () => {
                if (isSourceLocaleActive) {
                  void handlePublish()
                  return
                }
                if (activeTranslation && !activeTranslation.slug) {
                  setPendingSlug(suggestDocsSlug(activeTranslationDraft.title || activeTranslation.title || 'translation', 'translation'))
                  setPendingPublishLocale(activeLocale)
                  setSlugDialogOpen(true)
                  return
                }
                if (parentTranslationsMissing) {
                  setParentPublishConfirmOpen(true)
                  return
                }
                try {
                  const publishedContent = await preparePublishedContent(activeTranslationDraft.content as JSONContent | null | undefined)
                  await publishArticleTranslation.mutateAsync({ locale: activeLocale, published_content: publishedContent })
                  toast.success(`${getHelpcenterLocaleLabel(activeLocale)} translation published`)
                } catch (err) {
                  toast.error(err instanceof Error ? err.message : 'Failed to publish translation')
                }
              }}
              disabled={publishDisabled}
            >
              <SentIcon className="h-3 w-3" />
              {activePublishLabel}
            </Button>
          </>
        )}

        <QuickTooltip label="Document details">
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8 shrink-0"
            onClick={() => setMetaOpen((v) => !v)}
          >
            <MoreHorizontalIcon className="h-4 w-4" />
          </Button>
        </QuickTooltip>
      </div>

      {showLocalePills && (
        <div className="border-b border-border/60 bg-muted/10 px-3 py-2.5">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <ArticleLocalePillRail
              items={enabledLocales.map((locale) => ({
                locale,
                shortLabel: locale.toUpperCase(),
                isActive: activeLocale === locale,
                isSource: locale === defaultLocale,
                sourceStatus: doc.status,
                translationState: (localeRowsByLocale.get(locale)?.state ?? 'missing') as DocsHelpcenterTranslationState,
              }))}
              onSelectLocale={handleSelectLocale}
              onOpenSettings={(locale) => setEditingTranslationLocale(locale)}
            />
            {!isSourceLocaleActive && activeTranslation && (
              <Button
                variant="ghost"
                size="sm"
                className="h-8 shrink-0 gap-1.5 text-xs"
                disabled={regenerating}
                onClick={() => setRegenerateConfirmOpen(true)}
              >
                <MagicWand01Icon className="h-3.5 w-3.5" />
                Regenerate with AI
              </Button>
            )}
          </div>
        </div>
      )}

      {/* Archive banner */}
      {doc.status === 'archived' && (
        <div className="flex items-center gap-2 border-b border-amber-500/30 bg-amber-500/10 px-4 py-2">
          <ArchiveIcon className="h-4 w-4 text-amber-600 dark:text-amber-400" />
          <span className="text-sm text-amber-700 dark:text-amber-300">
            This document is archived and read-only.
          </span>
          {canEditDocs && (
            <Button
              variant="outline"
              size="sm"
              className="ml-auto h-7 gap-1.5 text-xs"
              onClick={handleUnarchive}
              disabled={unarchiveDoc.isPending}
            >
              <ArchiveRestoreIcon className="h-3 w-3" />
              Unarchive
            </Button>
          )}
        </div>
      )}

      {/* Lock banner */}
      {doc.is_locked && (() => {
        const locker = doc.locked_by ? members.find((m) => m.user_id === doc.locked_by) : null
        const lockerName = locker ? formatAssignableMemberName(locker) : 'someone'
        return (
          <div className="flex items-center gap-2 border-b border-blue-500/30 bg-blue-500/10 px-4 py-2">
            <LockIcon className="h-4 w-4 text-blue-600 dark:text-blue-400" />
            <span className="text-sm text-blue-700 dark:text-blue-300">
              {canUnlock
                ? `This document is locked${doc.locked_by ? ` by ${lockerName}` : ''}. Unlock to edit.`
                : `This document is locked by ${lockerName}. Contact them or an admin to unlock it.`}
            </span>
            {canUnlock && (
              <Button
                variant="outline"
                size="sm"
                className="ml-auto h-7 gap-1.5 text-xs"
                onClick={() => {
                  toggleLock.mutate(
                    { docId, isLocked: false },
                    {
                      onSuccess: () => toast.success('Document unlocked'),
                      onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to unlock'),
                    },
                  )
                }}
                disabled={toggleLock.isPending}
              >
                <SquareUnlock01Icon className="h-3 w-3" />
                Unlock
              </Button>
            )}
          </div>
        )
      })()}

      {/* Version preview banner */}
      {previewVersion && (
        <div className="flex items-center gap-3 border-b border-purple-500/30 bg-purple-500/10 px-4 py-2">
          <ViewIcon className="h-4 w-4 text-purple-600 dark:text-purple-400 shrink-0" />
          <div className="flex items-center gap-2 min-w-0 flex-1">
            <span className="text-sm font-medium text-purple-700 dark:text-purple-300 truncate">
              Previewing: {previewVersion.snapshot_label || 'Untitled snapshot'}
            </span>
            <VersionTypeBadge type={previewVersion.version_type} />
            <AuthorDisplay userId={previewVersion.created_by} members={members} />
            <span className="text-[11px] text-muted-foreground">
              {format(parseISO(previewVersion.created_at), 'MMM d, yyyy h:mm a')}
            </span>
          </div>
          {canEditDocs && (
            <Button
              variant="outline"
              size="sm"
              className="h-7 gap-1.5 text-xs shrink-0"
              onClick={handleRestoreFromPreview}
              disabled={revertVersion.isPending}
            >
              <RotateLeft01Icon className="h-3 w-3" />
              Restore
            </Button>
          )}
          <Button
            variant="ghost"
            size="sm"
            className="h-7 gap-1 text-xs shrink-0"
            onClick={() => setPreviewVersion(null)}
          >
            <Cancel01Icon className="h-3 w-3" />
            Exit preview
          </Button>
        </div>
      )}

      {/* Main content area */}
      <div className="flex min-h-0 flex-1">
        {/* Editor */}
        <div ref={editorShellRef} className="flex min-w-0 flex-1 flex-col">
          {previewVersion ? (
            <DocsEditor
              key={`preview-${previewVersion.id}`}
              title={titleDraft}
              initialContent={previewVersion.content as JSONContent | null}
              onSave={handleSave}
              readOnly
              members={members}
              teams={teams}
              agents={documentAgents}
              workspaceId={wsId}
              workspaceSlug={wsSlug}
              documentId={docId}
              commentAnchors={commentAnchors}
              onOpenComment={handleOpenComment}
            />
          ) : (
            <DocsEditor
              key={isSourceLocaleActive ? 'source-editor' : `translation-${activeLocale}`}
              title={isSourceLocaleActive ? titleDraft : activeTranslationDraft.title}
              onTitleChange={!effectiveReadOnly ? (isSourceLocaleActive ? handleTitleChange : handleTranslationTitleChange) : undefined}
              slug={isSourceLocaleActive ? doc?.hc_slug : activeTranslationDraft.slug}
              slugHelperText={
                isPublished && hasUnpublishedChanges
                  ? 'Slug changes will go live when you update this article.'
                  : undefined
              }
              onSlugChange={
                isSourceLocaleActive
                  ? doc?.hc_slug && !effectiveReadOnly
                    ? async (newSlug) => {
                        const res = await docsService.updateArticleSlug(wsId, docId, newSlug)
                        if (res.error) throw new Error(res.error)
                        toast.success(sourceLivePublished ? 'Slug saved. It will go live when you update the article.' : 'Slug saved')
                        queryClient.invalidateQueries({ queryKey: queryKeys.docs.document(wsId, docId) })
                      }
                    : undefined
                  : activeTranslationDraft.slug && !effectiveReadOnly
                    ? async (newSlug) => {
                        const res = await docsService.updateArticleTranslationSlug(wsId, docId, activeLocale, newSlug)
                        if (res.error) throw new Error(res.error)
                        toast.success(
                          translationLivePublished
                            ? `${getHelpcenterLocaleLabel(activeLocale)} slug saved. It will go live when you update the translation.`
                            : `${getHelpcenterLocaleLabel(activeLocale)} slug saved`,
                        )
                        queryClient.invalidateQueries({ queryKey: queryKeys.docs.helpcenterArticleTranslations(wsId, docId) })
                      }
                    : undefined
              }
              initialContent={
                isSourceLocaleActive
                  ? coverageInitialContent ?? (content?.content as JSONContent | null)
                  : activeTranslationDraft.content
              }
              onSave={isSourceLocaleActive ? handleSave : handleTranslationContentSave}
              readOnly={effectiveReadOnly}
              uploadConfig={
                !effectiveReadOnly
                  ? { workspaceId: wsId, entityType: 'editor_upload', entityId: docId }
                  : undefined
              }
              generatingOverlay={
                (regenerating || generateArticleTranslation.isPending) && !isSourceLocaleActive
                  ? 'Generating translation with AI...'
                  : null
              }
              onEditingPresenceChange={!effectiveReadOnly && !previewVersion ? handleEditingPresenceChange : undefined}
              onCreateCommentAnchor={!effectiveReadOnly ? handleCreateCommentAnchor : undefined}
              commentAnchors={commentAnchors}
              onOpenComment={handleOpenComment}
              members={members}
              teams={teams}
              agents={documentAgents}
              workspaceId={wsId}
              workspaceSlug={wsSlug}
              documentId={docId}
            />
          )}
        </div>

        <DocsOutlineSidebar items={outlineItems} onSelect={handleOutlineSelect} />

        {/* Metadata sidebar */}
        {metaOpen && (
          <aside className="w-64 shrink-0 overflow-y-auto border-l border-border/60 px-4 py-5">

            {/* ── Document/Translation status ── */}
            {showLocalePills && isSourceLocaleActive && (
              <>
                <div className={`flex items-center justify-between rounded-lg px-3 py-2 ${
                  doc.status === 'published' ? 'bg-emerald-500/8' :
                  doc.status === 'archived' ? 'bg-muted/40' :
                  'bg-amber-500/8'
                }`}>
                  <span className="text-xs font-medium">{getHelpcenterLocaleLabel(defaultLocale)} (Source)</span>
                  <span className={`text-[11px] font-medium ${
                    doc.status === 'published' ? 'text-emerald-600 dark:text-emerald-400' :
                    doc.status === 'archived' ? 'text-muted-foreground' :
                    'text-amber-600 dark:text-amber-400'
                  }`}>
                    {doc.status === 'published' ? 'Published' : doc.status === 'archived' ? 'Archived' : 'Draft'}
                  </span>
                </div>
                <Separator className="my-4" />
              </>
            )}
            {showLocalePills && !isSourceLocaleActive && activeTranslation && (
              <>
                <div className={`flex items-center justify-between rounded-lg px-3 py-2 ${
                  activeTranslationStatus === 'published' ? 'bg-emerald-500/8' :
                  activeTranslationStatus === 'needs_review' ? 'bg-blue-500/8' :
                  'bg-amber-500/8'
                }`}>
                  <span className="text-xs font-medium">{getHelpcenterLocaleLabel(activeLocale)}</span>
                  <span className={`text-[11px] font-medium ${
                    activeTranslationStatus === 'published' ? 'text-emerald-600 dark:text-emerald-400' :
                    activeTranslationStatus === 'needs_review' ? 'text-blue-600 dark:text-blue-400' :
                    'text-amber-600 dark:text-amber-400'
                  }`}>
                    {activeTranslationStatus === 'published' ? 'Published' : activeTranslationStatus === 'needs_review' ? 'Needs review' : 'Draft'}
                  </span>
                </div>
                <Separator className="my-4" />
              </>
            )}

            {/* ── Social metadata (source locale only) ── */}
            {isExternalHelpCenter && isSourceLocaleActive && canEditDocs && (
              <>
                <div className="space-y-3">
                  <div className="flex items-center gap-1.5">
                    <GlobeIcon className="h-3.5 w-3.5 text-muted-foreground" />
                    <span className="text-xs font-medium text-muted-foreground">Social preview</span>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="doc-og-title" className="text-[11px] text-muted-foreground">Title</Label>
                    <Input
                      id="doc-og-title"
                      value={sourceSocialDraft.og_title}
                      onChange={(event) => setSourceSocialDraft((prev) => ({ ...prev, og_title: event.target.value }))}
                      placeholder={titleDraft || 'Article title'}
                      className="h-8 text-xs"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="doc-og-description" className="text-[11px] text-muted-foreground">Description</Label>
                    <Textarea
                      id="doc-og-description"
                      value={sourceSocialDraft.og_description}
                      onChange={(event) => setSourceSocialDraft((prev) => ({ ...prev, og_description: event.target.value }))}
                      placeholder={doc.excerpt || 'Article summary'}
                      rows={3}
                      className="min-h-20 text-xs"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="doc-og-image" className="text-[11px] text-muted-foreground">Image URL</Label>
                    <div className="flex gap-1.5">
                      <Input
                        id="doc-og-image"
                        value={sourceSocialDraft.og_image_url}
                        onChange={(event) => setSourceSocialDraft((prev) => ({ ...prev, og_image_url: event.target.value }))}
                        placeholder="https://..."
                        className="h-8 text-xs"
                      />
                      <Button type="button" variant="outline" size="sm" className="h-8 px-2 text-xs" disabled={uploadingSourceOGImage} onClick={() => sourceOGImageInputRef.current?.click()}>
                        {uploadingSourceOGImage ? '...' : 'Upload'}
                      </Button>
                      <input
                        ref={sourceOGImageInputRef}
                        type="file"
                        accept="image/png,image/jpeg,image/webp"
                        className="hidden"
                        onChange={(event) => void handleUploadSourceOGImage(event.target.files?.[0])}
                      />
                    </div>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="doc-og-image-alt" className="text-[11px] text-muted-foreground">Image Alt</Label>
                    <Input
                      id="doc-og-image-alt"
                      value={sourceSocialDraft.og_image_alt}
                      onChange={(event) => setSourceSocialDraft((prev) => ({ ...prev, og_image_alt: event.target.value }))}
                      placeholder={titleDraft || 'Article preview image'}
                      className="h-8 text-xs"
                    />
                  </div>
                  <Button type="button" size="sm" className="h-8 w-full text-xs" disabled={savingSourceSocial || doc.is_locked} onClick={() => void handleSaveSourceSocial()}>
                    {savingSourceSocial ? 'Saving...' : 'Save social preview'}
                  </Button>
                </div>
                <Separator className="my-4" />
              </>
            )}

            {/* ── Sharing (source locale only) ── */}
            {isSourceLocaleActive && canEditDocs && (
              <>
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-1.5">
                      <Link01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                      <span className="text-xs text-muted-foreground">Public link</span>
                    </div>
                    <Switch
                      checked={doc.is_publicly_shared}
                      onCheckedChange={(checked) => {
                        toggleShare.mutate(
                          { docId, isPubliclyShared: checked },
                          {
                            onSuccess: () => toast.success(checked ? 'Public link enabled' : 'Public link disabled'),
                            onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to toggle share'),
                          },
                        )
                      }}
                      disabled={toggleShare.isPending || doc.is_locked}
                    />
                  </div>
                  {doc.is_publicly_shared && doc.share_token && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="w-full h-7 gap-1.5 text-xs"
                      onClick={() => {
                        const url = `${window.location.origin}/share/${doc.share_token}`
                        copyLink(url)
                        toast.success('Link copied to clipboard')
                      }}
                    >
                      {linkCopied ? <Tick01Icon className="h-3 w-3" /> : <Copy01Icon className="h-3 w-3" />}
                      {linkCopied ? 'Copied!' : 'Copy link'}
                    </Button>
                  )}
                </div>
                <Separator className="my-4" />
              </>
            )}

            {/* ── Properties ── */}
            <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
              {/* Owner (source only) */}
              {isSourceLocaleActive && (<>
              <UserIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
              <span className="text-[12px] text-muted-foreground self-center">Owner</span>
              <div className="min-w-0 self-center">
                {canEditDocs && !doc.is_locked ? (
                  <MemberPickerPopover
                    value={doc.owner_id ?? '__none__'}
                    members={members}
                    onChange={(id) =>
                      patchDoc({ owner_id: id === '__none__' ? null : id })
                    }
                    noneLabel="Unassigned"
                    renderTrigger={() => {
                      const owner = members.find((m) => m.id === doc.owner_id)
                      if (!owner) return <span className="text-xs text-muted-foreground">Unassigned</span>
                      return (
                        <span className="truncate text-xs">
                          {formatAssignableMemberName(owner)}
                        </span>
                      )
                    }}
                  />
                ) : (
                  (() => {
                    const owner = members.find((m) => m.id === doc.owner_id)
                    return owner ? (
                      <span className="truncate text-xs">{formatAssignableMemberName(owner)}</span>
                    ) : (
                      <span className="text-xs text-muted-foreground">Unassigned</span>
                    )
                  })()
                )}
              </div>

              {/* Created by */}
              <UserCheck01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
              <span className="text-[12px] text-muted-foreground self-center">Created by</span>
              <div className="min-w-0 self-center">
                {(() => {
                  const creator = members.find((m) => m.user_id === doc.created_by)
                  return creator ? (
                    <span className="truncate text-xs">{formatAssignableMemberName(creator)}</span>
                  ) : (
                    <span className="text-xs text-muted-foreground">Unknown</span>
                  )
                })()}
              </div>

              {/* Collection */}
              <FolderOpenIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />

              <span className="text-[12px] text-muted-foreground self-center">Collection</span>
              <div className="min-w-0 self-center">
                {canEditDocs && !doc.is_locked ? (
                  <Select
                    value={doc.collection_id ?? '__none__'}
                    onValueChange={async (v) => {
                      if (v === '__create__') {
                        const name = window.prompt('Collection name')
                        if (!name?.trim()) return
                        try {
                          const created = await createCollection.mutateAsync({ name: name.trim() })
                          patchDoc({ collection_id: created.id })
                          toast.success(`Collection "${name.trim()}" created`)
                        } catch (err) {
                          toast.error(err instanceof Error ? err.message : 'Failed to create')
                        }
                        return
                      }
                      patchDoc({ collection_id: v === '__none__' ? null : v })
                    }}
                  >
                    <SelectTrigger className="h-auto w-auto min-w-[80px] max-w-[130px] border-0 bg-transparent px-1.5 py-0.5 text-xs shadow-none hover:bg-accent">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="__none__">None</SelectItem>
                      {collections.map((c) => (
                        <SelectItem key={c.id} value={c.id}>
                          {c.name}
                        </SelectItem>
                      ))}
                      <SelectItem value="__create__" className="text-primary">
                        + New collection
                      </SelectItem>
                    </SelectContent>
                  </Select>
                ) : (
                  <span className="text-xs">
                    {collections.find((c) => c.id === doc.collection_id)?.name ?? 'None'}
                  </span>
                )}
              </div>
              </>)}

              {/* Created */}
              <Calendar03Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
              <span className="text-[12px] text-muted-foreground self-center">Created</span>
              <div className="min-w-0 self-center">
                <span className="text-xs">{timeAgo(!isSourceLocaleActive && activeTranslation?.created_at ? activeTranslation.created_at : doc.created_at)}</span>
              </div>

              {/* Updated */}
              <Clock01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
              <span className="text-[12px] text-muted-foreground self-center">Updated</span>
              <div className="min-w-0 self-center">
                <span className="text-xs">{timeAgo(!isSourceLocaleActive && activeTranslation?.updated_at ? activeTranslation.updated_at : doc.updated_at)}</span>
              </div>

              {/* Published */}
              {(isSourceLocaleActive ? doc.published_at : activeTranslation?.published_at) && (
                <>
                  <GlobeIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                  <span className="text-[12px] text-muted-foreground self-center">Published</span>
                  <div className="min-w-0 self-center">
                    <span className="text-xs">{timeAgo((isSourceLocaleActive ? doc.published_at : activeTranslation?.published_at)!)}</span>
                  </div>
                </>
              )}
            </div>

            {/* ── History & Links (source locale only) ── */}
            {isSourceLocaleActive && (
              <>
              <Separator className="my-4" />
              <div className="space-y-1">
                <button
                  type="button"
                  onClick={() => { setVersionsOpen(true); setMetaOpen(false) }}
                  className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
                >
                  <Clock01Icon className="h-3.5 w-3.5" />
                  Version History
                </button>
                <button
                  type="button"
                  onClick={() => setLinksOpen(true)}
                  className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
                >
                  <LinkSquare01Icon className="h-3.5 w-3.5" />
                  Linked Items
                </button>
                <button
                  type="button"
                  onClick={() => { setCommentsOpen(true); setMetaOpen(false) }}
                  className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
                >
                  <Message01Icon className="h-3.5 w-3.5" />
                  Comments
                  {comments.length > 0 && (
                    <span className="ml-auto rounded-full bg-muted px-1.5 py-0.5 text-[10px]">
                      {comments.length}
                    </span>
                  )}
                </button>
                <button
                  type="button"
                  onClick={() => { setReferencesOpen(true); setMetaOpen(false) }}
                  className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
                >
                  <Link01Icon className="h-3.5 w-3.5" />
                  References
                  {references.length > 0 && (
                    <span className="ml-auto rounded-full bg-muted px-1.5 py-0.5 text-[10px]">
                      {references.length}
                    </span>
                  )}
                </button>
              </div>
              </>
            )}

            {/* ── Translation Actions (non-source locale) ── */}
            {showLocalePills && !isSourceLocaleActive && activeTranslation && canEditDocs && (
              <>
                <Separator className="my-4" />
                <div className="space-y-1">
                  {activeTranslationStatus === 'needs_review' && (
                    <button
                      type="button"
                      onClick={() => {
                        markArticleTranslationReviewed.mutate(activeLocale, {
                          onSuccess: () => toast.success('Translation marked as reviewed'),
                          onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to mark reviewed'),
                        })
                      }}
                      disabled={markArticleTranslationReviewed.isPending}
                      className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
                    >
                      <Tick01Icon className="h-3.5 w-3.5" />
                      Mark as reviewed
                    </button>
                  )}
                  {activeTranslationStatus === 'published' && (
                    <button
                      type="button"
                      onClick={() => {
                        unpublishArticleTranslation.mutate(activeLocale, {
                          onSuccess: () => toast.success('Translation unpublished'),
                          onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to unpublish'),
                        })
                      }}
                      disabled={unpublishArticleTranslation.isPending}
                      className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
                    >
                      <RotateLeft01Icon className="h-3.5 w-3.5" />
                      Unpublish translation
                    </button>
                  )}
                </div>
              </>
            )}

            {/* ── Document Actions (source locale) ── */}
            {isSourceLocaleActive && canEditDocs && (
              <>
                <Separator className="my-4" />
                <div className="space-y-1">
                  {/* Lock/unlock: only show if user can toggle (can lock when unlocked, can unlock when locked) */}
                  {(!doc.is_locked || canUnlock) && (
                    <button
                      type="button"
                      onClick={() => {
                        const newLocked = !doc.is_locked
                        toggleLock.mutate(
                          { docId, isLocked: newLocked },
                          {
                            onSuccess: () => toast.success(newLocked ? 'Document locked' : 'Document unlocked'),
                            onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to toggle lock'),
                          },
                        )
                      }}
                      disabled={toggleLock.isPending}
                      className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
                    >
                      {doc.is_locked ? <SquareUnlock01Icon className="h-3.5 w-3.5" /> : <LockIcon className="h-3.5 w-3.5" />}
                      {doc.is_locked ? 'Unlock document' : 'Lock document'}
                    </button>
                  )}
                  {doc.status === 'published' && (
                    <button
                      type="button"
                      onClick={() => {
                        const publishedTranslationCount = articleTranslationRows.filter(r => !r.isDefaultLocale && r.state === 'published').length
                        if (publishedTranslationCount > 0) {
                          setRevertConfirmOpen(true)
                          return
                        }
                        unpublishDoc.mutate(docId, {
                          onSuccess: () => toast.success('Reverted to draft'),
                          onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to revert'),
                        })
                      }}
                      disabled={unpublishDoc.isPending || doc.is_locked}
                      className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
                    >
                      <RotateLeft01Icon className="h-3.5 w-3.5" />
                      Revert to draft
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => setMoveDialogOpen(true)}
                    disabled={doc.is_locked}
                    className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
                  >
                    <FolderInputIcon className="h-3.5 w-3.5" />
                    Move to...
                  </button>
                  {doc.status !== 'archived' && (
                    <button
                      type="button"
                      onClick={handleArchive}
                      disabled={doc.is_locked}
                      className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
                    >
                      <ArchiveIcon className="h-3.5 w-3.5" />
                      Archive
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => setDeleteConfirmOpen(true)}
                    disabled={doc.is_locked}
                    className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-destructive transition-colors hover:bg-destructive/10 disabled:opacity-50 disabled:pointer-events-none"
                  >
                    <Delete01Icon className="h-3.5 w-3.5" />
                    Delete
                  </button>
                </div>
              </>
            )}
          </aside>
        )}

        {/* Version history — inline sidebar */}
        <VersionHistoryPanel
          wsId={wsId}
          docId={docId}
          open={versionsOpen}
          onClose={() => { setVersionsOpen(false); setPreviewVersion(null) }}
          onPreview={handlePreview}
          members={members}
          canEdit={canEditDocs}
          previewingVersionId={previewVersion?.id}
        />
        {commentsOpen && (
          <aside className="flex w-96 shrink-0 flex-col overflow-hidden border-l border-border/60 bg-background">
            <div className="flex items-center justify-between border-b border-border/60 px-4 py-3">
              <div className="flex items-center gap-2">
                <Message01Icon className="h-4 w-4 text-muted-foreground" />
                <span className="text-sm font-medium">Comments</span>
                {comments.length > 0 && (
                  <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                    {comments.length}
                  </span>
                )}
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-7 w-7 p-0"
                onClick={() => setCommentsOpen(false)}
              >
                <Cancel01Icon className="h-3.5 w-3.5" />
              </Button>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto p-4">
              {commentsLoading ? (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
                  Loading comments...
                </div>
              ) : (
                <CommentThread
                  workspaceId={wsId}
                  entityType="doc"
                  entityId={docId}
                  comments={comments}
                  currentUserId={currentUserId}
                  teams={teams}
                  members={members}
                  commentService={docsCommentService}
                  attachmentsEnabled={false}
                  commentAnchor={commentAnchor}
                  activeCommentId={activeCommentId}
                  onCommentAnchorConsumed={() => setCommentAnchor(null)}
                  onCommentsChange={setComments}
                />
              )}
            </div>
          </aside>
        )}
        {referencesOpen && (
          <aside className="flex w-80 shrink-0 flex-col overflow-hidden border-l border-border/60 bg-background">
            <div className="flex items-center justify-between border-b border-border/60 px-4 py-3">
              <div className="flex items-center gap-2">
                <Link01Icon className="h-4 w-4 text-muted-foreground" />
                <span className="text-sm font-medium">References</span>
                {references.length > 0 && (
                  <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                    {references.length}
                  </span>
                )}
              </div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-7 w-7 p-0"
                onClick={() => setReferencesOpen(false)}
              >
                <Cancel01Icon className="h-3.5 w-3.5" />
              </Button>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto p-4">
              {referencesLoading ? (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
                  Loading references...
                </div>
              ) : references.length === 0 ? (
                <div className="rounded-md border border-dashed border-border/70 p-3 text-xs text-muted-foreground">
                  No references yet.
                </div>
              ) : (
                <div className="space-y-2">
                  {references.map((item) => (
                    <div key={item.id} className="rounded-md border border-border/60 p-3">
                      <div className="flex items-center justify-between gap-2">
                        <span className="truncate text-sm font-medium">{item.title}</span>
                        <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] uppercase text-muted-foreground">
                          {item.kind.replaceAll('_', ' ')}
                        </span>
                      </div>
                      {item.description && (
                        <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{item.description}</p>
                      )}
                      {(item.block_id || item.entity_type) && (
                        <div className="mt-2 flex flex-wrap gap-1 text-[10px] text-muted-foreground">
                          {item.entity_type && <span className="rounded bg-muted px-1.5 py-0.5">{item.entity_type}</span>}
                          {item.block_id && <span className="rounded bg-muted px-1.5 py-0.5">Block</span>}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </aside>
        )}
      </div>

      {/* Slide-out panels */}
      <DocumentLinksPanel
        wsId={wsId}
        docId={docId}
        open={linksOpen}
        onOpenChange={setLinksOpen}
        canEdit={canEditDocs}
      />

      {editingTranslationLocale && (
        <EditArticleTranslationDialog
          key={`${editingTranslationLocale}-${articleTranslationsByLocale.get(editingTranslationLocale)?.updated_at ?? 'new'}`}
          open={Boolean(editingTranslationLocale)}
          onOpenChange={(open) => {
            if (!open) setEditingTranslationLocale(null)
          }}
          locale={editingTranslationLocale}
          sourceExcerpt={doc.excerpt}
          translation={activeDialogTranslation}
          isSaving={upsertArticleTranslation.isPending}
          onSave={async (data) => {
            await handleSaveTranslation(data)
            if (editingTranslationLocale === activeLocale) {
              setTranslationDrafts((current) => ({
                ...current,
                [translationDraftKey(docId, data.locale)]: {
                  locale: data.locale,
                  title: data.title,
                  slug: current[translationDraftKey(docId, data.locale)]?.slug ?? activeTranslationDraft.slug,
                  excerpt: data.excerpt ?? '',
                  content: (data.content as JSONContent | null | undefined) ?? null,
                },
              }))
            }
          }}
        />
      )}

      <MissingArticleTranslationDialog
        open={Boolean(pendingTranslationLocale)}
        locale={pendingTranslationLocale ?? defaultLocale}
        onOpenChange={(open) => {
          if (!open) setPendingTranslationLocale(null)
        }}
        onCreateManually={handleCreateTranslationManually}
        onGenerateWithAI={handleGenerateTranslationWithAI}
        isGenerating={generateArticleTranslation.isPending}
      />

      {doc && (
        <>
          <MoveDocumentDialog
            wsId={wsId}
            open={moveDialogOpen}
            onOpenChange={setMoveDialogOpen}
            docId={docId}
            docTitle={doc.title}
            currentSpaceId={doc.space_id}
            currentCollectionId={doc.collection_id}
          />

          <ConfirmDialog
            open={deleteConfirmOpen}
            onOpenChange={setDeleteConfirmOpen}
            title="Delete document"
            description="This will permanently delete the document and its saved content. This action cannot be undone."
            confirmLabel="Delete"
            variant="destructive"
            onConfirm={handleDelete}
          />

          <ConfirmDialog
            open={regenerateConfirmOpen}
            onOpenChange={setRegenerateConfirmOpen}
            title="Regenerate translation"
            description={<div className="space-y-2"><p>This will replace the current translation content with a new AI-generated version from the source article.</p><p>Protected terms from your settings will be preserved.</p></div>}
            confirmLabel={regenerating ? 'Regenerating...' : 'Regenerate'}
            onConfirm={handleRegenerateTranslation}
          />

          <Dialog open={parentPublishConfirmOpen} onOpenChange={(open) => { if (!generatingParents) setParentPublishConfirmOpen(open) }}>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>Translate and publish</DialogTitle>
              </DialogHeader>
              <div className="space-y-2 text-sm text-muted-foreground">
                <p>The space and collection names for <strong className="text-foreground">{getHelpcenterLocaleLabel(activeLocale)}</strong> have not been translated yet.</p>
                <p>Our AI will automatically translate them and then publish this article translation.</p>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" disabled={generatingParents} onClick={() => setParentPublishConfirmOpen(false)}>
                  Cancel
                </Button>
                <Button type="button" disabled={generatingParents} onClick={() => void handleGenerateParentsAndPublish()}>
                  {generatingParents ? (
                    <><Loading01Icon className="h-3.5 w-3.5 animate-spin mr-1.5" /> Publishing...</>
                  ) : (
                    'Proceed'
                  )}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </>
      )}
      <ConfirmDialog
        open={revertConfirmOpen}
        onOpenChange={setRevertConfirmOpen}
        title="Revert to draft"
        variant="destructive"
        description={(() => {
          const count = articleTranslationRows.filter(r => !r.isDefaultLocale && r.state === 'published').length
          return `This will revert the source article and ${count} published translation${count !== 1 ? 's' : ''} to draft. They will no longer be visible on the public help center.`
        })()}
        confirmLabel="Revert all to draft"
        onConfirm={() => {
          // Unpublish all published translations first, then the source
          const publishedLocales = articleTranslationRows.filter(r => !r.isDefaultLocale && r.state === 'published').map(r => r.locale)
          Promise.all(publishedLocales.map(locale => unpublishArticleTranslation.mutateAsync(locale).catch(() => {})))
            .then(() => {
              unpublishDoc.mutate(docId, {
                onSuccess: () => toast.success(`Reverted to draft (${publishedLocales.length} translation${publishedLocales.length !== 1 ? 's' : ''} also reverted)`),
                onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to revert'),
              })
            })
          setRevertConfirmOpen(false)
        }}
      />

      <PublishSlugDialog
        open={slugDialogOpen}
        onOpenChange={(open) => {
          setSlugDialogOpen(open)
          if (!open) setPendingPublishLocale(null)
        }}
        title={
          pendingPublishLocale && pendingPublishLocale !== defaultLocale
            ? `Confirm ${getHelpcenterLocaleLabel(pendingPublishLocale)} URL slug`
            : 'Confirm article URL slug'
        }
        description={
          pendingPublishLocale && pendingPublishLocale !== defaultLocale
            ? 'This slug will be used in the localized public help center URL for the active translation.'
            : 'This slug will be used in the public help center URL.'
        }
        slug={pendingSlug}
        onSlugChange={setPendingSlug}
        onConfirm={handleConfirmPublish}
        isPublishing={pendingPublishLocale && pendingPublishLocale !== defaultLocale ? publishArticleTranslation.isPending : publishDoc.isPending}
      />
    </div>
  )
}
