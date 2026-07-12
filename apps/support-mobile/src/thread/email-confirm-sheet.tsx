import { Check, Mail } from 'lucide-react'
import { Sheet } from '@mobile/ui/sheet'
import { Pressable } from '@mobile/ui/pressable'
import { cn } from '@mobile/lib/cn'

export interface EmailConfirmSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  customerEmail?: string
  dontAskAgain: boolean
  onToggleDontAskAgain: () => void
  onConfirm: () => void
}

/**
 * Confirms sending a reply that will actually be delivered as an **email**
 * (the widget visitor is offline and email-fallback is enabled), mirroring
 * web's offline-email confirm. Mobile-native: bottom sheet, big buttons.
 */
export function EmailConfirmSheet({
  open,
  onOpenChange,
  customerEmail,
  dontAskAgain,
  onToggleDontAskAgain,
  onConfirm,
}: EmailConfirmSheetProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Send as email?">
      <div className="flex flex-col gap-3 px-4 pb-2">
        <div className="flex items-start gap-3">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
            <Mail className="h-5 w-5" />
          </span>
          <div className="min-w-0">
            <h2 className="text-headline font-semibold">This will be sent as an email</h2>
            <p className="mt-0.5 text-footnote text-muted-foreground">
              The customer isn’t online, so your reply will be emailed
              {customerEmail ? <> to <span className="text-foreground">{customerEmail}</span></> : ' to them'}.
            </p>
          </div>
        </div>

        <Pressable
          onPress={onToggleDontAskAgain}
          aria-label="Don't ask again"
          className="flex items-center gap-2.5 rounded-xl px-1 py-2 text-left active:bg-muted"
        >
          <span
            className={cn(
              'flex h-5 w-5 shrink-0 items-center justify-center rounded-md border',
              dontAskAgain ? 'border-primary bg-primary text-primary-foreground' : 'border-input',
            )}
          >
            {dontAskAgain && <Check className="h-3.5 w-3.5" />}
          </span>
          <span className="text-body">Don’t ask again on this device</span>
        </Pressable>

        <div className="mt-1 flex flex-col gap-2">
          <Pressable
            haptic="impactLight"
            onPress={onConfirm}
            className="flex h-12 items-center justify-center rounded-2xl bg-primary text-body font-medium text-primary-foreground active:opacity-90"
          >
            Send as email
          </Pressable>
          <Pressable
            onPress={() => onOpenChange(false)}
            className="flex h-12 items-center justify-center rounded-2xl text-body font-medium text-muted-foreground active:bg-muted"
          >
            Cancel
          </Pressable>
        </div>
      </div>
    </Sheet>
  )
}
