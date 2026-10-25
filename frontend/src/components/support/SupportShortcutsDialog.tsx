import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { SUPPORT_SHORTCUTS } from '@/lib/supportShortcuts'

/** Lists the inbox keyboard shortcuts. */
export function SupportShortcutsDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Keyboard shortcuts</DialogTitle>
          <DialogDescription>Shortcuts work in the inbox when you are not typing.</DialogDescription>
        </DialogHeader>
        <dl className="divide-y divide-border/60 text-sm">
          {SUPPORT_SHORTCUTS.map((shortcut) => (
            <div key={shortcut.key} className="flex items-center justify-between gap-4 py-1.5">
              <dt className="text-muted-foreground">{shortcut.label}</dt>
              <dd>
                <kbd className="inline-flex min-w-6 items-center justify-center rounded border border-border bg-muted px-1.5 font-mono text-xs text-foreground">
                  {shortcut.key}
                </kbd>
              </dd>
            </div>
          ))}
        </dl>
      </DialogContent>
    </Dialog>
  )
}
