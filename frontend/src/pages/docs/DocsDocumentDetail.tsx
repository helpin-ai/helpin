import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from '@tanstack/react-router'
import { format, parseISO } from 'date-fns'
import type { JSONContent } from '@tiptap/react'
import {
  ArrowLeft,
  Archive,
  ArchiveRestore,
  ChevronRight,
  Clock,
  ExternalLink,
  FileText,
  Globe,
  Link2,
  MoreHorizontal,
  Send,
  Trash2,
} from 'lucide-react'
import { toast } from 'sonner'
import { useTitle } from '@/hooks/useTitle'
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
  useArchiveDocsDocument,
  useUnarchiveDocsDocument,
  useDeleteDocsDocument,
  useAssignableMembers,
  useWorkspaceAccess,
  usePermissions,
} from '@/hooks/queries'
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover'
import { UserAvatar } from '@/components/pm/UserAvatar'
import { formatAssignableMemberName } from '@/lib/assignableMembers'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { DocsEditor } from '@/components/docs/DocsEditor'
import { VersionHistoryPanel } from '@/components/docs/VersionHistoryPanel'
import { DocumentLinksPanel } from '@/components/docs/DocumentLinksPanel'
import { ExternalPublishPanel } from '@/components/docs/ExternalPublishPanel'
import { DOC_TYPE_LABELS, DOC_STATUS_LABELS } from '@/lib/docsTypes'

