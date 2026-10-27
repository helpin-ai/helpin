// @vitest-environment jsdom
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { CustomerPortalRequest } from '@/lib/services/customerPortalService'

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, ...props }: { children: React.ReactNode; 'aria-label'?: string; className?: string }) => (
    <a aria-label={props['aria-label']} className={props.className} href="/request">{children}</a>
  ),
}))

import { PortalRequestTable } from '../PortalRequestTable'
import { requestedBy } from '../portalRequestFormat'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root
let container: HTMLDivElement
beforeEach(() => {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
})

const now = new Date().toISOString()
const request = (overrides: Partial<CustomerPortalRequest>): CustomerPortalRequest => ({
  reference: 'req_1', number: 1, subject: 'Invoices', status: 'active', last_activity_at: now, ...overrides,
})

describe('requestedBy', () => {
  it('names the requester, or the viewer as You', () => {
    expect(requestedBy(request({ own: true, requested_by: 'Alice' }))).toBe('You')
    expect(requestedBy(request({ own: false, requested_by: 'Bob Roe' }))).toBe('Bob Roe')
    expect(requestedBy(request({ own: false }))).toBe('A colleague')
  })
})

describe('PortalRequestTable', () => {
  it('says who asked for each company request and searches by requester', async () => {
    const requests = [
      request({ reference: 'mine', subject: 'Mine', own: true, requested_by: 'Alice', last_message_from: 'customer' }),
      request({ reference: 'bobs', subject: 'Outage', own: false, requested_by: 'Bob Roe' }),
    ]
    await act(async () => root.render(<PortalRequestTable requests={requests} slug="acme" supportName="Acme Support" onRefresh={() => {}} companyName="Acme" />))
    expect(container.textContent).toContain('Asked by You')
    expect(container.textContent).toContain('Asked by Bob Roe')
    // A colleague may have written the latest customer reply.
    expect(container.textContent).toContain('Your team replied')

    const search = container.querySelector<HTMLInputElement>('input[aria-label="Search requests"]')!
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set
    await act(async () => {
      setter?.call(search, 'bob')
      search.dispatchEvent(new Event('input', { bubbles: true }))
    })
    expect(container.textContent).toContain('Outage')
    expect(container.textContent).not.toContain('Mine')
  })

  it('keeps personal lists unchanged', async () => {
    await act(async () => root.render(<PortalRequestTable requests={[request({ last_message_from: 'customer' })]} slug="acme" supportName="Acme Support" onRefresh={() => {}} />))
    expect(container.textContent).toContain('You replied')
    expect(container.textContent).not.toContain('Asked by')
  })

  it('explains an empty company list', async () => {
    await act(async () => root.render(<PortalRequestTable requests={[]} slug="acme" supportName="Acme Support" onRefresh={() => {}} companyName="Acme" />))
    expect(container.textContent).toContain('No company requests yet')
    expect(container.textContent).toContain('anyone at Acme')
  })
})
