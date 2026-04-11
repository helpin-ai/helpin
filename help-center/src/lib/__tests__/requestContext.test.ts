// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'

const {
  getRequestHost,
  getRequestProtocol,
  getRequestUrl,
} = vi.hoisted(() => ({
  getRequestHost: vi.fn(),
  getRequestProtocol: vi.fn(),
  getRequestUrl: vi.fn(),
}))

vi.mock('@tanstack/react-start', () => ({
  createServerFn: () => ({
    handler: <T>(fn: T) => fn,
  }),
}))

vi.mock('@tanstack/react-start/server', () => ({
  getRequestHost,
  getRequestProtocol,
  getRequestUrl,
}))

import { getHelpCenterRequestContext } from '@/lib/requestContext'

describe('getHelpCenterRequestContext', () => {
  beforeEach(() => {
    getRequestHost.mockReset()
    getRequestProtocol.mockReset()
    getRequestUrl.mockReset()
    delete (globalThis as { __hcGetRequestContext__?: unknown }).__hcGetRequestContext__
  })

  it('uses the subdomain query override during server-side preview fallback resolution', async () => {
    getRequestHost.mockReturnValue('localhost:5174')
    getRequestProtocol.mockReturnValue('http')
    getRequestUrl.mockReturnValue(
      new URL('http://localhost:5174/preview/doc-1?subdomain=replug&token=preview-token'),
    )

    await expect(getHelpCenterRequestContext()).resolves.toEqual({
      host: 'localhost:5174',
      protocol: 'http',
      subdomain: 'replug',
      basepath: '',
    })
  })

  it('reuses the pre-resolved SSR snapshot when available', async () => {
    ;(globalThis as { __hcGetRequestContext__?: () => unknown }).__hcGetRequestContext__ = () => ({
      host: 'localhost:5174',
      protocol: 'http',
      pathname: '/preview/doc-1',
      search: '?subdomain=replug&token=preview-token',
      subdomain: 'replug',
      basepath: '',
    })

    await expect(getHelpCenterRequestContext()).resolves.toEqual({
      host: 'localhost:5174',
      protocol: 'http',
      subdomain: 'replug',
      basepath: '',
    })
  })
})
