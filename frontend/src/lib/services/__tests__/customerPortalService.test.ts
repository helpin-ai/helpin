import { afterEach, describe, expect, it, vi } from 'vitest'

const upload = vi.hoisted(() => ({ toS3: vi.fn() }))
vi.mock('@/lib/api', () => ({ API_BASE: '/api', uploadToS3: upload.toS3 }))

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
      reference: 'req_abc',
      subject: 'Cannot sign in',
      status: 'active',
      created_at: '2026-09-12T10:00:00Z',
      updated_at: '2026-09-12T10:00:00Z',
      last_activity_at: '2026-09-12T10:00:00Z',
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

  it('uploads with progress and surfaces the server reason for a refused file', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ attachment: { id: 'att-1' }, upload_url: 'https://storage.example/put' }), { status: 201, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)
    upload.toS3.mockImplementation(async (_url: string, _file: File, onProgress?: (percent: number) => void) => {
      onProgress?.(50)
      return { ok: true, error: null }
    })
    const onProgress = vi.fn()
    const file = new File(['x'], 'shot.png', { type: 'image/png' })
    await expect(customerPortalService.uploadAttachment('acme', file, 'req_1', { onProgress })).resolves.toEqual({ id: 'att-1', name: 'shot.png' })
    expect(fetchMock.mock.calls[0][0]).toContain('/public/portal/acme/requests/req_1/attachments')
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ file_name: 'shot.png', file_size: 1, content_type: 'image/png' })
    expect(onProgress).toHaveBeenCalledWith(50)
    expect(fetchMock.mock.calls[1][0]).toContain('/requests/req_1/attachments/att-1/confirm')

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: 'tool.exe files are not supported' }), { status: 400, headers: { 'Content-Type': 'application/json' } })))
    await expect(customerPortalService.uploadAttachment('acme', new File(['x'], 'tool.exe', { type: 'application/x-msdownload' }))).rejects.toThrow('tool.exe files are not supported')
  })
})
