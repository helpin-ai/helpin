import type { MessageSenderType, SupportMessage } from '@helpin-ai/support-core'

export type ThreadItem =
  | { kind: 'day'; label: string }
  | { kind: 'system'; message: SupportMessage }
  | {
      kind: 'cluster'
      senderType: MessageSenderType
      senderName: string
      senderAvatarUrl?: string
      messages: SupportMessage[]
    }

/** Consecutive same-sender messages within this gap collapse into one cluster. */
const CLUSTER_GAP_MS = 3 * 60_000

function startOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate())
}

/**
 * "Today" / "Yesterday" / "Mon, Jul 6" (weekday, month, day) — year appended
 * only when it differs from `now`'s year, e.g. "Mon, Jul 6, 2025".
 */
export function formatDayLabel(iso: string, now: Date = new Date()): string {
  const date = new Date(iso)
  const dayDiff = Math.round((startOfDay(now).getTime() - startOfDay(date).getTime()) / 86_400_000)
  if (dayDiff === 0) return 'Today'
  if (dayDiff === 1) return 'Yesterday'

  const sameYear = date.getFullYear() === now.getFullYear()
  return new Intl.DateTimeFormat('en-US', {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    year: sameYear ? undefined : 'numeric',
  }).format(date)
}

/** Groups messages by sender identity + internal-note flag — the same key means the same visual cluster. */
function clusterKey(message: SupportMessage): string {
  const actorId = message.sender_user_id ?? message.sender_agent_id ?? ''
  return `${message.sender_type}:${actorId}:${message.is_internal ? 'note' : 'msg'}`
}

/** Best-effort display name derived only from the message itself (groupMessages takes no second argument). */
function senderNameFor(message: SupportMessage): string {
  if (message.sender_display_name) return message.sender_display_name
  switch (message.sender_type) {
    case 'customer':
      return 'Customer'
    case 'ai':
      return 'AI Assistant'
    default:
      return 'Agent'
  }
}

/**
 * Splits a chronological (oldest-first) message list into day separators and
 * sender clusters for thread rendering. Day boundaries always start a fresh
 * cluster (even if the same sender posted right before midnight and right
 * after); within a day, a cluster breaks on sender change, is_internal
 * change, or a gap of more than `CLUSTER_GAP_MS` since the previous message.
 */
export function groupMessages(messages: SupportMessage[]): ThreadItem[] {
  const items: ThreadItem[] = []
  let currentDayLabel: string | null = null
  let currentCluster: Extract<ThreadItem, { kind: 'cluster' }> | null = null
  let currentClusterKey: string | null = null
  let lastMessage: SupportMessage | null = null

  for (const message of messages) {
    const dayLabel = formatDayLabel(message.created_at)
    if (dayLabel !== currentDayLabel) {
      items.push({ kind: 'day', label: dayLabel })
      currentDayLabel = dayLabel
      currentCluster = null
      currentClusterKey = null
      lastMessage = null
    }

    // System events (assigned, resolved, mailbox moved, ...) are narration, not
    // a chat turn — they are never bucketed into a sender cluster (which would
    // wrongly show an agent/team name header above them, unlike the web thread).
    // They stand alone and break any in-progress cluster.
    if (message.message_type === 'system') {
      items.push({ kind: 'system', message })
      currentCluster = null
      currentClusterKey = null
      lastMessage = message
      continue
    }

    const key = clusterKey(message)
    const gapMs = lastMessage
      ? new Date(message.created_at).getTime() - new Date(lastMessage.created_at).getTime()
      : Infinity
    const breaksCluster = !currentCluster || currentClusterKey !== key || gapMs > CLUSTER_GAP_MS

    if (breaksCluster) {
      currentCluster = {
        kind: 'cluster',
        senderType: message.sender_type,
        senderName: senderNameFor(message),
        senderAvatarUrl: message.sender_avatar_url,
        messages: [message],
      }
      currentClusterKey = key
      items.push(currentCluster)
    } else {
      currentCluster!.messages.push(message)
    }

    lastMessage = message
  }

  return items
}

export interface TextSegment {
  text: string
  mention?: boolean
}

/**
 * Splits text into plain runs and @mention runs. Mirrors the web thread's
 * `renderMentionHighlights` (MessageBubble.tsx) so mentions in internal notes
 * highlight identically on mobile.
 */
export function splitMentionSegments(content: string): TextSegment[] {
  const regex = /@([a-zA-Z0-9][a-zA-Z0-9._-]*)/g
  const segments: TextSegment[] = []
  let lastIndex = 0
  let match: RegExpExecArray | null
  while ((match = regex.exec(content)) !== null) {
    if (match.index > lastIndex) segments.push({ text: content.slice(lastIndex, match.index) })
    segments.push({ text: match[0], mention: true })
    lastIndex = match.index + match[0].length
  }
  if (lastIndex < content.length) segments.push({ text: content.slice(lastIndex) })
  return segments
}

