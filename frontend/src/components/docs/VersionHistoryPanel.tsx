import { format, parseISO } from 'date-fns'
import { Clock, RotateCcw, Plus } from 'lucide-react'
import { toast } from 'sonner'
import {
  useDocsVersions,
  useCreateDocsVersion,
  useRevertDocsVersion,
} from '@/hooks/queries'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'

interface VersionHistoryPanelProps {
  wsId: string
  docId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function VersionHistoryPanel({
  wsId,
  docId,
  open,
  onOpenChange,
}: VersionHistoryPanelProps) {
  const { data: versions, isLoading } = useDocsVersions(wsId, docId)
  const createVersion = useCreateDocsVersion(wsId)
  const revertVersion = useRevertDocsVersion(wsId)

  const handleSnapshot = async () => {
    const label = window.prompt('Snapshot label (optional)')
    try {
      await createVersion.mutateAsync({
        docId,
        snapshot_label: label || undefined,
      })
      toast.success('Version snapshot created')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create snapshot')
    }
  }

  const handleRevert = async (versionId: string) => {
    if (!window.confirm('Revert to this version? Current content will be preserved as a snapshot.')) return
    try {
      await revertVersion.mutateAsync({ docId, versionId })
      toast.success('Reverted to selected version')
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to revert')
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-80 sm:w-96">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2">
            <Clock className="h-4 w-4" />
            Version History
          </SheetTitle>
        </SheetHeader>

        <div className="mt-4 space-y-3">
          <Button
            variant="outline"
            size="sm"
            className="w-full gap-1.5"
            onClick={handleSnapshot}
            disabled={createVersion.isPending}
          >
            <Plus className="h-3.5 w-3.5" />
            Create Snapshot
          </Button>

          {isLoading ? (
            <div className="space-y-2 py-4">
              {[1, 2, 3].map((i) => (
                <div key={i} className="h-14 animate-pulse rounded-md bg-muted/60" />
              ))}
            </div>
          ) : !versions || versions.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              No versions yet. Snapshots are created automatically when you publish.
            </p>
          ) : (
            <div className="space-y-1">
              {versions.map((version) => (
                <div
                  key={version.id}
                  className="group flex items-start gap-3 rounded-md border border-border/40 p-3 transition-colors hover:bg-muted/30"
                >
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium">
                      {version.snapshot_label || 'Untitled snapshot'}
                    </p>
                    <p className="text-[11px] text-muted-foreground">
                      {format(parseISO(version.created_at), 'MMM d, yyyy h:mm a')}
                    </p>
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
                    title="Revert to this version"
                    onClick={() => handleRevert(version.id)}
                    disabled={revertVersion.isPending}
                  >
                    <RotateCcw className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
