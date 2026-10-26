import type { WebhookEventType } from './pmTypes'

export const WEBHOOK_EVENTS: { value: WebhookEventType; label: string; description: string }[] = [
  { value: 'conversation.created', label: 'Conversation created', description: 'A customer or teammate starts a conversation.' },
  { value: 'message.created', label: 'Message', description: 'A customer, teammate or the AI posts a public reply.' },
  { value: 'conversation.status_changed', label: 'Status changed', description: 'Opened, waiting, resolved or marked as spam.' },
  { value: 'conversation.assigned', label: 'Assigned', description: 'The assignee changes or is removed.' },
  { value: 'conversation.priority_changed', label: 'Priority changed', description: 'The priority is raised or lowered.' },
  { value: 'conversation.snoozed', label: 'Snoozed', description: 'A teammate snoozes the conversation.' },
  { value: 'sla.breached', label: 'SLA missed', description: 'A response or resolution target is missed.' },
  { value: 'csat.submitted', label: 'Rating', description: 'A customer rates a resolved conversation.' },
]

const LABELS = new Map<string, string>([...WEBHOOK_EVENTS.map((event) => [event.value, event.label] as const), ['ping', 'Test event']])

export function webhookEventLabel(event: string): string {
  return LABELS.get(event) ?? event
}

/** Short summary of an endpoint's subscription. */
export function describeWebhookEvents(events: WebhookEventType[]): string {
  if (events.length === 0) return 'All events'
  if (events.length <= 2) return events.map(webhookEventLabel).join(', ')
  return `${events.length} events`
}

/** Validates a webhook URL the way the server does, for instant feedback. */
export function webhookURLProblem(raw: string): string | null {
  const value = raw.trim()
  if (!value) return 'Enter the URL that receives events.'
  let url: URL
  try {
    url = new URL(value)
  } catch {
    return 'Enter a full URL, starting with https://.'
  }
  if (url.protocol !== 'https:' && url.protocol !== 'http:') return 'The URL must start with https://.'
  if (url.username || url.password) return 'Remove the user name and password from the URL.'
  return null
}