export function DocsDocumentDetail() {
  const navigate = useNavigate()
  const { docId } = useParams({ strict: false }) as { docId: string }
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const { data: access } = useWorkspaceAccess(wsId)
  const { canEditDocs, canPublishDocs } = usePermissions(access)

  const { data: doc, isLoading: docLoading } = useDocsDocument(wsId, docId)
  const { data: content, isLoading: contentLoading } = useDocsContent(wsId, docId)

  const saveContent = useSaveDocsContent(wsId)
  const updateDoc = useUpdateDocsDocument(wsId)
  const publishDoc = usePublishDocsDocument(wsId)
  const archiveDoc = useArchiveDocsDocument(wsId)
  const unarchiveDoc = useUnarchiveDocsDocument(wsId)
  const deleteDoc = useDeleteDocsDocument(wsId)

  const { data: space } = useDocsSpace(wsId, doc?.space_id ?? '')
  const { data: members = [] } = useAssignableMembers(wsId)
  const { data: collections = [] } = useDocsCollections(wsId, doc?.space_id ?? '')
  const createCollection = useCreateDocsCollection(wsId, doc?.space_id ?? '')

  const [metaOpen, setMetaOpen] = useState(false)
  const [versionsOpen, setVersionsOpen] = useState(false)
  const [linksOpen, setLinksOpen] = useState(false)
  const [publishOpen, setPublishOpen] = useState(false)

  // Title state — local draft synced from server, debounced save
  const [titleDraft, setTitleDraft] = useState('')
  const titleInitRef = useRef(false)
  const titleTimerRef = useRef<ReturnType<typeof setTimeout>>()

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

  const handlePublish = async () => {
    try {
      await publishDoc.mutateAsync(docId)
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
          onClick={() => navigate({ to: '/w/$slug/docs', params: { slug: wsSlug } })}
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
                <span className="truncate">{col.icon ? `${col.icon} ` : ''}{col.name}</span>
              </>
            )
          })()}
        </nav>

        <Badge
          variant={doc.status === 'published' ? 'default' : 'secondary'}
          className="shrink-0 text-[10px]"
        >
          {DOC_STATUS_LABELS[doc.status] ?? doc.status}
        </Badge>

        {canPublishDocs && doc.status === 'draft' && (
          <Button
            size="sm"
            className="h-7 gap-1.5 text-xs"
            onClick={handlePublish}
            disabled={publishDoc.isPending}
          >
            <Send className="h-3 w-3" />
            Publish
          </Button>
        )}

        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 shrink-0"
          onClick={() => setMetaOpen((v) => !v)}
          title="Document details"
        >
          <MoreHorizontal className="h-4 w-4" />
        </Button>
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

      {/* Main content area */}
      <div className="flex min-h-0 flex-1">
        {/* Editor */}
        <div className="flex min-w-0 flex-1 flex-col">
          <DocsEditor
            title={titleDraft}
            onTitleChange={canEditDocs && doc.status !== 'archived' ? handleTitleChange : undefined}
            initialContent={content?.content as JSONContent | null}
            onSave={handleSave}
            readOnly={!canEditDocs || doc.status === 'archived'}
            uploadConfig={
              canEditDocs && doc.status !== 'archived'
                ? { workspaceId: wsId, entityType: 'editor_upload', entityId: docId }
                : undefined
            }
          />
        </div>

        {/* Metadata sidebar */}
        {metaOpen && (
          <div className="w-64 shrink-0 overflow-y-auto border-l border-border/60 bg-muted/20 p-4">
            <div className="space-y-5">
              {/* Document info */}
              <section className="space-y-2">
                <dl className="space-y-2 text-xs">
                  <div className="flex justify-between">
                    <dt className="text-muted-foreground">Type</dt>
                    <dd>{DOC_TYPE_LABELS[doc.doc_type] ?? doc.doc_type}</dd>
                  </div>

                  {/* Owner */}
                  <div className="flex items-center justify-between">
                    <dt className="text-muted-foreground">Owner</dt>
                    <dd>
                      {canEditDocs ? (
                        <MemberPickerPopover
                          value={doc.owner_id ?? '__none__'}
                          members={members}
                          onChange={(id) =>
                            patchDoc({ owner_id: id === '__none__' ? null : id })
                          }
                          noneLabel="Unassigned"
                          renderTrigger={() => {
                            const owner = members.find((m) => m.id === doc.owner_id)
                            if (!owner) return <span className="text-muted-foreground">Unassigned</span>
                            return (
                              <>
                                <UserAvatar
                                  name={owner.display_name || owner.email}
                                  avatarUrl={owner.avatar_url}
                                  className="h-4 w-4"
                                  fallbackClassName="text-[7px]"
                                />
                                <span className="truncate max-w-[100px]">
                                  {formatAssignableMemberName(owner)}
                                </span>
                              </>
                            )
                          }}
                        />
                      ) : (
                        (() => {
                          const owner = members.find((m) => m.id === doc.owner_id)
                          return owner ? (
                            <span className="flex items-center gap-1">
                              <UserAvatar
                                name={owner.display_name || owner.email}
                                avatarUrl={owner.avatar_url}
                                className="h-4 w-4"
                                fallbackClassName="text-[7px]"
                              />
                              {formatAssignableMemberName(owner)}
                            </span>
                          ) : (
                            <span className="text-muted-foreground">Unassigned</span>
                          )
                        })()
                      )}
                    </dd>
                  </div>

                  {/* Collection */}
                  <div className="flex items-center justify-between">
                    <dt className="text-muted-foreground">Collection</dt>
                    <dd>
                      {canEditDocs ? (
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
                          <SelectTrigger className="h-6 w-auto min-w-[80px] max-w-[130px] border-0 bg-transparent px-1 py-0 text-xs shadow-none hover:bg-accent">
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
                        <span>
                          {collections.find((c) => c.id === doc.collection_id)?.name ?? 'None'}
                        </span>
                      )}
                    </dd>
                  </div>

                  <div className="flex justify-between">
                    <dt className="text-muted-foreground">Created</dt>
                    <dd>{format(parseISO(doc.created_at), 'MMM d, yyyy')}</dd>
                  </div>
                  {doc.published_at && (
                    <div className="flex justify-between">
                      <dt className="text-muted-foreground">Published</dt>
                      <dd>{format(parseISO(doc.published_at), 'MMM d, yyyy')}</dd>
                    </div>
                  )}
                </dl>
              </section>


              {/* Actions */}
              <section className="space-y-2">
                <h3 className="flex items-center gap-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  <Link2 className="h-3 w-3" />
                  Actions
                </h3>
                <div className="space-y-1">
                  <button
                    type="button"
                    onClick={() => setVersionsOpen(true)}
                    className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
                  >
                    <Clock className="h-3.5 w-3.5" />
                    Version History
                  </button>
                  <button
                    type="button"
                    onClick={() => setPublishOpen(true)}
                    className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
                  >
                    <Globe className="h-3.5 w-3.5" />
                    External Publish
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
              </section>

              {/* Danger zone */}
              {canEditDocs && (
                <section className="space-y-2">
                  <div className="space-y-1">
                    {doc.status !== 'archived' && (
                      <button
                        type="button"
                        onClick={handleArchive}
                        className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
                      >
                        <Archive className="h-3.5 w-3.5" />
                        Archive
                      </button>
                    )}
                    <button
                      type="button"
                      onClick={handleDelete}
                      className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-xs text-destructive transition-colors hover:bg-destructive/10"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                      Delete
                    </button>
                  </div>
                </section>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Slide-out panels */}
      <VersionHistoryPanel
        wsId={wsId}
        docId={docId}
        open={versionsOpen}
        onOpenChange={setVersionsOpen}
      />
      <DocumentLinksPanel
        wsId={wsId}
        docId={docId}
        open={linksOpen}
        onOpenChange={setLinksOpen}
        canEdit={canEditDocs}
      />
      {doc && (
        <ExternalPublishPanel
          wsId={wsId}
          doc={doc}
          open={publishOpen}
          onOpenChange={setPublishOpen}
        />
      )}
    </div>
  )
}