export type SupportReceiptStatus = 'delivered' | 'sent_email' | 'delivered_email' | 'read' | 'read_email'

/**
 * Derives the read-receipt state for the last outbound (our-side, public,
 * non-system) reply in a thread. Mirrors the web thread's `receiptMessageId` +
 * `receiptStatus` derivation (MessageThread.tsx) — kept in sync by hand; this
 * is a tiny, stable pure function not worth threading through the 1000-line web
 * component's aliasing.
 */
export function computeSupportReceipt(
  messages: SupportMessage[],
  conversation: { source?: string; contact_last_seen_at?: string } | null | undefined,
): { receiptMessageId: string | null; receiptStatus: SupportReceiptStatus | null } {
  let receiptMessageId: string | null = null
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const m = messages[i]
    if (!m.is_internal && m.sender_type !== 'customer' && (!m.message_type || m.message_type === 'reply')) {
      receiptMessageId = m.id
      break
    }
  }
  if (!receiptMessageId || !conversation) return { receiptMessageId: null, receiptStatus: null }
  const msg = messages.find((m) => m.id === receiptMessageId)
  if (!msg) return { receiptMessageId: null, receiptStatus: null }

  let receiptStatus: SupportReceiptStatus | null = null
  if (msg.email_read_at) receiptStatus = 'read_email'
  else if (msg.email_delivery_status === 'opened') receiptStatus = 'read_email'
  else if (msg.email_delivery_status === 'delivered') receiptStatus = 'delivered_email'
  else {
    const seen = conversation.contact_last_seen_at
    if (conversation.source === 'widget' && seen && new Date(seen) >= new Date(msg.created_at)) receiptStatus = 'read'
    else if (msg.email_notified_at) receiptStatus = 'sent_email'
    else if (conversation.source === 'widget') receiptStatus = 'delivered'
  }
  return { receiptMessageId, receiptStatus }
}

/**
 * Selectors matching known quoted-reply wrappers. Mirrors
 * server/internal/email/inboundhtml/convert.go's `quotedReplySelectors`
 * (which already annotates matches with `data-helpin-quote="true"` before
 * the HTML ever reaches the client) and the desktop web app's
 * `EmailBodyRendererLayout.ts` COLLAPSIBLE_SELECTOR, so a message rendered on
 * mobile collapses the same content a teammate on desktop would see
 * collapsed.
 */
const QUOTE_MARKER_SELECTOR = [
  '[data-helpin-quote="true"]',
  'div.gmail_quote',
  'div.gmail_extra',
  'div#appendonsend',
  'div#divRplyFwdMsg',
  'blockquote[type="cite"]',
  'div.yahoo_quoted',
  "div[id='yahoo_quoted']",
  'div.protonmail_quote',
  // Desktop's COLLAPSIBLE_SELECTOR also folds Gmail signature wrappers —
  // mirror it so mobile collapses the same content desktop does.
  '.gmail_signature',
  '.gmail_signature_prefix',
].join(', ')

/** Walks up from `el` to its top-level ancestor directly under `body`. */
function topLevelAncestor(el: Element, body: HTMLElement): Element {
  let node = el
  while (node.parentElement && node.parentElement !== body) {
    node = node.parentElement
  }
  return node
}

/**
 * Splits an email HTML body into a visible portion and a collapsed "quoted
 * history" portion (rendered behind a "···" expander). Detection order:
 *  1. Any element (at any nesting depth) matching a known quote-wrapper
 *     marker — its top-level ancestor and everything after it is quoted.
 *  2. Falling back to a trailing `<blockquote>` (no type/class needed) —
 *     some clients omit `type="cite"`. Only trailing runs count: a
 *     blockquote followed by real content is a citation, not history.
 * Returns `{ visible: html, quoted: null }` unchanged when nothing matches.
 */
export function splitQuotedHtml(html: string): { visible: string; quoted: string | null } {
  const trimmed = (html ?? '').trim()
  if (!trimmed) return { visible: html ?? '', quoted: null }

  const doc = new DOMParser().parseFromString(trimmed, 'text/html')
  const body = doc.body
  const children = Array.from(body.children)
  if (children.length === 0) return { visible: html, quoted: null }

  let splitIndex = -1
  const marker = body.querySelector(QUOTE_MARKER_SELECTOR)
  if (marker) {
    splitIndex = children.indexOf(topLevelAncestor(marker, body))
  }

  if (splitIndex === -1) {
    for (let i = children.length - 1; i >= 0; i -= 1) {
      const child = children[i]
      if (child.tagName === 'BLOCKQUOTE') {
        splitIndex = i
      } else if ((child.textContent ?? '').trim().length > 0) {
        break
      }
    }
  }

  if (splitIndex === -1) return { visible: html, quoted: null }

  const visible = children
    .slice(0, splitIndex)
    .map((child) => child.outerHTML)
    .join('')
  const quoted = children
    .slice(splitIndex)
    .map((child) => child.outerHTML)
    .join('')
  return { visible, quoted: quoted || null }
}
