// @vitest-environment jsdom
import { act, useEffect } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import type { CustomerPortalCompany } from '@/lib/services/customerPortalService'
import { MINE_SCOPE, usePortalRequestScope } from '../portalRequestScope'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root
let container: HTMLDivElement
const probe: { current?: ReturnType<typeof usePortalRequestScope> } = {}
beforeEach(() => {
  window.sessionStorage.clear()
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
})

function Probe({ companies }: { companies: CustomerPortalCompany[] }) {
  const scope = usePortalRequestScope('acme', companies)
  useEffect(() => { probe.current = scope })
  return null
}

const acme: CustomerPortalCompany = { id: 'co-acme', name: 'Acme', role: 'member' }
const globex: CustomerPortalCompany = { id: 'co-globex', name: 'Globex', role: 'requester' }

describe('usePortalRequestScope', () => {
  it('starts on the customer\'s own requests and offers only member companies', async () => {
    await act(async () => root.render(<Probe companies={[acme, globex]} />))
    expect(probe.current!.key).toBe(MINE_SCOPE)
    expect(probe.current!.memberCompanies.map((company) => company.id)).toEqual(['co-acme'])
  })

  it('remembers the chosen company for the browser tab', async () => {
    await act(async () => root.render(<Probe companies={[acme]} />))
    await act(async () => probe.current!.select('co-acme'))
    expect(probe.current!.key).toBe('co-acme')
    expect(probe.current!.company?.name).toBe('Acme')
    act(() => root.unmount())
    root = createRoot(container)
    await act(async () => root.render(<Probe companies={[acme]} />))
    expect(probe.current!.key).toBe('co-acme')
  })

  it('falls back to own requests when membership ends', async () => {
    window.sessionStorage.setItem('helpin.portal.acme.requestScope', 'co-acme')
    await act(async () => root.render(<Probe companies={[{ ...acme, role: 'requester' }]} />))
    expect(probe.current!.key).toBe(MINE_SCOPE)
    expect(probe.current!.company).toBeUndefined()
  })
})
