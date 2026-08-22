import { Briefcase, Maximize2, RefreshCw, Smile, SpellCheck, type LucideIcon } from 'lucide-react'
import type { SupportAIRewriteOperation } from '@helpin-ai/support-core'
import { Sheet } from '@mobile/ui/sheet'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'

interface AITool {
  operation: SupportAIRewriteOperation
  label: string
  hint: string
  icon: LucideIcon
}

/** The AI rewrite operations, mirroring the web composer's set. */
export const AI_TOOLS: AITool[] = [
  { operation: 'fix_grammar', label: 'Fix spelling & grammar', hint: 'Clean up mistakes', icon: SpellCheck },
  { operation: 'rephrase', label: 'Rephrase', hint: 'Say it a different way', icon: RefreshCw },
  { operation: 'more_friendly', label: 'More friendly', hint: 'Warmer, casual tone', icon: Smile },
  { operation: 'more_formal', label: 'More formal', hint: 'Professional tone', icon: Briefcase },
  { operation: 'expand', label: 'Expand', hint: 'Add more detail', icon: Maximize2 },
]

export interface AIToolsSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The operation currently running (disables the list + shows a spinner on that row), or null. */
  busyOperation: SupportAIRewriteOperation | null
  onSelect: (operation: SupportAIRewriteOperation) => void
}

export function AIToolsSheet({ open, onOpenChange, busyOperation, onSelect }: AIToolsSheetProps) {
  const busy = busyOperation !== null
  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="AI writing tools">
      <div className="px-4 pb-2">
        <h2 className="px-1 pb-1 text-headline font-semibold">AI writing tools</h2>
        <p className="px-1 pb-2 text-footnote text-muted-foreground">Rewrite your draft — you can undo after.</p>
        <div className="flex flex-col">
          {AI_TOOLS.map((tool) => {
            const Icon = tool.icon
            const rowBusy = busyOperation === tool.operation
            return (
              <Pressable
                key={tool.operation}
                disabled={busy}
                haptic="selection"
                onPress={() => onSelect(tool.operation)}
                aria-label={tool.label}
                className="flex items-center gap-3 rounded-xl px-2 py-3 text-left active:bg-muted disabled:opacity-50"
              >
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                  {rowBusy ? <Spinner className="h-4 w-4" /> : <Icon className="h-5 w-5" />}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-body font-medium">{tool.label}</span>
                  <span className="block truncate text-footnote text-muted-foreground">{tool.hint}</span>
                </span>
              </Pressable>
            )
          })}
        </div>
      </div>
    </Sheet>
  )
}
