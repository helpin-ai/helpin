import { PenLine, WandSparkles } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { getHelpcenterLocaleLabel } from '@/lib/docsTypes'

export function MissingArticleTranslationDialog({
  open,
  locale,
  onOpenChange,
  onCreateManually,
  onGenerateWithAI,
  isGenerating,
}: {
  open: boolean
  locale: string
  onOpenChange: (open: boolean) => void
  onCreateManually: () => void
  onGenerateWithAI: () => void
  isGenerating: boolean
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{getHelpcenterLocaleLabel(locale)} translation</DialogTitle>
          <DialogDescription>
            Choose how you want to start this locale draft. You can open an empty workspace or generate a first pass with AI.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-3 py-2">
          <button type="button" onClick={onGenerateWithAI} disabled={isGenerating} className="w-full rounded-xl border border-border/60 bg-muted/30 px-4 py-3 text-left transition-colors hover:bg-muted/60 disabled:opacity-50">
            <div className="flex items-start gap-3">
              <WandSparkles className="mt-0.5 h-4 w-4 shrink-0" />
              <div className="space-y-1">
                <div className="text-sm font-medium">{isGenerating ? 'Generating…' : 'Generate with AI'}</div>
                <div className="text-xs text-muted-foreground">Create a translated draft from the current source article. Protected terms from your settings will be preserved.</div>
              </div>
            </div>
          </button>

          <button type="button" onClick={onCreateManually} className="w-full rounded-xl border border-border/60 bg-muted/30 px-4 py-3 text-left transition-colors hover:bg-muted/60">
            <div className="flex items-start gap-3">
              <PenLine className="mt-0.5 h-4 w-4 shrink-0" />
              <div className="space-y-1">
                <div className="text-sm font-medium">Create manually</div>
                <div className="text-xs text-muted-foreground">Start with an empty localized draft and edit it directly on the article page.</div>
              </div>
            </div>
          </button>
        </div>

        <DialogFooter>
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
