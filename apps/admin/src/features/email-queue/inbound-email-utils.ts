import type { InboundEmail } from '@/lib/inboundEmailTypes'

export function emailDate(value: string) {
  const date = new Date(value)
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    ...(date.getFullYear() !== new Date().getFullYear() ? { year: 'numeric' as const } : {}),
  })
}

export function recoveryInProgress(email: InboundEmail) {
  return (
    email.status === 'processing' || (email.status === 'pending' && !!email.recovery_requested_at)
  )
}

export function emailStatus(email: InboundEmail) {
  if (email.status === 'completed') return 'Recovered'
  if (recoveryInProgress(email)) return 'Processing'
  if (email.status === 'pending' && !email.requires_destination) return 'Retry scheduled'
  return email.requires_destination ? 'Needs destination' : 'Processing failed'
}
