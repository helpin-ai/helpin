import { Check, Sparkles } from 'lucide-react'

import type { UpgradeRequiredReason } from '@mobile/lib/upgrade-required'
import { Pressable } from '@mobile/ui/pressable'
import { Sheet } from '@mobile/ui/sheet'

export interface UpgradeRequiredSheetProps {
  reason: UpgradeRequiredReason | null
  onOpenChange: (open: boolean) => void
}

export function UpgradeRequiredSheet({ reason, onOpenChange }: UpgradeRequiredSheetProps) {
  if (!reason) return null

  return (
    <Sheet
      open
      onOpenChange={onOpenChange}
      title={reason.title}
    >
      <div className="flex flex-col gap-4 px-5 pb-2">
        <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
          <Sparkles className="h-5 w-5" />
        </span>
        <div>
          <h2 className="text-title font-semibold text-foreground">{reason.title}</h2>
          <p className="mt-1 text-body text-muted-foreground">{reason.message}</p>
        </div>
        <div className="flex items-start gap-2 rounded-xl border border-border/70 bg-muted/40 px-3 py-3">
          <Check className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
          <span className="text-footnote text-foreground">{reason.primaryBenefit}</span>
        </div>
        <p className="text-footnote text-muted-foreground">
          Open Helpin on the web to manage billing, or ask a workspace owner to upgrade.
        </p>
        <Pressable
          haptic="selection"
          onPress={() => onOpenChange(false)}
          className="flex h-12 items-center justify-center rounded-2xl bg-primary text-body font-medium text-primary-foreground"
        >
          Close
        </Pressable>
      </div>
    </Sheet>
  )
}
