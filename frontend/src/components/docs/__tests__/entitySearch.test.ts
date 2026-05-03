import { describe, expect, it } from 'vitest'

import { entityMentionHref, parseEntityMentionQuery } from '../entitySearch'

describe('entity mention search helpers', () => {
  it('parses typed entity mention prefixes', () => {
    expect(parseEntityMentionQuery('task producer alert')).toEqual({
      entityType: 'task',
      query: 'producer alert',
      typed: true,
    })
    expect(parseEntityMentionQuery('contact ada')).toEqual({
      entityType: 'contact',
      query: 'ada',
      typed: true,
    })
    expect(parseEntityMentionQuery('conversation refund')).toEqual({
      entityType: 'support_conversation',
      query: 'refund',
      typed: true,
    })
    expect(parseEntityMentionQuery('doc roadmap')).toEqual({
      entityType: 'document',
      query: 'roadmap',
      typed: true,
    })
  })

  it('keeps untyped mention queries broad', () => {
    expect(parseEntityMentionQuery('ada')).toEqual({
      query: 'ada',
      typed: false,
    })
  })

  it('builds typed workspace hrefs', () => {
    expect(entityMentionHref('acme', { entityType: 'task', entityId: 'task-1' })).toBe('/w/acme/pm/tasks/task-1')
    expect(entityMentionHref('acme', { entityType: 'document', entityId: 'doc-1' })).toBe('/w/acme/docs/documents/doc-1')
    expect(entityMentionHref('acme', { entityType: 'support_conversation', entityId: 'conv-1' })).toBe('/w/acme/support/conv-1')
  })
})
