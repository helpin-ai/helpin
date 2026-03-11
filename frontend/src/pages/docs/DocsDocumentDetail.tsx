import { useCallback, useEffect, useRef, useState } from 'react'
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
  useCreateDocsCollection,
  useSaveDocsContent,
  useUpdateDocsDocument,
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
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { DOC_STATUS_LABELS } from '@/lib/docsTypes'
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

export function DocsDocumentDetail() {
  const navigate = useNavigate()
  const router = useRouter()
  const { docId } = useParams({ strict: false }) as { docId: string }
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const { data: access } = useWorkspaceAccess(wsId)
  const { canEditDocs, canPublishDocs, isAdmin } = usePermissions(access)
  const currentUserId = useAuthStore((s) => s.user?.id)

  const { data: doc, isLoading: docLoading } = useDocsDocument(wsId, docId)
  const { data: content, isLoading: contentLoading } = useDocsContent(wsId, docId)

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
  const createCollection = useCreateDocsCollection(wsId, doc?.space_id ?? '')

  const toggleShare = useToggleDocShare(wsId)
  const toggleLock = useToggleDocLock(wsId)

  // Lock-aware editing: locked docs are read-only for everyone — unlock to edit
  const effectiveReadOnly = !canEditDocs || doc?.status === 'archived' || !!doc?.is_locked
  const canUnlock = doc?.is_locked && (doc.locked_by === currentUserId || isAdmin)

  const [metaOpen, setMetaOpen] = useState(false)
  const [versionsOpen, setVersionsOpen] = useState(false)
  const [moveDialogOpen, setMoveDialogOpen] = useState(false)
  const [linksOpen, setLinksOpen] = useState(false)
  const [slugDialogOpen, setSlugDialogOpen] = useState(false)
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

  // Title state — local draft synced from server, debounced save
  const [titleDraft, setTitleDraft] = useState('')
  const titleInitRef = useRef(false)
  const titleTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined)

  // Sync title from server on first load (or when doc changes externally)
  useEffect(() => {
    if (doc && !titleInitRef.current) {
      setTitleDraft(doc.title)
      titleInitRef.current = true
    }
  }, [doc])

  useTitle(titleDraft || 'Document')

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
      setTitleDraft(newTitle)
      if (titleTimerRef.current) clearTimeout(titleTimerRef.current)
      titleTimerRef.current = setTimeout(() => {
        const trimmed = newTitle.trim()
        if (trimmed && trimmed !== doc?.title) {
          patchDoc({ title: trimmed })
        }
      }, 800)
    },
    [patchDoc, doc?.title],
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

        {canPublishDocs && doc.status === 'draft' && (
          <Button
            size="sm"
            className="h-7 gap-1.5 text-xs"
            onClick={handlePublish}
            disabled={publishDoc.isPending || doc.is_locked}
          >
            <Send className="h-3 w-3" />
            Publish
          </Button>
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
              title={titleDraft}
              onTitleChange={!effectiveReadOnly ? handleTitleChange : undefined}
              slug={doc?.hc_slug}
              initialContent={content?.content as JSONContent | null}
              onSave={handleSave}
              readOnly={effectiveReadOnly}
              uploadConfig={
                !effectiveReadOnly
                  ? { workspaceId: wsId, entityType: 'editor_upload', entityId: docId }
                  : undefined
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
                    onClick={handleDelete}
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
      {doc && (
        <MoveDocumentDialog
          wsId={wsId}
          open={moveDialogOpen}
          onOpenChange={setMoveDialogOpen}
          docId={docId}
          docTitle={doc.title}
          currentSpaceId={doc.space_id}
          currentCollectionId={doc.collection_id}
        />
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
