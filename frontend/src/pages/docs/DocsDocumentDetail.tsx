import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams, useRouter } from '@tanstack/react-router'
import { format, parseISO, isThisYear } from 'date-fns'
import type { JSONContent } from '@tiptap/react'
import {
  ArrowLeft,
  Archive,
  ArchiveRestore,
  CalendarDays,
  Check,
  ChevronRight,
  Clock,
  Copy,
  Eye,
  ExternalLink,
  FileText,
  FolderInput,
  FolderOpen,
  Globe,
  Link2,
  Lock,
  MoreHorizontal,
  RotateCcw,
  Send,
  Trash2,
  Unlock,
  User,
  UserCheck,
  X,
  Languages,
  Settings2,
  WandSparkles,
} from 'lucide-react'
import { ICON_MAP } from '@/components/ui/icon-picker'
import { toast } from 'sonner'
import { useTitle } from '@/hooks/useTitle'
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard'
import { useAuthStore } from '@/stores/authStore'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import {
  useDocsDocument,
  useDocsContent,
  useDocsSpace,
  useDocsCollections,
  useDocsHelpcenterArticleTranslations,
  useDocsHelpcenterCollectionTranslations,
  useDocsHelpcenterLocales,
  useDocsHelpcenterSpaceTranslations,
  useCreateDocsCollection,
  useGenerateDocsHelpcenterArticleTranslation,
  useMarkDocsHelpcenterArticleTranslationReviewed,
  usePublishDocsHelpcenterArticleTranslation,
  useSaveDocsContent,
  useUpdateDocsDocument,
  useUnpublishDocsHelpcenterArticleTranslation,
  useUpsertDocsHelpcenterArticleTranslation,
  usePublishDocsDocument,
  useUnpublishDocsDocument,
  useArchiveDocsDocument,
  useUnarchiveDocsDocument,
  useDeleteDocsDocument,
  useAssignableMembers,
  useWorkspaceAccess,
  usePermissions,
  useToggleDocShare,
  useToggleDocLock,
  useRevertDocsVersion,
} from '@/hooks/queries'
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover'
import { formatAssignableMemberName } from '@/lib/assignableMembers'
import { timeAgo } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { DocsEditor } from '@/components/docs/DocsEditor'
import { VersionHistoryPanel, VersionTypeBadge, AuthorDisplay } from '@/components/docs/VersionHistoryPanel'
import { DocumentLinksPanel } from '@/components/docs/DocumentLinksPanel'
import { MoveDocumentDialog } from '@/components/docs/MoveDocumentDialog'
import { EditArticleTranslationDialog } from '@/components/docs/helpcenter/EditArticleTranslationDialog'
import { ArticleLocalePillRail } from '@/components/docs/helpcenter/ArticleLocalePillRail'
import { MissingArticleTranslationDialog } from '@/components/docs/helpcenter/MissingArticleTranslationDialog'
import { TranslationStatusBadge } from '@/components/docs/helpcenter/TranslationStatusBadge'
import { TranslationsPanel } from '@/components/docs/helpcenter/TranslationsPanel'
import { ConfirmDialog } from '@/components/pm/ConfirmDialog'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { DOC_STATUS_LABELS, getHelpcenterLocaleLabel } from '@/lib/docsTypes'
import { docsService } from '@/lib/services/docsService'
import type { DocsVersion } from '@/lib/docsTypes'
import { QuickTooltip } from '@/components/ui/quick-tooltip'

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
  return <FolderOpen className="h-3 w-3 shrink-0" />;
}

interface ArticleTranslationDraftState {
  locale: string
  title: string
  slug: string
  excerpt: string
  seoTitle: string
  seoDescription: string
  content: JSONContent | null
}

function emptyTranslationDraft(locale: string): ArticleTranslationDraftState {
  return {
    locale,
    title: '',
    slug: '',
    excerpt: '',
    seoTitle: '',
    seoDescription: '',
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
    seoTitle: translation?.seo_title ?? '',
    seoDescription: translation?.seo_description ?? '',
    content: (translation?.content as JSONContent | null | undefined) ?? null,
  }
}

