import { describe, expect, it } from 'vitest'
import { requestLogLine } from './serverRequestLog.mjs'

describe('requestLogLine', () => {
  it('logs the page path and query keys without values', () => {
    expect(requestLogLine('GET', 'acme.helpin.center', '/articles/setup?token=secret&q=x&q=y'))
      .toBe('hc request GET acme.helpin.center /articles/setup query_keys=token,q')
  })

  it('skips health checks and static assets', () => {
    expect(requestLogLine('GET', 'h', '/healthz')).toBeNull()
    expect(requestLogLine('GET', 'h', '/assets/index-abc.js')).toBeNull()
  })
})
