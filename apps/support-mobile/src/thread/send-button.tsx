import { ArrowRight, Check } from 'lucide-react'
import { cn } from '@mobile/lib/cn'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'

export type SendButtonState = 'disabled' | 'active' | 'sending' | 'sent'

export interface SendButtonProps {
  state: SendButtonState
  onPress: () => void
}

/**
 * 36px circular send affordance. Fixed size across every state (`h-9 w-9`,
 * with `min-h-0 min-w-0` to override `Pressable`'s 44px touch-target
 * minimums) so morphing disabled → active → sending → sent never shifts
 * surrounding layout — only the fill color and the icon inside change.
 * Disabled while `state` is `'disabled'` or `'sending'`, which also means
 * `Pressable` itself swallows taps during those two states (see
 * `pressable.tsx`), so `onPress` cannot fire mid-send.
 */
/** Per-state accessible name so VoiceOver/TalkBack announce the send lifecycle, not just a static "Send message" (A8). */
const STATE_LABELS: Record<SendButtonState, string> = {
  disabled: 'Send message',
  active: 'Send message',
  sending: 'Sending message',
  sent: 'Message sent',
}

export function SendButton({ state, onPress }: SendButtonProps) {
  const disabled = state === 'disabled' || state === 'sending'

  return (
    <Pressable
      aria-label={STATE_LABELS[state]}
      disabled={disabled}
      onPress={onPress}
      className={cn(
        'flex h-9 w-9 min-h-0 min-w-0 shrink-0 items-center justify-center rounded-full transition-colors duration-150',
        state === 'disabled' && 'bg-muted text-muted-foreground opacity-30',
        state === 'active' && 'bg-primary text-primary-foreground',
        state === 'sending' && 'bg-primary/30 text-primary-foreground',
        state === 'sent' && 'bg-emerald-500 text-white',
      )}
    >
      {state === 'sending' ? (
        <Spinner size={16} className="text-current" />
      ) : state === 'sent' ? (
        <Check className="h-4 w-4" />
      ) : (
        <ArrowRight className="h-5 w-5" />
      )}
    </Pressable>
  )
}
