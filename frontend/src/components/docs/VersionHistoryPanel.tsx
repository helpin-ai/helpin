import { useState } from 'react'
import { format, parseISO } from 'date-fns'
import {
  Clock01Icon,
  RotateLeft01Icon,
  PlusSignIcon,
  ViewIcon,
  ViewOffIcon,
  PencilEdit01Icon,
  Tick01Icon,
  Cancel01Icon,
  UserIcon,
  ZapIcon,
  GlobeIcon,
  Bookmark01Icon,
  Clock03Icon,
} from '@/lib/icons'
import { toast } from 'sonner'
import {
  useDocsVersions,
  useCreateDocsVersion,
  useRevertDocsVersion,
  useUpdateDocsVersionLabel,
} from '@/hooks/queries'
import type { DocsVersion, VersionType } from '@/lib/docsTypes'
import { VERSION_TYPE_LABELS } from '@/lib/docsTypes'
import type { AssignableMember } from '@/lib/types'
import { formatAssignableMemberName } from '@/lib/assignableMembers'
import { UserAvatar } from '@/components/pm/UserAvatar'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { QuickTooltip } from '@/components/ui/quick-tooltip'

// ── Version type badge (exported for use in preview banner) ─────────────────

const TYPE_BADGE_STYLES: Record<VersionType, string> = {
  manual: 'bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20',
  auto: 'bg-gray-500/10 text-gray-600 dark:text-gray-400 border-gray-500/20',
  publish: 'bg-green-500/10 text-green-700 dark:text-green-400 border-green-500/20',
  revert: 'bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/20',
}

const TYPE_ICONS: Record<VersionType, React.ComponentType<{ className?: string }>> = {
  manual: Bookmark01Icon,
  auto: ZapIcon,
  publish: GlobeIcon,
  revert: Clock03Icon,
}

export function VersionTypeBadge({ type }: { type: VersionType }) {
  const Icon = TYPE_ICONS[type] || Clock01Icon
  return (
    <Badge
      variant="outline"
      className={`h-5 gap-1 px-1.5 text-[10px] font-medium ${TYPE_BADGE_STYLES[type] || ''}`}
    >
      <Icon className="h-2.5 w-2.5" />
      {VERSION_TYPE_LABELS[type] ?? type}
    </Badge>
  )
}

// ── Author display (exported for use in preview banner) ─────────────────────

export function AuthorDisplay({ userId, members }: { userId: string; members: AssignableMember[] }) {
  const member = members.find((m) => m.user_id === userId)
  if (!member) {
    return (
      <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
        <UserIcon className="h-3 w-3" />
        System
      </span>
    )
  }
  return (
    <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
      <UserAvatar
        name={member.display_name || member.email}
        avatarUrl={member.avatar_url}
        className="h-3.5 w-3.5"
        fallbackClassName="text-[6px]"
      />
      {formatAssignableMemberName(member)}
    </span>
  )
}

// ── Inline label editor ──────────────────────────────────────────────────────

function InlineLabelEditor({
  versionId,
  docId,
  currentLabel,
  wsId,
  onDone,
}: {
  versionId: string
  docId: string
  currentLabel: string
  wsId: string
  onDone: () => void
}) {
  const [label, setLabel] = useState(currentLabel)
  const updateLabel = useUpdateDocsVersionLabel(wsId)

  const handleSave = async () => {
    try {
      await updateLabel.mutateAsync({
        docId,
        versionId,
        snapshot_label: label.trim() || undefined,
      })
      toast.success('Label updated')
      onDone()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update label')
    }
  }

  return (
    <div className="flex items-center gap-1">
      <Input
        value={label}
        onChange={(e) => setLabel(e.target.value)}
        className="h-6 text-xs px-1.5"
        placeholder="Snapshot label"
        autoFocus
        onKeyDown={(e) => {
          if (e.key === 'Enter') handleSave()
          if (e.key === 'Escape') onDone()
        }}
      />
      <Button
        variant="ghost"
        size="icon"
        className="h-6 w-6"
        onClick={handleSave}
        disabled={updateLabel.isPending}
      >
        <Tick01Icon className="h-3 w-3" />
      </Button>
      <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onDone}>
        <Cancel01Icon className="h-3 w-3" />
      </Button>
    </div>
  )
}