function translationDraftKey(docId: string, locale: string): string {
  return `${docId}:${locale}`
}

export function DocsDocumentDetail() {
  const navigate = useNavigate()
  const router = useRouter()
  const { docId } = useParams({ strict: false }) as { docId: string }
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const { data: access } = useWorkspaceAccess(wsId)
  const { canEditDocs, canPublishDocs, canAdminDocs, isAdmin } = usePermissions(access)
  const currentUserId = useAuthStore((s) => s.user?.id)

  const { data: doc, isLoading: docLoading } = useDocsDocument(wsId, docId)
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

  // Lock-aware editing: locked docs are read-only for everyone — unlock to edit
  const effectiveReadOnly = !canEditDocs || doc?.status === 'archived' || !!doc?.is_locked
  const canUnlock = doc?.is_locked && (doc.locked_by === currentUserId || isAdmin)

  const [metaOpen, setMetaOpen] = useState(false)
  const [versionsOpen, setVersionsOpen] = useState(false)
  const [moveDialogOpen, setMoveDialogOpen] = useState(false)
  const [linksOpen, setLinksOpen] = useState(false)
  const [translationsOpen, setTranslationsOpen] = useState(false)
  const [editingTranslationLocale, setEditingTranslationLocale] = useState<string | null>(null)
  const [pendingTranslationLocale, setPendingTranslationLocale] = useState<string | null>(null)
  const [regenerateConfirmOpen, setRegenerateConfirmOpen] = useState(false)
  const [regenerating, setRegenerating] = useState(false)
  const [slugDialogOpen, setSlugDialogOpen] = useState(false)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [pendingSlug, setPendingSlug] = useState('')
  const { copied: linkCopied, copy: copyLink } = useCopyToClipboard()

  // Version preview state — when set, the editor shows version content read-only
  const [previewVersion, setPreviewVersion] = useState<DocsVersion | null>(null)
  const revertVersion = useRevertDocsVersion(wsId)

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

  const handleSave = useCallback(
    async (json: JSONContent) => {
      await saveContent.mutateAsync({ docId, content: json })
    },
    [saveContent, docId],
  )

  const slugifyTitle = (title: string) =>
    title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')

  const isExternalHelpCenter = space?.type === 'external_capable'
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
  const articleTranslationRows = enabledLocales.map((locale) => {
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

  useTitle(displayedTitle || 'Document')

  const handlePublish = async () => {
    // For external help center articles, show slug confirmation first
    if (isExternalHelpCenter && doc?.status === 'draft') {
      setPendingSlug(slugifyTitle(doc.title ?? 'untitled'))
      setSlugDialogOpen(true)
      return
    }
    try {
      await publishDoc.mutateAsync({ id: docId })
      toast.success('Document published')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to publish')
    }
  }

  const handleConfirmPublish = async () => {
    try {
      await publishDoc.mutateAsync({ id: docId, slug: pendingSlug })
      setSlugDialogOpen(false)
      toast.success('Document published')
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

    const fallbackBase = doc?.hc_slug || slugifyTitle(doc?.title ?? '') || 'article'
    const slug = draft.slug.trim() || slugifyTitle(title) || `${fallbackBase}-${draft.locale}`

    return {
      locale: draft.locale,
      title,
      slug,
      excerpt: draft.excerpt.trim() || undefined,
      content: (draft.content as JSONContent | null) ?? { type: 'doc', content: [] },
      seo_title: draft.seoTitle.trim() || undefined,
      seo_description: draft.seoDescription.trim() || undefined,
    }
  }, [doc?.hc_slug, doc?.title])

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
        slug: base.slug || slugifyTitle(newTitle),
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

  const activeLocaleShortLabel = activeLocale.toUpperCase()
  const activeContextTitle = isSourceLocaleActive
    ? 'Editing source article'
    : `Editing ${getHelpcenterLocaleLabel(activeLocale)} translation`
  const activeContextHelper = isSourceLocaleActive
    ? 'This internal article remains the source of truth for public locale variants.'
    : activeTranslation?.source_synced === false
      ? 'The source article changed after this locale was last reviewed. Update the translation before publishing.'
      : 'This locale draft is served independently in the public help center once it is published.'
  const isPublished = isSourceLocaleActive ? doc?.status === 'published' : activeTranslationStatus === 'published'
  const activePublishLabel = isPublished
    ? `Update (${activeLocaleShortLabel})`
    : `Publish (${activeLocaleShortLabel})`
  const showContextualPublish = canPublishDocs && doc?.status !== 'archived'
  const publishDisabled = isSourceLocaleActive
    ? publishDoc.isPending || doc?.is_locked
    : !activeTranslation || Boolean(activeLocaleRow?.publishBlockedReason) || publishArticleTranslation.isPending || doc?.is_locked
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
        seo_title: activeTranslationDraft.seoTitle || undefined,
        seo_description: activeTranslationDraft.seoDescription || undefined,
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
        <FileText className="h-12 w-12 text-muted-foreground/30 mb-3" />
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
          <ArrowLeft className="h-4 w-4" />
        </Button>

        {/* Breadcrumb */}
        <nav className="flex min-w-0 flex-1 items-center gap-1 text-xs text-muted-foreground">
          <button
            type="button"
            onClick={() => navigate({ to: '/w/$slug/docs', params: { slug: wsSlug } })}
            className="shrink-0 hover:text-foreground transition-colors"
          >
            Docs
          </button>
          {space && (
            <>
              <ChevronRight className="h-3 w-3 shrink-0" />
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
                {space.icon ? `${space.icon} ` : ''}{space.name}
              </button>
            </>
          )}
          {doc.collection_id && (() => {
            const col = collections.find((c) => c.id === doc.collection_id)
            if (!col) return null
            return (
              <>
                <ChevronRight className="h-3 w-3 shrink-0" />
                <DocCollectionIcon name={col.icon} />
                <span className="truncate">{col.name}</span>
              </>
            )
          })()}
        </nav>

        <span className={`shrink-0 text-xs font-medium ${docStatusColor(doc.status)}`}>
          {DOC_STATUS_LABELS[doc.status] ?? doc.status}
        </span>

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
                const hcUrl = import.meta.env.VITE_HELPCENTER_URL || 'http://localhost:5174'
                const { token, subdomain } = res.data
                window.open(
                  `${hcUrl}/preview/${docId}?subdomain=${subdomain}&token=${token}`,
                  '_blank',
                  'noopener',
                )
              } catch {
                toast.error('Failed to generate preview')
              }
            }}
          >
            <Eye className="h-3.5 w-3.5" />
            Preview
          </Button>
        )}

        {isExternalHelpCenter && canAdminDocs && (
          <Button
            variant="outline"
            size="sm"
            className="h-8 shrink-0 gap-1.5 text-xs"
            onClick={() => setTranslationsOpen(true)}
          >
            <Languages className="h-3.5 w-3.5" />
            Translations
          </Button>
        )}

        {showContextualPublish && (
          <QuickTooltip label={activeLocaleRow?.publishBlockedReason || (publishDisabled ? 'Cannot publish' : activePublishLabel)}>
            <Button
              size="sm"
              variant="default"
              className="h-7 gap-1.5 text-xs"
              onClick={() => {
                if (isSourceLocaleActive) {
                  void handlePublish()
                  return
                }
                publishArticleTranslation.mutate(activeLocale, {
                  onSuccess: () => toast.success(`${getHelpcenterLocaleLabel(activeLocale)} translation published`),
                  onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to publish translation'),
                })
              }}
              disabled={publishDisabled}
            >
              <Send className="h-3 w-3" />
              {activePublishLabel}
            </Button>
          </QuickTooltip>
        )}

        <QuickTooltip label="Document details">
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8 shrink-0"
            onClick={() => setMetaOpen((v) => !v)}
          >
            <MoreHorizontal className="h-4 w-4" />
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
                translationState: localeRowsByLocale.get(locale)?.state ?? 'missing',
              }))}
              onSelectLocale={handleSelectLocale}
              onOpenSettings={(locale) => setEditingTranslationLocale(locale)}
            />
          </div>
        </div>
      )}

      {/* Archive banner */}
      {doc.status === 'archived' && (
        <div className="flex items-center gap-2 border-b border-amber-500/30 bg-amber-500/10 px-4 py-2">
          <Archive className="h-4 w-4 text-amber-600 dark:text-amber-400" />
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
              <ArchiveRestore className="h-3 w-3" />
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
            <Lock className="h-4 w-4 text-blue-600 dark:text-blue-400" />
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
                <Unlock className="h-3 w-3" />
                Unlock
              </Button>
            )}
          </div>
        )
      })()}

      {/* Version preview banner */}
      {previewVersion && (
        <div className="flex items-center gap-3 border-b border-purple-500/30 bg-purple-500/10 px-4 py-2">
          <Eye className="h-4 w-4 text-purple-600 dark:text-purple-400 shrink-0" />
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
              <RotateCcw className="h-3 w-3" />
              Restore
            </Button>
          )}
          <Button
            variant="ghost"
            size="sm"
            className="h-7 gap-1 text-xs shrink-0"
            onClick={() => setPreviewVersion(null)}
          >
            <X className="h-3 w-3" />
            Exit preview
          </Button>
        </div>
      )}

      {/* Main content area */}
      <div className="flex min-h-0 flex-1">
        {/* Editor */}
        <div className="flex min-w-0 flex-1 flex-col">
          {previewVersion ? (
            <DocsEditor
              key={`preview-${previewVersion.id}`}
              title={titleDraft}
              initialContent={previewVersion.content as JSONContent | null}
              onSave={handleSave}
              readOnly
            />
          ) : (
            <DocsEditor
              key={isSourceLocaleActive ? 'source-editor' : `translation-${activeLocale}`}
              title={isSourceLocaleActive ? titleDraft : activeTranslationDraft.title}
              onTitleChange={!effectiveReadOnly ? (isSourceLocaleActive ? handleTitleChange : handleTranslationTitleChange) : undefined}
              slug={isSourceLocaleActive ? doc?.hc_slug : activeTranslationDraft.slug || undefined}
              initialContent={isSourceLocaleActive ? (content?.content as JSONContent | null) : activeTranslationDraft.content}
              onSave={isSourceLocaleActive ? handleSave : handleTranslationContentSave}
              readOnly={effectiveReadOnly}
              uploadConfig={
                !effectiveReadOnly
                  ? { workspaceId: wsId, entityType: 'editor_upload', entityId: docId }
                  : undefined
              }
              topBanner={showLocalePills && !isSourceLocaleActive && activeTranslation ? (
                <div className="flex items-center justify-center gap-2 bg-muted/20 px-4 py-2 mx-6 mt-3 rounded-lg">
                  <WandSparkles className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                  <span className="text-xs text-muted-foreground">
                    Regenerate this translation from the latest source article using AI. Protected terms from your settings will be preserved.
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-7 gap-1.5 text-xs shrink-0"
                    disabled={regenerating}
                    onClick={() => setRegenerateConfirmOpen(true)}
                  >
                    <WandSparkles className="h-3 w-3" />
                    {regenerating ? 'Regenerating...' : 'Regenerate with AI'}
                  </Button>
                </div>
              ) : undefined}
              generatingOverlay={
                (regenerating || generateArticleTranslation.isPending) && !isSourceLocaleActive
                  ? 'Generating translation with AI...'
                  : null
              }
            />
          )}
        </div>

        {/* Metadata sidebar */}
        {metaOpen && (
          <aside className="w-64 shrink-0 overflow-y-auto border-l border-border/60 px-4 py-5">
            {/* ── Sharing ── */}
            {canEditDocs && (
              <>
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-1.5">
                      <Link2 className="h-3.5 w-3.5 text-muted-foreground" />
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
                      {linkCopied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                      {linkCopied ? 'Copied!' : 'Copy link'}
                    </Button>
                  )}
                </div>
                <Separator className="my-4" />
              </>
            )}

            {/* ── Properties ── */}
            <div className="grid grid-cols-[16px_72px_1fr] items-center gap-x-2 gap-y-2.5">
              {/* Owner */}
              <User className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
              <span className="text-xs text-muted-foreground self-center">Owner</span>
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
              <UserCheck className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
              <span className="text-xs text-muted-foreground self-center">Created by</span>
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
              <FolderOpen className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
              <span className="text-xs text-muted-foreground self-center">Collection</span>
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

              {/* Created */}
              <CalendarDays className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
              <span className="text-xs text-muted-foreground self-center">Created</span>
              <div className="min-w-0 self-center">
                <span className="text-xs">{format(parseISO(doc.created_at), isThisYear(parseISO(doc.created_at)) ? 'MMM d, h:mm a' : 'MMM d, yyyy h:mm a')}</span>
              </div>

              {/* Updated */}
              <Clock className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
              <span className="text-xs text-muted-foreground self-center">Updated</span>
              <div className="min-w-0 self-center">
                <span className="text-xs">{format(parseISO(doc.updated_at), isThisYear(parseISO(doc.updated_at)) ? 'MMM d, h:mm a' : 'MMM d, yyyy h:mm a')}</span>
              </div>

              {/* Published */}
              {doc.published_at && (
                <>
                  <Globe className="h-3.5 w-3.5 shrink-0 text-muted-foreground self-center" />
                  <span className="text-xs text-muted-foreground self-center">Published</span>
                  <div className="min-w-0 self-center">
                    <span className="text-xs">{format(parseISO(doc.published_at), isThisYear(parseISO(doc.published_at)) ? 'MMM d, h:mm a' : 'MMM d, yyyy h:mm a')}</span>
                  </div>
                </>
              )}
            </div>

            <Separator className="my-4" />

            {/* ── History & Links ── */}
            <div className="space-y-1">
              <button
                type="button"
                onClick={() => { setVersionsOpen(true); setMetaOpen(false) }}
                className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
              >
                <Clock className="h-3.5 w-3.5" />
                Version History
              </button>
              <button
                type="button"
                onClick={() => setLinksOpen(true)}
                className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                Linked Items
              </button>
            </div>

            {/* ── Document Actions ── */}
            {canEditDocs && (
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
                      {doc.is_locked ? <Unlock className="h-3.5 w-3.5" /> : <Lock className="h-3.5 w-3.5" />}
                      {doc.is_locked ? 'Unlock document' : 'Lock document'}
                    </button>
                  )}
                  {doc.status === 'published' && (
                    <button
                      type="button"
                      onClick={() => {
                        unpublishDoc.mutate(docId, {
                          onSuccess: () => toast.success('Reverted to draft'),
                          onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to revert'),
                        })
                      }}
                      disabled={unpublishDoc.isPending || doc.is_locked}
                      className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
                    >
                      <RotateCcw className="h-3.5 w-3.5" />
                      Revert to draft
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => setMoveDialogOpen(true)}
                    disabled={doc.is_locked}
                    className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
                  >
                    <FolderInput className="h-3.5 w-3.5" />
                    Move to...
                  </button>
                  {doc.status !== 'archived' && (
                    <button
                      type="button"
                      onClick={handleArchive}
                      disabled={doc.is_locked}
                      className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
                    >
                      <Archive className="h-3.5 w-3.5" />
                      Archive
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => setDeleteConfirmOpen(true)}
                    disabled={doc.is_locked}
                    className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-destructive transition-colors hover:bg-destructive/10 disabled:opacity-50 disabled:pointer-events-none"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
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
      </div>

      {/* Slide-out panels */}
      <DocumentLinksPanel
        wsId={wsId}
        docId={docId}
        open={linksOpen}
        onOpenChange={setLinksOpen}
        canEdit={canEditDocs}
      />

      <Sheet open={translationsOpen} onOpenChange={setTranslationsOpen}>
        <SheetContent className="w-full sm:max-w-2xl">
          <SheetHeader className="border-b border-border/60">
            <SheetTitle className="flex items-center gap-2">
              <Languages className="h-4 w-4 text-primary" />
              Article translations
            </SheetTitle>
            <SheetDescription>
              Manage localized public variants for this article. The default locale stays mirrored from the source document.
            </SheetDescription>
          </SheetHeader>

          <div className="flex-1 overflow-y-auto p-4">
            <TranslationsPanel
              title="Public language variants"
              description="Publish each locale independently once its parent path and localized body are ready."
              locales={enabledLocales}
              rows={articleTranslationRows}
              onAdd={setEditingTranslationLocale}
              onEdit={setEditingTranslationLocale}
              onPublish={(locale) => {
                publishArticleTranslation.mutate(locale, {
                  onSuccess: () => toast.success('Article translation published'),
                  onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to publish translation'),
                })
              }}
              onUnpublish={(locale) => {
                unpublishArticleTranslation.mutate(locale, {
                  onSuccess: () => toast.success('Article translation reverted to draft'),
                  onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to unpublish translation'),
                })
              }}
              onMarkReviewed={(locale) => {
                markArticleTranslationReviewed.mutate(locale, {
                  onSuccess: () => toast.success('Article translation marked as reviewed'),
                  onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to mark translation reviewed'),
                })
              }}
            />
          </div>
        </SheetContent>
      </Sheet>

      {editingTranslationLocale && (
        <EditArticleTranslationDialog
          key={`${editingTranslationLocale}-${articleTranslationsByLocale.get(editingTranslationLocale)?.updated_at ?? 'new'}`}
          open={Boolean(editingTranslationLocale)}
          onOpenChange={(open) => {
            if (!open) setEditingTranslationLocale(null)
          }}
          locale={editingTranslationLocale}
          sourceTitle={doc.title}
          sourceSlug={doc.hc_slug}
          sourceExcerpt={doc.excerpt}
          sourceContent={(content?.content as JSONContent | null | undefined) ?? null}
          translation={activeDialogTranslation}
          isSaving={upsertArticleTranslation.isPending}
          mode={editingTranslationLocale === activeLocale && !isSourceLocaleActive ? 'details_only' : 'full'}
          onSave={async (data) => {
            await handleSaveTranslation(data)
            if (editingTranslationLocale === activeLocale) {
              setTranslationDrafts((current) => ({
                ...current,
                [translationDraftKey(docId, data.locale)]: {
                  locale: data.locale,
                  title: data.title,
                  slug: data.slug,
                  excerpt: data.excerpt ?? '',
                  seoTitle: data.seo_title ?? '',
                  seoDescription: data.seo_description ?? '',
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
        </>
      )}
      {/* Slug confirmation dialog for external help center articles */}
      <Dialog open={slugDialogOpen} onOpenChange={setSlugDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Confirm article URL slug</DialogTitle>
            <DialogDescription>
              This slug will be used in the public help center URL.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="slug">Slug</Label>
            <div className="flex items-center">
              <span className="inline-flex h-9 items-center rounded-l-md border border-r-0 border-input bg-muted px-3 text-sm text-muted-foreground">/</span>
              <Input
                id="slug"
                value={pendingSlug}
                onChange={(e) => setPendingSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
                placeholder="article-slug"
                className="rounded-l-none"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSlugDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleConfirmPublish} disabled={!pendingSlug || publishDoc.isPending}>
              <Send className="h-3.5 w-3.5 mr-1.5" />
              Publish
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
