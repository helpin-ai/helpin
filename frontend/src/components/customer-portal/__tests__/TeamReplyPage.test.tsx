// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TeamReplyPage } from '../TeamReplyPage'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const openPage = {
  state: 'open',
  branding: { name: 'Northwind' },
  subject: 'CSV export is missing the Notes column',
  customer_email: 'sarah@example.com',
  reply_time_text: 'We usually reply in a few hours.',
}

function respond(status: number, body: unknown) {
  return Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }))
}

function setValue(element: HTMLTextAreaElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')?.set
  setter?.call(element, value)
  element.dispatchEvent(new Event('input', { bubbles: true }))
}

async function settle() {
  for (let i = 0; i < 10; i++) {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
  }
}

describe('TeamReplyPage', () => {
  let container: HTMLDivElement
  let root: Root
  const fetchMock = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })
  afterEach(() => {
    act(() => root.unmount())
    container.remove()
    fetchMock.mockReset()
    vi.unstubAllGlobals()
  })

  function render() {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    act(() => root.render(<QueryClientProvider client={client}><TeamReplyPage token="signed.token" /></QueryClientProvider>))
  }

  it('only reads the conversation when the link opens', async () => {
    fetchMock.mockImplementation(() => respond(200, openPage))
    render()
    await settle()
    expect(container.textContent).toContain('Want a person to reply?')
    expect(container.textContent).toContain('Reply goes to sarah@example.com')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0][1]?.method).toBeUndefined()
  })

  it('hands off with the note when the customer presses the button', async () => {
    fetchMock.mockImplementationOnce(() => respond(200, openPage))
    fetchMock.mockImplementationOnce(() => respond(200, { ...openPage, state: 'done' }))
    render()
    await settle()
    act(() => setValue(container.querySelector('textarea')!, 'The column is still empty.'))
    await act(async () => {
      container.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })
    await settle()
    const [url, init] = fetchMock.mock.calls[1]
    expect(String(url)).toContain('/public/support/team-reply/signed.token')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body)).toEqual({ note: 'The column is still empty.' })
    expect(container.textContent).toContain('Our team has your conversation')
    expect(container.textContent).toContain('We usually reply in a few hours. You can close this page.')
  })

  it('explains a link that was not issued by Helpin', async () => {
    fetchMock.mockImplementation(() => respond(404, { error: 'This link is not valid.' }))
    render()
    await settle()
    expect(container.textContent).toContain('This link is not valid')
    expect(container.querySelector('form')).toBeNull()
  })
})