// ── Create snapshot inline form ──────────────────────────────────────────────

function CreateSnapshotInline({
  wsId,
  docId,
}: {
  wsId: string
  docId: string
}) {
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState('')
  const createVersion = useCreateDocsVersion(wsId)

  const handleCreate = async () => {
    try {
      await createVersion.mutateAsync({
        docId,
        snapshot_label: label.trim() || undefined,
      })
      toast.success('Version snapshot created')
      setLabel('')
      setOpen(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create snapshot')
    }
  }

  if (!open) {
    return (
      <Button
        variant="outline"
        size="sm"
        className="w-full gap-1.5"
        onClick={() => setOpen(true)}
      >
        <PlusSignIcon className="h-3.5 w-3.5" />
        Create Snapshot
      </Button>
    )
  }

  return (
    <div className="space-y-2 rounded-md border border-border/60 p-3">
      <Input
        value={label}
        onChange={(e) => setLabel(e.target.value)}
        placeholder="Snapshot label (optional)"
        className="h-8 text-sm"
        autoFocus
        onKeyDown={(e) => {
          if (e.key === 'Enter') handleCreate()
          if (e.key === 'Escape') { setOpen(false); setLabel('') }
        }}
      />
      <div className="flex gap-2">
        <Button
          size="sm"
          className="h-7 flex-1 gap-1 text-xs"
          onClick={handleCreate}
          disabled={createVersion.isPending}
        >
          <PlusSignIcon className="h-3 w-3" />
          Create
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="h-7 text-xs"
          onClick={() => { setOpen(false); setLabel('') }}
        >
          Cancel
        </Button>
      </div>
    </div>
  )
}

// ── Main panel (inline sidebar, not a Sheet overlay) ────────────────────────

interface VersionHistoryPanelProps {
  wsId: string
  docId: string
  open: boolean
  onClose: () => void
  onPreview: (version: DocsVersion) => void
  members: AssignableMember[]
  canEdit: boolean
  previewingVersionId?: string | null
}

export function VersionHistoryPanel({
  wsId,
  docId,
  open,
  onClose,
  onPreview,
  members,
  canEdit,
  previewingVersionId,
}: VersionHistoryPanelProps) {
  const { data: versions, isLoading } = useDocsVersions(wsId, docId)
  const revertVersion = useRevertDocsVersion(wsId)
  const [editingLabelId, setEditingLabelId] = useState<string | null>(null)
  const [restoreConfirmId, setRestoreConfirmId] = useState<string | null>(null)

  const handleRestore = async (versionId: string) => {
    try {
      await revertVersion.mutateAsync({ docId, versionId })
      toast.success('Reverted to selected version')
      setRestoreConfirmId(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to revert')
    }
  }

  if (!open) return null

  return (
    <>
      <aside className="w-80 shrink-0 border-l border-border/60 flex flex-col bg-background">
        {/* Header */}
        <div className="flex items-center gap-2 border-b border-border/40 px-4 py-3">
          <Clock01Icon className="h-4 w-4 text-muted-foreground" />
          <h3 className="text-sm font-semibold flex-1">Version History</h3>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose}>
            <Cancel01Icon className="h-3.5 w-3.5" />
          </Button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto px-3 py-3 space-y-3">
          {canEdit && (
            <CreateSnapshotInline wsId={wsId} docId={docId} />
          )}

          {isLoading ? (
            <div className="space-y-2 py-4">
              {[1, 2, 3].map((i) => (
                <div key={i} className="h-16 animate-pulse rounded-md bg-muted/60" />
              ))}
            </div>
          ) : !versions || versions.length === 0 ? (
            <div className="py-12 text-center">
              <Clock01Icon className="h-8 w-8 text-muted-foreground/30 mx-auto mb-3" />
              <p className="text-sm text-muted-foreground">
                No versions yet.
              </p>
              <p className="text-xs text-muted-foreground/70 mt-1">
                Snapshots are created when you publish, and automatically during editing.
              </p>
            </div>
          ) : (
            <div className="space-y-1.5">
              {versions.map((version) => {
                const isPreviewing = previewingVersionId === version.id
                return (
                  <div
                    key={version.id}
                    className={`group rounded-lg border p-2.5 transition-colors cursor-pointer ${
                      isPreviewing
                        ? 'border-primary/50 bg-primary/5 ring-1 ring-primary/20'
                        : 'border-border/40 hover:bg-muted/30'
                    }`}
                    onClick={() => onPreview(version)}
                  >
                    <div className="flex items-start gap-2">
                      <div className="min-w-0 flex-1">
                        {editingLabelId === version.id ? (
                          <div onClick={(e) => e.stopPropagation()}>
                            <InlineLabelEditor
                              versionId={version.id}
                              docId={docId}
                              currentLabel={version.snapshot_label || ''}
                              wsId={wsId}
                              onDone={() => setEditingLabelId(null)}
                            />
                          </div>
                        ) : (
                          <div className="flex items-center gap-1.5">
                            <span className="text-xs font-medium truncate">
                              {version.snapshot_label || 'Untitled snapshot'}
                            </span>
                            {canEdit && version.version_type === 'manual' && (
                              <QuickTooltip label="Rename label">
                                <button
                                  type="button"
                                  className="opacity-0 group-hover:opacity-100 transition-opacity"
                                  onClick={(e) => { e.stopPropagation(); setEditingLabelId(version.id) }}
                                >
                                  <PencilEdit01Icon className="h-2.5 w-2.5 text-muted-foreground hover:text-foreground" />
                                </button>
                              </QuickTooltip>
                            )}
                          </div>
                        )}

                        <div className="flex items-center gap-1.5 mt-1">
                          <VersionTypeBadge type={version.version_type} />
                          <span className="text-[10px] text-muted-foreground">
                            {format(parseISO(version.created_at), 'MMM d h:mm a')}
                          </span>
                        </div>

                        <div className="mt-1">
                          <AuthorDisplay userId={version.created_by} members={members} />
                        </div>
                      </div>

                      {/* Actions */}
                      <div className="flex flex-col items-center gap-0.5 shrink-0" onClick={(e) => e.stopPropagation()}>
                        {isPreviewing ? (
                          <QuickTooltip label="Currently previewing">
                            <Button
                              variant="secondary"
                              size="icon"
                              className="h-6 w-6"
                              onClick={() => onPreview(version)}
                            >
                              <ViewOffIcon className="h-3 w-3" />
                            </Button>
                          </QuickTooltip>
                        ) : (
                          <QuickTooltip label="Preview">
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6 opacity-0 group-hover:opacity-100 transition-opacity"
                              onClick={() => onPreview(version)}
                            >
                              <ViewIcon className="h-3 w-3" />
                            </Button>
                          </QuickTooltip>
                        )}
                        {canEdit && (
                          <QuickTooltip label="Restore">
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6 opacity-0 group-hover:opacity-100 transition-opacity"
                              onClick={() => setRestoreConfirmId(version.id)}
                              disabled={revertVersion.isPending}
                            >
                              <RotateLeft01Icon className="h-3 w-3" />
                            </Button>
                          </QuickTooltip>
                        )}
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </aside>

      {/* Restore confirmation dialog */}
      <AlertDialog
        open={!!restoreConfirmId}
        onOpenChange={(v) => { if (!v) setRestoreConfirmId(null) }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Restore version?</AlertDialogTitle>
            <AlertDialogDescription>
              This will replace the current document content with this version.
              A snapshot of the current content will be saved automatically before restoring.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => restoreConfirmId && handleRestore(restoreConfirmId)}
              disabled={revertVersion.isPending}
            >
              Restore
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
