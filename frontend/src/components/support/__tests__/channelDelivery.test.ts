import { describe, expect, it } from 'vitest'
import type { SupportMessage } from '@/lib/pmTypes'
import { providerChannelLabel } from '@/lib/supportChannels'
import { getMessageDelivery } from '../messageDelivery'

function reply(channel_delivery: SupportMessage['channel_delivery']): SupportMessage {
  return {
    id: 'm1', workspace_id: 'ws', conversation_id: 'c1', sender_type: 'user', content: 'Hi', message_type: 'reply',
    is_internal: false, created_at: '2026-11-13T00:00:00Z', updated_at: '2026-11-13T00:00:00Z', channel_delivery,
  } as SupportMessage
}

describe('Telegram reply delivery', () => {
  it('shows only the Telegram delivery, never chat or email', () => {
    const delivery = getMessageDelivery(reply({ status: 'sent', retryable: false }), { source: 'telegram' })
    expect(delivery).toEqual({ channels: [{ channel: 'telegram', state: 'sent', description: 'Sent to Telegram' }], state: 'sent' })
  })

  it('explains failures in plain words', () => {
    expect(getMessageDelivery(reply({ status: 'failed', error_code: 'blocked', retryable: true }))?.channels[0].description)
      .toBe('Not delivered to Telegram: the customer blocked the bot')
    expect(getMessageDelivery(reply({ status: 'unconfirmed', retryable: true }))?.channels[0])
      .toEqual({ channel: 'telegram', state: 'failed', description: 'Telegram did not confirm delivery' })
    expect(getMessageDelivery(reply({ status: 'cancelled', error_code: 'disconnected', retryable: false }))?.channels[0].description)
      .toBe('Not sent: the bot was disconnected')
    expect(getMessageDelivery(reply({ status: 'sending', retryable: false }))?.state).toBe('sending')
  })

  it('names the messaging app that carries chat replies', () => {
    expect(providerChannelLabel({ channel: 'telegram' })).toBe('Telegram')
    expect(providerChannelLabel({ channel: 'email', merge_has_channel: true })).toBe('Telegram')
    expect(providerChannelLabel({ channel: 'widget' })).toBeUndefined()
    expect(providerChannelLabel(null)).toBeUndefined()
  })
})
