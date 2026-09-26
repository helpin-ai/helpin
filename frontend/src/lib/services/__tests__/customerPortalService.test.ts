import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  CustomerPortalApiError,
  customerPortalService,
} from '../customerPortalService'

describe('customerPortalService', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('loads public configuration with an encoded slug and cookie credentials', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          enabled: true,
          requests_only: true,
          intake_enabled: true,
          branding: { name: 'Acme Support' },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    await customerPortalService.configuration('acme & co')

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/public/portal/acme%20%26%20co'),
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('posts only the email when requesting a magic link', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)

    await customerPortalService.requestMagicLink('acme', 'customer@example.com')

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/public/portal/acme/auth/magic-link'),
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'customer@example.com' }),
      }),
    )
  })

  it('creates a request with only customer-authored content', async () => {
    const createdRequest = {
      id: 'request-1',
      subject: 'Cannot sign in',
      status: 'open',
      updated_at: '2026-09-12T10:00:00Z',
    }
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(createdRequest), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await customerPortalService.createRequest('acme', {
      subject: 'Cannot sign in',
      message: 'The form sends me back to the start.',
    })

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/public/portal/acme/requests'),
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          subject: 'Cannot sign in',
          message: 'The form sends me back to the start.',
        }),
      }),
    )
  })

  it('exposes unauthorized responses without touching employee authentication', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: 'Session expired' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    )

    await expect(customerPortalService.requests('acme')).rejects.toEqual(
      expect.objectContaining<CustomerPortalApiError>({
        name: 'CustomerPortalApiError',
        status: 401,
        message: 'Session expired',
      }),
    )
  })
})
