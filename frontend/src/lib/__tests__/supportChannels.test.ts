import { describe, expect, it } from 'vitest'
import { SUPPORT_CHANNELS, hasChatTransport, supportChannel, supportChannelLabel } from '../supportChannels'

describe('support channel registry', () => {
  it('lists channels in display order with unique keys', () => {
    expect(SUPPORT_CHANNELS.map((channel) => channel.key)).toEqual(['widget', 'email', 'portal', 'api', 'internal'])
  })

  it('matches keys exactly and ignores provenance values', () => {
    expect(supportChannel('widget')?.label).toBe('Chat')
    for (const key of ['Widget', ' email', 'intercom', 'constructor', '', null, undefined]) {
      expect(supportChannel(key)).toBeUndefined()
    }
  })

  it('keeps existing labels per surface', () => {
    expect(SUPPORT_CHANNELS.map((channel) => channel.label)).toEqual(['Chat', 'Email', 'Portal', 'API', 'Started by team'])
    expect(SUPPORT_CHANNELS.map((channel) => channel.inbox?.label ?? null)).toEqual(['Live chat', 'Email', 'Customer portal', null, null])
    expect(SUPPORT_CHANNELS.map((channel) => channel.messageLabel ?? null)).toEqual(['Chat', 'Email', null, 'API', 'Internal'])
    expect(supportChannelLabel('intercom')).toBe('Intercom')
  })

  it('treats only the widget as a chat session', () => {
    expect(SUPPORT_CHANNELS.filter((channel) => hasChatTransport(channel.key)).map((channel) => channel.key)).toEqual(['widget'])
    expect(hasChatTransport('intercom')).toBe(false)
  })
})
