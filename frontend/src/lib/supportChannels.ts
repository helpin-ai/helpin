import { GlobeIcon, Mail01Icon, Message01Icon } from '@/lib/icons'
import type { TicketSource } from '@/lib/pm-types/support'

/**
 * Support channel registry. Mirrors server/internal/model/support_channels.go;
 * every surface that enumerates channels reads it, so adding a channel is one
 * entry here plus the TicketSource union.
 */
export type SupportChannelTransport = 'chat' | 'portal' | 'email' | 'none'

export interface SupportChannelSpec {
  key: TicketSource
  /** Reports, SLA policies, and other settings. */
  label: string
  /** Conversation list icon and tooltip; omitted channels show no icon. */
  inbox?: { icon: typeof Message01Icon; label: string }
  /** Message source badge; omitted channels show the raw value. */
  messageLabel?: string
  transport: SupportChannelTransport
  /** AI reply setting that governs the channel. */
  aiMode: 'chat' | 'email' | 'portal' | null
  /** SLA policies may target the channel. */
  slaChannel: boolean
}

const SUPPORT_CHANNEL_SPECS: Record<TicketSource, SupportChannelSpec> = {
  widget: { key: 'widget', label: 'Chat', inbox: { icon: Message01Icon, label: 'Live chat' }, messageLabel: 'Chat', transport: 'chat', aiMode: 'chat', slaChannel: true },
  email: { key: 'email', label: 'Email', inbox: { icon: Mail01Icon, label: 'Email' }, messageLabel: 'Email', transport: 'email', aiMode: 'email', slaChannel: true },
  portal: { key: 'portal', label: 'Portal', inbox: { icon: GlobeIcon, label: 'Customer portal' }, transport: 'portal', aiMode: 'portal', slaChannel: true },
  api: { key: 'api', label: 'API', messageLabel: 'API', transport: 'none', aiMode: null, slaChannel: true },
  internal: { key: 'internal', label: 'Started by team', messageLabel: 'Internal', transport: 'none', aiMode: null, slaChannel: true },
}

/** Registered channels in display order. */
export const SUPPORT_CHANNELS: readonly SupportChannelSpec[] = Object.values(SUPPORT_CHANNEL_SPECS)

/** Exact-key lookup; imported provenance such as "intercom" is not a channel. */
export function supportChannel(key: string | null | undefined): SupportChannelSpec | undefined {
  return key && Object.hasOwn(SUPPORT_CHANNEL_SPECS, key) ? SUPPORT_CHANNEL_SPECS[key as TicketSource] : undefined
}

export function supportChannelLabel(key: string): string {
  return supportChannel(key)?.label ?? key.charAt(0).toUpperCase() + key.slice(1)
}

/** A conversation source with a live chat session (the widget). */
export function hasChatTransport(source: string | null | undefined): boolean {
  return supportChannel(source)?.transport === 'chat'
}
